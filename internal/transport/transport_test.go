// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

const q1Resp = "(209.7 209.7 219.7 019 50.0 2.32 23.3 00000001\r"

func echoQ1(req []byte) []byte {
	if string(req) == "Q1\r" {
		return []byte(q1Resp)
	}
	return []byte("(NAK\r")
}

func openFake(t *testing.T, p *FakePort) *Stream {
	t.Helper()
	s := NewFake(p)
	if err := s.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestExchange(t *testing.T) {
	p := &FakePort{Respond: echoQ1}
	s := openFake(t, p)
	r, err := s.Exchange(context.Background(), []byte("Q1\r"))
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Raw) != q1Resp {
		t.Errorf("raw %q", r.Raw)
	}
	if r.Millis < 0 {
		t.Errorf("millis %d", r.Millis)
	}
}

func TestExchangeRejectsNotRead(t *testing.T) {
	p := &FakePort{Respond: echoQ1}
	s := openFake(t, p)
	for _, req := range []string{"Q\r", "\r", "", "Q1", "Q1\r\r", "S01R0001\r", "q1\r", "D\r", "Q1 \r", "T\r"} {
		if _, err := s.Exchange(context.Background(), []byte(req)); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("%q: %v; want ErrNotPermitted", req, err)
		}
	}
	if p.WriteHits != 0 {
		t.Errorf("%d writes; want 0", p.WriteHits)
	}
}

func TestExchangeFlushesOldInput(t *testing.T) {
	p := &FakePort{Respond: echoQ1}
	s := openFake(t, p)
	p.Inject([]byte("(OLD DATA\r(MORE"))
	r, err := s.Exchange(context.Background(), []byte("Q1\r"))
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Raw) != q1Resp {
		t.Errorf("raw %q; old data not removed", r.Raw)
	}
}

func TestExchangeSplitResponse(t *testing.T) {
	p := &FakePort{Respond: echoQ1, Chunk: 3}
	s := openFake(t, p)
	r, err := s.Exchange(context.Background(), []byte("Q1\r"))
	if err != nil || string(r.Raw) != q1Resp {
		t.Fatalf("%q %v", r.Raw, err)
	}
}

func TestExchangePartialWriteCloses(t *testing.T) {
	p := &FakePort{Respond: echoQ1, MaxWrite: 1}
	s := openFake(t, p)
	_, err := s.Exchange(context.Background(), []byte("Q1\r"))
	if !errors.Is(err, ErrPartialWrite) {
		t.Fatalf("err %v", err)
	}
	if !p.Closed {
		t.Error("transport not closed after a partial write")
	}
	if p.WriteHits != 1 {
		t.Errorf("%d writes; the remaining bytes must not be written", p.WriteHits)
	}
	if _, err := s.Exchange(context.Background(), []byte("Q1\r")); !errors.Is(err, ErrClosed) {
		t.Errorf("after close: %v", err)
	}
}

func TestExchangeTimeout(t *testing.T) {
	p := &FakePort{Respond: func([]byte) []byte { return []byte("(209.7") }}
	s := openFake(t, p)
	r, err := s.Exchange(context.Background(), []byte("Q1\r"))
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err %v", err)
	}
	if string(r.Raw) != "(209.7" {
		t.Errorf("partial raw %q", r.Raw)
	}
}

func TestExchangeTooLong(t *testing.T) {
	p := &FakePort{Respond: func([]byte) []byte { return bytes.Repeat([]byte("1"), 300) }}
	s := openFake(t, p)
	if _, err := s.Exchange(context.Background(), []byte("Q1\r")); !errors.Is(err, ErrTooLong) {
		t.Fatalf("err %v", err)
	}
}

func TestExchangeDefaultTimeout(t *testing.T) {
	s := NewStream(FakeOpener(&FakePort{}), Options{})
	if s.opts.Timeout != 3*time.Second || s.opts.MinInterval != 500*time.Millisecond || s.opts.FlushTime != 100*time.Millisecond {
		t.Errorf("defaults %+v", s.opts)
	}
}

func TestExchangeMinInterval(t *testing.T) {
	now := time.Unix(0, 0)
	var slept []time.Duration
	p := &FakePort{Respond: echoQ1}
	s := NewStream(FakeOpener(p), Options{
		Timeout:   time.Hour,
		FlushTime: time.Nanosecond,
		Now:       func() time.Time { return now },
		Sleep: func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			now = now.Add(d)
			return nil
		},
	})
	_ = s.Open(context.Background())
	for i := 0; i < 3; i++ {
		if _, err := s.Exchange(context.Background(), []byte("Q1\r")); err != nil {
			t.Fatal(err)
		}
		now = now.Add(100 * time.Millisecond)
	}
	if len(slept) != 2 || slept[0] != 400*time.Millisecond {
		t.Errorf("slept %v; want two waits of 400ms", slept)
	}
}

