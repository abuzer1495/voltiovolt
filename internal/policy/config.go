// SPDX-License-Identifier: AGPL-3.0-or-later

// Package policy is the Policy Engine and the Simulation of vltd
// (VLT-IS-004 TASK 03-00-00-006).
//
// WARNING: An error in this package can turn off a computer at the wrong
// time or not turn it off before the battery is empty.
//
// The Policy Engine is the only caller of the shutdown executor (SEC-092).
// A policy is active only after a passed Simulation of its current
// version; a change of a Setting makes it inactive (SEC-055, SEC-093).
package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// Actions.
const (
	ActionShutdown  = "shutdown"
	ActionHibernate = "hibernate"
)

// Trigger names (the Setting names without "policy.trigger.").
const (
	TriggerOnBattery  = "on_battery_seconds"
	TriggerRuntime    = "runtime_below_minutes"
	TriggerCharge     = "charge_below_percent"
	TriggerLowBattery = "low_battery_flag"
	TriggerCommLost   = "comm_lost_on_battery_seconds"
)

// Config is the Shutdown Policy (step 1).
type Config struct {
	Action                 string `json:"action"`
	OnBatterySeconds       int    `json:"trigger.on_battery_seconds"`
	RuntimeBelowMinutes    int    `json:"trigger.runtime_below_minutes"`
	ChargeBelowPercent     int    `json:"trigger.charge_below_percent"`
	LowBatteryFlag         bool   `json:"trigger.low_battery_flag"`
	CommLostOnBatterySecs  int    `json:"trigger.comm_lost_on_battery_seconds"`
	CountdownSeconds       int    `json:"countdown_seconds"`
	CancelWhenMainsReturns bool   `json:"cancel_when_mains_returns"`
	ForceCloseApplications bool   `json:"force_close_applications"`
}

// DefaultConfig is the default policy of step 1.
func DefaultConfig() Config {
	return Config{
		Action:                 ActionShutdown,
		OnBatterySeconds:       300,
		RuntimeBelowMinutes:    5,
		ChargeBelowPercent:     30,
		LowBatteryFlag:         true,
		CommLostOnBatterySecs:  180,
		CountdownSeconds:       60,
		CancelWhenMainsReturns: true,
		ForceCloseApplications: true,
	}
}

// FallbackConfig is the Fallback Policy (VLT-IS-005 TASK 03-00-00-001
// step 1, Q-IA-1): fixed values in the code. vltd uses it only when the
// Vault state is "locked", because a safety function must not depend on
// the Vault. Its Simulation runs in CI (SEC-055 exception).
func FallbackConfig() Config {
	return Config{
		Action:                 ActionShutdown,
		OnBatterySeconds:       0,
		RuntimeBelowMinutes:    2,
		ChargeBelowPercent:     0,
		LowBatteryFlag:         true,
		CommLostOnBatterySecs:  300,
		CountdownSeconds:       30,
		CancelWhenMainsReturns: true,
		ForceCloseApplications: true,
	}
}

// Keys are the Setting keys of the policy.
var Keys = []string{
	store.SettingPolicyAction,
	store.SettingPolicyOnBatterySeconds,
	store.SettingPolicyRuntimeBelow,
	store.SettingPolicyChargeBelow,
	store.SettingPolicyLowBatteryFlag,
	store.SettingPolicyCommLostSeconds,
	store.SettingPolicyCountdown,
	store.SettingPolicyCancelOnMains,
	store.SettingPolicyForceClose,
}

func (c *Config) field(key string) any {
	switch key {
	case store.SettingPolicyAction:
		return &c.Action
	case store.SettingPolicyOnBatterySeconds:
		return &c.OnBatterySeconds
	case store.SettingPolicyRuntimeBelow:
		return &c.RuntimeBelowMinutes
	case store.SettingPolicyChargeBelow:
		return &c.ChargeBelowPercent
	case store.SettingPolicyLowBatteryFlag:
		return &c.LowBatteryFlag
	case store.SettingPolicyCommLostSeconds:
		return &c.CommLostOnBatterySecs
	case store.SettingPolicyCountdown:
		return &c.CountdownSeconds
	case store.SettingPolicyCancelOnMains:
		return &c.CancelWhenMainsReturns
	case store.SettingPolicyForceClose:
		return &c.ForceCloseApplications
	}
	return nil
}

// Load reads the policy Settings.
func Load(ctx context.Context, st *store.Store) (Config, error) {
	c := DefaultConfig()
	for _, k := range Keys {
		s, err := st.GetSetting(ctx, k)
		if err != nil {
			return DefaultConfig(), err
		}
		if err := json.Unmarshal(s.Value, c.field(k)); err != nil {
			return DefaultConfig(), fmt.Errorf("policy: setting %s: %w", k, err)
		}
	}
	return c, nil
}

// Version is the policy version: the lowercase hex SHA-256 of the
// canonical JSON of the Settings (keys sorted, no spaces) (step 5.2).
func Version(c Config) string {
	m := map[string]any{}
	for _, k := range Keys {
		m[k] = c.field(k)
	}
	// encoding/json writes the keys of a map in sorted order, without
	// spaces.
	b, err := json.Marshal(m)
	if err != nil {
		panic(err) // only basic types
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
