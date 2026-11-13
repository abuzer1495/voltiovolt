// SPDX-License-Identifier: AGPL-3.0-or-later

// Package wire is the vltd IPC protocol, version 1 (VLT-IS-003 TASK
// 03-00-00-007): frames, requests, responses and the method parameter and
// result types. The server (internal/ipc) and the client
// (internal/ipc/client) use it.
//
// Frame: a 4-byte big-endian length, then JSON. Maximum frame size: 1 MiB.
// The decoder rejects unknown fields.
package wire

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Version is the protocol version.
const Version = 1

// MaxFrame is the maximum frame size (JSON bytes) in bytes.
const MaxFrame = 1 << 20

// Errors.
var (
	ErrFrameTooLarge = errors.New("ipc: frame is larger than 1 MiB")
	ErrEmptyFrame    = errors.New("ipc: empty frame")
)

// WriteFrame writes v as one frame.
func WriteFrame(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > MaxFrame {
		return ErrFrameTooLarge
	}
	buf := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(buf, uint32(len(b)))
	copy(buf[4:], b)
	_, err = w.Write(buf)
	return err
}

// ReadLength returns the frame length from the 4 header bytes.
func ReadLength(h [4]byte) (int, error) {
	n := binary.BigEndian.Uint32(h[:])
	if n == 0 {
		return 0, ErrEmptyFrame
	}
	if n > MaxFrame {
		return 0, ErrFrameTooLarge
	}
	return int(n), nil
}

// ReadFrame reads one frame. It does not set deadlines.
func ReadFrame(r io.Reader) ([]byte, error) {
	var h [4]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, err
	}
	n, err := ReadLength(h)
	if err != nil {
		return nil, err
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	return b, nil
}

// Strict decodes one JSON value into out. Unknown fields and extra data
// are errors.
func Strict(b []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("extra data after the JSON value")
	}
	return nil
}

