// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"

	"github.com/abuzer1495/voltiovolt/internal/redact"
)

// SettingDef is one known Setting: the key, the Go type of the value, the
// default value and a check.
type SettingDef struct {
	Key string
	// Type is the Go type of the value (int, string, bool). A Setting never
	// contains a Secret (VLT-IS-004 QI-6): CheckDefs refuses a type that is
	// or contains redact.Secret.
	Type    reflect.Type
	Default json.RawMessage
	// Check returns an error if the value is not permitted.
	Check func(json.RawMessage) error
}

// Setting is the value that a read returns.
type Setting struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Version   int64           `json:"version"`
	UpdatedAt string          `json:"updated_at,omitempty"`
	UpdatedBy string          `json:"updated_by,omitempty"`
	// Default is true when the value is the default value (no row, a MAC
	// that is not correct, or no MAC key).
	Default bool `json:"default"`
}

// Setting keys.
const (
	SettingSessionIdleMinutes = "auth.session_idle_minutes"
	SettingLanguage           = "ui.language"

	// Device Settings (VLT-IS-004 TASK 03-00-00-003 step 1).
	SettingDeviceTransport = "device.transport"   // auto, hid or serial
	SettingDeviceSerial    = "device.serial_port" // empty: all serial ports
	SettingDeviceDTR       = "device.serial_dtr"
	SettingDeviceRTS       = "device.serial_rts"

	// Event thresholds (VLT-IS-004 TASK 03-00-00-005 step 3).
	SettingEventOverload   = "event.overload_percent"
	SettingEventOverTemp   = "event.over_temperature_c"
	SettingEventRuntimeLow = "event.runtime_low_minutes"

	// Shutdown Policy (VLT-IS-004 TASK 03-00-00-006 step 1).
	SettingPolicyAction           = "policy.action"
	SettingPolicyOnBatterySeconds = "policy.trigger.on_battery_seconds"
	SettingPolicyRuntimeBelow     = "policy.trigger.runtime_below_minutes"
	SettingPolicyChargeBelow      = "policy.trigger.charge_below_percent"
	SettingPolicyLowBatteryFlag   = "policy.trigger.low_battery_flag"
	SettingPolicyCommLostSeconds  = "policy.trigger.comm_lost_on_battery_seconds"
	SettingPolicyCountdown        = "policy.countdown_seconds"
	SettingPolicyCancelOnMains    = "policy.cancel_when_mains_returns"
	SettingPolicyForceClose       = "policy.force_close_applications"
)

// serialPortName accepts an empty text, a Linux device path /dev/<name>
// or a Windows COM port name.
var serialPortName = regexp.MustCompile(`^(|/dev/[A-Za-z0-9._-]{1,32}|COM[1-9][0-9]{0,2})$`)

func serialPort(v json.RawMessage) error {
	var s string
	if err := strictJSON(v, &s); err != nil {
		return fmt.Errorf("value must be a text: %w", err)
	}
	if !serialPortName.MatchString(s) {
		return errors.New("value must be empty, /dev/<name> or COM<n>")
	}
	return nil
}

func isBool(v json.RawMessage) error {
	var b bool
	if err := strictJSON(v, &b); err != nil {
		return fmt.Errorf("value must be true or false: %w", err)
	}
	return nil
}

func intRange(lo, hi int) func(json.RawMessage) error {
	return func(v json.RawMessage) error {
		var n int
		if err := strictJSON(v, &n); err != nil {
			return fmt.Errorf("value must be an integer: %w", err)
		}
		if n < lo || n > hi {
			return fmt.Errorf("value must be %d to %d", lo, hi)
		}
		return nil
	}
}

func oneOf(vals ...string) func(json.RawMessage) error {
	return func(v json.RawMessage) error {
		var s string
		if err := strictJSON(v, &s); err != nil {
			return fmt.Errorf("value must be a text: %w", err)
		}
		for _, x := range vals {
			if s == x {
				return nil
			}
		}
		return fmt.Errorf("value must be one of %v", vals)
	}
}

