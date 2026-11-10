// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

// Unit markers. Each Measurement value carries its unit in its type.
type (
	unitVolt    struct{}
	unitHertz   struct{}
	unitAmpere  struct{}
	unitPercent struct{}
	unitCelsius struct{}
	unitMinute  struct{}
)

func (unitVolt) Symbol() string    { return "V" }
func (unitHertz) Symbol() string   { return "Hz" }
func (unitAmpere) Symbol() string  { return "A" }
func (unitPercent) Symbol() string { return "%" }
func (unitCelsius) Symbol() string { return "°C" }
func (unitMinute) Symbol() string  { return "min" }

// Unit is the set of permitted units.
type Unit interface {
	unitVolt | unitHertz | unitAmpere | unitPercent | unitCelsius | unitMinute
	Symbol() string
}

// Quantity is one numeric field of a response.
//
// If Valid is false, the UPS sent a field with only "-", "@" or "#"
// characters. The field has no value. It is not zero.
type Quantity[U Unit] struct {
	Value float64
	Valid bool
	Raw   string
}

// Unit returns the unit symbol of the quantity.
func (q Quantity[U]) Unit() string {
	var u U
	return u.Symbol()
}

// Unit types.
type (
	Volts   = Quantity[unitVolt]
	Hertz   = Quantity[unitHertz]
	Amperes = Quantity[unitAmpere]
	Percent = Quantity[unitPercent]
	Celsius = Quantity[unitCelsius]
	Minutes = Quantity[unitMinute]
)
