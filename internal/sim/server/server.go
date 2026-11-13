// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server is the protocol server of vlt-simulator (VLT-IS-010
// TASK 03-00-00-002). It speaks the same bytes as a serial line (OD-45)
// on the local endpoint of OD-44.
package server

import (
	"bufio"
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/sim"
)

// English status texts that the console shows (TASK 03-00-00-002 step 2).
const (
	MsgConnected    = "Client connected"
	MsgDisconnected = "Client disconnected"
)

// Fault is a write-time fault injection (TASK 03-00-00-002 step 7).
type Fault string

// Faults.
const (
	FaultNone        Fault = "none"
	FaultNoResponse  Fault = "no-response"
	FaultGarbage     Fault = "garbage"
	FaultSplit       Fault = "split" // two parts, 200 ms between
	FaultWrongLength Fault = "wrong-length"
	FaultOldBytes    Fault = "old-bytes" // old bytes before the response
)

// OtherReply is the reply to a command that the profile does not support.
type OtherReply string

// Other replies.
const (
	OtherEcho OtherReply = "echo"
	OtherNAK  OtherReply = "nak"
	OtherNone OtherReply = "none"
)

// SplitWait is the time between the two parts of a split response.
const SplitWait = 200 * time.Millisecond

// LogLimit is the number of request/response pairs that the log keeps.
const LogLimit = 200

// Entry is one request and its response in the log.
type Entry struct {
	Time     time.Time
	Request  string
	Response string // without CR; empty if none
}

// Server listens on the OD-44 endpoint and answers as a UPS. It accepts
// one client at a time; a second client gets no connection.
type Server struct {
	Model *sim.Model
	// Delay is the response delay, 0 to 2000 ms.
	Delay time.Duration
	Other OtherReply
	Fault Fault
	// OnStatus is called with MsgConnected or MsgDisconnected.
	OnStatus func(string)

	mu      sync.Mutex
	busy    bool
	log     []Entry
	serving net.Conn
}

// New returns a Server for m. m must not be nil.
func New(m *sim.Model) *Server {
	return &Server{Model: m, Other: OtherEcho, Fault: FaultNone}
}

// Listen opens the default endpoint of OD-44.
func Listen() (net.Listener, error) { return ListenAt(Path()) }

// Serve accepts connections on ln until ctx is done.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		s.mu.Lock()
		if s.serving != nil {
			_ = s.serving.Close()
		}
		s.mu.Unlock()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		s.mu.Lock()
		if s.busy {
			s.mu.Unlock()
			_ = c.Close()
			continue
		}
		s.busy = true
		s.serving = c
		s.mu.Unlock()
		s.status(MsgConnected)
		s.serveOne(ctx, c)
		_ = c.Close()
		s.mu.Lock()
		s.busy = false
		s.serving = nil
		s.mu.Unlock()
		s.status(MsgDisconnected)
	}
}

func (s *Server) status(msg string) {
	if s.OnStatus != nil {
		s.OnStatus(msg)
	}
}

func (s *Server) serveOne(ctx context.Context, c net.Conn) {
	r := bufio.NewReaderSize(c, 256)
	var buf []byte
	for {
		if ctx.Err() != nil {
			return
		}
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		if b == '\r' {
			s.exchange(ctx, c, string(buf))
			buf = buf[:0]
			continue
		}
		if len(buf) < 128 && b != 0 {
			buf = append(buf, b)
		} else {
			buf = buf[:0] // overflow: drop the command
		}
	}
}

