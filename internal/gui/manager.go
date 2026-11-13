// SPDX-License-Identifier: AGPL-3.0-or-later

// Package gui is the Go side of the VLT GUI (VLT-IS-005 TASK 03-00-00-002
// to 008). It uses internal/ipc/client only: it does not open the Store,
// the Vault or SQLite. The front end calls the methods of API through the
// Wails bindings (cmd/vlt-gui) and gets the push messages of vltd as Wails
// events. This package does not import Wails, so that it builds and tests
// with CGO_ENABLED=0.
//
// A close of the VLT GUI closes the IPC connection only. It never stops
// vltd.
package gui

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// Wails event names (push messages of vltd and GUI state).
const (
	EventConn      = "vlt:conn"      // ConnInfo
	EventAuth      = "vlt:auth"      // AuthInfo
	EventStatus    = "vlt:status"    // the status push of vltd
	EventEvent     = "vlt:event"     // one Event push of vltd
	EventCountdown = "vlt:countdown" // the countdown push of vltd
	EventTray      = "vlt:tray"      // TrayState
)

// Connection states.
const (
	ConnConnecting   = "connecting"
	ConnConnected    = "connected"
	ConnDisconnected = "disconnected"
)

// Connection failure reasons. The front end shows a text for each.
const (
	ReasonAccessDenied = "access_denied"
	ReasonNotRunning   = "not_running"
	ReasonProtocol     = "protocol"
	ReasonLost         = "connection_lost"
	ReasonOther        = "error"
)

// RetryInterval is the time between two connection attempts (step 1.3 of
// TASK 03-00-00-003).
const RetryInterval = 5 * time.Second

// lockMargin is the time after the end of the idle time before the GUI
// asks vltd for the auth state. A request while elevated starts the idle
// time again, so the GUI does not ask before the end.
const lockMargin = 2 * time.Second

// maxElevated is the maximum elevated time of vltd (VLT-IS-003).
const maxElevated = 8 * time.Hour

// Emitter sends a Wails event to the front end.
type Emitter interface {
	Emit(name string, data any)
}

// Dialer opens an IPC connection.
type Dialer func(ctx context.Context, addr string) (*client.Client, error)

// Mode values for AboutInfo.Mode (VLT-IS-008).
const (
	ModeService  = "service"
	ModePortable = "portable"
)

// Autostart turns the start-at-logon shortcut on or off (OD-39). The
// implementation writes only to the startup folder of the user.
type Autostart interface {
	Enabled() (bool, error)
	Set(on bool) error
}

// ConnInfo is the connection state.
type ConnInfo struct {
	State    string    `json:"state"`
	Reason   string    `json:"reason,omitempty"`
	Version  string    `json:"vltd_version,omitempty"`
	Protocol int       `json:"protocol,omitempty"`
	Since    time.Time `json:"since"`
	NextTry  time.Time `json:"next_try,omitzero"`
}

// AuthInfo is the PIN and elevated state.
type AuthInfo struct {
	PINSet     bool   `json:"pin_set"`
	Elevated   bool   `json:"elevated"`
	BlockUntil string `json:"block_until,omitempty"`
}

// Manager keeps the IPC connection to vltd.
type Manager struct {
	addr  string
	dial  Dialer
	emit  Emitter
	log   *slog.Logger
	now   func() time.Time
	retry time.Duration

	mu        sync.Mutex
	c         *client.Client
	conn      ConnInfo
	auth      AuthInfo
	last      map[string]json.RawMessage
	critical  map[string]bool
	lastReq   time.Time
	elevAt    time.Time
	idle      time.Duration
	lockT     *time.Timer
	onTray    func(TrayState)
	texts     TrayTexts
	openURL   func(string) error
	onNotify  func(OSNotification)
	mode      string
	autostart Autostart
	dataDir   string
}

// OSNotification is a desktop Notification of vltd (VLT-IS-007 TASK
// 03-00-00-004). The VLT GUI shows it as an OS notification.
type OSNotification struct {
	Title    string    `json:"subject"`
	Body     string    `json:"text"`
	Severity string    `json:"severity"`
	Type     string    `json:"type"`
	Time     time.Time `json:"time"`
}

// OnNotify sets the function that shows a desktop Notification.
func (m *Manager) OnNotify(f func(OSNotification)) {
	m.mu.Lock()
	m.onNotify = f
	m.mu.Unlock()
}

// NewManager returns a Manager. An empty addr is the default endpoint.
func NewManager(addr string, emit Emitter, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{addr: addr, dial: client.Dial, emit: emit, log: log, now: time.Now, retry: RetryInterval,
		conn: ConnInfo{State: ConnConnecting, Since: time.Now().UTC()}, last: map[string]json.RawMessage{}, critical: map[string]bool{}, mode: ModeService}
}

