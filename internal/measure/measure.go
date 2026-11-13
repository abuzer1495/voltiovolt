// SPDX-License-Identifier: AGPL-3.0-or-later

// Package measure writes and reads the Measurements (VLT-IS-004 TASK
// 03-00-00-004). A field with no value is not stored. All Measurements of
// one poll cycle are written in one transaction. An hourly job makes the
// 1-minute and 1-hour summaries (minimum, maximum, mean) and deletes old
// values: raw values after 7 days, 1-minute values after 400 days, 1-hour
// values never.
package measure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

// Measurement names (step 1).
const (
	InputVoltage      = "input.voltage"
	InputFaultVoltage = "input.fault_voltage"
	InputFrequency    = "input.frequency"
	OutputVoltage     = "output.voltage"
	OutputFrequency   = "output.frequency"
	OutputCurrent     = "output.current"
	LoadPercent       = "load.percent"
	BatteryVoltage    = "battery.voltage"
	BatteryCharge     = "battery.charge"
	// BatteryChargeEstimate is the charge estimate of the megatec Dialect
	// from the Q1 battery voltage (VLT-IS-006 TASK 03-00-00-003 step 8).
	// It is separate from battery.charge (QBV).
	BatteryChargeEstimate = "battery.charge_estimate"
	BatteryRuntime        = "battery.runtime"
	UPSTemperature        = "ups.temperature"
)

// Units of the names.
var Units = map[string]string{
	InputVoltage:          "V",
	InputFaultVoltage:     "V",
	InputFrequency:        "Hz",
	OutputVoltage:         "V",
	OutputFrequency:       "Hz",
	OutputCurrent:         "A",
	LoadPercent:           "%",
	BatteryVoltage:        "V",
	BatteryCharge:         "%",
	BatteryChargeEstimate: "%",
	BatteryRuntime:        "min",
	UPSTemperature:        "°C",
}

// Retention (step 4).
const (
	RawRetention    = 7 * 24 * time.Hour
	MinuteRetention = 400 * 24 * time.Hour
	JobPeriod       = time.Hour
)

// Point is one Measurement value.
type Point struct {
	Name   string  `json:"name"`
	Unit   string  `json:"unit"`
	Value  float64 `json:"value"`
	Source string  `json:"source"` // the command, for example "QGS"
	Demo   bool    `json:"demo,omitempty"`
}

func add[U dialect.Unit](out []Point, name, src string, q dialect.Quantity[U]) []Point {
	if !q.Valid || math.IsNaN(q.Value) || math.IsInf(q.Value, 0) {
		return out // step 2: no value, no row; never zero
	}
	return append(out, Point{Name: name, Unit: Units[name], Value: q.Value, Source: src})
}

// FromValue returns the Measurements of one parsed response (step 1
// table). Other commands give no Measurements.
func FromValue(cmd string, v any) []Point {
	var out []Point
	switch x := v.(type) {
	case dialect.Status: // Q1 and QS
		src := x.Command
		if src == "" {
			src = cmd
		}
		out = add(out, InputVoltage, src, x.InputVoltage)
		out = add(out, InputFaultVoltage, src, x.InputFaultVolt)
		out = add(out, InputFrequency, src, x.InputFrequency)
		out = add(out, OutputVoltage, src, x.OutputVoltage)
		out = add(out, LoadPercent, src, x.Load)
		// Q1: the battery value as received (VLT-IS-002 TASK 03-00-00-005
		// step 4f); it can be a cell voltage.
		out = add(out, BatteryVoltage, src, x.BatteryVoltage)
		out = add(out, UPSTemperature, src, x.Temperature)
	case dialect.StatusQGS:
		out = add(out, InputVoltage, "QGS", x.InputVoltage)
		out = add(out, InputFrequency, "QGS", x.InputFrequency)
		out = add(out, OutputVoltage, "QGS", x.OutputVoltage)
		out = add(out, OutputFrequency, "QGS", x.OutputFrequency)
		out = add(out, OutputCurrent, "QGS", x.OutputCurrent)
		out = add(out, LoadPercent, "QGS", x.Load)
		out = add(out, BatteryVoltage, "QGS", x.BatteryVoltage)
		out = add(out, UPSTemperature, "QGS", x.Temperature)
	case dialect.Battery:
		out = add(out, BatteryVoltage, "QBV", x.Voltage)
		out = add(out, BatteryCharge, "QBV", x.Charge)
		out = add(out, BatteryRuntime, "QBV", x.RuntimeEstimate)
	}
	return out
}

// Sample is the Measurements of one response with its time.
type Sample struct {
	Time   time.Time
	Points []Point
}

// ErrUnknownName is returned for a name that is not in the table.
var ErrUnknownName = errors.New("measure: unknown Measurement name")

// Writer writes Measurements to the Store database.
type Writer struct {
	DB *sql.DB
}

func ms(t time.Time) int64 { return t.UnixMilli() }

// WriteCycle writes all Measurements of one poll cycle in one transaction
// (step 3). If one row fails, no row is stored.
func (w *Writer) WriteCycle(ctx context.Context, samples []Sample) error {
	n := 0
	for _, s := range samples {
		for _, p := range s.Points {
			u, ok := Units[p.Name]
			if !ok || u != p.Unit {
				return fmt.Errorf("%w: %q %q", ErrUnknownName, p.Name, p.Unit)
			}
			if math.IsNaN(p.Value) || math.IsInf(p.Value, 0) {
				return fmt.Errorf("measure: %s: no value", p.Name)
			}
			n++
		}
	}
	if n == 0 {
		return nil
	}
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.PrepareContext(ctx, "INSERT INTO measurements (ts, name, value, unit, source, demo) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer st.Close()
	for _, s := range samples {
		for _, p := range s.Points {
			demo := 0
			if p.Demo {
				demo = 1
			}
			if _, err := st.ExecContext(ctx, ms(s.Time), p.Name, p.Value, p.Unit, p.Source, demo); err != nil {
				return fmt.Errorf("measure: write %s: %w", p.Name, err)
			}
		}
	}
	return tx.Commit()
}
