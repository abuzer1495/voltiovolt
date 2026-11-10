// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Step kinds of the Engine and the Simulation.
const (
	StepEvent           = "event"
	StepTrigger         = "trigger"
	StepCountdownStart  = "countdown_start"
	StepCancel          = "cancel"
	StepCancelRefused   = "cancel_refused"
	StepShutdownRequest = "shutdown_request"
	StepShutdownFailed  = "shutdown_failed"
)

// Cancel reasons.
const (
	ReasonMains    = "mains returned"
	ReasonOperator = "operator"
	ReasonInactive = "policy inactive"
)

// RetryAfter is the time between two executor calls after a failed call.
const RetryAfter = 30 * time.Second

// Errors of Cancel.
var (
	ErrNoCountdown = errors.New("policy: no countdown")
	ErrLowBattery  = errors.New("policy: LOW_BATTERY is active; the countdown cannot be cancelled")
)

// Input is the UPS state for one Tick. It comes from the Event Engine
// state and the last Measurements.
type Input struct {
	OnBattery  bool     // ON_BATTERY active (the last known state during comm lost)
	LowBattery bool     // LOW_BATTERY active
	CommLost   bool     // COMM_LOST active
	Charge     *float64 // battery.charge, percent
	Runtime    *float64 // battery.runtime, minutes
}

// Step is one result step.
type Step struct {
	Time   time.Time      `json:"-"`
	T      float64        `json:"t"` // seconds from the start (Simulation)
	Kind   string         `json:"kind"`
	Detail string         `json:"detail"`
	Data   map[string]any `json:"data,omitempty"`
}

// Auditor writes audit entries (internal/audit.Log).
type Auditor interface {
	Record(ctx context.Context, actor, action, target, result string, details map[string]any) error
}

// Countdown is the countdown state for shutdown.status and the push
// "countdown".
type Countdown struct {
	Active    bool      `json:"active"`
	Trigger   string    `json:"trigger,omitempty"`
	Started   time.Time `json:"started,omitempty"`
	Ends      time.Time `json:"ends,omitempty"`
	Remaining int       `json:"remaining_seconds"`
	Action    string    `json:"action,omitempty"`
	// Cancelable is false while LOW_BATTERY is active.
	Cancelable bool `json:"cancelable"`
	Requested  bool `json:"requested"`
}

// Engine is the Policy Engine state machine. It is deterministic: the
// time comes with each call.
type Engine struct {
	exec  Executor
	audit Auditor
	log   *slog.Logger

	mu         sync.Mutex
	cfg        Config
	active     bool
	onBattery  time.Time // start of the on-battery period
	commLost   time.Time // start of the comm lost period
	cd         *Countdown
	suppressed map[string]bool // triggers that an operator cancelled
	requested  bool
	lastCall   time.Time
	lastErr    error
	last       Input
}

// NewEngine returns an inactive Engine.
func NewEngine(cfg Config, exec Executor, audit Auditor, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	return &Engine{cfg: cfg, exec: exec, audit: audit, log: log, suppressed: map[string]bool{}}
}

// SetConfig sets the policy. The caller sets the active state again.
func (e *Engine) SetConfig(c Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = c
}

// SetActive sets the active state.
func (e *Engine) SetActive(a bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.active = a
}

// Active returns the active state.
func (e *Engine) Active() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active
}

func (e *Engine) record(ctx context.Context, actor, action, result string, d map[string]any) {
	if e.audit == nil {
		return
	}
	if err := e.audit.Record(ctx, actor, action, "policy", result, d); err != nil {
		e.log.Error("policy: audit write failed", "action", action, "err", err)
	}
}

// triggers returns the true triggers, in the order of step 1.
func (e *Engine) triggers(now time.Time, in Input) []string {
	c := e.cfg
	var out []string
	if c.OnBatterySeconds > 0 && in.OnBattery && !e.onBattery.IsZero() && now.Sub(e.onBattery) >= time.Duration(c.OnBatterySeconds)*time.Second {
		out = append(out, TriggerOnBattery)
	}
	if c.RuntimeBelowMinutes > 0 && in.OnBattery && !in.CommLost && in.Runtime != nil && *in.Runtime < float64(c.RuntimeBelowMinutes) {
		out = append(out, TriggerRuntime)
	}
	if c.ChargeBelowPercent > 0 && in.OnBattery && !in.CommLost && in.Charge != nil && *in.Charge < float64(c.ChargeBelowPercent) {
		out = append(out, TriggerCharge)
	}
	if c.LowBatteryFlag && in.LowBattery {
		out = append(out, TriggerLowBattery)
	}
	if c.CommLostOnBatterySecs > 0 && in.CommLost && in.OnBattery && !e.commLost.IsZero() && now.Sub(e.commLost) >= time.Duration(c.CommLostOnBatterySecs)*time.Second {
		out = append(out, TriggerCommLost)
	}
	return out
}

func (e *Engine) cancelLocked(ctx context.Context, now time.Time, reason, actor string) Step {
	cd := e.cd
	e.cd = nil
	e.log.Warn("policy: countdown cancelled", "reason", reason, "trigger", cd.Trigger)
	e.record(ctx, actor, "policy_countdown_cancel", "success", map[string]any{"reason": reason, "trigger": cd.Trigger})
	return Step{Time: now, Kind: StepCancel, Detail: reason, Data: map[string]any{"trigger": cd.Trigger}}
}

