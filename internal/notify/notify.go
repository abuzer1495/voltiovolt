// SPDX-License-Identifier: AGPL-3.0-or-later

// Package notify is the Notification Router of vltd (VLT-IS-007 TASK
// 03-00-00-003). It takes the Events of the Event Engine and the countdown
// state of the Policy Engine, and gives Notifications to the Channels
// (desktop, Telegram, e-mail).
//
//  1. Each Channel has a Setting on or off and a minimum severity (INFO,
//     WARNING or CRITICAL; default WARNING).
//  2. Flood control: the first Event of a type is sent at once. More Events
//     of the same type in the next 60 seconds go in one combined message at
//     the end of that time.
//  3. Queue: when a send fails, the Router tries again after 10 s, 30 s,
//     1 min, 5 min, then every 5 min for 1 hour after the Event. A message
//     that is sent late has the text "late" with the original time.
//  4. Text from templates in Turkish and English (Setting ui.language).
//  5. Content: Event, time, mode, input voltage, load, battery charge and
//     runtime when available. No Secret, no serial number: the Router uses
//     only the fields of Snapshot, never the Event data as a whole.
package notify

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// Severities. They are the same as the Event severities.
const (
	Info     = "INFO"
	Warning  = "WARNING"
	Critical = "CRITICAL"
)

// Countdown message types (from the Policy Engine).
const (
	TypeCountdownStart  = "SHUTDOWN_COUNTDOWN"
	TypeCountdownCancel = "SHUTDOWN_COUNTDOWN_CANCELLED"
	TypeShutdownRequest = "SHUTDOWN_REQUESTED"
	TypeTest            = "TEST_MESSAGE"
)

// FloodWindow is the flood control time (step 3).
const FloodWindow = 60 * time.Second

// MaxRetryAge is the time after the Event when the queue stops (step 4).
const MaxRetryAge = time.Hour

// RetryDelays are the delays after the 1st, 2nd, 3rd and 4th failure.
// After that the delay is RetryEvery.
var RetryDelays = []time.Duration{10 * time.Second, 30 * time.Second, time.Minute, 5 * time.Minute}

// RetryEvery is the delay after the 5th and each later failure.
const RetryEvery = 5 * time.Minute

// SeverityRank gives the order of the severities. An unknown severity is 0.
func SeverityRank(s string) int {
	switch s {
	case Info:
		return 1
	case Warning:
		return 2
	case Critical:
		return 3
	}
	return 0
}

// ValidSeverity reports whether s is a severity.
func ValidSeverity(s string) bool { return SeverityRank(s) > 0 }

// Modes of the Snapshot.
const (
	ModeMains    = "mains"
	ModeBattery  = "battery"
	ModeCommLost = "comm_lost"
	ModeUnknown  = ""
)

// Snapshot is the UPS state at the time of the Event. A nil value is "not
// available" and is not in the text.
type Snapshot struct {
	Mode         string
	InputVoltage *float64
	Load         *float64
	Charge       *float64
	Runtime      *float64 // minutes
}

// Item is one Event (or countdown change) in a message.
type Item struct {
	Time     time.Time
	Type     string
	Severity string
	// Active is false for the end of a condition (a clear Event).
	Active bool
	// Remaining and Action are for the countdown types only.
	Remaining int
	Action    string
}

// Message is one Notification: one Item, or the combined Items of one type
// (flood control).
type Message struct {
	Type     string
	Severity string // the highest severity of the Items
	Items    []Item
	Snapshot Snapshot
	// Late is true when the message is sent after a failed send. Original
	// is then the time of the first Item.
	Late bool
}

// Time is the time of the first Item.
func (m Message) Time() time.Time {
	if len(m.Items) == 0 {
		return time.Time{}
	}
	return m.Items[0].Time
}

// Notification is the text for a Channel.
type Notification struct {
	Subject  string // for e-mail: "[VLT][severity] Event"
	Text     string // the full text
	Severity string
	Type     string
	Time     time.Time
}

// Channel sends Notifications. Send must return an error when the message
// was not delivered, so that the Router tries again.
type Channel interface {
	Name() string
	Send(ctx context.Context, n Notification) error
}

// ChannelSettings are the Settings of one Channel.
type ChannelSettings struct {
	Enabled     bool
	MinSeverity string
}

// Config is the configuration of a Router.
type Config struct {
	// Settings returns the Settings of a Channel. Nil: all on, WARNING.
	Settings func(ctx context.Context, channel string) ChannelSettings
	// Language returns "en" or "tr" (Setting ui.language). Nil: "en".
	Language func(ctx context.Context) string
	// Snapshot returns the UPS state. Nil: an empty Snapshot.
	Snapshot func() Snapshot
	// Location of the times in the text. Nil: time.Local.
	Location *time.Location
	// SendTimeout is the time limit of one send. 0: 30 s.
	SendTimeout time.Duration
	Log         *slog.Logger
}

