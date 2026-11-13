// SPDX-License-Identifier: AGPL-3.0-or-later

// Package event is the Event Engine of vltd (VLT-IS-004 TASK 03-00-00-005).
// It makes Events from the Q1 or QS status bits, the QWS value, the
// threshold Settings and the communication state. Each Event is stored in
// the events table. Each CRITICAL Event is written to the log.
package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// Severities.
const (
	Info     = "INFO"
	Warning  = "WARNING"
	Critical = "CRITICAL"
)

// Event types.
const (
	OnBattery          = "ON_BATTERY"
	Online             = "ONLINE"
	LowBattery         = "LOW_BATTERY"
	BypassActive       = "BYPASS_ACTIVE"
	AVRActive          = "AVR_ACTIVE"
	UPSFault           = "UPS_FAULT"
	TestStarted        = "TEST_STARTED"
	UPSShutdownActive  = "UPS_SHUTDOWN_ACTIVE"
	WarningBitsChanged = "UPS_WARNING_BITS_CHANGED"
	Overload           = "OVERLOAD"
	OverTemperature    = "OVER_TEMPERATURE"
	RuntimeLow         = "RUNTIME_LOW"
	CommLost           = "COMM_LOST"
	CommRestored       = "COMM_RESTORED"
)

// Hysteresis is the part of the threshold value before a threshold Event
// clears (step 4).
const Hysteresis = 0.05

// Event is one Event.
type Event struct {
	Time     time.Time      `json:"ts"`
	Type     string         `json:"type"`
	Severity string         `json:"severity"`
	Data     map[string]any `json:"data"`
	Demo     bool           `json:"demo,omitempty"`
}

// Thresholds are the threshold Settings (step 3).
type Thresholds struct {
	OverloadPercent   float64
	OverTemperatureC  float64
	RuntimeLowMinutes float64
}

// ThresholdsFromStore reads the threshold Settings.
func ThresholdsFromStore(ctx context.Context, st *store.Store) (Thresholds, error) {
	th := DefaultThresholds()
	for key, out := range map[string]*float64{
		store.SettingEventOverload:   &th.OverloadPercent,
		store.SettingEventOverTemp:   &th.OverTemperatureC,
		store.SettingEventRuntimeLow: &th.RuntimeLowMinutes,
	} {
		s, err := st.GetSetting(ctx, key)
		if err != nil {
			return DefaultThresholds(), err
		}
		if err := json.Unmarshal(s.Value, out); err != nil {
			return DefaultThresholds(), fmt.Errorf("event: setting %s: %w", key, err)
		}
	}
	return th, nil
}

// DefaultThresholds are the defaults of step 3.
func DefaultThresholds() Thresholds {
	return Thresholds{OverloadPercent: 100, OverTemperatureC: 50, RuntimeLowMinutes: 5}
}

// Observation is the data of one poll cycle. A nil field was not in the
// cycle.
type Observation struct {
	Time    time.Time
	Flags   *dialect.Q1Flags // Q1 or QS status bits
	QWS     *string          // raw QWS bits
	Load    *float64         // load.percent
	Temp    *float64         // ups.temperature
	Runtime *float64         // battery.runtime
	// Input and output voltage of the same Q1 or QS response as Flags
	// (AVR_ACTIVE "boost" or "buck", VLT-IS-006 TASK 03-00-00-003 step 9).
	InV, OutV *float64
}

func valid[U dialect.Unit](q dialect.Quantity[U]) *float64 {
	if !q.Valid {
		return nil
	}
	v := q.Value
	return &v
}

// Add puts the data of one parsed response into the Observation. QMOD and
// the QGS bits are not used (step 2: UNVERIFIED, data only).
func (o *Observation) Add(cmd string, v any) {
	switch x := v.(type) {
	case dialect.Status:
		// QS has priority over Q1 when both are in one cycle.
		if o.Flags == nil || cmd == "QS" {
			f := x.Flags
			o.Flags = &f
			o.InV, o.OutV = valid(x.InputVoltage), valid(x.OutputVoltage)
		}
		if p := valid(x.Load); p != nil {
			o.Load = p
		}
		if p := valid(x.Temperature); p != nil {
			o.Temp = p
		}
	case dialect.StatusQGS:
		if p := valid(x.Load); p != nil {
			o.Load = p
		}
		if p := valid(x.Temperature); p != nil {
			o.Temp = p
		}
	case dialect.Battery:
		if p := valid(x.RuntimeEstimate); p != nil {
			o.Runtime = p
		}
	case dialect.Warnings:
		s := x.Bits.Raw
		o.QWS = &s
	}
}

