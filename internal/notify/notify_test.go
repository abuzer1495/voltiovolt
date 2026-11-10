// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeChannel struct {
	name string
	mu   sync.Mutex
	fail int // the number of sends that fail
	sent []Notification
	at   []time.Time
	clk  func() time.Time
}

func (f *fakeChannel) Name() string { return f.name }

func (f *fakeChannel) Send(_ context.Context, n Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.clk != nil {
		f.at = append(f.at, f.clk())
	}
	if f.fail > 0 {
		f.fail--
		return errors.New("fake: send failed")
	}
	f.sent = append(f.sent, n)
	return nil
}

func (f *fakeChannel) got() []Notification {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Notification(nil), f.sent...)
}

var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func f64(v float64) *float64 { return &v }

func newRouter(settings map[string]ChannelSettings, chs ...Channel) *Router {
	return New(Config{
		Settings: func(_ context.Context, ch string) ChannelSettings { return settings[ch] },
		Location: time.UTC,
		Snapshot: func() Snapshot {
			return Snapshot{Mode: ModeBattery, InputVoltage: f64(0), Load: f64(23), Charge: f64(91.5), Runtime: f64(12)}
		},
	}, chs...)
}

func TestSeverityFilter(t *testing.T) {
	a := &fakeChannel{name: "a"}
	b := &fakeChannel{name: "b"}
	c := &fakeChannel{name: "c"}
	d := &fakeChannel{name: "d"}
	r := newRouter(map[string]ChannelSettings{
		"a": {Enabled: true, MinSeverity: Info},
		"b": {Enabled: true, MinSeverity: Warning},
		"c": {Enabled: true, MinSeverity: Critical},
		"d": {Enabled: false, MinSeverity: Info},
	}, a, b, c, d)
	ctx := context.Background()
	r.Event(t0, "AVR_ACTIVE", Info, true)
	r.Event(t0, "ON_BATTERY", Warning, true)
	r.Event(t0, "LOW_BATTERY", Critical, true)
	r.Process(ctx, t0)
	for _, x := range []struct {
		ch   *fakeChannel
		want int
	}{{a, 3}, {b, 2}, {c, 1}, {d, 0}} {
		if n := len(x.ch.got()); n != x.want {
			t.Errorf("channel %s: %d messages, want %d", x.ch.name, n, x.want)
		}
	}
	if g := c.got(); g[0].Type != "LOW_BATTERY" {
		t.Errorf("c got %s", g[0].Type)
	}
	// The default minimum severity is WARNING (an empty or wrong value).
	e := &fakeChannel{name: "e"}
	r2 := newRouter(map[string]ChannelSettings{"e": {Enabled: true, MinSeverity: "x"}}, e)
	r2.Event(t0, "AVR_ACTIVE", Info, true)
	r2.Event(t0, "ON_BATTERY", Warning, true)
	r2.Process(ctx, t0)
	if g := e.got(); len(g) != 1 || g[0].Type != "ON_BATTERY" {
		t.Errorf("default severity: %+v", g)
	}
}

func TestFloodControl(t *testing.T) {
	a := &fakeChannel{name: "a"}
	r := newRouter(map[string]ChannelSettings{"a": {Enabled: true, MinSeverity: Info}}, a)
	ctx := context.Background()
	// The first Event is sent at once.
	r.Event(t0, "ON_BATTERY", Warning, true)
	r.Process(ctx, t0)
	if n := len(a.got()); n != 1 {
		t.Fatalf("%d messages after the first Event", n)
	}
	// 3 more in the next 60 s: no message before the end of the window.
	for i, s := range []int{10, 20, 59} {
		at := t0.Add(time.Duration(s) * time.Second)
		r.Event(at, "ON_BATTERY", Warning, i%2 == 0)
		r.Process(ctx, at)
	}
	// Another type is not combined.
	r.Event(t0.Add(30*time.Second), "OVERLOAD", Warning, true)
	r.Process(ctx, t0.Add(30*time.Second))
	if n := len(a.got()); n != 2 {
		t.Fatalf("%d messages in the window, want 2", n)
	}
	r.Process(ctx, t0.Add(60*time.Second))
	g := a.got()
	if len(g) != 3 {
		t.Fatalf("%d messages after the window, want 3", len(g))
	}
	if !strings.Contains(g[2].Text, "3 times in 60 seconds") || !strings.Contains(g[2].Text, "12:00:59 On battery") ||
		!strings.Contains(g[2].Text, "12:00:20 On battery: ended") {
		t.Fatalf("combined text:\n%s", g[2].Text)
	}
	// After a quiet window, the next Event is sent at once again.
	r.Process(ctx, t0.Add(121*time.Second))
	r.Event(t0.Add(200*time.Second), "ON_BATTERY", Warning, true)
	r.Process(ctx, t0.Add(200*time.Second))
	if n := len(a.got()); n != 4 {
		t.Fatalf("%d messages after a quiet window, want 4", n)
	}
}

