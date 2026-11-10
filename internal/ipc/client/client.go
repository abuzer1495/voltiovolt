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
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// Client is one IPC connection. It is safe for concurrent use; requests
// are sent one at a time.
type Client struct {
	mu     sync.Mutex
	c      net.Conn
	nextID int64
}

// New returns a Client on an open connection.
func New(c net.Conn) *Client { return &Client{c: c} }

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

// ErrProtocol is returned for a response that is not valid.
var ErrProtocol = errors.New("ipc client: response is not valid")

// Call sends one request and decodes the result into out (can be nil). An
// error response gives a *wire.Error.
func (c *Client) Call(ctx context.Context, method string, params, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	if err := c.c.SetDeadline(dl); err != nil {
		return err
	}
	if err := wire.WriteFrame(c.c, wire.Request{V: wire.Version, ID: &id, Method: method, Params: p}); err != nil {
		return err
	}
	b, err := wire.ReadFrame(c.c)
	if err != nil {
		return err
	}
	var r wire.Response
	if err := json.Unmarshal(b, &r); err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
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

func (c *Client) Login(ctx context.Context, pin string) error {
	return c.Call(ctx, wire.MethodLogin, wire.PINParams{PIN: pin}, nil)
}

func (c *Client) Logout(ctx context.Context) error {
	return c.Call(ctx, wire.MethodLogout, nil, nil)
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
