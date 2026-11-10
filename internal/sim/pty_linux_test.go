// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package sim

import (
	"context"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

func TestSerialTransportOverPTY(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(NewModel(ProfileVoltronic))
	p, err := StartPTY(ctx, s)
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	defer p.Close()
	// A pty has no modem control lines. With DTR and RTS set to "on" the
	// Transport gives a warning for each line (VLT-IS-006: the default
	// "unchanged" makes no line call, see TestSerialTransportUnchangedLines).
	var warnings []string
	so := transport.DefaultSerialOptions()
	so.DTR, so.RTS = transport.LineOn, transport.LineOff
	so.Warn = func(m string) { warnings = append(warnings, m) }
	tr := transport.NewSerial(p.Path, so, transport.Options{Timeout: time.Second, MinInterval: time.Millisecond, FlushTime: 20 * time.Millisecond})
	if err := tr.Open(ctx); err != nil {
		t.Fatalf("open %s: %v", p.Path, err)
	}
	defer tr.Close()
	for _, cmd := range []string{"Q1", "QGS", "QBV"} {
		req, _ := dialect.EncodeRead(cmd)
		r, err := tr.Exchange(ctx, req)
		if err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		if _, err := dialect.Parse(cmd, r.Raw); err != nil {
			t.Errorf("%s %q: %v", cmd, r.Raw, err)
		}
	}
	if len(warnings) != 2 {
		t.Errorf("warnings %q", warnings)
	}
	if got := s.Received(); len(got) != 3 {
		t.Errorf("received %q", got)
	}
}

// TestSerialTransportUnchangedLines: with the default "unchanged" the
// Transport does not set DTR or RTS, so a pty gives no warning.
func TestSerialTransportUnchangedLines(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(NewModel(ProfileVoltronic))
	p, err := StartPTY(ctx, s)
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	defer p.Close()
	var warnings []string
	so := transport.DefaultSerialOptions()
	so.Warn = func(m string) { warnings = append(warnings, m) }
	tr := transport.NewSerial(p.Path, so, transport.Options{Timeout: time.Second, MinInterval: time.Millisecond, FlushTime: 20 * time.Millisecond})
	if err := tr.Open(ctx); err != nil {
		t.Fatalf("open %s: %v", p.Path, err)
	}
	defer tr.Close()
	if len(warnings) != 0 {
		t.Errorf("warnings %q", warnings)
	}
}
