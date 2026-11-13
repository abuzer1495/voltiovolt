// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/event"
	"github.com/abuzer1495/voltiovolt/internal/sim"
)

// Scenario names (step 6.1).
const (
	ScenarioPowerFailLong     = "power_fail_long"
	ScenarioPowerFailShort    = "power_fail_short"
	ScenarioLowBattery        = "low_battery"
	ScenarioCommLostOnBattery = "comm_lost_on_battery"
)

// Scenarios in the order of step 6.1.
var Scenarios = []string{ScenarioPowerFailLong, ScenarioPowerFailShort, ScenarioLowBattery, ScenarioCommLostOnBattery}

// Outcomes of a scenario.
const (
	OutcomeShutdown = "shutdown_request"
	OutcomeCancel   = "cancel"
	OutcomeNone     = "none"
)

// Simulation timing (virtual time).
const (
	simPoll      = 2 * time.Second
	simFailAt    = 10 * time.Second
	simLowAt     = 30 * time.Second
	simCommAt    = 20 * time.Second
	simExtraTime = 10 * time.Minute
)

// ScenarioResult is the result of one scenario.
type ScenarioResult struct {
	Name     string  `json:"name"`
	Steps    []Step  `json:"steps"`
	Outcome  string  `json:"outcome"`
	Expected string  `json:"expected,omitempty"` // empty: no pass rule
	Deadline float64 `json:"deadline,omitempty"` // seconds; comm_lost_on_battery only
	At       float64 `json:"at,omitempty"`       // virtual time of the outcome, seconds
	Pass     bool    `json:"pass"`
	Calls    []Call  `json:"executor_calls"` // dry-run calls
	RealMs   int64   `json:"real_ms"`
	// UPSOffFailed is true when the UPS output off step failed (the
	// scenario does not pass).
	UPSOffFailed bool `json:"ups_off_failed,omitempty"`
}

// dryUPS is the UPS output off step of the Simulation: it makes the
// command bytes and sends nothing (VLT-IS-006 TASK 03-00-00-006 step 5).
type dryUPS struct{}

func (dryUPS) ShutdownRestore(_ context.Context, g control.Grant, delay time.Duration, restore int) (control.Result, error) {
	return control.DryShutdownRestore(g, delay, restore)
}

// Result is the Simulation result (step 6.4).
type Result struct {
	Version   string           `json:"version"`
	Config    Config           `json:"config"`
	Time      time.Time        `json:"time"`
	Passed    bool             `json:"passed"`
	Fallback  bool             `json:"fallback,omitempty"`
	Scenarios []ScenarioResult `json:"scenarios"`
	// OSShutdown is the Simulation step "OS shutdown" (VLT-IS-008
	// TASK 03-00-00-004). If Pass is false, Passed is false and the
	// policy cannot be activated.
	OSShutdown *OSShutdownStep `json:"os_shutdown,omitempty"`
}

// simDischarge is the battery discharge of the simulator model on battery,
// in percent for each second of virtual time (VLT-IS-005: without a
// discharge a policy without an on-battery time trigger, for example the
// Fallback Policy, never starts a countdown in power_fail_short).
const simDischarge = 0.1

// CommLostMargin is the margin of the comm_lost_on_battery pass rule
// (VLT-IS-005 Q-IA-4).
const CommLostMargin = 10 * time.Second

// expected outcomes (VLT-IS-004 step 6.5, VLT-IS-005 Q-IA-4).
// comm_lost_on_battery: a shutdown request at a time equal to or less than
// comm lost time + trigger.comm_lost_on_battery_seconds + countdown_seconds
// + 10 s, when the trigger is on. Policies without an expected outcome for
// a scenario (the Fallback Policy has no rule for power_fail_long) report it
// only.
func expectedFor(c Config, name string, fallback bool) (string, bool) {
	switch name {
	case ScenarioPowerFailLong:
		if fallback {
			return "", false
		}
		return OutcomeShutdown, true
	case ScenarioPowerFailShort:
		return OutcomeCancel, true
	case ScenarioLowBattery:
		return OutcomeShutdown, true
	case ScenarioCommLostOnBattery:
		if c.CommLostOnBatterySecs > 0 {
			return OutcomeShutdown, true
		}
	}
	return "", false
}

// CommLostDeadline is the latest virtual time (from the scenario start) of
// the shutdown request in comm_lost_on_battery.
func CommLostDeadline(c Config) time.Duration {
	return simCommAt + time.Duration(c.CommLostOnBatterySecs+c.CountdownSeconds)*time.Second + CommLostMargin
}

// simEpoch is the virtual start time of each scenario.
var simEpoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Simulate runs all scenarios with a virtual clock and a dry-run executor.
// It does not send a command to the UPS and does not call the OS: the UPS
// is the simulator model (no Transport) and the executor is DryRun.
func Simulate(c Config, now time.Time) Result {
	return simulate(c, now, false)
}