// SetDial sets the function that opens the IPC connection. Portable mode
// gives an in-process net.Pipe dial (VLT-IS-008 TASK 03-00-00-001 step 3).
func (m *Manager) SetDial(d Dialer) {
	if d != nil {
		m.dial = d
	}
}

// SetMode sets the application mode ("portable" or "service").
func (m *Manager) SetMode(mode string) {
	m.mu.Lock()
	m.mode = mode
	m.mu.Unlock()
}

// Mode returns the application mode.
func (m *Manager) Mode() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mode == "" {
		return ModeService
	}
	return m.mode
}

// SetAutostart sets the start-at-logon helper. Nil: the methods return
// "unavailable".
func (m *Manager) SetAutostart(a Autostart) {
	m.mu.Lock()
	m.autostart = a
	m.mu.Unlock()
}

// SetDataDir sets the data folder path shown on the General screen
// (VLT-IS-008 TASK 03-00-00-002 step 1).
func (m *Manager) SetDataDir(dir string) {
	m.mu.Lock()
	m.dataDir = dir
	m.mu.Unlock()
}

// DataDir returns the data folder path.
func (m *Manager) DataDir() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dataDir
}

// OnTray sets the function that gets each new tray state.
func (m *Manager) OnTray(f func(TrayState)) {
	m.mu.Lock()
	m.onTray = f
	m.mu.Unlock()
}

// Run connects to vltd and connects again after RetryInterval when the
// connection fails or ends. It returns when ctx is done.
func (m *Manager) Run(ctx context.Context) {
	for {
		c, err := m.connect(ctx)
		if err == nil {
			m.serve(ctx, c)
		}
		if ctx.Err() != nil {
			return
		}
		m.mu.Lock()
		m.conn.NextTry = m.now().Add(m.retry).UTC()
		ci := m.conn
		m.mu.Unlock()
		m.send(EventConn, ci)
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.retry):
		}
	}
}

// Close closes the IPC connection. It does not stop vltd.
func (m *Manager) Close() {
	m.mu.Lock()
	c := m.c
	m.c = nil
	if m.lockT != nil {
		m.lockT.Stop()
	}
	m.mu.Unlock()
	if c != nil {
		c.Close()
	}
}

// Reason returns the failure reason of a dial or a hello error.
func Reason(err error) string {
	var we *wire.Error
	switch {
	case err == nil:
		return ""
	case errors.Is(err, fs.ErrPermission), errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return ReasonAccessDenied
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ENOENT):
		return ReasonNotRunning
	case errors.As(err, &we), errors.Is(err, client.ErrProtocol):
		return ReasonProtocol
	case strings.Contains(strings.ToLower(err.Error()), "access is denied"):
		return ReasonAccessDenied
	}
	return ReasonOther
}

func (m *Manager) setConn(ci ConnInfo) {
	ci.Since = m.now().UTC()
	m.mu.Lock()
	m.conn = ci
	m.mu.Unlock()
	m.send(EventConn, ci)
	m.updateTray()
}

func (m *Manager) connect(ctx context.Context) (*client.Client, error) {
	m.setConn(ConnInfo{State: ConnConnecting})
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c, err := m.dial(dctx, m.addr)
	if err != nil {
		m.log.Info("gui: no connection to vltd", "err", err)
		m.setConn(ConnInfo{State: ConnDisconnected, Reason: Reason(err)})
		return nil, err
	}
	h, err := c.Hello(dctx)
	if err == nil && h.Protocol != wire.Version {
		err = client.ErrProtocol
	}
	if err != nil {
		c.Close()
		m.log.Info("gui: system.hello failed", "err", err)
		m.setConn(ConnInfo{State: ConnDisconnected, Reason: Reason(err)})
		return nil, err
	}
	m.mu.Lock()
	m.c = c
	m.last = map[string]json.RawMessage{}
	m.critical = map[string]bool{}
	m.auth = AuthInfo{}
	m.mu.Unlock()
	m.setConn(ConnInfo{State: ConnConnected, Version: h.Version, Protocol: h.Protocol})
	m.refreshAuth(dctx)
	m.subscribe(dctx)
	return c, nil
}

// subscribe calls status.subscribe. Before a PIN is set vltd refuses it
// (pin_not_set); SetPIN calls it again.
func (m *Manager) subscribe(ctx context.Context) {
	c := m.client()
	if c == nil {
		return
	}
	if err := c.Subscribe(ctx); err != nil && client.Code(err) != wire.CodePINNotSet {
		m.log.Warn("gui: status.subscribe failed", "err", err)
	}
}