func TestExchangeOneAtATime(t *testing.T) {
	var mu sync.Mutex
	active, maxActive := 0, 0
	p := &FakePort{Respond: func(req []byte) []byte {
		mu.Lock()
		active++
		maxActive = max(maxActive, active)
		mu.Unlock()
		time.Sleep(2 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return echoQ1(req)
	}}
	s := openFake(t, p)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Exchange(context.Background(), []byte("Q1\r")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if maxActive != 1 {
		t.Errorf("max parallel exchanges %d", maxActive)
	}
}

// fakeReports is a ReportDevice for tests.
type fakeReports struct {
	written [][]byte
	failAt  int // 1-based report number that is written partially
	in      [][]byte
}

func (f *fakeReports) WriteReport(r []byte) (int, error) {
	f.written = append(f.written, append([]byte(nil), r...))
	if f.failAt == len(f.written) {
		return len(r) - 1, nil
	}
	return len(r), nil
}

func (f *fakeReports) ReadReport(p []byte, _ time.Duration) (int, error) {
	if len(f.in) == 0 {
		return 0, nil
	}
	n := copy(p, f.in[0])
	f.in = f.in[1:]
	return n, nil
}

func (f *fakeReports) Close() error { return nil }

func TestHIDPortSplitsReports(t *testing.T) {
	dev := &fakeReports{}
	p, err := NewHIDPort(dev, ReportLengths{Input: 8, Output: 8})
	if err != nil {
		t.Fatal(err)
	}
	req := []byte("QFLAG\r")
	if n, err := p.Write(req); err != nil || n != len(req) {
		t.Fatalf("%d %v", n, err)
	}
	if len(dev.written) != 1 {
		t.Fatalf("%d reports", len(dev.written))
	}
	want := []byte{0, 'Q', 'F', 'L', 'A', 'G', '\r', 0, 0}
	if !bytes.Equal(dev.written[0], want) {
		t.Errorf("report %v; want %v", dev.written[0], want)
	}

	dev2 := &fakeReports{}
	p2, _ := NewHIDPort(dev2, ReportLengths{Input: 8, Output: 2})
	_, _ = p2.Write([]byte("QGS\r"))
	if len(dev2.written) != 2 || !bytes.Equal(dev2.written[1], []byte{0, 'S', '\r'}) {
		t.Errorf("reports %v", dev2.written)
	}
}

func TestHIDPartialReportStops(t *testing.T) {
	dev := &fakeReports{failAt: 1}
	p, _ := NewHIDPort(dev, ReportLengths{Input: 8, Output: 2})
	n, err := p.Write([]byte("QGS\r"))
	if err == nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
	if len(dev.written) != 1 {
		t.Errorf("%d reports written; the remaining reports must not be written", len(dev.written))
	}
}

func TestHIDStreamJoinsReports(t *testing.T) {
	var resp [][]byte
	for i := 0; i < len(q1Resp); i += 8 {
		r := make([]byte, 8)
		copy(r, q1Resp[i:min(i+8, len(q1Resp))])
		resp = append(resp, r)
	}
	// A stale report is in the buffer before the request.
	dev := &delayed{fakeReports: &fakeReports{in: [][]byte{[]byte("STALE...")}}, after: resp}
	port, err := NewHIDPort(dev, ReportLengths{Input: 8, Output: 8})
	if err != nil {
		t.Fatal(err)
	}
	s := NewFake(port)
	if err := s.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	r, err := s.Exchange(context.Background(), []byte("Q1\r"))
	if err != nil || string(r.Raw) != q1Resp {
		t.Fatalf("%q %v", r.Raw, err)
	}
}

// delayed gives the response reports only after the first write.
type delayed struct {
	*fakeReports
	after [][]byte
}

func (d *delayed) WriteReport(r []byte) (int, error) {
	n, err := d.fakeReports.WriteReport(r)
	d.in = append(d.in, d.after...)
	d.after = nil
	return n, err
}

func TestParseReportDescriptor(t *testing.T) {
	// Synthetic vendor-defined descriptor: 8-byte input and output reports,
	// no report IDs. It is not the descriptor of the Reference UPS.
	d := []byte{
		0x06, 0x00, 0xFF, // Usage Page (vendor)
		0x09, 0x01, // Usage
		0xA1, 0x01, // Collection (Application)
		0x15, 0x00, // Logical Minimum 0
		0x26, 0xFF, 0x00, // Logical Maximum 255
		0x75, 0x08, // Report Size 8
		0x95, 0x08, // Report Count 8
		0x09, 0x01, 0x81, 0x02, // Input
		0x09, 0x01, 0x91, 0x02, // Output
		0xC0, // End Collection
	}
	l, err := ParseReportDescriptor(d)
	if err != nil {
		t.Fatal(err)
	}
	if l.Input != 8 || l.Output != 8 || l.Feature != 0 || l.HasReportIDs {
		t.Errorf("%+v", l)
	}
	withID := append([]byte{0x85, 0x02}, d...)
	l, _ = ParseReportDescriptor(withID)
	if !l.HasReportIDs || l.OutputID != 2 {
		t.Errorf("%+v", l)
	}
	if _, err := ParseReportDescriptor([]byte{0x75}); err == nil {
		t.Error("cut item must fail")
	}
}

func FuzzParseReportDescriptor(f *testing.F) {
	f.Add([]byte{0x75, 0x08, 0x95, 0x08, 0x81, 0x02})
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseReportDescriptor(b) })
}
