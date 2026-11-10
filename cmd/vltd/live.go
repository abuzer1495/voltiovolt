// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/event"
	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/measure"
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
	d      *daemon

	mu       sync.Mutex
	commLost bool
	charge   *float64
	runtime  *float64
	rated    *dialect.Rating // last F response (megatec charge estimate)
	last     map[string]measure.Point

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
	return s
}

// run starts the goroutines: Device Manager, hourly Measurement job, the
// Policy tick every second with the countdown push, and the status push.
func (l *live) run(ctx context.Context) {
	ctx, l.cancel = context.WithCancel(ctx)
	unsub := l.events.Subscribe(func(e event.Event) { _ = l.hub.Publish(wire.PushEvent, e) })
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
			if cd := l.pol.Countdown(); cd.Active {
				_ = l.hub.Publish(wire.PushCountdown, cd)
			}
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

// stateSink passes the Poller state to the Device Manager and to live.
type stateSink struct {
	l        *live
	setState func(string)
}

func (s stateSink) OnCycle(c poller.Cycle) { s.l.OnCycle(c) }

func (s stateSink) OnState(state string) {
	s.setState(state)
	s.l.OnState(state)
}

// startLive makes the UPS part. Without cfg.Lister there is no Device
// Manager (the IPC tests). Without cfg.MakeExecutor the Policy Engine gets
// a dry-run executor; main always gives the OS executor.
func (d *daemon) startLive(ctx context.Context) error {
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
	var exec policy.Executor = &policy.DryRun{}
	if d.cfg.MakeExecutor != nil {
		exec = d.cfg.MakeExecutor(forceClose, d.au)
	}
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
		hub: ipc.NewHub(), last: map[string]measure.Point{}}
	if d.cfg.Lister != nil {
		l.mgr = &device.Manager{
			Config: func(ctx context.Context) (device.Config, error) { return device.ConfigFromStore(ctx, d.st) },
			List:   d.cfg.Lister,
			Log:    d.log,
			Poll: func(ctx context.Context, tr transport.Transport, p device.Profile, setState func(string)) error {
				return poller.New(tr, p, stateSink{l: l, setState: setState}, poller.Options{Log: d.log}).Run(ctx)
			},
		}
	}
	d.live = l
	l.run(context.Background())
	return nil
}
