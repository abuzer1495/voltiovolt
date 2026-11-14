// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/measure"
	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// liveMethods calls each method of VLT-IS-004 TASK 03-00-00-008 in the
// elevated state (part of TestAllMethodsNoSecretInResponses, step 4).
func liveMethods(t *testing.T, ctx context.Context, e *env, c *typed.Client, secret string) {
	t.Helper()
	now := time.Now().UTC()
	db := e.st.DB()
	if _, err := db.ExecContext(ctx, "INSERT INTO events (ts, type, severity, data) VALUES (?, 'ON_BATTERY', 'WARNING', '{\"active\":true}')", now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	w := measure.Writer{DB: db}
	if err := w.WriteCycle(ctx, []measure.Sample{{Time: now.Add(-time.Second), Points: []measure.Point{{Name: measure.InputVoltage, Unit: "V", Value: 230, Source: "Q1"}}}}); err != nil {
		t.Fatal(err)
	}

	ds, err := c.DeviceStatus(ctx)
	if err != nil || ds.State != "connected" || ds.Profile == nil || ds.Profile.SerialNumber != redact.Redacted {
		t.Fatalf("device.status: %+v %v", ds, err)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{store.SettingDeviceTransport: "serial", store.SettingDeviceDTR: "on"}); err != nil {
		t.Fatalf("device.settings.set: %v", err)
	}
	if e.dev.restarts != 1 {
		t.Fatalf("restarts %d", e.dev.restarts)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{store.SettingDeviceTransport: "usb"}); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("bad transport: %v", err)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{store.SettingDeviceDTR: true}); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("old DTR type: %v", err)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{store.SettingDeviceAllowedPorts: []string{"COM3"}}); err != nil {
		t.Fatalf("allowed ports: %v", err)
	}
	deviceScan(t, ctx, e, c)
	upsMethods(t, ctx, e, c)
	if err := c.SetDeviceSettings(ctx, map[string]any{store.SettingSessionIdleMinutes: 5}); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("not a Device Setting: %v", err)
	}
	unit, rows, err := c.Measurements(ctx, measure.InputVoltage, now.Add(-time.Hour), now.Add(time.Hour), measure.StepRaw)
	if err != nil || unit != "V" || len(rows) != 1 || rows[0].Mean != 230 {
		t.Fatalf("measurements.query: %s %+v %v", unit, rows, err)
	}
	if _, _, err := c.Measurements(ctx, "secret.value", now.Add(-time.Hour), now, measure.StepRaw); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("unknown name: %v", err)
	}
	ev, _, err := c.Events(ctx, 0, 10)
	if err != nil || len(ev) != 1 || ev[0].Type != "ON_BATTERY" {
		t.Fatalf("events.list: %+v %v", ev, err)
	}
	if _, _, err := c.Events(ctx, 0, 501); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("events.list limit: %v", err)
	}

	in, err := c.Policy(ctx)
	if err != nil || in.Active || in.Version == "" {
		t.Fatalf("policy.get: %+v %v", in, err)
	}
	if err := c.ActivatePolicy(ctx); client.Code(err) != wire.CodeNoSimulation {
		t.Fatalf("activate without Simulation: %v", err)
	}
	in2, err := c.SetPolicy(ctx, map[string]any{"countdown_seconds": 20})
	if err != nil || in2.Version == in.Version || in2.Active {
		t.Fatalf("policy.set: %+v %v", in2, err)
	}
	if _, err := c.SetPolicy(ctx, map[string]any{"countdown_seconds": 5}); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("policy.set out of range: %v", err)
	}
	r, err := c.Simulate(ctx)
	if err != nil || !r.Passed || r.Version != in2.Version {
		t.Fatalf("policy.simulate: %+v %v", r, err)
	}
	if err := c.ActivatePolicy(ctx); err != nil {
		t.Fatalf("policy.activate: %v", err)
	}
	if in3, err := c.Policy(ctx); err != nil || !in3.Active || in3.LastSimulation == nil {
		t.Fatalf("policy.get after activate: %+v %v", in3, err)
	}
	cd, err := c.ShutdownStatus(ctx)
	if err != nil || cd.Active {
		t.Fatalf("shutdown.status: %+v %v", cd, err)
	}
	if err := c.CancelShutdown(ctx); client.Code(err) != wire.CodeNotFound {
		t.Fatalf("shutdown.cancel without a countdown: %v", err)
	}
	if err := c.DeactivatePolicy(ctx); err != nil {
		t.Fatalf("policy.deactivate: %v", err)
	}
	// The Fallback Policy in the Vault state "locked" (VLT-IS-005).
	e.pol.SetVaultState(func() string { return "locked" })
	if fb, err := c.Policy(ctx); err != nil || !fb.Fallback || !fb.Active {
		t.Fatalf("policy.get fallback: %+v %v", fb, err)
	}
	if _, err := c.SetPolicy(ctx, map[string]any{"countdown_seconds": 90}); client.Code(err) != wire.CodeVaultLocked {
		t.Fatalf("policy.set locked: %v", err)
	}
	if err := c.ActivatePolicy(ctx); client.Code(err) != wire.CodeVaultLocked {
		t.Fatalf("policy.activate locked: %v", err)
	}
	if err := c.DeactivatePolicy(ctx); client.Code(err) != wire.CodeVaultLocked {
		t.Fatalf("policy.deactivate locked: %v", err)
	}
	e.pol.SetVaultState(nil)
	if fb, err := c.Policy(ctx); err != nil || fb.Fallback {
		t.Fatalf("policy.get after fallback: %+v %v", fb, err)
	}

	// Push messages only after status.subscribe.
	if err := e.hub.Publish(wire.PushStatus, map[string]any{"n": 0}); err != nil {
		t.Fatal(err)
	}
	if err := c.Subscribe(ctx); err != nil {
		t.Fatal(err)
	}
	waitSubs(t, e.hub, 1)
	_ = e.hub.Publish(wire.PushStatus, map[string]any{"n": 1, "device": e.dev.Status()})
	p := nextPush(t, c)
	if p.Push != wire.PushStatus || !strings.Contains(string(p.Data), `"n":1`) {
		t.Fatalf("push %s %s", p.Push, p.Data)
	}
	// A request between push frames still works.
	if _, err := c.ShutdownStatus(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Unsubscribe(ctx); err != nil {
		t.Fatal(err)
	}
	waitSubs(t, e.hub, 0)
	if len(e.dry.Calls()) != 0 {
		t.Fatalf("an IPC method called the executor: %+v", e.dry.Calls())
	}
	_ = secret
}

