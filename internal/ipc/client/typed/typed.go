// SPDX-License-Identifier: AGPL-3.0-or-later

// Package typed adds the IPC client methods with the result types of the
// vltd packages (device, event, measure, policy). The VLT GUI does not use
// this package: it uses internal/ipc/client and passes the JSON results to
// the front end, so that it does not link the Store and SQLite.
package typed

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/event"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/measure"
	"github.com/abuzer1495/voltiovolt/internal/policy"
)

// Client is a client.Client with the typed methods.
type Client struct {
	*client.Client
}

// New returns a Client on an open connection.
func New(c net.Conn) *Client { return &Client{client.New(c)} }

// Dial connects to the vltd endpoint (client.Dial).
func Dial(ctx context.Context, addr string) (*Client, error) {
	c, err := client.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	return &Client{c}, nil
}

func (c *Client) DeviceStatus(ctx context.Context) (r device.Status, err error) {
	err = c.Call(ctx, wire.MethodDeviceStatus, nil, &r)
	return
}

func (c *Client) Measurements(ctx context.Context, name string, from, to time.Time, step string) (unit string, rows []measure.Row, err error) {
	var r wire.MeasurementsResult
	err = c.Call(ctx, wire.MethodMeasurements, wire.MeasurementsParams{Name: name,
		From: from.UTC().Format(time.RFC3339Nano), To: to.UTC().Format(time.RFC3339Nano), Step: step}, &r)
	if err == nil {
		err = json.Unmarshal(r.Rows, &rows)
	}
	return r.Unit, rows, err
}

func (c *Client) Events(ctx context.Context, before int64, limit int) (list []event.Stored, next int64, err error) {
	var r wire.EventsListResult
	err = c.Call(ctx, wire.MethodEventsList, wire.EventsListParams{Before: before, Limit: limit}, &r)
	if err == nil {
		err = json.Unmarshal(r.Events, &list)
	}
	return list, r.Next, err
}

func (c *Client) Policy(ctx context.Context) (r policy.Info, err error) {
	err = c.Call(ctx, wire.MethodPolicyGet, nil, &r)
	return
}

func (c *Client) SetPolicy(ctx context.Context, values map[string]any) (r policy.Info, err error) {
	m, err := client.RawMap(values)
	if err != nil {
		return r, err
	}
	err = c.Call(ctx, wire.MethodPolicySet, wire.SettingsMapParams{Settings: m}, &r)
	return
}

func (c *Client) Simulate(ctx context.Context) (r policy.Result, err error) {
	err = c.Call(ctx, wire.MethodPolicySimulate, nil, &r)
	return
}

func (c *Client) ShutdownStatus(ctx context.Context) (r policy.Countdown, err error) {
	err = c.Call(ctx, wire.MethodShutdownStatus, nil, &r)
	return
}