func (m *Manager) serve(ctx context.Context, c *client.Client) {
	pushes := c.Pushes()
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-pushes:
			if !ok {
				m.mu.Lock()
				if m.c == c {
					m.c = nil
				}
				m.auth.Elevated = false
				m.mu.Unlock()
				m.setConn(ConnInfo{State: ConnDisconnected, Reason: ReasonLost})
				return
			}
			m.onPush(p)
		}
	}
}

func (m *Manager) onPush(p wire.Push) {
	if p.Push == wire.PushNotification {
		var n OSNotification
		m.mu.Lock()
		f := m.onNotify
		m.mu.Unlock()
		if f != nil && json.Unmarshal(p.Data, &n) == nil && n.Title != "" {
			f(n)
		}
		return
	}
	m.mu.Lock()
	m.last[p.Push] = p.Data
	if p.Push == wire.PushEvent {
		var e struct {
			Type     string         `json:"type"`
			Severity string         `json:"severity"`
			Data     map[string]any `json:"data"`
		}
		if json.Unmarshal(p.Data, &e) == nil {
			active, _ := e.Data["active"].(bool)
			switch {
			case e.Severity == "CRITICAL" && active:
				m.critical[e.Type] = true
			case !active:
				delete(m.critical, e.Type)
			}
		}
	}
	m.mu.Unlock()
	switch p.Push {
	case wire.PushStatus:
		m.send(EventStatus, p.Data)
	case wire.PushEvent:
		m.send(EventEvent, p.Data)
	case wire.PushCountdown:
		m.send(EventCountdown, p.Data)
	}
	m.updateTray()
}

func (m *Manager) send(name string, data any) {
	m.mu.Lock()
	e := m.emit
	m.mu.Unlock()
	if e != nil {
		e.Emit(name, data)
	}
}

func (m *Manager) client() *client.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.c
}

// ErrNotConnected is returned when there is no connection to vltd.
var ErrNotConnected = errors.New("not_connected")

// call runs f on the connection. It records the request time for the lock
// timer and maps a "not_elevated" answer to the locked state.
func (m *Manager) call(ctx context.Context, f func(*client.Client) error) error {
	c := m.client()
	if c == nil {
		return ErrNotConnected
	}
	m.mu.Lock()
	m.lastReq = m.now()
	m.mu.Unlock()
	err := f(c)
	if client.Code(err) == wire.CodeNotElevated {
		m.setElevated(false)
	}
	m.armLock()
	return err
}

func (m *Manager) setElevated(on bool) {
	m.mu.Lock()
	changed := m.auth.Elevated != on
	m.auth.Elevated = on
	if on {
		m.elevAt = m.now()
	}
	a := m.auth
	m.mu.Unlock()
	if changed {
		m.send(EventAuth, a)
	}
}

// armLock starts the timer for the end of the idle time.
func (m *Manager) armLock() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lockT != nil {
		m.lockT.Stop()
		m.lockT = nil
	}
	if !m.auth.Elevated || m.idle <= 0 {
		return
	}
	end := m.lastReq.Add(m.idle)
	if hard := m.elevAt.Add(maxElevated); hard.Before(end) {
		end = hard
	}
	m.lockT = time.AfterFunc(end.Sub(m.now())+lockMargin, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		m.refreshAuth(ctx)
	})
}

// refreshAuth reads auth.status and the idle time Setting.
func (m *Manager) refreshAuth(ctx context.Context) {
	c := m.client()
	if c == nil {
		return
	}
	t := m.now()
	st, err := c.AuthStatus(ctx)
	if err != nil {
		return
	}
	m.mu.Lock()
	if st.Elevated {
		// auth.status while elevated starts the idle time of vltd again.
		m.lastReq = t
	}
	was := m.auth
	m.auth = AuthInfo{PINSet: st.PINSet, Elevated: st.Elevated, BlockUntil: st.BlockUntil}
	if st.Elevated && !was.Elevated {
		m.elevAt = m.now()
	}
	a := m.auth
	m.mu.Unlock()
	if a != was {
		m.send(EventAuth, a)
	}
	m.armLock()
}

// readIdle reads auth.session_idle_minutes.
func (m *Manager) readIdle(ctx context.Context, c *client.Client) {
	list, err := c.Settings(ctx)
	if err != nil {
		return
	}
	for _, s := range list {
		if s.Key == "auth.session_idle_minutes" {
			var n int
			if json.Unmarshal(s.Value, &n) == nil && n > 0 {
				m.mu.Lock()
				m.idle = time.Duration(n) * time.Minute
				m.mu.Unlock()
			}
		}
	}
}

// SetEmitter sets the Wails event emitter (after the Wails application is
// made).
func (m *Manager) SetEmitter(e Emitter) {
	m.mu.Lock()
	m.emit = e
	m.mu.Unlock()
}