func waitSubs(t *testing.T, h *Hub, n int) {
	t.Helper()
	for i := 0; i < 200 && h.Subscribers() != n; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if h.Subscribers() != n {
		t.Fatalf("subscribers %d, want %d", h.Subscribers(), n)
	}
}

func nextPush(t *testing.T, c *typed.Client) wire.Push {
	t.Helper()
	select {
	case p := <-c.Pushes():
		return p
	case <-time.After(5 * time.Second):
		t.Fatal("no push message")
	}
	return wire.Push{}
}

// TestPushAccess: the new methods need a PIN, the changing ones need the
// elevated state, and a client without status.subscribe gets no push.
func TestPushAccess(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	c := typed.New(e.ln.dial(t))
	other := typed.New(e.ln.dial(t))
	if err := c.Subscribe(ctx); client.Code(err) != wire.CodePINNotSet {
		t.Fatalf("subscribe before PIN: %v", err)
	}
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	// OD-50 (VLT-IS-011): the connection keeps the elevated state after
	// set_pin. Lock to test the access without elevation.
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{wire.MethodDeviceSettingsSet, wire.MethodPolicySet, wire.MethodPolicySimulate,
		wire.MethodPolicyActivate, wire.MethodPolicyDeactivate, wire.MethodShutdownCancel, wire.MethodDeviceScan,
		wire.MethodUPSTestStart, wire.MethodUPSTestCancel, wire.MethodUPSBeeperSet} {
		var params any
		if m == wire.MethodDeviceSettingsSet || m == wire.MethodPolicySet {
			params = wire.SettingsMapParams{Settings: map[string]json.RawMessage{"x": json.RawMessage(`1`)}}
		}
		if m == wire.MethodDeviceScan {
			params = wire.DeviceScanParams{Ports: []string{"sim1"}, Confirmation: wire.ScanConfirmation}
		}
		if err := c.Call(ctx, m, params, nil); client.Code(err) != wire.CodeNotElevated {
			t.Errorf("%s without elevation: %v", m, err)
		}
	}
	for _, m := range []string{wire.MethodDeviceStatus, wire.MethodPolicyGet, wire.MethodShutdownStatus, wire.MethodSubscribe, wire.MethodDevicePorts} {
		// ups.control.capabilities is read only; without the UPS part it is
		// "unavailable" (TestUPSControlAccess has the UPS part).
		if err := c.Call(ctx, m, nil, nil); err != nil {
			t.Errorf("%s: %v", m, err)
		}
	}
	if n := len(e.sims["sim1"].Received()); n != 0 {
		t.Errorf("device.scan without elevation sent %d commands", n)
	}
	waitSubs(t, e.hub, 1)
	for i := 0; i < 3; i++ {
		_ = e.hub.Publish(wire.PushCountdown, map[string]any{"i": i})
	}
	for i := 0; i < 3; i++ {
		if p := nextPush(t, c); p.Push != wire.PushCountdown {
			t.Fatalf("push %+v", p)
		}
	}
	if _, err := other.Hello(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-other.Pushes():
		t.Fatalf("push without subscribe: %+v", p)
	case <-time.After(100 * time.Millisecond):
	}
	// The subscription ends with the connection.
	c.Close()
	waitSubs(t, e.hub, 0)
}