// ErrNoChannel is returned by SendTest for an unknown Channel.
var ErrNoChannel = errors.New("notify: no such channel")

type delivery struct {
	msg      Message
	failures int
	due      time.Time
}

type flood struct {
	start   time.Time // start of the window
	pending []Item
	snap    Snapshot
}

// Router is the Notification Router. Use New, then Run (or Process in
// tests).
type Router struct {
	cfg      Config
	mu       sync.Mutex
	channels []Channel
	queues   map[string][]*delivery
	floods   map[string]*flood
	incoming []Item
	sending  map[string]bool
	cd       struct {
		active    bool
		requested bool
	}
	wake chan struct{}
}

// New makes a Router.
func New(cfg Config, channels ...Channel) *Router {
	if cfg.SendTimeout <= 0 {
		cfg.SendTimeout = 30 * time.Second
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	r := &Router{cfg: cfg, queues: map[string][]*delivery{}, floods: map[string]*flood{}, sending: map[string]bool{}, wake: make(chan struct{}, 1)}
	for _, c := range channels {
		r.AddChannel(c)
	}
	return r
}

// AddChannel adds a Channel.
func (r *Router) AddChannel(c Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels = append(r.channels, c)
}

// Channels returns the names of the Channels.
func (r *Router) Channels() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.channels))
	for _, c := range r.channels {
		out = append(out, c.Name())
	}
	return out
}

func (r *Router) poke() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// Event gives one Event of the Event Engine to the Router. active is false
// for a clear Event ("active": false in the Event data).
func (r *Router) Event(t time.Time, typ, severity string, active bool) {
	r.mu.Lock()
	r.incoming = append(r.incoming, Item{Time: t, Type: typ, Severity: severity, Active: active})
	r.mu.Unlock()
	r.poke()
}

// Countdown gives the countdown state of the Policy Engine. The Router
// makes a message when a countdown starts (CRITICAL), when it stops
// without a shutdown request (WARNING), and at the shutdown request
// (CRITICAL).
func (r *Router) Countdown(t time.Time, active, requested bool, remaining int, action string) {
	r.mu.Lock()
	var it *Item
	switch {
	case active && !r.cd.active:
		it = &Item{Time: t, Type: TypeCountdownStart, Severity: Critical, Active: true, Remaining: remaining, Action: action}
	case requested && !r.cd.requested:
		it = &Item{Time: t, Type: TypeShutdownRequest, Severity: Critical, Active: true, Action: action}
	case !active && r.cd.active && !requested:
		it = &Item{Time: t, Type: TypeCountdownCancel, Severity: Warning, Active: false}
	}
	r.cd.active, r.cd.requested = active, requested
	if it != nil {
		r.incoming = append(r.incoming, *it)
	}
	r.mu.Unlock()
	if it != nil {
		r.poke()
	}
}

func (r *Router) snapshot() Snapshot {
	if r.cfg.Snapshot == nil {
		return Snapshot{}
	}
	return r.cfg.Snapshot()
}

func (r *Router) settings(ctx context.Context, ch string) ChannelSettings {
	if r.cfg.Settings == nil {
		return ChannelSettings{Enabled: true, MinSeverity: Warning}
	}
	s := r.cfg.Settings(ctx, ch)
	if !ValidSeverity(s.MinSeverity) {
		s.MinSeverity = Warning
	}
	return s
}

func (r *Router) language(ctx context.Context) string {
	if r.cfg.Language == nil {
		return "en"
	}
	return r.cfg.Language(ctx)
}

// emit puts a message in the queue of each Channel that is on and has a
// minimum severity equal to or less than the message severity. r.mu is
// held.
func (r *Router) emitLocked(ctx context.Context, m Message, now time.Time) {
	for _, c := range r.channels {
		s := r.settings(ctx, c.Name())
		if !s.Enabled || SeverityRank(m.Severity) < SeverityRank(s.MinSeverity) {
			continue
		}
		r.queues[c.Name()] = append(r.queues[c.Name()], &delivery{msg: m, due: now})
	}
}

func maxSeverity(items []Item) string {
	best := ""
	for _, it := range items {
		if SeverityRank(it.Severity) > SeverityRank(best) {
			best = it.Severity
		}
	}
	return best
}