// Request is {"v":1,"id":number,"method":text,"params":object}.
type Request struct {
	V      int             `json:"v"`
	ID     *int64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// DecodeRequest decodes and checks a request frame.
func DecodeRequest(b []byte) (Request, error) {
	var r Request
	if err := Strict(b, &r); err != nil {
		return r, fmt.Errorf("ipc: bad request: %w", err)
	}
	if r.V != Version {
		return r, fmt.Errorf("ipc: protocol version %d is not supported", r.V)
	}
	if r.ID == nil {
		return r, errors.New("ipc: request has no id")
	}
	if r.Method == "" {
		return r, errors.New("ipc: request has no method")
	}
	p := bytes.TrimSpace(r.Params)
	if len(p) == 0 || string(p) == "null" {
		r.Params = json.RawMessage(`{}`)
	} else if p[0] != '{' {
		return r, errors.New("ipc: params must be an object")
	}
	return r, nil
}

// Error is the error object of a response.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return "ipc: " + e.Code + ": " + e.Message }

// Response is {"v":1,"id":number,"result":object} or
// {"v":1,"id":number,"error":{"code":text,"message":text}}.
type Response struct {
	V      int             `json:"v"`
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Error codes.
const (
	CodeBadRequest       = "bad_request"
	CodeUnknownMethod    = "unknown_method"
	CodePINNotSet        = "pin_not_set"
	CodeNotElevated      = "not_elevated"
	CodePINIncorrect     = "pin_incorrect"
	CodeBlocked          = "blocked"
	CodeVaultUnavailable = "vault_unavailable"
	CodeNotFound         = "not_found"
	CodeInvalid          = "invalid"
	CodeRateLimited      = "rate_limited"
	CodeFrameTooLarge    = "frame_too_large"
	CodeInternal         = "internal"
	CodeUnavailable      = "unavailable"   // the part of vltd is not running
	CodeNoSimulation     = "no_simulation" // policy.activate without a passed Simulation
	CodeLowBattery       = "low_battery"   // shutdown.cancel while LOW_BATTERY is active
	CodeVaultLocked      = "vault_locked"  // policy.set/activate/deactivate while the Fallback Policy is in use
	// VLT-IS-006 TASK 03-00-00-007 (UPS Control).
	CodeOnBattery    = "on_battery"       // ups.test.start while the UPS is on battery
	CodeCountdown    = "countdown_active" // ups.test.start while a shutdown countdown runs
	CodeNotVerified  = "not_verified"     // the UPS status did not confirm the command
	CodeNotSupported = "not_supported"    // the Dialect does not support the function
	CodeNoUPS        = "no_ups"           // no UPS connected
	CodeControlBusy  = "control_busy"     // another UPS Control request runs
	CodeSendFailed   = "send_failed"      // channels.test: the Channel did not send the test message
)

// Methods of protocol version 1.
const (
	MethodHello        = "system.hello"
	MethodStatus       = "system.status"
	MethodAuthStatus   = "auth.status"
	MethodSetPIN       = "auth.set_pin"
	MethodRemovePIN    = "auth.remove_pin"
	MethodForgotPIN    = "auth.forgot_pin"
	MethodLogin        = "auth.login"
	MethodLogout       = "auth.logout"
	MethodSettingsList = "settings.list"
	MethodSettingsSet  = "settings.set"
	MethodSecretStatus = "secret.status"
	MethodSecretSet    = "secret.set"
	MethodSecretDelete = "secret.delete"
	MethodVaultReset   = "vault.reset"
	MethodAuditList    = "audit.list"

	// VLT-IS-004 TASK 03-00-00-008.
	MethodDeviceStatus      = "device.status"
	MethodDeviceSettingsSet = "device.settings.set"
	MethodSubscribe         = "status.subscribe"
	MethodUnsubscribe       = "status.unsubscribe"
	MethodMeasurements      = "measurements.query"
	MethodEventsList        = "events.list"
	MethodPolicyGet         = "policy.get"
	MethodPolicySet         = "policy.set"
	MethodPolicySimulate    = "policy.simulate"
	MethodPolicyActivate    = "policy.activate"
	MethodPolicyDeactivate  = "policy.deactivate"
	MethodShutdownStatus    = "shutdown.status"
	MethodShutdownCancel    = "shutdown.cancel"

	// VLT-IS-006 TASK 03-00-00-004 step 3 (OD-27, SEC-099).
	// device.ports lists the serial ports and opens no port. device.scan
	// sends only "Q1" CR to each marked port.
	MethodDevicePorts = "device.ports"
	MethodDeviceScan  = "device.scan"

	// VLT-IS-006 TASK 03-00-00-007: UPS Control. No IPC method sends the
	// shutdown and restore command or the cancel shutdown command (SEC-098).
	MethodUPSControlCaps = "ups.control.capabilities"
	MethodUPSTestStart   = "ups.test.start"
	MethodUPSTestCancel  = "ups.test.cancel"
	MethodUPSBeeperSet   = "ups.beeper.set"

	// VLT-IS-007 TASK 03-00-00-007: Notification Channels. All changes
	// need the elevated state.
	MethodChannelsList        = "channels.list"
	MethodChannelsSettingsSet = "channels.settings.set"
	MethodChannelsTest        = "channels.test"
	MethodTelegramPairStart   = "telegram.pair.start"
	MethodTelegramChatsList   = "telegram.chats.list"
	MethodTelegramChatsRemove = "telegram.chats.remove"

	// VLT-IS-009 TASK 03-00-00-005: Simulate power failure in demo mode.
	MethodDemoPowerFail = "device.demo.power_fail"
)

// Confirmation texts of ups.test.start.
const (
	TestConfirmation         = "START TEST"
	TestUntilLowConfirmation = "TEST UNTIL LOW"
)

// Test types of ups.test.start.
const (
	TestType10s      = "10s"
	TestTypeMinutes  = "minutes"
	TestTypeUntilLow = "until_low"
)

// ScanConfirmation is the confirmation text of device.scan.
const ScanConfirmation = "SCAN PORTS"

// ForgotPINConfirmation is the confirmation text of auth.forgot_pin
// (VLT-IS-009 OD-40). The user types "RESET".
const ForgotPINConfirmation = "RESET"

// Methods lists all methods.
var Methods = []string{
	MethodHello, MethodStatus, MethodAuthStatus, MethodSetPIN, MethodRemovePIN, MethodForgotPIN, MethodLogin, MethodLogout,
	MethodSettingsList, MethodSettingsSet, MethodSecretStatus, MethodSecretSet, MethodSecretDelete,
	MethodVaultReset, MethodAuditList,
	MethodDeviceStatus, MethodDeviceSettingsSet, MethodSubscribe, MethodUnsubscribe, MethodMeasurements,
	MethodEventsList, MethodPolicyGet, MethodPolicySet, MethodPolicySimulate, MethodPolicyActivate,
	MethodPolicyDeactivate, MethodShutdownStatus, MethodShutdownCancel,
	MethodDevicePorts, MethodDeviceScan,
	MethodUPSControlCaps, MethodUPSTestStart, MethodUPSTestCancel, MethodUPSBeeperSet,
	MethodChannelsList, MethodChannelsSettingsSet, MethodChannelsTest,
	MethodTelegramPairStart, MethodTelegramChatsList, MethodTelegramChatsRemove,
	MethodDemoPowerFail,
}

// Push messages (VLT-IS-004 TASK 03-00-00-008 step 1). The server sends
// them only after status.subscribe.
const (
	PushStatus    = "status"    // every 2 seconds
	PushEvent     = "event"     // one for each Event
	PushCountdown = "countdown" // every second during a countdown
	// PushNotification is a Notification of the Router for the desktop
	// Channel (VLT-IS-007 TASK 03-00-00-004). Data: notify.Notification.
	PushNotification = "notification"
)

// Push is {"v":1,"push":text,"data":object}. It has no id.
type Push struct {
	V    int             `json:"v"`
	Push string          `json:"push"`
	Data json.RawMessage `json:"data"`
}

// Frame is the union of Response and Push for a client decoder.
type Frame struct {
	V      int             `json:"v"`
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
	Push   string          `json:"push,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// Parameters.
type (
	NoParams  struct{}
	PINParams struct {
		PIN string `json:"pin"`
	}
	SettingsSetParams struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	SecretIDParams struct {
		ID string `json:"id"`
	}
	SecretSetParams struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
	VaultResetParams struct {
		Confirmation string `json:"confirmation"`
	}
	// ForgotPINParams is the confirmation of auth.forgot_pin (OD-40).
	ForgotPINParams struct {
		Confirmation string `json:"confirmation"`
	}
	AuditListParams struct {
		After int64 `json:"after"`
		Limit int   `json:"limit"`
	}
	// SettingsMapParams is for device.settings.set and policy.set:
	// {"settings":{"key":value}}.
	SettingsMapParams struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	MeasurementsParams struct {
		Name        string `json:"name"`
		From        string `json:"from"` // RFC 3339
		To          string `json:"to"`   // RFC 3339
		Step        string `json:"step"` // raw, 1m or 1h
		IncludeDemo bool   `json:"include_demo,omitempty"`
	}
	DeviceScanParams struct {
		Ports        []string `json:"ports"`
		Confirmation string   `json:"confirmation"`
	}
	UPSTestStartParams struct {
		Type         string `json:"type"`              // 10s, minutes or until_low
		Minutes      int    `json:"minutes,omitempty"` // 1 to 99 for "minutes"
		Confirmation string `json:"confirmation"`
	}
	UPSBeeperSetParams struct {
		On bool `json:"on"`
	}
	EventsListParams struct {
		Before      int64 `json:"before"` // 0: the newest
		Limit       int   `json:"limit"`
		IncludeDemo bool  `json:"include_demo,omitempty"`
	}
)

// Results.
type (
	OKResult struct {
		OK bool `json:"ok"`
	}
	HelloResult struct {
		Version  string `json:"version"`
		Protocol int    `json:"protocol"`
	}
	VaultState struct {
		State   string `json:"state"`
		Reason  string `json:"reason,omitempty"`
		KeyMode string `json:"key_mode,omitempty"`
	}
	StoreState struct {
		State         string `json:"state"`
		SchemaVersion int    `json:"schema_version"`
	}
	StatusResult struct {
		Vault         VaultState `json:"vault"`
		Store         StoreState `json:"store"`
		UptimeSeconds int64      `json:"uptime_seconds"`
	}
	AuthStatusResult struct {
		PINSet     bool   `json:"pin_set"`
		BlockUntil string `json:"block_until,omitempty"`
		Elevated   bool   `json:"elevated"`
	}
	Setting struct {
		Key       string          `json:"key"`
		Value     json.RawMessage `json:"value"`
		Version   int64           `json:"version"`
		Default   bool            `json:"default"`
		UpdatedAt string          `json:"updated_at,omitempty"`
		UpdatedBy string          `json:"updated_by,omitempty"`
	}
	SettingsListResult struct {
		Settings []Setting `json:"settings"`
	}
	SettingsSetResult struct {
		Version int64 `json:"version"`
	}
	SecretStatusResult struct {
		ID        string `json:"id"`
		Set       bool   `json:"set"`
		Hint      string `json:"hint,omitempty"`
		UpdatedAt string `json:"updated_at,omitempty"`
	}
	AuditRow struct {
		Seq      int64           `json:"seq"`
		TS       string          `json:"ts"`
		Actor    string          `json:"actor"`
		Action   string          `json:"action"`
		Target   string          `json:"target"`
		Result   string          `json:"result"`
		Details  json.RawMessage `json:"details"`
		PrevHash string          `json:"prev_hash"`
		Hash     string          `json:"hash"`
	}
	AuditListResult struct {
		Rows []AuditRow `json:"rows"`
		Next int64      `json:"next"`
	}
	MeasurementsResult struct {
		Name string          `json:"name"`
		Unit string          `json:"unit"`
		Step string          `json:"step"`
		Rows json.RawMessage `json:"rows"`
	}
	// SerialPort is one serial port of the OS (device.ports). It has no
	// USB serial number.
	SerialPort struct {
		Name    string `json:"name"`
		IsUSB   bool   `json:"is_usb"`
		VID     string `json:"vid,omitempty"`
		PID     string `json:"pid,omitempty"`
		Product string `json:"product,omitempty"`
	}
	DevicePortsResult struct {
		Ports []SerialPort `json:"ports"`
	}
	// ScanPortResult is the result of one marked port: "response",
	// "no response", "busy" or "error".
	ScanPortResult struct {
		Name    string `json:"name"`
		Result  string `json:"result"`
		ValidQ1 bool   `json:"valid_q1"`
	}
	DeviceScanResult struct {
		Results []ScanPortResult `json:"results"`
	}
	UPSTestResult struct {
		Result     string `json:"result"` // TEST_PASSED or TEST_FAILED
		Start      string `json:"start"`  // RFC 3339
		End        string `json:"end"`
		UPSFailed  bool   `json:"ups_failed"`
		BatteryLow bool   `json:"battery_low"`
		Cancelled  bool   `json:"cancelled"`
	}
	UPSControlCapsResult struct {
		Connected bool           `json:"connected"`
		Functions []string       `json:"functions"` // test.10s, test.minutes, test.until_low, test.cancel, beeper
		LastTest  *UPSTestResult `json:"last_test,omitempty"`
	}
	UPSControlResult struct {
		Function string `json:"function"`
		Bytes    string `json:"bytes"` // the command without CR; empty if nothing was sent
		Sends    int    `json:"sends"`
		Verified bool   `json:"verified"`
		NoSend   bool   `json:"no_send,omitempty"`
		DryRun   bool   `json:"dry_run,omitempty"`
	}
	EventsListResult struct {
		Events json.RawMessage `json:"events"`
		Next   int64           `json:"next"` // the before value of the next page; 0: no more
	}
)

// ChannelInfo is one Notification Channel in channels.list (VLT-IS-007
// TASK 03-00-00-007). Secret has only the set state and the hint, never
// the value.
type ChannelInfo struct {
	Name        string                     `json:"name"`
	Enabled     bool                       `json:"enabled"`
	MinSeverity string                     `json:"min_severity"`
	Secret      *SecretStatusResult        `json:"secret,omitempty"`
	Settings    map[string]json.RawMessage `json:"settings,omitempty"`
	Pairing     *TelegramPairResult        `json:"pairing,omitempty"` // an active pairing code (telegram)
	Chats       int                        `json:"chats"`             // paired chats (telegram)
}

// EmailPreset is a set of SMTP server Settings (the preset "Gmail").
type EmailPreset struct {
	Name     string `json:"name"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Security string `json:"security"`
}

// ChannelsListResult is the result of channels.list.
type ChannelsListResult struct {
	Channels []ChannelInfo `json:"channels"`
	Presets  []EmailPreset `json:"email_presets"`
}

// ChannelSettingsParams are the params of channels.settings.set. Settings
// is for the e-mail Channel: server, port, security, user, from, to.
type ChannelSettingsParams struct {
	Channel     string                     `json:"channel"`
	Enabled     *bool                      `json:"enabled,omitempty"`
	MinSeverity *string                    `json:"min_severity,omitempty"`
	Settings    map[string]json.RawMessage `json:"settings,omitempty"`
}

// ChannelParams are the params of channels.test.
type ChannelParams struct {
	Channel string `json:"channel"`
}

// TelegramPairResult is the result of telegram.pair.start: the one-time
// code of 6 digits and its end time (RFC 3339).
type TelegramPairResult struct {
	Code    string `json:"code,omitempty"`
	Expires string `json:"expires"`
}

// TelegramChat is one paired chat.
type TelegramChat struct {
	ChatID   int64  `json:"chat_id"`
	Name     string `json:"name"`
	PairedAt string `json:"paired_at"`
}

// TelegramChatsResult is the result of telegram.chats.list.
type TelegramChatsResult struct {
	Chats []TelegramChat `json:"chats"`
}

// TelegramChatParams are the params of telegram.chats.remove.
type TelegramChatParams struct {
	ChatID int64 `json:"chat_id"`
}