// TestHubDropsForSlowClient: Publish never blocks.
func TestHubDropsForSlowClient(t *testing.T) {
	h := NewHub()
	s := h.add()
	for i := 0; i < PushQueue+10; i++ {
		if err := h.Publish(wire.PushStatus, i); err != nil {
			t.Fatal(err)
		}
	}
	if h.Dropped() != 10 || len(s.ch) != PushQueue {
		t.Fatalf("dropped %d queued %d", h.Dropped(), len(s.ch))
	}
	h.remove(s)
	h.remove(s)
}

// deviceScan: device.ports opens no port; device.scan needs the text
// "SCAN PORTS" and sends only "Q1" CR to the marked ports (SEC-099).
func deviceScan(t *testing.T, ctx context.Context, e *env, c *typed.Client) {
	t.Helper()
	ps, err := c.DevicePorts(ctx)
	if err != nil || len(ps) != 3 || ps[1].Product != "USB Serial" || !ps[1].IsUSB {
		t.Fatalf("device.ports: %+v %v", ps, err)
	}
	for name, s := range e.sims {
		if n := len(s.Received()); n != 0 {
			t.Fatalf("device.ports sent %d commands to %s", n, name)
		}
	}
	if _, err := c.DeviceScan(ctx, []string{"sim1"}, "scan ports"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("wrong confirmation: %v", err)
	}
	if _, err := c.DeviceScan(ctx, []string{"COM9"}, wire.ScanConfirmation); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("port not in the list: %v", err)
	}
	res, err := c.DeviceScan(ctx, []string{"sim1", "sim0"}, wire.ScanConfirmation)
	if err != nil || len(res) != 2 || res[0].Result != "response" || !res[0].ValidQ1 || res[1].Result != "busy" {
		t.Fatalf("device.scan: %+v %v", res, err)
	}
	if got := e.sims["sim1"].Received(); len(got) != 1 || got[0] != "Q1" {
		t.Fatalf("sim1 got %q", got)
	}
	// sim0 is the port of the Poller; sim2 is not marked.
	for _, name := range []string{"sim0", "sim2"} {
		if n := len(e.sims[name].Received()); n != 0 {
			t.Fatalf("%s got %d commands", name, n)
		}
	}
	rows, err := e.au.List(ctx, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, r := range rows {
		if r.Action == "device_scan" {
			found++
		}
	}
	if found != 3 { // wrong confirmation, port not in the list, scan
		t.Fatalf("%d device_scan audit entries, want 3", found)
	}
}

// upsMethods calls the UPS Control methods of VLT-IS-006 TASK 03-00-00-007
// (step 3: part of TestAllMethodsNoSecretInResponses).
func upsMethods(t *testing.T, ctx context.Context, e *env, c *typed.Client) {
	t.Helper()
	if e.srv.b.UPS == nil {
		withUPS(t, e)
	}
	if _, err := c.UPSControlCapabilities(ctx); err != nil {
		t.Fatalf("ups.control.capabilities: %v", err)
	}
	if r, err := c.UPSBeeperSet(ctx, false); err != nil || !r.Verified {
		t.Fatalf("ups.beeper.set: %+v %v", r, err)
	}
	if r, err := c.UPSTestStart(ctx, wire.TestTypeMinutes, 1, wire.TestConfirmation); err != nil || !r.Verified {
		t.Fatalf("ups.test.start: %+v %v", r, err)
	}
	if r, err := c.UPSTestCancel(ctx); err != nil || !r.Verified {
		t.Fatalf("ups.test.cancel: %+v %v", r, err)
	}
}
