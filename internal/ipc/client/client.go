// SPDX-License-Identifier: AGPL-3.0-or-later

// Package client is the vltd IPC client (VLT-IS-003 TASK 03-00-00-007).
// The GUI uses it. One Client is one connection; the elevated state after
// Login is for this connection only.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// Client is one IPC connection. It is safe for concurrent use; requests
// are sent one at a time. A reader goroutine reads all frames: responses
// go to the waiting call, push frames go to Pushes.
type Client struct {
	callMu sync.Mutex // one request at a time
	c      net.Conn
	nextID int64

	mu      sync.Mutex
	waiting chan wire.Frame
	done    chan struct{}
	err     error
	pushes  chan wire.Push
	dropped int64
}

// PushBuffer is the number of push messages that the Client keeps. When
// the buffer is full, new push messages are dropped.
const PushBuffer = 256

// New returns a Client on an open connection.
func New(c net.Conn) *Client {
	cl := &Client{c: c, done: make(chan struct{}), pushes: make(chan wire.Push, PushBuffer)}
	go cl.read()
	return cl
}

// Dial connects to the vltd endpoint. An empty address gives the default
// (Linux: /run/vltd/vltd.sock, Windows: \\.\pipe\vltd).
func Dial(ctx context.Context, addr string) (*Client, error) {
	c, err := dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	return New(c), nil
}

// Close closes the connection.
func (c *Client) Close() error { return c.c.Close() }

// Pushes returns the push messages (after status.subscribe). The channel
// is closed when the connection ends.
func (c *Client) Pushes() <-chan wire.Push { return c.pushes }

// Dropped returns the number of push messages that were dropped.
func (c *Client) Dropped() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropped
}

// ErrProtocol is returned for a response that is not valid.
var ErrProtocol = errors.New("ipc client: response is not valid")

func (c *Client) read() {
	defer close(c.pushes)
	for {
		b, err := wire.ReadFrame(c.c)
		if err != nil {
			c.mu.Lock()
			c.err = err
			c.mu.Unlock()
			close(c.done)
			return
		}
		var f wire.Frame
		if err := json.Unmarshal(b, &f); err != nil {
			f = wire.Frame{V: -1}
		}
		if f.Push != "" && f.Error == nil && f.Result == nil {
			select {
			case c.pushes <- wire.Push{V: f.V, Push: f.Push, Data: f.Data}:
			default:
				c.mu.Lock()
				c.dropped++
				c.mu.Unlock()
			}
			continue
		}
		c.mu.Lock()
		w := c.waiting
		c.waiting = nil
		c.mu.Unlock()
		if w != nil {
			w <- f
		}
	}
}

