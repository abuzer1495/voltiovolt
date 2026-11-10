// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

type auditRow struct{ action, target, result string }

type fakeAudit struct {
	mu   sync.Mutex
	rows []auditRow
}

func (a *fakeAudit) Record(_ context.Context, _, action, target, result string, _ map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, auditRow{action, target, result})
	return nil
}

func (a *fakeAudit) count(action, result string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, r := range a.rows {
		if r.action == action && r.result == result {
			n++
		}
	}
	return n
}

// fakeLink records the sends. After each send, it gives the controller the
// statuses of the next polls from react.
type fakeLink struct {
	mu    sync.Mutex
	c     *Controller
	now   time.Time
	sent  []string
	react func(sent int) []Status
}

func (l *fakeLink) SendControl(_ context.Context, cmd Command) (time.Time, error) {
	l.mu.Lock()
	l.now = l.now.Add(2 * time.Second)
	at := l.now
	l.sent = append(l.sent, cmd.Text())
	n := len(l.sent)
	var next []Status
	if l.react != nil {
		next = l.react(n)
	}
	for i := range next {
		l.now = l.now.Add(2 * time.Second)
		next[i].At = l.now
	}
	l.mu.Unlock()
	go func() {
		for _, s := range next {
			l.c.Observe(s)
		}
	}()
	return at, nil
}

func st(f func(*dialect.Q1Flags)) Status {
	var s Status
	f(&s.Flags)
	return s
}

func newCtl(t *testing.T, react func(int) []Status) (*Controller, *fakeLink, *fakeAudit) {
	t.Helper()
	a := &fakeAudit{}
	c := &Controller{Audit: a, PollWait: 50 * time.Millisecond}
	l := &fakeLink{c: c, now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), react: react}
	c.SetLink(l, dialect.DialectMegatec)
	c.Observe(Status{At: l.now, Flags: dialect.Q1Flags{BeeperOn: true}})
	return c, l, a
}

var on = func(f *dialect.Q1Flags) { f.BeeperOn = true }

func TestControllerTestVerified(t *testing.T) {
	c, l, a := newCtl(t, func(n int) []Status {
		return []Status{st(func(f *dialect.Q1Flags) { f.BeeperOn = true; f.TestInProgress = true })}
	})
	r, err := c.StartTest(context.Background(), ipc(t, FuncTest10s), FuncTest10s, 0)
	if err != nil || !r.Verified || r.Sends != 1 || len(l.sent) != 1 || l.sent[0] != "T" {
		t.Fatalf("%+v %v %v", r, err, l.sent)
	}
	if a.count("ups_control", "sent") != 1 || a.count("ups_control_verify", "verified") != 1 {
		t.Fatalf("audit %v", a.rows)
	}
}

// TestControllerTestRetries: b2 stays 0, so the Controller sends the test
// command 3 times, then gives ErrNotVerify with an audit entry.
func TestControllerTestRetries(t *testing.T) {
	c, l, a := newCtl(t, func(n int) []Status { return []Status{st(on), st(on)} })
	_, err := c.StartTest(context.Background(), ipc(t, FuncTestMinutes), FuncTestMinutes, 5)
	if !errors.Is(err, ErrNotVerify) || len(l.sent) != 3 || l.sent[2] != "T05" {
		t.Fatalf("%v %v", err, l.sent)
	}
	if a.count("ups_control", "sent") != 3 || a.count("ups_control_verify", "not verified") != 1 || a.count("ups_control", "failure") != 1 {
		t.Fatalf("audit %v", a.rows)
	}
}

func TestControllerCancelTestOneSend(t *testing.T) {
	c, l, _ := newCtl(t, func(n int) []Status {
		return []Status{st(func(f *dialect.Q1Flags) { f.TestInProgress = true }), st(func(f *dialect.Q1Flags) { f.TestInProgress = true })}
	})
	_, err := c.CancelTest(context.Background(), ipc(t, FuncCancelTest))
	if !errors.Is(err, ErrNotVerify) || len(l.sent) != 1 || l.sent[0] != "CT" {
		t.Fatalf("%v %v", err, l.sent)
	}
}

