// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// Results of one command.
const (
	resultData         = "data"
	resultNotSupported = "not supported"
	resultTimeout      = "timeout"
	resultError        = "error"
)

// Entry is one command in the Capture.
type Entry struct {
	UTC           string `json:"utc"`
	Command       string `json:"command"`
	RequestHex    string `json:"request_hex"`
	ResponseHex   string `json:"response_hex"`
	ResponseASCII string `json:"response_ascii"`
	TimeMillis    int64  `json:"time_ms"`
	Result        string `json:"result"`
	Error         string `json:"error,omitempty"`
}

// runner sends the Probe List or the Watch List through a Transport.
type runner struct {
	t     transport.Transport
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

func newRunner(t transport.Transport) *runner {
	return &runner{t: t, now: time.Now, sleep: sleepCtx}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	tm := time.NewTimer(d)
	defer tm.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-tm.C:
		return nil
	}
}

// redacted is the replacement for personal data.
func redacted(n int) string { return fmt.Sprintf("REDACTED (%d bytes)", n) }

func printable(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		switch {
		case c == '\r':
			sb.WriteString(`\r`)
		case c >= 0x20 && c <= 0x7e:
			sb.WriteByte(c)
		default:
			fmt.Fprintf(&sb, `\x%02x`, c)
		}
	}
	return sb.String()
}

// one sends one command. cmd must be in a constant list.
func (r *runner) one(ctx context.Context, cmd string) Entry {
	e := Entry{UTC: r.now().UTC().Format(time.RFC3339Nano), Command: cmd}
	req, err := dialect.EncodeRead(cmd)
	if err != nil {
		e.Result, e.Error = resultError, err.Error()
		return e
	}
	e.RequestHex = hex.EncodeToString(req)
	cctx, cancel := context.WithTimeout(ctx, timeoutMillis*time.Millisecond)
	defer cancel()
	resp, err := r.t.Exchange(cctx, req)
	e.TimeMillis = resp.Millis
	e.ResponseHex = hex.EncodeToString(resp.Raw)
	e.ResponseASCII = printable(resp.Raw)
	switch {
	case errors.Is(err, transport.ErrTimeout):
		e.Result = resultTimeout
	case err != nil:
		e.Result, e.Error = resultError, err.Error()
	default:
		if _, perr := dialect.Parse(cmd, resp.Raw); errors.Is(perr, dialect.ErrNotSupported) {
			e.Result = resultNotSupported
		} else if perr != nil {
			e.Result, e.Error = resultError, perr.Error()
		} else {
			e.Result = resultData
		}
	}
	// Privacy: the QID data can be a serial number.
	if cmd == "QID" && e.Result != resultNotSupported && len(resp.Raw) > 0 {
		e.ResponseHex = redacted(len(resp.Raw))
		e.ResponseASCII = redacted(len(resp.Raw))
		e.Error = ""
	}
	return e
}

// probe sends each Probe List command one time, in order.
func (r *runner) probe(ctx context.Context) ([]Entry, error) {
	var out []Entry
	for i, cmd := range probeList {
		if i > 0 {
			if err := r.sleep(ctx, commandGapMillis*time.Millisecond); err != nil {
				return out, err
			}
		}
		out = append(out, r.one(ctx, cmd))
	}
	return out, nil
}

// watch sends the Watch List every watch period for the given minutes.
// A command that returns "not supported" two times is not sent again.
func (r *runner) watch(ctx context.Context, minutes int) ([]Entry, error) {
	if minutes < minWatchMinutes || minutes > maxWatchMinutes {
		return nil, fmt.Errorf("--minutes must be %d to %d", minWatchMinutes, maxWatchMinutes)
	}
	end := r.now().Add(time.Duration(minutes) * time.Minute)
	nak := map[string]int{}
	var out []Entry
	first := true
	for {
		cycle := r.now()
		if !cycle.Before(end) {
			return out, nil
		}
		for _, cmd := range watchList {
			if nak[cmd] >= notSupportedLimit {
				continue
			}
			if !first {
				if err := r.sleep(ctx, commandGapMillis*time.Millisecond); err != nil {
					return out, err
				}
			}
			first = false
			e := r.one(ctx, cmd)
			if e.Result == resultNotSupported {
				nak[cmd]++
			}
			out = append(out, e)
		}
		if active(nak) == 0 {
			return out, nil
		}
		next := cycle.Add(watchPeriodMillis * time.Millisecond)
		if wait := next.Sub(r.now()); wait > 0 {
			if err := r.sleep(ctx, wait); err != nil {
				return out, err
			}
		}
	}
}

func active(nak map[string]int) int {
	n := 0
	for _, c := range watchList {
		if nak[c] < notSupportedLimit {
			n++
		}
	}
	return n
}
