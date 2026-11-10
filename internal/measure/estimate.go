// SPDX-License-Identifier: AGPL-3.0-or-later

package measure

import (
	"context"
	"encoding/json"
	"math"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// EstimateConfig is the Settings of the charge estimate (VLT-IS-006 TASK
// 03-00-00-003 step 8). Voltages in V.
type EstimateConfig struct {
	CellEmptyV, CellFullV   float64 // on-line UPS, per cell
	BlockEmptyV, BlockFullV float64 // standby UPS, per 12 V block
}

// DefaultEstimateConfig gives the default Settings.
func DefaultEstimateConfig() EstimateConfig {
	return EstimateConfig{CellEmptyV: 1.667, CellFullV: 2.267, BlockEmptyV: 10.0, BlockFullV: 13.6}
}

// EstimateConfigFromStore reads the Settings. A Setting that cannot be
// read keeps its default value.
func EstimateConfigFromStore(ctx context.Context, st *store.Store) EstimateConfig {
	c := DefaultEstimateConfig()
	for key, out := range map[string]*float64{
		store.SettingBatteryCellEmptyV:  &c.CellEmptyV,
		store.SettingBatteryCellFullV:   &c.CellFullV,
		store.SettingBatteryBlockEmptyV: &c.BlockEmptyV,
		store.SettingBatteryBlockFullV:  &c.BlockFullV,
	} {
		s, err := st.GetSetting(ctx, key)
		if err != nil {
			continue
		}
		var v float64
		if json.Unmarshal(s.Value, &v) == nil {
			*out = v
		}
	}
	return c
}

// ChargeEstimate calculates battery.charge_estimate from one Q1 response
// and the F rating. The method comes from VLT-REF-001 section 7 (STATIC
// only: UNVERIFIED). It returns false if an input is not available; then
// no value is stored.
//
//   - On-line UPS (b3 = 0): the Q1 battery voltage is per cell. Linear
//     between CellEmptyV (0 %) and CellFullV (100 %).
//   - Standby UPS (b3 = 1): the Q1 battery voltage is the total. blocks =
//     F rated battery voltage / 12. Linear between blocks × BlockEmptyV and
//     blocks × BlockFullV. Without a valid F rated battery voltage there is
//     no value.
//
// The result is limited to 0 to 100. It is not rounded: a value more than
// 90 does not become 100 (VLT-IS-006, not the UPSilon 2000 rule of
// VLT-REF-001 section 7 step 4).
func ChargeEstimate(st dialect.Status, rated *dialect.Rating, c EstimateConfig) (float64, bool) {
	if !st.BatteryVoltage.Valid || len(st.Bits.Raw) != 8 {
		return 0, false
	}
	vb := st.BatteryVoltage.Value
	var lo, hi float64
	if st.Flags.StandbyType {
		if rated == nil || !rated.BatteryVoltage.Valid || rated.BatteryVoltage.Value <= 0 {
			return 0, false
		}
		blocks := rated.BatteryVoltage.Value / 12
		lo, hi = blocks*c.BlockEmptyV, blocks*c.BlockFullV
	} else {
		lo, hi = c.CellEmptyV, c.CellFullV
	}
	if !(hi > lo) || math.IsNaN(vb) || math.IsInf(vb, 0) {
		return 0, false
	}
	p := 100 * (vb - lo) / (hi - lo)
	return math.Min(100, math.Max(0, p)), true
}