func strictJSON(v json.RawMessage, out any) error {
	d := json.NewDecoder(bytes.NewReader(v))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.More() {
		return errors.New("extra data")
	}
	return nil
}

// Go types of Setting values.
var (
	TypeInt    = reflect.TypeOf(0)
	TypeString = reflect.TypeOf("")
	TypeBool   = reflect.TypeOf(false)
)

// ErrSecretSetting is returned for a Setting with the Secret type.
var ErrSecretSetting = errors.New("store: a Setting must not have the Secret type (VLT-IS-004 QI-6)")

var secretType = reflect.TypeOf(redact.Secret{})

func containsSecret(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	if t == secretType {
		return true
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
		return containsSecret(t.Elem(), seen)
	case reflect.Map:
		return containsSecret(t.Key(), seen) || containsSecret(t.Elem(), seen)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if containsSecret(t.Field(i).Type, seen) {
				return true
			}
		}
	case reflect.Interface:
		return true // an interface can hold a Secret
	}
	return false
}

// CheckDefs checks Setting definitions: a key, a type that is not and does
// not contain redact.Secret, a default value of that type that passes the
// check, and no key two times.
func CheckDefs(defs []SettingDef) error {
	seen := map[string]bool{}
	for _, d := range defs {
		if d.Key == "" {
			return errors.New("store: Setting without a key")
		}
		if seen[d.Key] {
			return fmt.Errorf("store: Setting %q two times", d.Key)
		}
		seen[d.Key] = true
		if d.Type == nil {
			return fmt.Errorf("store: Setting %q has no type", d.Key)
		}
		if containsSecret(d.Type, map[reflect.Type]bool{}) {
			return fmt.Errorf("%w: %q", ErrSecretSetting, d.Key)
		}
		v := reflect.New(d.Type).Interface()
		if err := strictJSON(d.Default, v); err != nil {
			return fmt.Errorf("store: Setting %q: default value is not of type %v: %w", d.Key, d.Type, err)
		}
		if d.Check != nil {
			if err := d.Check(d.Default); err != nil {
				return fmt.Errorf("store: Setting %q: default value: %w", d.Key, err)
			}
		}
	}
	return nil
}

// DefaultSettings returns the known Settings of this instruction. None of
// them is a Secret.
func DefaultSettings() []SettingDef {
	return []SettingDef{
		// VLT-IS-003 TASK 03-00-00-005 step 7.2: 10 minutes without a request.
		{Key: SettingSessionIdleMinutes, Type: TypeInt, Default: json.RawMessage(`10`), Check: intRange(1, 120)},
		{Key: SettingLanguage, Type: TypeString, Default: json.RawMessage(`"en"`), Check: oneOf("en", "tr")},
		// VLT-IS-004 TASK 03-00-00-003 step 1. DTR on and RTS off are the
		// defaults of VLT-IS-002 (transport.DefaultSerialOptions).
		{Key: SettingDeviceTransport, Type: TypeString, Default: json.RawMessage(`"auto"`), Check: oneOf("auto", "hid", "serial")},
		{Key: SettingDeviceSerial, Type: TypeString, Default: json.RawMessage(`""`), Check: serialPort},
		{Key: SettingDeviceDTR, Type: TypeBool, Default: json.RawMessage(`true`), Check: isBool},
		{Key: SettingDeviceRTS, Type: TypeBool, Default: json.RawMessage(`false`), Check: isBool},

		// VLT-IS-004 TASK 03-00-00-005 step 3.
		{Key: SettingEventOverload, Type: TypeInt, Default: json.RawMessage(`100`), Check: intRange(50, 150)},
		{Key: SettingEventOverTemp, Type: TypeInt, Default: json.RawMessage(`50`), Check: intRange(30, 90)},
		{Key: SettingEventRuntimeLow, Type: TypeInt, Default: json.RawMessage(`5`), Check: intRange(1, 120)},
		// VLT-IS-004 TASK 03-00-00-006 step 1. 0 = off for the triggers.
		{Key: SettingPolicyAction, Type: TypeString, Default: json.RawMessage(`"shutdown"`), Check: oneOf("shutdown", "hibernate")},
		{Key: SettingPolicyOnBatterySeconds, Type: TypeInt, Default: json.RawMessage(`300`), Check: intRange(0, 86400)},
		{Key: SettingPolicyRuntimeBelow, Type: TypeInt, Default: json.RawMessage(`5`), Check: intRange(0, 600)},
		{Key: SettingPolicyChargeBelow, Type: TypeInt, Default: json.RawMessage(`30`), Check: intRange(0, 100)},
		{Key: SettingPolicyLowBatteryFlag, Type: TypeBool, Default: json.RawMessage(`true`), Check: isBool},
		{Key: SettingPolicyCommLostSeconds, Type: TypeInt, Default: json.RawMessage(`180`), Check: intRange(0, 86400)},
		{Key: SettingPolicyCountdown, Type: TypeInt, Default: json.RawMessage(`60`), Check: intRange(10, 600)},
		{Key: SettingPolicyCancelOnMains, Type: TypeBool, Default: json.RawMessage(`true`), Check: isBool},
		{Key: SettingPolicyForceClose, Type: TypeBool, Default: json.RawMessage(`true`), Check: isBool},
	}
}