// Call sends one request and decodes the result into out (can be nil). An
// error response gives a *wire.Error.
func (c *Client) Call(ctx context.Context, method string, params, out any) error {
	c.callMu.Lock()
	defer c.callMu.Unlock()
	if params == nil {
		params = wire.NoParams{}
	}
	p, err := json.Marshal(params)
	if err != nil {
		return err
	}
	c.nextID++
	id := c.nextID
	dl, ok := ctx.Deadline()
	if !ok {
		dl = time.Now().Add(60 * time.Second)
	}
	ch := make(chan wire.Frame, 1)
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return err
	}
	c.waiting = ch
	c.mu.Unlock()
	if err := c.c.SetWriteDeadline(dl); err != nil {
		return err
	}
	if err := wire.WriteFrame(c.c, wire.Request{V: wire.Version, ID: &id, Method: method, Params: p}); err != nil {
		return err
	}
	timer := time.NewTimer(time.Until(dl))
	defer timer.Stop()
	var r wire.Frame
	select {
	case r = <-ch:
	case <-c.done:
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		return err
	case <-timer.C:
		// The connection state is not known now; close it.
		c.c.Close()
		return fmt.Errorf("ipc client: %s: %w", method, os.ErrDeadlineExceeded)
	case <-ctx.Done():
		c.c.Close()
		return ctx.Err()
	}
	if r.V == -1 {
		return ErrProtocol
	}
	if r.Error != nil {
		return r.Error
	}
	if r.V != wire.Version || r.ID != id {
		return ErrProtocol
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

// Code returns the error code of an IPC error response, or "".
func Code(err error) string {
	var e *wire.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func (c *Client) Hello(ctx context.Context) (r wire.HelloResult, err error) {
	err = c.Call(ctx, wire.MethodHello, nil, &r)
	return
}

func (c *Client) Status(ctx context.Context) (r wire.StatusResult, err error) {
	err = c.Call(ctx, wire.MethodStatus, nil, &r)
	return
}

func (c *Client) AuthStatus(ctx context.Context) (r wire.AuthStatusResult, err error) {
	err = c.Call(ctx, wire.MethodAuthStatus, nil, &r)
	return
}

func (c *Client) SetPIN(ctx context.Context, pin string) error {
	return c.Call(ctx, wire.MethodSetPIN, wire.PINParams{PIN: pin}, nil)
}

func (c *Client) RemovePIN(ctx context.Context, pin string) error {
	return c.Call(ctx, wire.MethodRemovePIN, wire.PINParams{PIN: pin}, nil)
}

func (c *Client) Login(ctx context.Context, pin string) error {
	return c.Call(ctx, wire.MethodLogin, wire.PINParams{PIN: pin}, nil)
}

func (c *Client) Logout(ctx context.Context) error {
	return c.Call(ctx, wire.MethodLogout, nil, nil)
}

func (c *Client) ForgotPIN(ctx context.Context, confirmation string) error {
	return c.Call(ctx, wire.MethodForgotPIN, wire.ForgotPINParams{Confirmation: confirmation}, nil)
}

func (c *Client) Settings(ctx context.Context) ([]wire.Setting, error) {
	var r wire.SettingsListResult
	err := c.Call(ctx, wire.MethodSettingsList, nil, &r)
	return r.Settings, err
}

func (c *Client) SetSetting(ctx context.Context, key string, value any) (int64, error) {
	v, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	var r wire.SettingsSetResult
	err = c.Call(ctx, wire.MethodSettingsSet, wire.SettingsSetParams{Key: key, Value: v}, &r)
	return r.Version, err
}

func (c *Client) SecretStatus(ctx context.Context, id string) (r wire.SecretStatusResult, err error) {
	err = c.Call(ctx, wire.MethodSecretStatus, wire.SecretIDParams{ID: id}, &r)
	return
}

func (c *Client) SetSecret(ctx context.Context, id, value string) error {
	return c.Call(ctx, wire.MethodSecretSet, wire.SecretSetParams{ID: id, Value: value}, nil)
}

func (c *Client) DeleteSecret(ctx context.Context, id string) error {
	return c.Call(ctx, wire.MethodSecretDelete, wire.SecretIDParams{ID: id}, nil)
}

func (c *Client) ResetVault(ctx context.Context, confirmation string) error {
	return c.Call(ctx, wire.MethodVaultReset, wire.VaultResetParams{Confirmation: confirmation}, nil)
}

func (c *Client) Audit(ctx context.Context, after int64, limit int) (r wire.AuditListResult, err error) {
	err = c.Call(ctx, wire.MethodAuditList, wire.AuditListParams{After: after, Limit: limit}, &r)
	return
}

// VLT-IS-004 TASK 03-00-00-008 methods. The methods with the result types
// of vltd packages are in internal/ipc/client/typed, so that this package
// does not link the Store and SQLite (VLT-IS-005 TASK 03-00-00-002 step 4.1).

func (c *Client) SetDeviceSettings(ctx context.Context, values map[string]any) error {
	m, err := RawMap(values)
	if err != nil {
		return err
	}
	return c.Call(ctx, wire.MethodDeviceSettingsSet, wire.SettingsMapParams{Settings: m}, nil)
}

func (c *Client) Subscribe(ctx context.Context) error {
	return c.Call(ctx, wire.MethodSubscribe, nil, nil)
}

func (c *Client) Unsubscribe(ctx context.Context) error {
	return c.Call(ctx, wire.MethodUnsubscribe, nil, nil)
}

func (c *Client) ActivatePolicy(ctx context.Context) error {
	return c.Call(ctx, wire.MethodPolicyActivate, nil, nil)
}

func (c *Client) DeactivatePolicy(ctx context.Context) error {
	return c.Call(ctx, wire.MethodPolicyDeactivate, nil, nil)
}

func (c *Client) CancelShutdown(ctx context.Context) error {
	return c.Call(ctx, wire.MethodShutdownCancel, nil, nil)
}

// RawMap marshals each value of a settings map.
func RawMap(values map[string]any) (map[string]json.RawMessage, error) {
	m := make(map[string]json.RawMessage, len(values))
	for k, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		m[k] = b
	}
	return m, nil
}