func (e *Engine) call(ctx context.Context, now time.Time, trigger string) Step {
	e.lastCall = now
	action := e.cfg.Action
	reason := fmt.Sprintf("VoltioVolt: UPS power failure (trigger %s)", trigger)
	var err error
	if action == ActionHibernate {
		err = e.exec.Hibernate(ctx, reason)
	} else {
		action = ActionShutdown
		err = e.exec.Shutdown(ctx, reason)
	}
	e.lastErr = err
	d := map[string]any{"action": action, "trigger": trigger}
	if err != nil {
		e.log.Error("policy: shutdown request failed", "action", action, "err", err)
		e.record(ctx, "system", "policy_shutdown_request", "failure", map[string]any{"action": action, "trigger": trigger, "error": err.Error()})
		return Step{Time: now, Kind: StepShutdownFailed, Detail: err.Error(), Data: d}
	}
	e.log.Warn("policy: shutdown request sent", "action", action, "trigger", trigger)
	e.record(ctx, "system", "policy_shutdown_request", "success", d)
	return Step{Time: now, Kind: StepShutdownRequest, Detail: action, Data: d}
}

// Tick evaluates the policy at now. It returns the new steps.
func (e *Engine) Tick(ctx context.Context, now time.Time, in Input) []Step {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.last = in
	var steps []Step
	if in.OnBattery {
		if e.onBattery.IsZero() {
			e.onBattery = now
		}
	} else {
		e.onBattery = time.Time{}
	}
	if in.CommLost {
		if e.commLost.IsZero() {
			e.commLost = now
		}
	} else {
		e.commLost = time.Time{}
	}
	mainsBack := !in.OnBattery && !in.CommLost
	if mainsBack {
		e.suppressed = map[string]bool{}
		e.requested, e.lastErr = false, nil
	}
	trig := e.triggers(now, in)
	isTrue := map[string]bool{}
	for _, t := range trig {
		isTrue[t] = true
	}
	for t := range e.suppressed {
		if !isTrue[t] {
			delete(e.suppressed, t) // a new true value later starts a countdown
		}
	}

	if !e.active {
		if e.cd != nil {
			steps = append(steps, e.cancelLocked(ctx, now, ReasonInactive, "system"))
		}
		return steps
	}
	if e.cd != nil {
		if mainsBack && e.cfg.CancelWhenMainsReturns {
			return append(steps, e.cancelLocked(ctx, now, ReasonMains, "system"))
		}
		if !now.Before(e.cd.Ends) {
			tr := e.cd.Trigger
			e.cd = nil
			e.requested = true
			return append(steps, e.call(ctx, now, tr))
		}
		return steps
	}
	if e.requested {
		// After a failed call: try again while a trigger is true.
		if e.lastErr != nil && len(trig) > 0 && now.Sub(e.lastCall) >= RetryAfter {
			steps = append(steps, e.call(ctx, now, trig[0]))
		}
		return steps
	}
	for _, t := range trig {
		if e.suppressed[t] {
			continue
		}
		d := time.Duration(e.cfg.CountdownSeconds) * time.Second
		e.cd = &Countdown{Active: true, Trigger: t, Started: now, Ends: now.Add(d), Action: e.cfg.Action}
		e.log.Warn("policy: countdown started", "trigger", t, "seconds", e.cfg.CountdownSeconds, "action", e.cfg.Action)
		e.record(ctx, "system", "policy_countdown_start", "success", map[string]any{"trigger": t, "seconds": e.cfg.CountdownSeconds, "action": e.cfg.Action})
		steps = append(steps,
			Step{Time: now, Kind: StepTrigger, Detail: t},
			Step{Time: now, Kind: StepCountdownStart, Detail: t, Data: map[string]any{"seconds": e.cfg.CountdownSeconds}})
		break
	}
	return steps
}

// Cancel cancels the countdown (an operator with the elevated state). It
// is not possible while LOW_BATTERY is active (step 2.3).
func (e *Engine) Cancel(ctx context.Context, now time.Time, actor string) (Step, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cd == nil {
		return Step{}, ErrNoCountdown
	}
	if e.last.LowBattery {
		e.record(ctx, actor, "policy_countdown_cancel", "rejected", map[string]any{"reason": "LOW_BATTERY active", "trigger": e.cd.Trigger})
		return Step{Time: now, Kind: StepCancelRefused, Detail: "LOW_BATTERY active"}, ErrLowBattery
	}
	// The triggers that are true now do not start a new countdown until
	// they are false again or the mains power returns.
	for _, t := range e.triggers(now, e.last) {
		e.suppressed[t] = true
	}
	e.suppressed[e.cd.Trigger] = true
	return e.cancelLocked(ctx, now, ReasonOperator, actor), nil
}

// Countdown returns the countdown state at now.
func (e *Engine) Countdown(now time.Time) Countdown {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cd == nil {
		return Countdown{Requested: e.requested, Cancelable: false}
	}
	c := *e.cd
	rem := c.Ends.Sub(now)
	if rem < 0 {
		rem = 0
	}
	c.Remaining = int((rem + time.Second - 1) / time.Second)
	c.Cancelable = !e.last.LowBattery
	return c
}