// cond is one debounced condition.
type cond struct {
	typ      string
	severity string
	onPolls  int // polls in sequence before the Event
	active   bool
	on, off  int // counters
}

func (c *cond) step(raw bool) (changed bool) {
	switch {
	case raw && !c.active:
		c.off = 0
		c.on++
		if c.on >= c.onPolls {
			c.active, c.on = true, 0
			return true
		}
	case !raw && c.active:
		c.on = 0
		c.off++
		if c.off >= 2 { // step 5.2: all clear conditions after 2 polls
			c.active, c.off = false, 0
			return true
		}
	default:
		c.on, c.off = 0, 0
	}
	return false
}

// Engine makes Events. Its methods are safe for concurrent use.
type Engine struct {
	db  *sql.DB
	log *slog.Logger

	mu     sync.Mutex
	th     Thresholds
	conds  map[string]*cond
	order  []string
	qws    *string
	qwsNew *string
	qwsN   int
	lost   bool
	subs   map[int]func(Event)
	nextID int
	demo   func() bool
}

// SetDemo sets a function that reports whether the Event is from the
// demo mode (VLT-IS-009 TASK 03-00-00-005).
func (e *Engine) SetDemo(f func() bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.demo = f
}

// New returns an Engine that stores Events in db (the events table).
func New(db *sql.DB, th Thresholds, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	e := &Engine{db: db, log: log, th: th, conds: map[string]*cond{}, subs: map[int]func(Event){}}
	for _, c := range []cond{
		{typ: OnBattery, severity: Warning, onPolls: 1},   // step 5.1
		{typ: LowBattery, severity: Critical, onPolls: 1}, // step 5.1
		{typ: BypassActive, severity: Warning, onPolls: 2},
		{typ: AVRActive, severity: Info, onPolls: 2},
		{typ: UPSFault, severity: Critical, onPolls: 2},
		{typ: TestStarted, severity: Info, onPolls: 2},
		{typ: UPSShutdownActive, severity: Critical, onPolls: 2},
		{typ: Overload, severity: Warning, onPolls: 2},
		{typ: OverTemperature, severity: Warning, onPolls: 2},
		{typ: RuntimeLow, severity: Warning, onPolls: 2},
	} {
		c := c
		e.conds[c.typ] = &c
		e.order = append(e.order, c.typ)
	}
	return e
}

// SetThresholds changes the thresholds (a Settings change).
func (e *Engine) SetThresholds(th Thresholds) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.th = th
}

// Subscribe calls f for each new Event. It returns a function that ends
// the subscription. f must not block.
func (e *Engine) Subscribe(f func(Event)) (cancel func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	id := e.nextID
	e.nextID++
	e.subs[id] = f
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.subs, id)
	}
}

// State is the active conditions.
type State struct {
	OnBattery  bool     `json:"on_battery"`
	LowBattery bool     `json:"low_battery"`
	CommLost   bool     `json:"comm_lost"`
	Active     []string `json:"active"`
}

// State returns the active conditions.
func (e *Engine) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := State{OnBattery: e.conds[OnBattery].active, LowBattery: e.conds[LowBattery].active, CommLost: e.lost, Active: []string{}}
	for _, t := range e.order {
		if e.conds[t].active {
			s.Active = append(s.Active, t)
		}
	}
	return s
}

// threshold evaluates "value >= limit" with the hysteresis: an active
// Event clears below limit - 5 % of limit.
func above(active bool, v, limit float64) bool {
	if active {
		return v >= limit-Hysteresis*limit
	}
	return v >= limit
}