// SetMACKey sets the settings MAC key. The Vault derives it from the DEK
// (HKDF-SHA256, info "vlt-settings-mac-v1"). nil means: the Vault is not
// available.
func (s *Store) SetMACKey(key []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.macKey {
		s.macKey[i] = 0
	}
	s.macKey = nil
	s.noKeyNoted = false
	if key != nil {
		s.macKey = append([]byte(nil), key...)
	}
}

// settingMAC is HMAC-SHA256 over key, value and version. Each part has a
// 4-byte big-endian length (key, value) or is 8 bytes (version).
func settingMAC(macKey []byte, key string, value []byte, version int64) []byte {
	m := hmac.New(sha256.New, macKey)
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(key)))
	m.Write([]byte("vlt-settings-row-v1"))
	m.Write(l[:])
	m.Write([]byte(key))
	binary.BigEndian.PutUint32(l[:], uint32(len(value)))
	m.Write(l[:])
	m.Write(value)
	var v [8]byte
	binary.BigEndian.PutUint64(v[:], uint64(version))
	m.Write(v[:])
	return m.Sum(nil)
}

type settingRow struct {
	value     []byte
	version   int64
	updatedAt string
	updatedBy string
	mac       []byte
}

func (s *Store) readRow(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, key string) (settingRow, error) {
	var r settingRow
	var v string
	err := q.QueryRowContext(ctx, "SELECT value, version, updated_at, updated_by, mac FROM settings WHERE key = ?", key).
		Scan(&v, &r.version, &r.updatedAt, &r.updatedBy, &r.mac)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	r.value = []byte(v)
	return r, err
}

// GetSetting returns the value of a known Setting. If there is no row, if
// the MAC is not correct or if there is no MAC key, it returns the default
// value. A MAC that is not correct gives a log entry and a call to
// OnMACFailure (audit entry).
func (s *Store) GetSetting(ctx context.Context, key string) (Setting, error) {
	def, ok := s.defs[key]
	if !ok {
		return Setting{}, fmt.Errorf("store: unknown setting %q", key)
	}
	dflt := Setting{Key: key, Value: def.Default, Default: true}
	r, err := s.readRow(ctx, s.db, key)
	if errors.Is(err, ErrNotFound) {
		return dflt, nil
	}
	if err != nil {
		return Setting{}, err
	}
	s.mu.Lock()
	mk := s.macKey
	if mk == nil {
		if !s.noKeyNoted {
			s.noKeyNoted = true
			s.log.Warn("vault not available: settings use the default values, settings writes are not possible")
		}
		s.mu.Unlock()
		dflt.Version = r.version
		return dflt, nil
	}
	good := hmac.Equal(r.mac, settingMAC(mk, key, r.value, r.version))
	s.mu.Unlock()
	if !good {
		s.log.Error("settings row MAC is not correct; the default value is used", "key", key, "version", r.version)
		if s.opts.OnMACFailure != nil {
			s.opts.OnMACFailure(key)
		}
		dflt.Version = r.version
		return dflt, nil
	}
	if def.Check != nil && def.Check(r.value) != nil {
		s.log.Error("settings row value is not permitted; the default value is used", "key", key)
		dflt.Version = r.version
		return dflt, nil
	}
	return Setting{Key: key, Value: json.RawMessage(r.value), Version: r.version, UpdatedAt: r.updatedAt, UpdatedBy: r.updatedBy}, nil
}