func (s *Server) exchange(ctx context.Context, c net.Conn, cmd string) {
	ops := s.replies(cmd)
	for _, op := range ops {
		if op.wait > 0 {
			t := time.NewTimer(op.wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
		if len(op.b) == 0 {
			continue
		}
		if _, err := c.Write(op.b); err != nil {
			return
		}
	}
}

type writeOp struct {
	b    []byte
	wait time.Duration
}

func (s *Server) replies(cmd string) []writeOp {
	s.mu.Lock()
	delay, other, fault := s.Delay, s.Other, s.Fault
	s.mu.Unlock()

	raw := s.answer(cmd, other)
	resp := string(raw)
	if len(raw) > 0 && raw[len(raw)-1] == '\r' {
		resp = string(raw[:len(raw)-1])
	}
	s.appendLog(cmd, resp)

	if fault == FaultNoResponse {
		s.replaceLog(cmd, "")
		return nil
	}
	if fault == FaultGarbage {
		raw = []byte("@@@GARBAGE\r")
		s.replaceLog(cmd, "@@@GARBAGE")
	}
	if fault == FaultWrongLength {
		raw = wrongLength(raw)
		s.replaceLog(cmd, trimCR(raw))
	}
	var ops []writeOp
	if delay > 0 {
		ops = append(ops, writeOp{wait: delay})
	}
	if fault == FaultOldBytes {
		ops = append(ops, writeOp{b: []byte("(OLD\r")})
	}
	if fault == FaultSplit && len(raw) >= 2 {
		mid := len(raw) / 2
		ops = append(ops, writeOp{b: raw[:mid]}, writeOp{b: raw[mid:], wait: SplitWait})
		return ops
	}
	if len(raw) > 0 {
		ops = append(ops, writeOp{b: raw})
	}
	return ops
}

func (s *Server) answer(cmd string, other OtherReply) []byte {
	if isControl(cmd) || s.modelSupports(cmd) {
		return s.Model.Respond(cmd)
	}
	switch other {
	case OtherNAK:
		return []byte("(NAK\r")
	case OtherNone:
		return nil
	default:
		return []byte(cmd + "\r")
	}
}

func (s *Server) modelSupports(cmd string) bool {
	if s.Model == nil {
		return false
	}
	return s.Model.Supports(cmd)
}

func isControl(cmd string) bool {
	switch {
	case cmd == "T" || cmd == "TL" || cmd == "CT" || cmd == "Q" || cmd == "C":
		return true
	case len(cmd) == 3 && cmd[0] == 'T' && isNum(cmd[1:]):
		return true
	case len(cmd) == 8 && cmd[0] == 'S' && cmd[3] == 'R':
		return true
	}
	return false
}

func isNum(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func wrongLength(raw []byte) []byte {
	s := trimCR(raw)
	if i := lastSpace(s); i > 0 {
		s = s[:i]
	} else if len(s) > 1 {
		s = s[:len(s)-1]
	}
	return []byte(s + "\r")
}

func lastSpace(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

func trimCR(b []byte) string {
	s := string(b)
	if n := len(s); n > 0 && s[n-1] == '\r' {
		return s[:n-1]
	}
	return s
}

func (s *Server) appendLog(req, resp string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = append(s.log, Entry{Time: time.Now(), Request: req, Response: resp})
	if len(s.log) > LogLimit {
		s.log = append([]Entry(nil), s.log[len(s.log)-LogLimit:]...)
	}
}

func (s *Server) replaceLog(req, resp string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.log); n > 0 && s.log[n-1].Request == req {
		s.log[n-1].Response = resp
	}
}

// Log returns a copy of the last request/response pairs, oldest first.
func (s *Server) Log() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Entry(nil), s.log...)
}

// ClientConnected reports whether a client is served.
func (s *Server) ClientConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy
}

// SetDelay sets the response delay, clamped to 0..2000 ms.
func (s *Server) SetDelay(d time.Duration) {
	if d < 0 {
		d = 0
	}
	if d > 2*time.Second {
		d = 2 * time.Second
	}
	s.mu.Lock()
	s.Delay = d
	s.mu.Unlock()
}

// SetFault sets the write-time fault.
func (s *Server) SetFault(f Fault) {
	s.mu.Lock()
	s.Fault = f
	s.mu.Unlock()
}

// SetOther sets the reply to unknown commands.
func (s *Server) SetOther(o OtherReply) {
	s.mu.Lock()
	s.Other = o
	s.mu.Unlock()
}

// Dial connects to the default endpoint of OD-44. It does not use TCP.
func Dial(ctx context.Context) (net.Conn, error) {
	return DialPath(ctx, Path())
}

// A helper used by tests: write a command and read until CR (or timeout).
func Exchange(ctx context.Context, c net.Conn, cmd string, wait time.Duration) ([]byte, error) {
	if err := c.SetDeadline(time.Now().Add(wait)); err != nil {
		return nil, err
	}
	if _, err := c.Write([]byte(cmd + "\r")); err != nil {
		return nil, err
	}
	var out []byte
	tmp := make([]byte, 1)
	for {
		n, err := c.Read(tmp)
		if n == 1 {
			out = append(out, tmp[0])
			if tmp[0] == '\r' {
				_ = c.SetDeadline(time.Time{})
				return out, nil
			}
			if len(out) > 512 {
				_ = c.SetDeadline(time.Time{})
				return out, io.ErrShortBuffer
			}
			continue
		}
		if err != nil {
			_ = c.SetDeadline(time.Time{})
			return out, err
		}
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
	}
}