func TestControllerShutdownRetries(t *testing.T) {
	c, l, _ := newCtl(t, func(n int) []Status {
		if n == 2 {
			return []Status{st(func(f *dialect.Q1Flags) { f.ShutdownActive = true })}
		}
		return []Status{st(on)}
	})
	r, err := c.ShutdownRestore(context.Background(), PolicyGrant(), 2*time.Minute, 1)
	if err != nil || !r.Verified || r.Sends != 2 || l.sent[0] != "S02R0001" {
		t.Fatalf("%+v %v %v", r, err, l.sent)
	}
	// 3 sends without b1.
	c, l, _ = newCtl(t, func(n int) []Status { return []Status{st(on), st(on)} })
	if _, err := c.ShutdownRestore(context.Background(), PolicyGrant(), 30*time.Second, 1); !errors.Is(err, ErrNotVerify) || len(l.sent) != 3 {
		t.Fatalf("%v %v", err, l.sent)
	}
}

// TestControllerBeeper: step 5. The beeper is on: "on" sends nothing.
// "off" sends Q once. Without a change, one retry follows.
func TestControllerBeeper(t *testing.T) {
	c, l, a := newCtl(t, func(n int) []Status { return []Status{st(func(*dialect.Q1Flags) {})} })
	r, err := c.SetBeeper(context.Background(), ipc(t, FuncBeeper), true)
	if err != nil || !r.NoSend || len(l.sent) != 0 || a.count("ups_control", "no send") != 1 {
		t.Fatalf("%+v %v %v", r, err, l.sent)
	}
	r, err = c.SetBeeper(context.Background(), ipc(t, FuncBeeper), false)
	if err != nil || !r.Verified || len(l.sent) != 1 || l.sent[0] != "Q" {
		t.Fatalf("%+v %v %v", r, err, l.sent)
	}
	c, l, _ = newCtl(t, func(n int) []Status { return []Status{st(on), st(on)} })
	if _, err := c.SetBeeper(context.Background(), ipc(t, FuncBeeper), false); !errors.Is(err, ErrNotVerify) || len(l.sent) != 2 {
		t.Fatalf("%v %v", err, l.sent)
	}
}

func TestControllerRejects(t *testing.T) {
	c, l, a := newCtl(t, nil)
	c.Observe(st(func(f *dialect.Q1Flags) { f.UtilityFail = true }))
	if _, err := c.StartTest(context.Background(), ipc(t, FuncTest10s), FuncTest10s, 0); !errors.Is(err, ErrOnBattery) {
		t.Fatalf("on battery: %v", err)
	}
	if _, err := c.StartTest(context.Background(), ipc(t, FuncTestUntilLow), FuncTest10s, 0); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("wrong grant: %v", err)
	}
	c.SetLink(nil, "")
	if _, err := c.CancelTest(context.Background(), ipc(t, FuncCancelTest)); !errors.Is(err, ErrNoLink) {
		t.Fatalf("no link: %v", err)
	}
	if len(l.sent) != 0 || a.count("ups_control", "failure") != 3 {
		t.Fatalf("sent %v audit %v", l.sent, a.rows)
	}
}

// TestControllerTestEnd: step 7. b2 1 to 0 ends the test. TEST_PASSED if
// b4 and b6 stayed 0, else TEST_FAILED.
func TestControllerTestEnd(t *testing.T) {
	c, _, a := newCtl(t, nil)
	t0 := time.Date(2026, 11, 10, 13, 0, 0, 0, time.UTC)
	var got []TestResult
	c.OnTestEnd = func(r TestResult) { got = append(got, r) }
	seq := func(flags ...func(*dialect.Q1Flags)) {
		for i, f := range flags {
			s := st(f)
			s.At = t0.Add(time.Duration(i) * 2 * time.Second)
			c.Observe(s)
		}
		t0 = t0.Add(time.Hour)
	}
	test := func(f *dialect.Q1Flags) { f.TestInProgress = true }
	idle := func(f *dialect.Q1Flags) {}
	seq(idle, test, test, idle)
	seq(test, func(f *dialect.Q1Flags) { f.TestInProgress = true; f.BatteryLow = true }, idle)
	seq(test, func(f *dialect.Q1Flags) { f.TestInProgress = true; f.UPSFailed = true }, idle)
	if len(got) != 3 || got[0].Result != TestPassed || got[1].Result != TestFailed || !got[1].BatteryLow || got[2].Result != TestFailed || !got[2].UPSFailed {
		t.Fatalf("%+v", got)
	}
	if got[0].End.Sub(got[0].Start) != 4*time.Second {
		t.Fatalf("times %+v", got[0])
	}
	if lt, ok := c.LastTest(); !ok || lt.Result != TestFailed {
		t.Fatalf("last %+v", lt)
	}
	if a.count("ups_test_end", TestPassed) != 1 || a.count("ups_test_end", TestFailed) != 2 {
		t.Fatalf("audit %v", a.rows)
	}
}
