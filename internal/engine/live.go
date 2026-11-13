// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/demo"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/event"
	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/measure"
	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/notify/email"
	"github.com/abuzer1495/voltiovolt/internal/notify/telegram"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/poller"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// Push periods (VLT-IS-004 TASK 03-00-00-008 step 2).
const (
	statusPushPeriod = 2 * time.Second
	tickPeriod       = time.Second
)

// live is the UPS part of vltd: Device Manager, Poller, Measurements,
// Event Engine, Policy Service and the push messages.
type live struct {
	mgr    *device.Manager
	writer measure.Writer
	events *event.Engine
	pol    *policy.Service
	hub    *ipc.Hub
	d      *Daemon
	// ctl sends the CONTROL commands and verifies them with the fast-group
	// statuses (VLT-IS-006 TASK 03-00-00-005).
	ctl *control.Controller
	// router is the Notification Router (VLT-IS-007 TASK 03-00-00-003).
	router *notify.Router
	// telegram is the Telegram Channel (VLT-IS-007 TASK 03-00-00-005).
	telegram *telegram.Bot
	email    *email.Mailer
	demo     *demo.Session

	mu       sync.Mutex
	commLost bool
	charge   *float64
	runtime  *float64
	rated    *dialect.Rating // last F response (megatec charge estimate)
	last     map[string]measure.Point
	lastEv   *notify.Item // the last Event (Telegram /status)

	tickMu sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// StatusPush is the data of the "status" push message.
type StatusPush struct {
	Time         time.Time                `json:"ts"`
	Device       device.Status            `json:"device"`
	UPS          event.State              `json:"ups"`
	Measurements map[string]measure.Point `json:"measurements"`
	Countdown    policy.Countdown         `json:"countdown"`
	PolicyActive bool                     `json:"policy_active"`
	Demo         bool                     `json:"demo,omitempty"`
	DemoMessage  string                   `json:"demo_message,omitempty"`
}

// OnCycle is the Poller sink: Measurements, Events, then the Policy tick.
func (l *live) OnCycle(c poller.Cycle) {
	ctx := context.Background()
	obs := event.Observation{Time: c.End}
	var samples []measure.Sample
	var megatec bool
	var nominal *dialect.Rating
	if l.mgr != nil {
		if p := l.mgr.Status().Profile; p != nil {
			megatec = p.Dialect == dialect.DialectMegatec
			nominal = p.Nominal
		}
	}
	est := measure.EstimateConfigFromStore(ctx, l.d.st)
	l.mu.Lock()
	for _, r := range c.Results {
		if r.Err != nil || r.Value == nil {
			continue
		}
		if f, ok := r.Value.(dialect.Rating); ok && r.Command == "F" {
			l.rated = &f
		}
		pts := measure.FromValue(r.Command, r.Value)
		// VLT-IS-006 TASK 03-00-00-003 step 8: battery.charge_estimate for
		// the megatec Dialect only.
		if st, ok := r.Value.(dialect.Status); ok && megatec && r.Command == "Q1" {
			rated := l.rated
			if rated == nil {
				rated = nominal
			}
			if v, ok := measure.ChargeEstimate(st, rated, est); ok {
				pts = append(pts, measure.Point{Name: measure.BatteryChargeEstimate, Unit: measure.Units[measure.BatteryChargeEstimate], Value: v, Source: "Q1"})
			}
		}
		if len(pts) > 0 {
			samples = append(samples, measure.Sample{Time: r.Time, Points: pts})
		}
		for _, p := range pts {
			l.last[p.Name] = p
			v := p.Value
			switch p.Name {
			case measure.BatteryCharge:
				l.charge = &v
			case measure.BatteryRuntime:
				l.runtime = &v
			}
		}
		obs.Add(r.Command, r.Value)
	}
	l.mu.Unlock()
	if l.demo != nil && l.demo.Active() {
		for i := range samples {
			for j := range samples[i].Points {
				samples[i].Points[j].Demo = true
			}
		}
	}
	if len(samples) > 0 {
		if err := l.writer.WriteCycle(ctx, samples); err != nil {
			l.d.log.Error("measurements not stored", "err", err)
		}
	}
	if th, err := event.ThresholdsFromStore(ctx, l.d.st); err == nil {
		l.events.SetThresholds(th)
	}
	l.events.Observe(ctx, obs)
	l.tick(ctx)
}

// OnState is the Poller sink for the connection state.
func (l *live) OnState(state string) {
	lost := state == device.StateCommLost
	l.mu.Lock()
	changed := lost != l.commLost
	l.commLost = lost
	l.mu.Unlock()
	if changed {
		l.events.CommState(context.Background(), time.Now(), lost)
		l.tick(context.Background())
	}
}

func (l *live) input() policy.Input {
	st := l.events.State()
	l.mu.Lock()
	defer l.mu.Unlock()
	return policy.Input{OnBattery: st.OnBattery, LowBattery: st.LowBattery, CommLost: st.CommLost,
		Charge: l.charge, Runtime: l.runtime}
}

// tick runs the Policy Engine. Only the Policy Engine calls the executor
// (SEC-092).
func (l *live) tick(ctx context.Context) {
	l.tickMu.Lock()
	defer l.tickMu.Unlock()
	l.pol.Tick(ctx, l.input())
}

func (l *live) status() StatusPush {
	l.mu.Lock()
	m := make(map[string]measure.Point, len(l.last))
	for k, v := range l.last {
		m[k] = v
	}
	l.mu.Unlock()
	s := StatusPush{Time: time.Now().UTC(), UPS: l.events.State(), Measurements: m, Countdown: l.pol.Countdown()}
	if l.mgr != nil {
		s.Device = l.mgr.Status()
	} else {
		s.Device = device.Status{State: device.StateStopped}
	}
	if in, err := l.pol.Info(context.Background()); err == nil {
		s.PolicyActive = in.Active
	}
	if l.demo != nil {
		if l.demo.Active() {
			s.Demo = true
		}
		s.DemoMessage = l.demo.ShutdownMessage()
	}
	if l.mgr != nil && l.mgr.Status().Config.Transport == device.TransportDemo {
		s.Demo = true
	}
	return s
}

// run starts the goroutines: Device Manager, hourly Measurement job, the
// Policy tick every second with the countdown push, and the status push.
func (l *live) run(ctx context.Context) {
	ctx, l.cancel = context.WithCancel(ctx)
	unsub := l.events.Subscribe(func(e event.Event) {
		_ = l.hub.Publish(wire.PushEvent, e)
		active, ok := e.Data["active"].(bool)
		if !ok {
			active = true
		}
		l.mu.Lock()
		l.lastEv = &notify.Item{Time: e.Time, Type: e.Type, Severity: e.Severity, Active: active}
		l.mu.Unlock()
		l.router.Event(e.Time, e.Type, e.Severity, active)
	})
	l.wg.Add(1)
	go func() { defer l.wg.Done(); l.router.Run(ctx) }()
	l.wg.Add(1)
	go func() { defer l.wg.Done(); l.telegram.Run(ctx) }()
	if l.mgr != nil {
		l.wg.Add(1)
		go func() { defer l.wg.Done(); _ = l.mgr.Run(ctx) }()
	}
	l.wg.Add(1)
	go func() { defer l.wg.Done(); measure.RunJob(ctx, l.d.st.DB(), device.RealClock{}, l.d.log) }()
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		defer unsub()
		tk := time.NewTicker(tickPeriod)
		defer tk.Stop()
		n := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
			}
			n++
			l.tick(ctx)
			cd := l.pol.Countdown()
			if cd.Active {
				_ = l.hub.Publish(wire.PushCountdown, cd)
			}
			l.router.Countdown(time.Now(), cd.Active, cd.Requested, cd.Remaining, cd.Action)
			if n%int(statusPushPeriod/tickPeriod) == 0 && l.hub.Subscribers() > 0 {
				_ = l.hub.Publish(wire.PushStatus, l.status())
			}
		}
	}()
}

