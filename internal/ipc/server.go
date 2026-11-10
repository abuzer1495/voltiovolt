// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ipc is the vltd local IPC server (VLT-IS-003 TASK 03-00-00-007).
// Linux: a Unix socket with SO_PEERCRED check. Windows: a named pipe with a
// protected DACL (go-winio). The protocol is in package wire.
//
// Limits: 8 connections or fewer, 20 requests each second or fewer for each
// connection, 1 MiB frames, 30 seconds for a partial frame. No response
// contains a Secret value.
package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/auth"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// Limits (step 4).
const (
	MaxConns        = 8
	RequestsPerSec  = 20
	PartialFrameMax = 30 * time.Second
)

// Backend is what the server uses.
type Backend struct {
	Store   *store.Store
	Vault   *vault.Vault
	Auth    *auth.Manager
	Audit   *audit.Log
	Version string
	Log     *slog.Logger

	// VLT-IS-004 TASK 03-00-00-008. A nil field gives the error
	// "unavailable" for its methods.
	Device DeviceControl
	Policy PolicyControl
	Hub    *Hub
}

// PeerCheck accepts or refuses a new connection and returns the peer
// identity (Linux: "uid:<n>" from SO_PEERCRED; Windows: "sid:<SID>" of the
// client process). The PIN lockout is per actor and peer (VLT-IS-004 QI-2).
type PeerCheck func(net.Conn) (peer string, err error)

// PeerUnknown is the peer identity when Serve has no PeerCheck (tests).
const PeerUnknown = "unknown"

// Server serves IPC connections.
type Server struct {
	b       Backend
	log     *slog.Logger
	start   time.Time
	now     func() time.Time
	partial time.Duration
	rate    float64

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	lns    []net.Listener
	closed bool
	wg     sync.WaitGroup
}

// NewServer returns a server.
func NewServer(b Backend) *Server {
	l := b.Log
	if l == nil {
		l = slog.Default()
	}
	return &Server{b: b, log: l, start: time.Now(), now: time.Now, partial: PartialFrameMax,
		rate: RequestsPerSec, conns: map[net.Conn]struct{}{}}
}

// ErrClosed is returned by Serve after Close.
var ErrClosed = errors.New("ipc: server closed")

// Serve accepts connections on ln until Close. check can be nil.
func (s *Server) Serve(ln net.Listener, check PeerCheck) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		ln.Close()
		return ErrClosed
	}
	s.lns = append(s.lns, ln)
	s.mu.Unlock()
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return ErrClosed
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		peer := PeerUnknown
		if check != nil {
			p, err := check(c)
			if err != nil {
				s.log.Warn("IPC connection refused", "reason", err.Error())
				c.Close()
				continue
			}
			peer = p
		}
		s.mu.Lock()
		if s.closed || len(s.conns) >= MaxConns {
			full := !s.closed
			s.mu.Unlock()
			if full {
				s.log.Warn("IPC connection refused: too many connections", "max", MaxConns)
			}
			c.Close()
			continue
		}
		s.conns[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.handle(c, peer)
	}
}

// Close stops all listeners, closes all connections and waits for the
// connection handlers.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	for _, ln := range s.lns {
		ln.Close()
	}
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return nil
}

// Conns returns the number of open connections.
func (s *Server) Conns() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

type conn struct {
	s       *Server
	c       net.Conn
	peer    string
	session auth.Session
	tokens  float64
	last    time.Time

	wmu   sync.Mutex // one frame at a time (responses and push frames)
	subMu sync.Mutex
	sub   *subscriber
}

func (s *Server) handle(c net.Conn, peer string) {
	defer s.wg.Done()
	cn := &conn{s: s, c: c, peer: peer, tokens: s.rate, last: s.now()}
	defer func() {
		cn.unsubscribe()
		cn.session.Drop()
		c.Close()
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
	}()
	for {
		b, err := cn.readFrame()
		if err != nil {
			if errors.Is(err, wire.ErrFrameTooLarge) || errors.Is(err, wire.ErrEmptyFrame) {
				_ = cn.write(wire.Response{V: wire.Version, Error: &wire.Error{Code: wire.CodeFrameTooLarge, Message: err.Error()}})
			} else if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				s.log.Debug("IPC connection closed", "reason", err.Error())
			}
			return
		}
		resp := cn.serve(b)
		if err := cn.write(resp); err != nil {
			return
		}
	}
}