// ListSettings returns all known Settings, sorted by key.
func (s *Store) ListSettings(ctx context.Context) ([]Setting, error) {
	keys := make([]string, 0, len(s.defs))
	for k := range s.defs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Setting, 0, len(keys))
	for _, k := range keys {
		st, err := s.GetSetting(ctx, k)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// SetSetting writes a known Setting with a new MAC. It returns the new
// version. Without a MAC key it returns ErrVaultUnavailable.
func (s *Store) SetSetting(ctx context.Context, key string, value json.RawMessage, actor string) (int64, error) {
	def, ok := s.defs[key]
	if !ok {
		return 0, fmt.Errorf("store: unknown setting %q", key)
	}
	if def.Check != nil {
		if err := def.Check(value); err != nil {
			return 0, fmt.Errorf("store: setting %q: %w", key, err)
		}
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, value); err != nil {
		return 0, fmt.Errorf("store: setting %q: %w", key, err)
	}
	s.mu.Lock()
	mk := append([]byte(nil), s.macKey...)
	hasKey := s.macKey != nil
	s.mu.Unlock()
	if !hasKey {
		s.log.Warn("settings write refused: vault not available", "key", key)
		return 0, ErrVaultUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var ver int64 = 1
	r, err := s.readRow(ctx, tx, key)
	switch {
	case err == nil:
		ver = r.version + 1
	case !errors.Is(err, ErrNotFound):
		return 0, err
	}
	mac := settingMAC(mk, key, compact.Bytes(), ver)
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value, version, updated_at, updated_by, mac)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, version = excluded.version,
		updated_at = excluded.updated_at, updated_by = excluded.updated_by, mac = excluded.mac`,
		key, compact.String(), ver, Now(), actor, mac); err != nil {
		return 0, err
	}
	return ver, tx.Commit()
}

// RekeySettings writes a new MAC with newKey for each row that has a
// correct MAC with the current key. Then newKey becomes the current key.
// Rows with a MAC that is not correct do not change.
func (s *Store) RekeySettings(ctx context.Context, newKey []byte) error {
	s.mu.Lock()
	old := append([]byte(nil), s.macKey...)
	hasOld := s.macKey != nil
	s.mu.Unlock()
	if hasOld {
		rows, err := s.db.QueryContext(ctx, "SELECT key, value, version, mac FROM settings")
		if err != nil {
			return err
		}
		type upd struct {
			key string
			mac []byte
		}
		var ups []upd
		for rows.Next() {
			var k, v string
			var ver int64
			var mac []byte
			if err := rows.Scan(&k, &v, &ver, &mac); err != nil {
				rows.Close()
				return err
			}
			if hmac.Equal(mac, settingMAC(old, k, []byte(v), ver)) {
				ups = append(ups, upd{k, settingMAC(newKey, k, []byte(v), ver)})
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, u := range ups {
			if _, err := tx.ExecContext(ctx, "UPDATE settings SET mac = ? WHERE key = ?", u.mac, u.key); err != nil {
				tx.Rollback()
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	s.SetMACKey(newKey)
	return nil
}