func (l *live) stop() {
	if l.cancel != nil {
		l.cancel()
	}
	l.wg.Wait()
}

// snapshot is the UPS state for the Notifications: mode, input voltage,
// load, battery charge and runtime (VLT-IS-007 TASK 03-00-00-003 step 6).
func (l *live) snapshot() notify.Snapshot {
	st := l.events.State()
	l.mu.Lock()
	defer l.mu.Unlock()
	val := func(name string) *float64 {
		if p, ok := l.last[name]; ok {
			v := p.Value
			return &v
		}
		return nil
	}
	s := notify.Snapshot{InputVoltage: val(measure.InputVoltage), Load: val(measure.LoadPercent), Charge: val(measure.BatteryCharge), Runtime: val(measure.BatteryRuntime)}
	if s.Charge == nil {
		s.Charge = val(measure.BatteryChargeEstimate)
	}
	switch {
	case st.CommLost:
		s.Mode = notify.ModeCommLost
	case st.OnBattery:
		s.Mode = notify.ModeBattery
	case len(l.last) > 0:
		s.Mode = notify.ModeMains
	}
	return s
}

// telegramStatus is the data of the Telegram commands /status and /durum
// (VLT-IS-007 TASK 03-00-00-005 step 5).
func (l *live) telegramStatus(ctx context.Context) notify.Status {
	st := notify.Status{Snapshot: l.snapshot()}
	l.mu.Lock()
	if p, ok := l.last[measure.OutputVoltage]; ok {
		v := p.Value
		st.OutputVoltage = &v
	}
	if l.lastEv != nil {
		e := *l.lastEv
		st.LastEvent = &e
	}
	l.mu.Unlock()
	if in, err := l.pol.Info(ctx); err == nil {
		st.PolicyActive = in.Active
		st.PolicyAction = in.Config.Action
	}
	return st
}

