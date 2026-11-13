// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
)

func TestParseArgs(t *testing.T) {
	t.Setenv("LANG", "C")
	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_MESSAGES", "")
	cfg, err := parseArgs([]string{"--lang", "tr", "--headless", "--scenario", "1", "--dialect", "megatec"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.lang.Code != "tr" || !cfg.headless || cfg.scenario != 1 || cfg.dialect != sim.ProfileQ1 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.lang.AlreadyRuns != "Simülatör zaten çalışıyor" {
		t.Fatalf("turkish already-runs %q", cfg.lang.AlreadyRuns)
	}
	if _, err := parseArgs([]string{"--lang", "de"}, &bytes.Buffer{}); err == nil {
		t.Fatal("bad lang")
	}
	if _, err := parseArgs([]string{"--scenario", "10"}, &bytes.Buffer{}); err == nil {
		t.Fatal("bad scenario")
	}
	if _, err := parseArgs([]string{"--dialect", "tcp"}, &bytes.Buffer{}); err == nil {
		t.Fatal("tcp dialect")
	}
	t.Setenv("LANG", "tr_TR.UTF-8")
	t.Setenv("LC_ALL", "tr_TR.UTF-8")
	cfg, err = parseArgs(nil, &bytes.Buffer{})
	if err != nil || cfg.lang.Code != "tr" {
		t.Fatalf("default tr: %+v %v", cfg, err)
	}
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	cfg, err = parseArgs(nil, &bytes.Buffer{})
	if err != nil || cfg.lang.Code != "en" {
		t.Fatalf("default en: %+v %v", cfg, err)
	}
}

func TestDefaultLangTurkishFromOS(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "tr_TR.UTF-8")
	if defaultLangCode() != "tr" {
		t.Fatalf("got %s", defaultLangCode())
	}
}

func TestHeadlessQ1AndSecondInstance(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errw bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--headless", "--lang", "en", "--dialect", "megatec"}, &out, &errw, nil)
	}()

	// Wait until the server listens. Do not leave a spare connection open:
	// the server accepts only one client (OD-44).
	deadline := time.Now().Add(3 * time.Second)
	var raw []byte
	var err error
	for time.Now().Before(deadline) {
		raw, err = exchangeDial(t)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("exchange: %v out=%s err=%s", err, out.String(), errw.String())
	}
	if len(raw) != 47 {
		t.Fatalf("Q1 length %d %q", len(raw), raw)
	}
	st, err := dialect.ParseQ1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if st.InputVoltage.Value != 230 || st.OutputVoltage.Value != 220 || st.Load.Value != 30 {
		t.Fatalf("%+v", st)
	}

	var out2, err2 bytes.Buffer
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	code := run(ctx2, []string{"--headless", "--lang", "en"}, &out2, &err2, nil)
	if code != 1 || !strings.Contains(err2.String(), "The simulator already runs") {
		t.Fatalf("second: code=%d err=%q out=%q", code, err2.String(), out2.String())
	}
	// Turkish message
	err2.Reset()
	code = run(ctx2, []string{"--headless", "--lang", "tr"}, &out2, &err2, nil)
	if code != 1 || !strings.Contains(err2.String(), "Simülatör zaten çalışıyor") {
		t.Fatalf("second tr: code=%d err=%q", code, err2.String())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("first process did not stop")
	}
}

func exchangeDial(t *testing.T) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, err := server.Dial(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return server.Exchange(ctx, c, "Q1", 2*time.Second)
}

func TestHeadlessScenario1(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errw bytes.Buffer
	go run(ctx, []string{"--headless", "--scenario", "1", "--lang", "en"}, &out, &errw, nil)
	deadline := time.Now().Add(3 * time.Second)
	var raw []byte
	var err error
	for time.Now().Before(deadline) {
		raw, err = exchangeDial(t)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("exchange: %v out=%s err=%s", err, out.String(), errw.String())
	}
	st, err := dialect.ParseQ1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Flags.UtilityFail || st.InputVoltage.Value != 0 {
		t.Fatalf("%+v raw=%q", st, raw)
	}
	if st.InputFaultVolt.Value != 230 {
		t.Fatalf("fault %v", st.InputFaultVolt.Value)
	}
}