// below evaluates "value < limit"; an active Event clears at or above
// limit + 5 % of limit.
func below(active bool, v, limit float64) bool {
	if active {
		return v < limit+Hysteresis*limit
	}
	return v < limit
}

// Observe processes one poll cycle and returns the new Events. The Events
// are stored and sent to the subscribers.
func (e *Engine) Observe(ctx context.Context, o Observation) []Event {
	e.mu.Lock()
	var out []Event
	emit := func(c *cond, extra map[string]any) {
		d := map[string]any{"active": c.active}
		for k, v := range extra {
			d[k] = v
		}
		switch {
		case c.typ == OnBattery && !c.active:
			out = append(out, Event{Time: o.Time, Type: Online, Severity: Info, Data: map[string]any{}})
		case c.active:
			out = append(out, Event{Time: o.Time, Type: c.typ, Severity: c.severity, Data: d})
		default:
			// A clear: the same type, INFO, "active": false.
			out = append(out, Event{Time: o.Time, Type: c.typ, Severity: Info, Data: d})
		}
	}
	if f := o.Flags; f != nil {
		for _, x := range []struct {
			typ string
			raw bool
		}{
			{OnBattery, f.UtilityFail},                          // bit 7
			{LowBattery, f.BatteryLow},                          // bit 6
			{BypassActive, f.BypassBoostBuck && !f.StandbyType}, // bit 5 = 1, bit 3 = 0
			{AVRActive, f.BypassBoostBuck && f.StandbyType},     // bit 5 = 1, bit 3 = 1
			{UPSFault, f.UPSFailed},                             // bit 4
			{TestStarted, f.TestInProgress},                     // bit 2
			{UPSShutdownActive, f.ShutdownActive},               // bit 1
		} {
			c := e.conds[x.typ]
			if c.step(x.raw) {
				var extra map[string]any
				if x.typ == AVRActive && c.active {
					extra = avrMode(o.InV, o.OutV)
				}
				emit(c, extra)
			}
		}
	}
	if o.Load != nil {
		c := e.conds[Overload]
		if c.step(above(c.active, *o.Load, e.th.OverloadPercent)) {
			emit(c, map[string]any{"load_percent": *o.Load, "threshold": e.th.OverloadPercent})
		}
	}
	if o.Temp != nil {
		c := e.conds[OverTemperature]
		if c.step(above(c.active, *o.Temp, e.th.OverTemperatureC)) {
			emit(c, map[string]any{"temperature_c": *o.Temp, "threshold": e.th.OverTemperatureC})
		}
	}
	if o.Runtime != nil {
		c := e.conds[RuntimeLow]
		raw := e.conds[OnBattery].active && below(c.active, *o.Runtime, e.th.RuntimeLowMinutes)
		if c.step(raw) {
			emit(c, map[string]any{"runtime_minutes": *o.Runtime, "threshold": e.th.RuntimeLowMinutes})
		}
	}
	if o.QWS != nil {
		switch {
		case e.qws == nil:
			v := *o.QWS
			e.qws = &v // the first value: no Event
		case *o.QWS == *e.qws:
			e.qwsNew, e.qwsN = nil, 0
		default:
			if e.qwsNew == nil || *e.qwsNew != *o.QWS {
				v := *o.QWS
				e.qwsNew, e.qwsN = &v, 0
			}
			e.qwsN++
			if e.qwsN >= 2 {
				out = append(out, Event{Time: o.Time, Type: WarningBitsChanged, Severity: Info,
					Data: map[string]any{"old": *e.qws, "new": *e.qwsNew}})
				e.qws, e.qwsNew, e.qwsN = e.qwsNew, nil, 0
			}
		}
	}
	e.mu.Unlock()
	e.publish(ctx, out)
	return out
}

