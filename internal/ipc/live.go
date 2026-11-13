// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/event"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/measure"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// DeviceControl is the Device Manager part that the server uses.
type DeviceControl interface {
	Status() device.Status
	Restart()
}

// PortScanner lists the serial ports and scans the marked ports
// (device.Scanner). Scan sends only "Q1" CR to each marked port.
type PortScanner interface {
	Ports(ctx context.Context) ([]transport.SerialPortInfo, error)
	Scan(ctx context.Context, marked []string, so transport.SerialOptions, inUse string) ([]device.ScanResult, error)
}

// PolicyControl is the Policy Service part that the server uses. It has
// no executor call: no IPC method starts a real shutdown (SEC-092).
type PolicyControl interface {
	Info(ctx context.Context) (policy.Info, error)
	Set(ctx context.Context, values map[string]json.RawMessage, actor string) (policy.Info, error)
	Simulate(ctx context.Context, actor string) (policy.Result, error)
	Activate(ctx context.Context, actor string) error
	Deactivate(ctx context.Context, actor string) error
	Cancel(ctx context.Context, actor string) error
	Countdown() policy.Countdown
}

// PushQueue is the number of push frames that wait for one slow client.
// When the queue is full, new push frames for that client are dropped.
const PushQueue = 64

// Hub sends push messages to the subscribed connections.
type Hub struct {
	mu      sync.Mutex
	subs    map[*subscriber]struct{}
	dropped int64
}

type subscriber struct {
	ch   chan []byte
	done chan struct{}
}

// NewHub returns an empty Hub.
func NewHub() *Hub { return &Hub{subs: map[*subscriber]struct{}{}} }

// Publish sends one push message to each subscriber. It does not block.
func (h *Hub) Publish(kind string, data any) error {
	d, err := json.Marshal(data)
	if err != nil {
		return err
	}
	b, err := json.Marshal(wire.Push{V: wire.Version, Push: kind, Data: d})
	if err != nil {
		return err
	}
	if len(b) > wire.MaxFrame {
		return wire.ErrFrameTooLarge
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.ch <- b:
		default:
			h.dropped++
		}
	}
	return nil
}

// Subscribers returns the number of subscribed connections.
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// Dropped returns the number of dropped push frames.
func (h *Hub) Dropped() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dropped
}