func TestScenarios3to9AndTimed(t *testing.T) {
	m := sim.NewConsoleModel(sim.ProfileQ1)
	s := server.New(m)
	r := NewRunner()
	ss := newSession(m, s, r, langEN(), "test")

	ss.applyScenario(3)
	if v := m.Snap().Input; v != 180 {
		t.Fatalf("sc3 input %v", v)
	}
	ss.applyScenario(4)
	if v := m.Snap().Input; v != 260 {
		t.Fatalf("sc4 input %v", v)
	}
	ss.applyScenario(5)
	if m.Snap().Load != 110 {
		t.Fatalf("sc5 load %d", m.Snap().Load)
	}
	ss.applyScenario(6)
	sn := m.Snap()
	if absf(sn.Charge-15) > 0.05 || sn.Mains || !sn.BatteryLow {
		t.Fatalf("sc6 %+v", sn)
	}
	ss.applyScenario(8)
	if m.Snap().Temperature != 55 {
		t.Fatalf("sc8 temp %v", m.Snap().Temperature)
	}
	ss.applyScenario(9)
	sn = m.Snap()
	if !sn.Mains || sn.Input != 230 || sn.Load != 30 || sn.Charge != 100 || sn.Temperature != 25 {
		t.Fatalf("sc9 %+v", sn)
	}

	old2, old7 := scenario2Wait, scenario7Wait
	scenario2Wait = 80 * time.Millisecond
	scenario7Wait = 80 * time.Millisecond
	defer func() { scenario2Wait, scenario7Wait = old2, old7 }()

	ss.applyScenario(2)
	if m.Snap().Mains {
		t.Fatal("sc2 should take mains off")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !m.Snap().Mains {
		time.Sleep(10 * time.Millisecond)
	}
	if !m.Snap().Mains {
		t.Fatal("sc2 did not restore mains")
	}

	ss.applyScenario(7)
	if ss.fault != server.FaultNoResponse {
		t.Fatal("sc7 fault")
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && ss.fault == server.FaultNoResponse {
		// The session fault field is not cleared by the timer; the server is.
		// We probe via a pipe? Just wait and check by setting a getter...
		// The After callback only calls s.SetFault(None). Probe by sending
		// would need a server. For this unit test, wait the timer and
		// confirm Left() is empty.
		time.Sleep(10 * time.Millisecond)
		if d, _ := r.Left(); d == 0 {
			break
		}
	}
	if d, _ := r.Left(); d != 0 {
		t.Fatal("sc7 timer still running")
	}
}

func TestMenuKeysChangeValues(t *testing.T) {
	m := sim.NewConsoleModel(sim.ProfileQ1)
	s := server.New(m)
	ss := newSession(m, s, NewRunner(), langEN(), "test")
	// Input voltage is item 1; Right adds 1, Page Up adds 10.
	ss.cursor = itemInputV
	ss.change(1)
	if v := m.Snap().Input; v != 231 {
		t.Fatalf("input +1 = %v", v)
	}
	ss.change(10)
	if v := m.Snap().Input; v != 241 {
		t.Fatalf("input +10 = %v", v)
	}
	ss.cursor = itemMains
	ss.activate()
	if m.Snap().Mains {
		t.Fatal("mains should be off")
	}
	ss.cursor = itemLoad
	ss.change(10)
	if m.Snap().Load != 40 {
		t.Fatalf("load %d", m.Snap().Load)
	}
	ss.reset()
	sn := m.Snap()
	if !sn.Mains || sn.Input != 230 || sn.Load != 30 {
		t.Fatalf("reset %+v", sn)
	}
}

func TestUILayoutAndKeys(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("LANG", "C")
	ln, err := server.ListenExclusive()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	m := sim.NewConsoleModel(sim.ProfileQ1)
	srv := server.New(m)
	ss := newSession(m, srv, NewRunner(), langEN(), server.Path())
	var mu sync.Mutex
	var last string
	drawn := make(chan struct{}, 16)
	ss.onDraw = func(s string) {
		mu.Lock()
		last = s
		mu.Unlock()
		select {
		case drawn <- struct{}{}:
		default:
		}
	}
	sc := tcell.NewSimulationScreen("UTF-8")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- ss.loop(ctx, sc) }()

	waitDraw := func(t *testing.T) string {
		t.Helper()
		select {
		case <-drawn:
		case <-time.After(3 * time.Second):
			t.Fatal("no draw")
		}
		// Drain extra draws from the ticker so last is fresh.
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return last
	}
	screen := waitDraw(t)
	if !strings.Contains(screen, "VoltioVolt UPS Simulator") {
		t.Fatalf("title %q", screen)
	}
	if !strings.Contains(screen, "waiting") && !strings.Contains(screen, "connected") {
		t.Fatalf("client state %q", screen)
	}
	if !strings.Contains(screen, "megatec") {
		t.Fatalf("dialect %q", screen)
	}
	if !strings.Contains(screen, "Mains power") {
		t.Fatalf("menu %q", screen)
	}

	sc.InjectKey(tcell.KeyEnter, 0, 0)
	ok := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		screen = waitDraw(t)
		if strings.Contains(screen, "utility fail") {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("after enter, screen:\n%s", screen)
	}

	sc.InjectKey(tcell.KeyRune, 'q', 0)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		screen = waitDraw(t)
		if strings.Contains(screen, "Quit?") {
			break
		}
	}
	if !strings.Contains(screen, "Quit?") {
		t.Fatalf("quit prompt:\n%s", screen)
	}
	sc.InjectKey(tcell.KeyRune, 'y', 0)
	select {
	case c := <-done:
		if c != 0 {
			t.Fatalf("exit %d", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("UI did not quit")
	}
}

func readRow(sc tcell.SimulationScreen, y int) string {
	w, _ := sc.Size()
	var b strings.Builder
	for x := 0; x < w; x++ {
		r, _, _, _ := sc.GetContent(x, y)
		if r == 0 {
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), " ")
}

func readScreen(sc tcell.SimulationScreen) string {
	_, h := sc.Size()
	var b strings.Builder
	for y := 0; y < h; y++ {
		b.WriteString(readRow(sc, y))
		b.WriteByte('\n')
	}
	return b.String()
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