// readFrame waits for the first byte without a time limit. After the first
// byte the full frame must arrive in s.partial (step 4.3).
func (cn *conn) readFrame() ([]byte, error) {
	var h [4]byte
	if err := cn.c.SetReadDeadline(time.Time{}); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(cn.c, h[:1]); err != nil {
		return nil, err
	}
	if err := cn.c.SetReadDeadline(time.Now().Add(cn.s.partial)); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(cn.c, h[1:]); err != nil {
		return nil, err
	}
	n, err := wire.ReadLength(h)
	if err != nil {
		return nil, err
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(cn.c, b); err != nil {
		return nil, err
	}
	return b, cn.c.SetReadDeadline(time.Time{})
}

func (cn *conn) write(r wire.Response) error {
	cn.wmu.Lock()
	defer cn.wmu.Unlock()
	if err := cn.c.SetWriteDeadline(time.Now().Add(cn.s.partial)); err != nil {
		return err
	}
	return wire.WriteFrame(cn.c, r)
}

// writeRaw writes one encoded JSON value (a push frame) as a frame.
func (cn *conn) writeRaw(b []byte) error {
	cn.wmu.Lock()
	defer cn.wmu.Unlock()
	if err := cn.c.SetWriteDeadline(time.Now().Add(cn.s.partial)); err != nil {
		return err
	}
	return wire.WriteFrame(cn.c, json.RawMessage(b))
}

// allow is a token bucket: 20 requests each second, burst 20.
func (cn *conn) allow() bool {
	now := cn.s.now()
	cn.tokens += now.Sub(cn.last).Seconds() * cn.s.rate
	if cn.tokens > cn.s.rate {
		cn.tokens = cn.s.rate
	}
	cn.last = now
	if cn.tokens < 1 {
		return false
	}
	cn.tokens--
	return true
}

func fail(id int64, code, msg string) wire.Response {
	return wire.Response{V: wire.Version, ID: id, Error: &wire.Error{Code: code, Message: msg}}
}

// Access classes (step 5).
type access int

const (
	open       access = iota // always (also before a PIN is set)
	readOnly                 // after a PIN is set, no elevation
	elevated                 // after a PIN is set, elevated
	setPINRule               // first start: open; after: elevated
)

var methodAccess = map[string]access{
	wire.MethodHello:        open,
	wire.MethodStatus:       open,
	wire.MethodAuthStatus:   open,
	wire.MethodSetPIN:       setPINRule,
	wire.MethodLogin:        readOnly,
	wire.MethodLogout:       readOnly,
	wire.MethodSettingsList: readOnly,
	wire.MethodSecretStatus: readOnly,
	wire.MethodSettingsSet:  elevated,
	wire.MethodSecretSet:    elevated,
	wire.MethodSecretDelete: elevated,
	wire.MethodVaultReset:   elevated,
	wire.MethodAuditList:    elevated,

	// VLT-IS-004 TASK 03-00-00-008 step 2.
	wire.MethodDeviceStatus:      readOnly,
	wire.MethodDeviceSettingsSet: elevated,
	wire.MethodSubscribe:         readOnly,
	wire.MethodUnsubscribe:       readOnly,
	wire.MethodMeasurements:      readOnly,
	wire.MethodEventsList:        readOnly,
	wire.MethodPolicyGet:         readOnly,
	wire.MethodPolicySet:         elevated,
	wire.MethodPolicySimulate:    elevated,
	wire.MethodPolicyActivate:    elevated,
	wire.MethodPolicyDeactivate:  elevated,
	wire.MethodShutdownStatus:    readOnly,
	wire.MethodShutdownCancel:    elevated,
}

func (cn *conn) idle(ctx context.Context) time.Duration {
	st, err := cn.s.b.Store.GetSetting(ctx, store.SettingSessionIdleMinutes)
	var m int
	if err == nil {
		_ = json.Unmarshal(st.Value, &m)
	}
	if m < 1 {
		m = 10
	}
	return time.Duration(m) * time.Minute
}

func (cn *conn) serve(b []byte) wire.Response {
	if !cn.allow() {
		return fail(0, wire.CodeRateLimited, "more than 20 requests in one second")
	}
	req, err := wire.DecodeRequest(b)
	if err != nil {
		var id int64
		if req.ID != nil {
			id = *req.ID
		}
		return fail(id, wire.CodeBadRequest, err.Error())
	}
	id := *req.ID
	acc, ok := methodAccess[req.Method]
	if !ok {
		return fail(id, wire.CodeUnknownMethod, "unknown method")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pinSet, err := cn.s.b.Auth.PINSet(ctx)
	if err != nil {
		cn.s.log.Error("IPC: auth state not available", "err", err)
		return fail(id, wire.CodeInternal, "internal error")
	}
	now := cn.s.now()
	elev := pinSet && cn.session.Elevated(now, cn.idle(ctx))
	switch {
	case acc == open:
	case !pinSet && acc != setPINRule:
		return fail(id, wire.CodePINNotSet, "set a PIN first")
	case acc == elevated && !elev, acc == setPINRule && pinSet && !elev:
		return fail(id, wire.CodeNotElevated, "enter the PIN first")
	}
	if elev {
		cn.session.Touch(now)
	}
	res, werr := cn.dispatch(ctx, req, pinSet, elev)
	if werr != nil {
		return fail(id, werr.Code, werr.Message)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return fail(id, wire.CodeInternal, "internal error")
	}
	return wire.Response{V: wire.Version, ID: id, Result: raw}
}

// source is the lockout source of this connection.
func (cn *conn) source() auth.Source { return auth.Source{Actor: audit.ActorGUI, Peer: cn.peer} }

func bad(err error) *wire.Error { return &wire.Error{Code: wire.CodeBadRequest, Message: err.Error()} }

func invalid(msg string) *wire.Error { return &wire.Error{Code: wire.CodeInvalid, Message: msg} }

func internal() *wire.Error { return &wire.Error{Code: wire.CodeInternal, Message: "internal error"} }

func (cn *conn) record(ctx context.Context, action, target, result string, details map[string]any) {
	if cn.s.b.Audit == nil {
		return
	}
	if err := cn.s.b.Audit.Record(ctx, audit.ActorGUI, action, target, result, details); err != nil {
		cn.s.log.Error("audit entry not written", "action", action, "err", err)
	}
}

func vaultErr(err error) *wire.Error {
	switch {
	case errors.Is(err, vault.ErrBadID):
		return invalid("secret ID is not permitted")
	case errors.Is(err, vault.ErrNotFound):
		return &wire.Error{Code: wire.CodeNotFound, Message: "secret not set"}
	case errors.Is(err, vault.ErrUnavailable), errors.Is(err, vault.ErrNotConfigured):
		return &wire.Error{Code: wire.CodeVaultUnavailable, Message: "vault not available"}
	case errors.Is(err, vault.ErrConfirmation):
		return invalid("confirmation text is not correct")
	}
	return internal()
}

func (cn *conn) dispatch(ctx context.Context, req wire.Request, pinSet, elev bool) (any, *wire.Error) {
	b := cn.s.b
	switch req.Method {
	case wire.MethodHello:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		return wire.HelloResult{Version: b.Version, Protocol: wire.Version}, nil

	case wire.MethodStatus:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		vs := b.Vault.Status()
		res := wire.StatusResult{
			Vault:         wire.VaultState{State: string(vs.State), Reason: vs.Reason, KeyMode: vs.KeyMode},
			Store:         wire.StoreState{State: "ok"},
			UptimeSeconds: int64(cn.s.now().Sub(cn.s.start) / time.Second),
		}
		if v, err := b.Store.SchemaVersion(ctx); err == nil {
			res.Store.SchemaVersion = v
		} else {
			res.Store.State = "error"
		}
		return res, nil

	case wire.MethodAuthStatus:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		st, err := b.Auth.Status(ctx, cn.source())
		if err != nil {
			return nil, internal()
		}
		res := wire.AuthStatusResult{PINSet: st.PINSet, Elevated: elev}
		if st.BlockUntil.After(cn.s.now()) {
			res.BlockUntil = st.BlockUntil.UTC().Format(time.RFC3339)
		}
		return res, nil

	case wire.MethodSetPIN:
		var p wire.PINParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		var err error
		if pinSet {
			err = b.Auth.SetPIN(ctx, audit.ActorGUI, p.PIN)
		} else {
			err = b.Auth.SetInitialPIN(ctx, audit.ActorGUI, p.PIN)
		}
		switch {
		case err == nil:
			cn.session.Drop()
			return wire.OKResult{OK: true}, nil
		case errors.Is(err, auth.ErrPINIsSet):
			return nil, &wire.Error{Code: wire.CodeNotElevated, Message: "a PIN is already set; enter the PIN first"}
		case errors.Is(err, auth.ErrPINWeak), errors.Is(err, auth.ErrPINLength), errors.Is(err, auth.ErrPINChars):
			return nil, invalid(err.Error())
		}
		return nil, internal()

	case wire.MethodLogin:
		var p wire.PINParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		until, err := b.Auth.Login(ctx, cn.source(), p.PIN)
		switch {
		case err == nil:
			cn.session.Elevate(cn.s.now())
			return wire.OKResult{OK: true}, nil
		case errors.Is(err, auth.ErrBlocked):
			return nil, &wire.Error{Code: wire.CodeBlocked, Message: "PIN entry is blocked until " + until.UTC().Format(time.RFC3339)}
		case errors.Is(err, auth.ErrIncorrect):
			msg := "PIN is not correct"
			if !until.IsZero() {
				msg += "; PIN entry is blocked until " + until.UTC().Format(time.RFC3339)
			}
			return nil, &wire.Error{Code: wire.CodePINIncorrect, Message: msg}
		}
		return nil, internal()

	case wire.MethodLogout:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		cn.session.Drop()
		return wire.OKResult{OK: true}, nil

	case wire.MethodSettingsList:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		list, err := b.Store.ListSettings(ctx)
		if err != nil {
			return nil, internal()
		}
		res := wire.SettingsListResult{Settings: make([]wire.Setting, 0, len(list))}
		for _, s := range list {
			res.Settings = append(res.Settings, wire.Setting{Key: s.Key, Value: s.Value, Version: s.Version,
				Default: s.Default, UpdatedAt: s.UpdatedAt, UpdatedBy: s.UpdatedBy})
		}
		return res, nil

	case wire.MethodSettingsSet:
		var p wire.SettingsSetParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if len(p.Value) == 0 {
			return nil, invalid("value is missing")
		}
		v, err := b.Store.SetSetting(ctx, p.Key, p.Value, audit.ActorGUI)
		if err != nil {
			cn.record(ctx, "settings_set", p.Key, "failure", map[string]any{"reason": err.Error()})
			if errors.Is(err, store.ErrVaultUnavailable) {
				return nil, &wire.Error{Code: wire.CodeVaultUnavailable, Message: "vault not available"}
			}
			return nil, invalid(err.Error())
		}
		cn.record(ctx, "settings_set", p.Key, "success", map[string]any{"version": v, "value": p.Value})
		return wire.SettingsSetResult{Version: v}, nil

	case wire.MethodSecretStatus:
		var p wire.SecretIDParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		st, err := b.Vault.SecretStatus(ctx, p.ID)
		if err != nil {
			return nil, vaultErr(err)
		}
		return wire.SecretStatusResult{ID: st.ID, Set: st.Set, Hint: st.Hint, UpdatedAt: st.UpdatedAt}, nil

	case wire.MethodSecretSet:
		var p wire.SecretSetParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if p.Value == "" {
			return nil, invalid("value is empty")
		}
		val := []byte(p.Value)
		p.Value = ""
		err := b.Vault.Set(ctx, p.ID, val)
		clear(val)
		if err != nil {
			cn.record(ctx, "secret_set", p.ID, "failure", map[string]any{"reason": err.Error()})
			return nil, vaultErr(err)
		}
		cn.record(ctx, "secret_set", p.ID, "success", nil)
		return wire.OKResult{OK: true}, nil

	case wire.MethodSecretDelete:
		var p wire.SecretIDParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if err := b.Vault.Delete(ctx, p.ID); err != nil {
			cn.record(ctx, "secret_delete", p.ID, "failure", map[string]any{"reason": err.Error()})
			return nil, vaultErr(err)
		}
		cn.record(ctx, "secret_delete", p.ID, "success", nil)
		return wire.OKResult{OK: true}, nil

	case wire.MethodVaultReset:
		var p wire.VaultResetParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if err := b.Vault.Reset(ctx, p.Confirmation); err != nil {
			return nil, vaultErr(err)
		}
		return wire.OKResult{OK: true}, nil

	case wire.MethodAuditList:
		p := wire.AuditListParams{Limit: 100}
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if p.Limit < 1 || p.Limit > 500 || p.After < 0 {
			return nil, invalid("limit must be 1 to 500 and after must be 0 or more")
		}
		rows, err := b.Audit.List(ctx, p.After, p.Limit)
		if err != nil {
			return nil, internal()
		}
		res := wire.AuditListResult{Rows: make([]wire.AuditRow, 0, len(rows)), Next: p.After}
		for _, r := range rows {
			res.Rows = append(res.Rows, wire.AuditRow{Seq: r.Seq, TS: r.TS, Actor: r.Actor, Action: r.Action,
				Target: r.Target, Result: r.Result, Details: r.Details, PrevHash: r.PrevHash, Hash: r.Hash})
			res.Next = r.Seq
		}
		return res, nil

	default:
		return cn.dispatchLive(ctx, req)
	}
}