func (h *Hub) add() *subscriber {
	s := &subscriber{ch: make(chan []byte, PushQueue), done: make(chan struct{})}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

func (h *Hub) remove(s *subscriber) {
	h.mu.Lock()
	_, ok := h.subs[s]
	delete(h.subs, s)
	h.mu.Unlock()
	if ok {
		close(s.done)
	}
}

// subscribe starts the push writer of the connection.
func (cn *conn) subscribe() *wire.Error {
	h := cn.s.b.Hub
	if h == nil {
		return unavailable("status push")
	}
	cn.subMu.Lock()
	defer cn.subMu.Unlock()
	if cn.sub != nil {
		return nil
	}
	s := h.add()
	cn.sub = s
	go func() {
		for {
			select {
			case b := <-s.ch:
				if err := cn.writeRaw(b); err != nil {
					h.remove(s)
					return
				}
			case <-s.done:
				return
			}
		}
	}()
	return nil
}

func (cn *conn) unsubscribe() {
	cn.subMu.Lock()
	defer cn.subMu.Unlock()
	if cn.sub != nil && cn.s.b.Hub != nil {
		cn.s.b.Hub.remove(cn.sub)
	}
	cn.sub = nil
}

func unavailable(what string) *wire.Error {
	return &wire.Error{Code: wire.CodeUnavailable, Message: what + " is not available"}
}

// deviceSettingKeys are the keys of device.settings.set.
var deviceSettingKeys = map[string]bool{
	store.SettingDeviceTransport: true, store.SettingDeviceSerial: true,
	store.SettingDeviceDTR: true, store.SettingDeviceRTS: true,
	store.SettingDeviceAllowedPorts: true,
}

// checkAll checks all values before a change, so that a bad value changes
// nothing.
func checkAll(values map[string]json.RawMessage) error {
	defs := map[string]store.SettingDef{}
	for _, d := range store.DefaultSettings() {
		defs[d.Key] = d
	}
	for k, v := range values {
		d, ok := defs[k]
		if !ok {
			return errors.New("unknown Setting " + k)
		}
		if d.Check != nil {
			if err := d.Check(v); err != nil {
				return errors.New(k + ": " + err.Error())
			}
		}
	}
	return nil
}

// dispatchLive serves the methods of VLT-IS-004 TASK 03-00-00-008.
func (cn *conn) dispatchLive(ctx context.Context, req wire.Request) (any, *wire.Error) {
	b := cn.s.b
	switch req.Method {
	case wire.MethodDeviceStatus:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		if b.Device == nil {
			return nil, unavailable("device manager")
		}
		// Profile.SerialNumber is "[REDACTED]" (TASK 03-00-00-003).
		return b.Device.Status(), nil

	case wire.MethodDeviceSettingsSet:
		var p wire.SettingsMapParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if b.Device == nil {
			return nil, unavailable("device manager")
		}
		if len(p.Settings) == 0 {
			return nil, invalid("no Setting to change")
		}
		for k := range p.Settings {
			if !deviceSettingKeys[k] {
				return nil, invalid("not a Device Setting: " + k)
			}
		}
		if err := checkAll(p.Settings); err != nil {
			return nil, invalid(err.Error())
		}
		for _, k := range []string{store.SettingDeviceTransport, store.SettingDeviceSerial, store.SettingDeviceDTR, store.SettingDeviceRTS, store.SettingDeviceAllowedPorts} {
			v, ok := p.Settings[k]
			if !ok {
				continue
			}
			ver, err := b.Store.SetSetting(ctx, k, v, audit.ActorGUI)
			if err != nil {
				cn.record(ctx, "settings_set", k, "failure", map[string]any{"reason": err.Error()})
				if errors.Is(err, store.ErrVaultUnavailable) {
					return nil, &wire.Error{Code: wire.CodeVaultUnavailable, Message: "vault not available"}
				}
				return nil, invalid(err.Error())
			}
			cn.record(ctx, "settings_set", k, "success", map[string]any{"version": ver, "value": v})
		}
		b.Device.Restart()
		return wire.OKResult{OK: true}, nil

	case wire.MethodDevicePorts:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		if b.Ports == nil {
			return nil, unavailable("port list")
		}
		l, err := b.Ports.Ports(ctx)
		if err != nil {
			return nil, &wire.Error{Code: wire.CodeInternal, Message: "cannot list the serial ports"}
		}
		out := wire.DevicePortsResult{Ports: make([]wire.SerialPort, 0, len(l))}
		for _, x := range l {
			out.Ports = append(out.Ports, wire.SerialPort{Name: x.Name, IsUSB: x.IsUSB, VID: x.VID, PID: x.PID, Product: x.Product})
		}
		return out, nil

	case wire.MethodDeviceScan:
		var p wire.DeviceScanParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if b.Ports == nil || b.Device == nil {
			return nil, unavailable("port scan")
		}
		if p.Confirmation != wire.ScanConfirmation {
			cn.record(ctx, "device_scan", "serial", "failure", map[string]any{"ports": p.Ports, "reason": "confirmation text"})
			return nil, invalid("confirmation text is not " + wire.ScanConfirmation)
		}
		st := b.Device.Status()
		so := transport.SerialOptions{DTR: st.Config.DTR, RTS: st.Config.RTS}
		if so.DTR == "" {
			so.DTR = transport.LineUnchanged
		}
		if so.RTS == "" {
			so.RTS = transport.LineUnchanged
		}
		inUse := ""
		if st.Profile != nil && st.Profile.Transport == device.TransportSerial {
			inUse = st.Profile.Port
		}
		res, err := b.Ports.Scan(ctx, p.Ports, so, inUse)
		if err != nil {
			cn.record(ctx, "device_scan", "serial", "failure", map[string]any{"ports": p.Ports, "reason": err.Error()})
			if errors.Is(err, device.ErrScanPort) {
				return nil, invalid(err.Error())
			}
			return nil, &wire.Error{Code: wire.CodeInternal, Message: "scan failed"}
		}
		cn.record(ctx, "device_scan", "serial", "success", map[string]any{"ports": p.Ports, "command": device.ScanCommand, "results": res})
		out := wire.DeviceScanResult{Results: make([]wire.ScanPortResult, 0, len(res))}
		for _, r := range res {
			out.Results = append(out.Results, wire.ScanPortResult{Name: r.Name, Result: r.Result, ValidQ1: r.ValidQ1})
		}
		return out, nil

	case wire.MethodSubscribe:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		if werr := cn.subscribe(); werr != nil {
			return nil, werr
		}
		return wire.OKResult{OK: true}, nil

	case wire.MethodUnsubscribe:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		cn.unsubscribe()
		return wire.OKResult{OK: true}, nil

	case wire.MethodMeasurements:
		var p wire.MeasurementsParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		from, err1 := time.Parse(time.RFC3339, p.From)
		to, err2 := time.Parse(time.RFC3339, p.To)
		if err1 != nil || err2 != nil {
			return nil, invalid("from and to must be RFC 3339 times")
		}
		origin := currentOrigin(ctx, b)
		unit, rows, err := measure.QueryByOrigin(ctx, b.Store.DB(), p.Name, from, to, p.Step, origin, p.IncludeDemo)
		if errors.Is(err, measure.ErrBadQuery) {
			return nil, invalid(err.Error())
		}
		if err != nil {
			return nil, internal()
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			return nil, internal()
		}
		return wire.MeasurementsResult{Name: p.Name, Unit: unit, Step: p.Step, Rows: raw}, nil

	case wire.MethodEventsList:
		p := wire.EventsListParams{Limit: 100}
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if p.Limit < 1 || p.Limit > event.MaxPage || p.Before < 0 {
			return nil, invalid("limit must be 1 to 500 and before must be 0 or more")
		}
		list, err := event.ListByOrigin(ctx, b.Store.DB(), p.Before, p.Limit, event.OriginFilter{Current: currentOrigin(ctx, b), IncludeOthers: p.IncludeDemo})
		if err != nil {
			return nil, internal()
		}
		raw, err := json.Marshal(list)
		if err != nil {
			return nil, internal()
		}
		res := wire.EventsListResult{Events: raw}
		if len(list) == p.Limit {
			res.Next = list[len(list)-1].ID
		}
		return res, nil

	case wire.MethodPolicyGet, wire.MethodPolicySimulate, wire.MethodPolicyActivate, wire.MethodPolicyDeactivate,
		wire.MethodShutdownStatus, wire.MethodShutdownCancel:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		if b.Policy == nil {
			return nil, unavailable("policy")
		}
		return cn.policyMethod(ctx, req.Method)

	case wire.MethodPolicySet:
		var p wire.SettingsMapParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if b.Policy == nil {
			return nil, unavailable("policy")
		}
		in, err := b.Policy.Set(ctx, p.Settings, audit.ActorGUI)
		if err != nil {
			cn.record(ctx, "policy_set", "policy", "failure", map[string]any{"reason": err.Error()})
			if errors.Is(err, policy.ErrVaultLocked) {
				return nil, vaultLocked()
			}
			if errors.Is(err, store.ErrVaultUnavailable) {
				return nil, &wire.Error{Code: wire.CodeVaultUnavailable, Message: "vault not available"}
			}
			if errors.Is(err, policy.ErrNotPolicyKey) || strings.HasPrefix(err.Error(), "policy:") {
				return nil, invalid(err.Error())
			}
			return nil, internal()
		}
		return in, nil
	case wire.MethodDemoPowerFail:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		if b.Demo == nil {
			return nil, unavailable("demo")
		}
		if !b.Demo.Active() {
			return nil, &wire.Error{Code: wire.CodeUnavailable, Message: "demo mode is not active"}
		}
		b.Demo.PowerFail(0)
		return wire.OKResult{OK: true}, nil
	}
	return nil, &wire.Error{Code: wire.CodeUnknownMethod, Message: "unknown method"}
}

func vaultLocked() *wire.Error {
	return &wire.Error{Code: wire.CodeVaultLocked, Message: "the Vault is locked; the Fallback Policy is in use"}
}

func (cn *conn) policyMethod(ctx context.Context, method string) (any, *wire.Error) {
	pc := cn.s.b.Policy
	switch method {
	case wire.MethodPolicyGet:
		in, err := pc.Info(ctx)
		if err != nil {
			return nil, internal()
		}
		return in, nil
	case wire.MethodPolicySimulate:
		r, err := pc.Simulate(ctx, audit.ActorGUI)
		if err != nil {
			return nil, internal()
		}
		return r, nil
	case wire.MethodPolicyActivate:
		err := pc.Activate(ctx, audit.ActorGUI)
		if errors.Is(err, policy.ErrVaultLocked) {
			return nil, vaultLocked()
		}
		if errors.Is(err, policy.ErrNoSimulation) {
			return nil, &wire.Error{Code: wire.CodeNoSimulation, Message: "no passed Simulation of the current policy version"}
		}
		if err != nil {
			return nil, internal()
		}
		return wire.OKResult{OK: true}, nil
	case wire.MethodPolicyDeactivate:
		if err := pc.Deactivate(ctx, audit.ActorGUI); err != nil {
			if errors.Is(err, policy.ErrVaultLocked) {
				return nil, vaultLocked()
			}
			return nil, internal()
		}
		return wire.OKResult{OK: true}, nil
	case wire.MethodShutdownStatus:
		return pc.Countdown(), nil
	case wire.MethodShutdownCancel:
		err := pc.Cancel(ctx, audit.ActorGUI)
		switch {
		case errors.Is(err, policy.ErrLowBattery):
			return nil, &wire.Error{Code: wire.CodeLowBattery, Message: "LOW_BATTERY is active; the countdown cannot be cancelled"}
		case errors.Is(err, policy.ErrNoCountdown):
			return nil, &wire.Error{Code: wire.CodeNotFound, Message: "no countdown"}
		case err != nil:
			return nil, internal()
		}
		return wire.OKResult{OK: true}, nil
	}
	return nil, internal()
}

// currentOrigin is the origin of the selected Transport (OD-48).
func currentOrigin(ctx context.Context, b Backend) string {
	if b.Device != nil {
		return device.OriginOf(b.Device.Status().Config.Transport)
	}
	if b.Store != nil {
		cfg, err := device.ConfigFromStore(ctx, b.Store)
		if err == nil {
			return device.OriginOf(cfg.Transport)
		}
	}
	return "real"
}