// CommState processes a change of the communication state. lost is true
// for "comm lost".
func (e *Engine) CommState(ctx context.Context, t time.Time, lost bool) []Event {
	e.mu.Lock()
	if lost == e.lost {
		e.mu.Unlock()
		return nil
	}
	e.lost = lost
	var ev Event
	if lost {
		sev := Warning
		if e.conds[OnBattery].active {
			sev = Critical // step 6: the last known state is ON_BATTERY
		}
		ev = Event{Time: t, Type: CommLost, Severity: sev, Data: map[string]any{"on_battery": e.conds[OnBattery].active}}
	} else {
		ev = Event{Time: t, Type: CommRestored, Severity: Info, Data: map[string]any{}}
		// The poll data starts again: the debounce counters restart.
		for _, c := range e.conds {
			c.on, c.off = 0, 0
		}
	}
	e.mu.Unlock()
	out := []Event{ev}
	e.publish(ctx, out)
	return out
}

func (e *Engine) publish(ctx context.Context, evs []Event) {
	for _, ev := range evs {
		e.mu.Lock()
		demo := e.demo != nil && e.demo()
		e.mu.Unlock()
		if demo {
			ev.Demo = true
			if ev.Data == nil {
				ev.Data = map[string]any{}
			}
			ev.Data["demo"] = true
		}
		if err := e.store(ctx, ev); err != nil {
			e.log.Error("event: store failed", "type", ev.Type, "err", err)
		}
		if ev.Severity == Critical {
			e.log.Error("CRITICAL event", "type", ev.Type, "data", ev.Data)
		} else {
			e.log.Info("event", "type", ev.Type, "severity", ev.Severity)
		}
		e.mu.Lock()
		subs := make([]func(Event), 0, len(e.subs))
		for _, f := range e.subs {
			subs = append(subs, f)
		}
		e.mu.Unlock()
		for _, f := range subs {
			f(ev)
		}
	}
}

func (e *Engine) store(ctx context.Context, ev Event) error {
	if e.db == nil {
		return nil
	}
	b, err := json.Marshal(ev.Data)
	if err != nil {
		return err
	}
	demo := 0
	if ev.Demo {
		demo = 1
	}
	_, err = e.db.ExecContext(ctx, "INSERT INTO events (ts, type, severity, data, demo) VALUES (?, ?, ?, ?, ?)",
		ev.Time.UnixMilli(), ev.Type, ev.Severity, string(b), demo)
	return err
}

// Stored is one row of the events table.
type Stored struct {
	ID int64 `json:"id"`
	Event
}

// MaxPage is the maximum page size of List.
const MaxPage = 500

// List returns Events, newest first, with an ID less than before (0: from
// the newest). limit is 1 to MaxPage.
func List(ctx context.Context, db *sql.DB, before int64, limit int) ([]Stored, error) {
	return ListFilter(ctx, db, before, limit, false)
}

// ListFilter is List with an option to include demo Events. The default
// (includeDemo false) hides them (VLT-IS-009 TASK 03-00-00-005).
func ListFilter(ctx context.Context, db *sql.DB, before int64, limit int, includeDemo bool) ([]Stored, error) {
	if limit < 1 || limit > MaxPage {
		return nil, fmt.Errorf("event: limit must be 1 to %d", MaxPage)
	}
	q := "SELECT rowid, ts, type, severity, data, demo FROM events WHERE (? = 0 OR rowid < ?)"
	if !includeDemo {
		q += " AND demo = 0"
	}
	q += " ORDER BY rowid DESC LIMIT ?"
	rows, err := db.QueryContext(ctx, q, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Stored{}
	for rows.Next() {
		var s Stored
		var ts int64
		var data string
		var demo int
		if err := rows.Scan(&s.ID, &ts, &s.Type, &s.Severity, &data, &demo); err != nil {
			return nil, err
		}
		s.Time = time.UnixMilli(ts).UTC()
		s.Demo = demo != 0
		if err := json.Unmarshal([]byte(data), &s.Data); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// avrMode is the AVR_ACTIVE data for b5 = 1 and b3 = 1 (VLT-REF-001 6.1,
// STATIC, PUBLIC): "boost" when the input voltage is less than the output
// voltage, else "buck". Without both voltages there is no "mode".
func avrMode(in, out *float64) map[string]any {
	if in == nil || out == nil {
		return nil
	}
	if *in < *out {
		return map[string]any{"mode": "boost"}
	}
	return map[string]any{"mode": "buck"}
}