func TestRetry(t *testing.T) {
	now := t0
	a := &fakeChannel{name: "a", fail: 5, clk: func() time.Time { return now }}
	r := newRouter(map[string]ChannelSettings{"a": {Enabled: true, MinSeverity: Warning}}, a)
	ctx := context.Background()
	r.Event(t0, "ON_BATTERY", Warning, true)
	// Step 1 second at a time for 20 minutes.
	for s := 0; s <= 1200; s++ {
		now = t0.Add(time.Duration(s) * time.Second)
		r.Process(ctx, now)
	}
	want := []time.Duration{0, 10 * time.Second, 40 * time.Second, 100 * time.Second, 400 * time.Second, 700 * time.Second}
	if len(a.at) != len(want) {
		t.Fatalf("tries at %v", a.at)
	}
	for i, w := range want {
		if g := a.at[i].Sub(t0); g != w {
			t.Errorf("try %d at %v, want %v", i, g, w)
		}
	}
	g := a.got()
	if len(g) != 1 || !strings.Contains(g[0].Text, "Late message. Original time: 2026-11-10 12:00:00") {
		t.Fatalf("late text: %+v", g)
	}
	if r.Pending("a") != 0 {
		t.Fatal("queue not empty")
	}
}

func TestRetryStopsAfterOneHour(t *testing.T) {
	a := &fakeChannel{name: "a", fail: 1000}
	r := newRouter(map[string]ChannelSettings{"a": {Enabled: true, MinSeverity: Warning}}, a)
	ctx := context.Background()
	r.Event(t0, "ON_BATTERY", Warning, true)
	for s := 0; s <= 2*3600; s += 5 {
		r.Process(ctx, t0.Add(time.Duration(s)*time.Second))
	}
	if r.Pending("a") != 0 {
		t.Fatal("the queue did not stop")
	}
	// 0, 10, 40, 100, 400 s, then every 300 s up to 3600 s: 400+300*10 = 3400.
	if a.fail != 1000-15 {
		t.Fatalf("%d tries, want 15", 1000-a.fail)
	}
	if len(a.got()) != 0 {
		t.Fatal("sent")
	}
}

func TestNextDue(t *testing.T) {
	m := Message{Items: []Item{{Time: t0}}}
	for i, want := range []time.Duration{10 * time.Second, 30 * time.Second, time.Minute, 5 * time.Minute, 5 * time.Minute, 5 * time.Minute} {
		due, ok := nextDue(m, i+1, t0)
		if !ok || due.Sub(t0) != want {
			t.Errorf("failure %d: %v %v", i+1, due.Sub(t0), ok)
		}
	}
	if _, ok := nextDue(m, 5, t0.Add(56*time.Minute)); ok {
		t.Error("a try after 1 hour")
	}
}

func TestCountdownMessages(t *testing.T) {
	a := &fakeChannel{name: "a"}
	r := newRouter(map[string]ChannelSettings{"a": {Enabled: true, MinSeverity: Warning}}, a)
	ctx := context.Background()
	r.Countdown(t0, true, false, 60, "shutdown")
	r.Countdown(t0.Add(time.Second), true, false, 59, "shutdown") // no change
	r.Countdown(t0.Add(2*time.Second), false, false, 0, "")
	r.Countdown(t0.Add(3*time.Second), true, false, 60, "hibernate")
	r.Countdown(t0.Add(63*time.Second), false, true, 0, "hibernate")
	r.Process(ctx, t0.Add(64*time.Second))
	r.Process(ctx, t0.Add(130*time.Second))
	var types []string
	for _, n := range a.got() {
		types = append(types, n.Type)
	}
	want := TypeCountdownStart + "," + TypeCountdownCancel + "," + TypeShutdownRequest + "," + TypeCountdownStart
	if strings.Join(types, ",") != want {
		t.Fatalf("types %v", types)
	}
	if g := a.got(); !strings.Contains(g[0].Text, "The computer shuts down in 60 seconds.") || g[0].Severity != Critical {
		t.Fatalf("countdown text:\n%s", g[0].Text)
	}
}

func TestSendTest(t *testing.T) {
	a := &fakeChannel{name: "a"}
	r := newRouter(map[string]ChannelSettings{"a": {Enabled: false}}, a)
	if err := r.SendTest(context.Background(), "a", t0); err != nil {
		t.Fatal(err)
	}
	if g := a.got(); len(g) != 1 || g[0].Type != TypeTest || !strings.Contains(g[0].Text, "test message") {
		t.Fatalf("%+v", g)
	}
	if err := r.SendTest(context.Background(), "x", t0); !errors.Is(err, ErrNoChannel) {
		t.Fatal(err)
	}
}

func TestRunSendsAtOnce(t *testing.T) {
	a := &fakeChannel{name: "a"}
	r := newRouter(map[string]ChannelSettings{"a": {Enabled: true, MinSeverity: Warning}}, a)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()
	r.Event(time.Now(), "ON_BATTERY", Warning, true)
	end := time.Now().Add(5 * time.Second)
	for len(a.got()) == 0 && time.Now().Before(end) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if len(a.got()) != 1 {
		t.Fatal("no message")
	}
}