// SimulateFallback runs the Simulation of the Fallback Policy with its pass
// rules (VLT-IS-005 TASK 03-00-00-001 step 1.4).
func SimulateFallback(now time.Time) Result {
	return simulate(FallbackConfig(), now, true)
}

func simulate(c Config, now time.Time, fallback bool) Result {
	r := Result{Version: Version(c), Config: c, Time: now.UTC(), Passed: true, Fallback: fallback}
	for _, name := range Scenarios {
		sr := runScenario(c, name)
		if want, ok := expectedFor(c, name, fallback); ok {
			sr.Expected = want
			sr.Pass = sr.Outcome == want
			if name == ScenarioCommLostOnBattery {
				sr.Deadline = CommLostDeadline(c).Seconds()
				sr.Pass = sr.Pass && sr.At <= sr.Deadline
			}
		} else {
			sr.Pass = true
		}
		if sr.UPSOffFailed {
			sr.Pass = false
		}
		if !sr.Pass {
			r.Passed = false
		}
		r.Scenarios = append(r.Scenarios, sr)
	}
	return r
}

func runScenario(c Config, name string) ScenarioResult {
	start := time.Now()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := sim.NewModel(sim.ProfileVoltronic)
	ev := event.New(nil, event.DefaultThresholds(), quiet)
	dry := &DryRun{}
	eng := NewEngine(c, dry, nil, quiet)
	eng.SetActive(true)
	ups := &dryUPS{}
	eng.SetUPSOff(ups, false)
	ctx := context.Background()

	res := ScenarioResult{Name: name, Outcome: OutcomeNone}
	at := func(t time.Time) float64 { return t.Sub(simEpoch).Seconds() }
	add := func(s Step) {
		s.T = at(s.Time)
		res.Steps = append(res.Steps, s)
	}
	limit := simFailAt + time.Duration(max(c.OnBatterySeconds, c.CommLostOnBatterySecs)+c.CountdownSeconds)*time.Second + simExtraTime
	commLost := false
	var cdStart time.Time
	triedCancel := false

	for t := time.Duration(0); t <= limit; t += simPoll {
		now := simEpoch.Add(t)
		// The scenario script.
		if t == simFailAt {
			m.SetMains(false)
		}
		switch name {
		case ScenarioPowerFailShort:
			if !cdStart.IsZero() && now.Sub(cdStart) >= time.Duration(c.CountdownSeconds)*time.Second/2 && !m.Mains {
				m.SetMains(true)
			}
		case ScenarioLowBattery:
			if t == simLowAt {
				m.SetCharge(15) // the model sets bit 6 below 20 %
			}
		case ScenarioCommLostOnBattery:
			if t == simCommAt {
				commLost = true
				for _, e := range ev.CommState(ctx, now, true) {
					add(Step{Time: now, Kind: StepEvent, Detail: e.Type, Data: map[string]any{"severity": e.Severity}})
				}
			}
		}
		// The battery discharges on battery.
		if !m.Mains && m.Charge > 0 {
			m.SetCharge(max(0, m.Charge-simDischarge*simPoll.Seconds()))
		}
		// One poll cycle (QS and QBV, the commands of the Event Engine and
		// the policy). No poll during comm lost.
		var in Input
		if !commLost {
			o := event.Observation{Time: now}
			for _, cmd := range []string{"QS", "QBV"} {
				v, err := dialect.Parse(cmd, m.Respond(cmd))
				if err != nil {
					continue
				}
				o.Add(cmd, v)
				if b, ok := v.(dialect.Battery); ok {
					if b.Charge.Valid {
						x := b.Charge.Value
						in.Charge = &x
					}
					if b.RuntimeEstimate.Valid {
						x := b.RuntimeEstimate.Value
						in.Runtime = &x
					}
				}
			}
			for _, e := range ev.Observe(ctx, o) {
				add(Step{Time: now, Kind: StepEvent, Detail: e.Type, Data: map[string]any{"severity": e.Severity}})
			}
		}
		st := ev.State()
		in.OnBattery, in.LowBattery, in.CommLost = st.OnBattery, st.LowBattery, st.CommLost
		for _, s := range eng.Tick(ctx, now, in) {
			add(s)
			switch s.Kind {
			case StepCountdownStart:
				cdStart = now
			case StepCancel:
				res.Outcome, res.At = OutcomeCancel, at(now)
			case StepShutdownRequest:
				res.Outcome, res.At = OutcomeShutdown, at(now)
			case StepUPSOffFailed:
				res.UPSOffFailed = true
			}
		}
		// low_battery: an operator cancel is refused.
		if name == ScenarioLowBattery && !cdStart.IsZero() && !triedCancel && now.After(cdStart) {
			triedCancel = true
			s, err := eng.Cancel(ctx, now, "simulation")
			if errors.Is(err, ErrLowBattery) {
				add(s)
			} else if err == nil {
				add(s)
				res.Outcome, res.At = OutcomeCancel, at(now)
			}
		}
		if res.Outcome != OutcomeNone {
			break
		}
	}
	res.Calls = dry.Calls()
	res.RealMs = time.Since(start).Milliseconds()
	return res
}