// route does the flood control for the new Items and the ended windows.
func (r *Router) routeLocked(ctx context.Context, now time.Time) {
	in := r.incoming
	r.incoming = nil
	for _, it := range in {
		f := r.floods[it.Type]
		if f == nil || now.Sub(f.start) >= FloodWindow && len(f.pending) == 0 {
			r.floods[it.Type] = &flood{start: now}
			r.emitLocked(ctx, Message{Type: it.Type, Severity: it.Severity, Items: []Item{it}, Snapshot: r.snapshot()}, now)
			continue
		}
		f.pending = append(f.pending, it)
		f.snap = r.snapshot()
	}
	types := make([]string, 0, len(r.floods))
	for t := range r.floods {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		f := r.floods[t]
		if now.Sub(f.start) < FloodWindow {
			continue
		}
		if len(f.pending) == 0 {
			delete(r.floods, t)
			continue
		}
		items := f.pending
		r.floods[t] = &flood{start: now}
		r.emitLocked(ctx, Message{Type: t, Severity: maxSeverity(items), Items: items, Snapshot: f.snap}, now)
	}
}

// nextDue returns the time of the next try after a failure, and false when
// the queue stops (1 hour after the Event).
func nextDue(m Message, failures int, now time.Time) (time.Time, bool) {
	d := RetryEvery
	if failures-1 < len(RetryDelays) {
		d = RetryDelays[failures-1]
	}
	due := now.Add(d)
	if due.Sub(m.Time()) > MaxRetryAge {
		return time.Time{}, false
	}
	return due, true
}

// Process routes the new Items and sends the messages that are due at the
// time now. Run calls it; tests call it with a virtual time.
func (r *Router) Process(ctx context.Context, now time.Time) {
	r.mu.Lock()
	r.routeLocked(ctx, now)
	chans := append([]Channel(nil), r.channels...)
	r.mu.Unlock()
	for _, c := range chans {
		r.processChannel(ctx, c, now)
	}
}

func (r *Router) processChannel(ctx context.Context, c Channel, now time.Time) {
	name := c.Name()
	r.mu.Lock()
	if r.sending[name] {
		r.mu.Unlock()
		return
	}
	var due []*delivery
	var keep []*delivery
	for _, d := range r.queues[name] {
		if !d.due.After(now) {
			due = append(due, d)
		} else {
			keep = append(keep, d)
		}
	}
	r.queues[name] = keep
	if len(due) > 0 {
		r.sending[name] = true
	}
	r.mu.Unlock()
	if len(due) == 0 {
		return
	}
	lang := r.language(ctx)
	var again []*delivery
	for _, d := range due {
		m := d.msg
		m.Late = d.failures > 0
		n := Render(lang, m, r.cfg.Location)
		sctx, cancel := context.WithTimeout(ctx, r.cfg.SendTimeout)
		err := c.Send(sctx, n)
		cancel()
		if err == nil {
			continue
		}
		d.failures++
		next, ok := nextDue(d.msg, d.failures, now)
		if !ok {
			r.cfg.Log.Warn("notification not sent; the queue stopped after 1 hour", "channel", name, "type", m.Type, "failures", d.failures, "err", err)
			continue
		}
		r.cfg.Log.Info("notification not sent; try again later", "channel", name, "type", m.Type, "failures", d.failures, "next", next, "err", err)
		d.due = next
		again = append(again, d)
	}
	r.mu.Lock()
	r.queues[name] = append(r.queues[name], again...)
	r.sending[name] = false
	r.mu.Unlock()
}

// Pending returns the number of messages in the queue of a Channel.
func (r *Router) Pending(channel string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.queues[channel])
}

// SendTest sends a test message to one Channel at once. It does not use
// the queue, the Settings or the flood control.
func (r *Router) SendTest(ctx context.Context, channel string, now time.Time) error {
	r.mu.Lock()
	var ch Channel
	for _, c := range r.channels {
		if c.Name() == channel {
			ch = c
		}
	}
	r.mu.Unlock()
	if ch == nil {
		return ErrNoChannel
	}
	m := Message{Type: TypeTest, Severity: Info, Items: []Item{{Time: now, Type: TypeTest, Severity: Info, Active: true}}, Snapshot: r.snapshot()}
	sctx, cancel := context.WithTimeout(ctx, r.cfg.SendTimeout)
	defer cancel()
	return ch.Send(sctx, Render(r.language(ctx), m, r.cfg.Location))
}

// Run calls Process every second and after each new Item, until ctx ends.
// Each Channel sends in its own goroutine, so a slow Channel does not stop
// the others.
func (r *Router) Run(ctx context.Context) {
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		case <-r.wake:
		}
		now := time.Now()
		r.mu.Lock()
		r.routeLocked(ctx, now)
		chans := append([]Channel(nil), r.channels...)
		r.mu.Unlock()
		for _, c := range chans {
			r.mu.Lock()
			busy := r.sending[c.Name()]
			r.mu.Unlock()
			if busy {
				continue
			}
			wg.Add(1)
			go func(c Channel) { defer wg.Done(); r.processChannel(ctx, c, now) }(c)
		}
	}
}
