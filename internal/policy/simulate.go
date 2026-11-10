// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

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
	Name     string `json:"name"`
	Steps    []Step `json:"steps"`
	Outcome  string `json:"outcome"`
	Expected string `json:"expected,omitempty"` // empty: no pass rule
	Pass     bool   `json:"pass"`
	Calls    []Call `json:"executor_calls"` // dry-run calls
	RealMs   int64  `json:"real_ms"`
}

// Result is the Simulation result (step 6.4).
type Result struct {
	Version   string           `json:"version"`
	Config    Config           `json:"config"`
	Time      time.Time        `json:"time"`
	Passed    bool             `json:"passed"`
	Scenarios []ScenarioResult `json:"scenarios"`
}

// expected outcomes (step 6.5). comm_lost_on_battery has no pass rule.
var expected = map[string]string{
	ScenarioPowerFailLong:  OutcomeShutdown,
	ScenarioPowerFailShort: OutcomeCancel,
	ScenarioLowBattery:     OutcomeShutdown,
}

// simEpoch is the virtual start time of each scenario.
var simEpoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Simulate runs all scenarios with a virtual clock and a dry-run executor.
// It does not send a command to the UPS and does not call the OS: the UPS
// is the simulator model (no Transport) and the executor is DryRun.
func Simulate(c Config, now time.Time) Result {
	r := Result{Version: Version(c), Config: c, Time: now.UTC(), Passed: true}
	for _, name := range Scenarios {
		sr := runScenario(c, name)
		if want, ok := expected[name]; ok {
			sr.Expected = want
			sr.Pass = sr.Outcome == want
			if !sr.Pass {
				r.Passed = false
			}
		} else {
			sr.Pass = true
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
				m.Charge = 15 // the model sets bit 6 below 20 %
			}
		case ScenarioCommLostOnBattery:
			if t == simCommAt {
				commLost = true
				for _, e := range ev.CommState(ctx, now, true) {
					add(Step{Time: now, Kind: StepEvent, Detail: e.Type, Data: map[string]any{"severity": e.Severity}})
				}
			}
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
				res.Outcome = OutcomeCancel
			case StepShutdownRequest:
				res.Outcome = OutcomeShutdown
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
				res.Outcome = OutcomeCancel
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