// stateSink passes the Poller state to the Device Manager and to live.
type stateSink struct {
	l        *live
	setState func(string)
}

func (s stateSink) OnCycle(c poller.Cycle) {
	if c.Group == poller.GroupFast && s.l.ctl != nil {
		for _, r := range c.Results {
			if st, ok := control.StatusOf(r.Value, r.Time); ok && r.Err == nil {
				s.l.ctl.Observe(st)
			}
		}
	}
	s.l.OnCycle(c)
}

func (s stateSink) OnState(state string) {
	s.setState(state)
	s.l.OnState(state)
}

// startLive makes the UPS part. Without cfg.Lister there is no Device
// Manager (the IPC tests). Without cfg.MakeExecutor the Policy Engine gets
// a dry-run executor; main always gives the OS executor.
func (d *Daemon) startLive(ctx context.Context) error {
	th, err := event.ThresholdsFromStore(ctx, d.st)
	if err != nil {
		d.log.Error("event thresholds not read; use the defaults", "err", err)
		th = event.DefaultThresholds()
	}
	var pol *policy.Service
	// force_close_applications is read at the time of the call.
	forceClose := func() bool {
		in, err := pol.Info(context.Background())
		if err != nil {
			return true
		}
		return in.Config.ForceCloseApplications
	}
	sess := demo.New()
	var exec policy.Executor = &policy.DryRun{}
	if d.cfg.MakeExecutor != nil {
		exec = d.cfg.MakeExecutor(forceClose, d.au)
	}
	exec = &policy.Guarded{Real: exec, Dry: sess.Active, OnDry: func(string) { sess.SetShutdownMessage(demo.ShutdownMessageEN) }}
	pol, err = policy.NewService(ctx, d.st, d.au, exec, nil, d.log)
	if err != nil {
		return fmt.Errorf("policy: %w", err)
	}
	// The Fallback Policy in the Vault state "locked" (VLT-IS-005 Q-IA-1).
	pol.SetVaultState(func() string { return string(d.vt.Status().State) })
	if err := pol.Refresh(ctx); err != nil {
		return fmt.Errorf("policy: %w", err)
	}
	l := &live{d: d, writer: measure.Writer{DB: d.st.DB()}, events: event.New(d.st.DB(), th, d.log), pol: pol,
		hub: ipc.NewHub(), last: map[string]measure.Point{}, ctl: &control.Controller{Audit: d.au, Log: d.log}, demo: sess}
	l.events.SetDemo(sess.Active)
	l.router = notify.New(notify.Config{Settings: notify.StoreSettings(d.st), Language: notify.StoreLanguage(d.st), Snapshot: l.snapshot, Log: d.log, Demo: sess.Active},
		desktopChannel{l.hub})
	settings := notify.StoreSettings(d.st)
	l.telegram = telegram.New(telegram.Config{API: d.cfg.TelegramAPI, Secrets: d.vt, Chats: d.st, Audit: d.au, Log: d.log,
		Enabled:  func(ctx context.Context) bool { return settings(ctx, notify.ChannelTelegram).Enabled },
		Status:   l.telegramStatus,
		Language: notify.StoreLanguage(d.st)})
	l.router.AddChannel(l.telegram)
	// The e-mail Channel (VLT-IS-007 TASK 03-00-00-006).
	l.email = email.New(email.Config{Settings: email.StoreSettings(d.st), Secrets: d.vt, Log: d.log})
	l.router.AddChannel(l.email)
	// The UPS output off step of the Shutdown Policy (VLT-IS-006 TASK
	// 03-00-00-006) uses the Controller with the Policy Grant.
	pol.SetUPSOff(l.ctl)
	if d.cfg.Lister != nil {
		l.mgr = &device.Manager{
			Config: func(ctx context.Context) (device.Config, error) { return device.ConfigFromStore(ctx, d.st) },
			List:   device.WithDemo(d.cfg.Lister, sess.Candidates),
			Log:    d.log,
			Poll: func(ctx context.Context, tr transport.Transport, p device.Profile, setState func(string)) error {
				demoOn := p.Transport == device.TransportDemo
				sess.SetActive(demoOn)
				l.ctl.SetDryRun(demoOn)
				defer func() {
					sess.SetActive(false)
					l.ctl.SetDryRun(false)
				}()
				pl := poller.New(tr, p, stateSink{l: l, setState: setState}, poller.Options{Log: d.log})
				l.ctl.SetLink(pl, p.Dialect)
				defer l.ctl.SetLink(nil, "")
				return pl.Run(ctx)
			},
		}
	}
	d.live = l
	l.run(context.Background())
	return nil
}
