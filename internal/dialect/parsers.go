// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

// StatusBits keeps status bits as raw bits. Raw[0] is the highest bit.
type StatusBits struct {
	Raw string
}

// Bit returns bit n. Bit 0 is the last character of Raw.
func (s StatusBits) Bit(n int) bool {
	i := len(s.Raw) - 1 - n
	if n < 0 || i < 0 {
		return false
	}
	return s.Raw[i] == '1'
}

// Q1Flags are the named bits of the Q1 and QS status byte, from the public
// Megatec document. Bit 7 is the first character.
type Q1Flags struct {
	UtilityFail     bool // bit 7
	BatteryLow      bool // bit 6
	BypassBoostBuck bool // bit 5
	UPSFailed       bool // bit 4
	StandbyType     bool // bit 3 (0 = on-line type)
	TestInProgress  bool // bit 2
	ShutdownActive  bool // bit 1
	BeeperOn        bool // bit 0
}

func q1Flags(b StatusBits) Q1Flags {
	return Q1Flags{
		UtilityFail:     b.Bit(7),
		BatteryLow:      b.Bit(6),
		BypassBoostBuck: b.Bit(5),
		UPSFailed:       b.Bit(4),
		StandbyType:     b.Bit(3),
		TestInProgress:  b.Bit(2),
		ShutdownActive:  b.Bit(1),
		BeeperOn:        b.Bit(0),
	}
}

// Status is the response of Q1 or QS.
//
// The battery voltage is kept as received. The parser does not change a
// per-cell value to a total value. BatteryForm is "S.SS" or "SS.S".
type Status struct {
	Command        string
	InputVoltage   Volts
	InputFaultVolt Volts
	OutputVoltage  Volts
	Load           Percent
	InputFrequency Hertz
	BatteryVoltage Volts
	BatteryForm    string
	Temperature    Celsius
	Bits           StatusBits
	Flags          Q1Flags
}

var (
	specVolt5   = spec("ddd.d")
	specPct3    = spec("ddd")
	specHz4     = spec("dd.d")
	specBattQ1  = spec("d.dd", "dd.d")
	specTemp4   = spec("dd.d")
	specAmp5    = spec("ddd.d")
	specBattQBV = spec("ddd.d")
	specCount2  = spec("dd")
	specMin3    = spec("ddd")
	specBattR   = spec("ddd.d", "dd.dd")
)

func parseStatus(cmd string, resp []byte) (Status, error) {
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return Status{}, err
	}
	f, err := splitFields(cmd, body, 8)
	if err != nil {
		return Status{}, err
	}
	s := Status{Command: cmd}
	if s.InputVoltage, err = quantity[unitVolt](cmd, 1, f[0], specVolt5); err != nil {
		return Status{}, err
	}
	if s.InputFaultVolt, err = quantity[unitVolt](cmd, 2, f[1], specVolt5); err != nil {
		return Status{}, err
	}
	if s.OutputVoltage, err = quantity[unitVolt](cmd, 3, f[2], specVolt5); err != nil {
		return Status{}, err
	}
	if s.Load, err = quantity[unitPercent](cmd, 4, f[3], specPct3); err != nil {
		return Status{}, err
	}
	if s.InputFrequency, err = quantity[unitHertz](cmd, 5, f[4], specHz4); err != nil {
		return Status{}, err
	}
	if s.BatteryVoltage, err = quantity[unitVolt](cmd, 6, f[5], specBattQ1); err != nil {
		return Status{}, err
	}
	switch {
	case matches(f[5], "d.dd"):
		s.BatteryForm = "S.SS"
	case matches(f[5], "dd.d"):
		s.BatteryForm = "SS.S"
	}
	if s.Temperature, err = quantity[unitCelsius](cmd, 7, f[6], specTemp4); err != nil {
		return Status{}, err
	}
	raw, err := bitsField(cmd, 8, f[7], 8)
	if err != nil {
		return Status{}, err
	}
	s.Bits = StatusBits{Raw: raw}
	s.Flags = q1Flags(s.Bits)
	return s, nil
}

// ParseQ1 parses the response of Q1.
func ParseQ1(resp []byte) (Status, error) { return parseStatus("Q1", resp) }

// ParseQS parses the response of QS.
func ParseQS(resp []byte) (Status, error) { return parseStatus("QS", resp) }

// StatusQGS is the response of QGS. The meaning of the fields comes from a
// family sample (UNVERIFIED for the Reference UPS). The status bits stay raw.
// Unknown fields are kept as text only.
type StatusQGS struct {
	InputVoltage    Volts
	InputFrequency  Hertz
	OutputVoltage   Volts
	OutputFrequency Hertz
	OutputCurrent   Amperes
	Load            Percent
	Unknown7        string
	Unknown8        string
	BatteryVoltage  Volts
	Unknown10       string
	Temperature     Celsius
	Bits            StatusBits
}

// ParseQGS parses the response of QGS.
func ParseQGS(resp []byte) (StatusQGS, error) {
	const cmd = "QGS"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return StatusQGS{}, err
	}
	f, err := splitFields(cmd, body, 12)
	if err != nil {
		return StatusQGS{}, err
	}
	var s StatusQGS
	if s.InputVoltage, err = quantity[unitVolt](cmd, 1, f[0], specVolt5); err != nil {
		return StatusQGS{}, err
	}
	if s.InputFrequency, err = quantity[unitHertz](cmd, 2, f[1], specHz4); err != nil {
		return StatusQGS{}, err
	}
	if s.OutputVoltage, err = quantity[unitVolt](cmd, 3, f[2], specVolt5); err != nil {
		return StatusQGS{}, err
	}
	if s.OutputFrequency, err = quantity[unitHertz](cmd, 4, f[3], specHz4); err != nil {
		return StatusQGS{}, err
	}
	if s.OutputCurrent, err = quantity[unitAmpere](cmd, 5, f[4], specAmp5); err != nil {
		return StatusQGS{}, err
	}
	if s.Load, err = quantity[unitPercent](cmd, 6, f[5], specPct3); err != nil {
		return StatusQGS{}, err
	}
	for _, u := range []struct {
		idx int
		dst *string
	}{{7, &s.Unknown7}, {8, &s.Unknown8}, {10, &s.Unknown10}} {
		if _, err := specVolt5.check(cmd, u.idx, f[u.idx-1]); err != nil {
			return StatusQGS{}, err
		}
		*u.dst = f[u.idx-1]
	}
	if s.BatteryVoltage, err = quantity[unitVolt](cmd, 9, f[8], specVolt5); err != nil {
		return StatusQGS{}, err
	}
	if s.Temperature, err = quantity[unitCelsius](cmd, 11, f[10], specVolt5); err != nil {
		return StatusQGS{}, err
	}
	raw, err := bitsField(cmd, 12, f[11], 12)
	if err != nil {
		return StatusQGS{}, err
	}
	s.Bits = StatusBits{Raw: raw}
	return s, nil
}

// Mode is the response of QMOD: one mode letter, kept raw.
type Mode struct {
	Code byte
}

// ParseQMOD parses the response of QMOD.
func ParseQMOD(resp []byte) (Mode, error) {
	const cmd = "QMOD"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return Mode{}, err
	}
	if len(body) != 1 {
		return Mode{}, frameErr(cmd, "length %d, expected 1", len(body))
	}
	c := body[0]
	if c < 'A' || c > 'Z' {
		return Mode{}, fieldErr(cmd, 1, "mode %q is not an upper-case letter", c)
	}
	return Mode{Code: c}, nil
}

// Battery is the response of QBV. Field meaning from a family sample.
type Battery struct {
	Voltage         Volts
	SeriesCount     int
	ParallelCount   int
	CountsValid     [2]bool
	Charge          Percent
	RuntimeEstimate Minutes
}

// ParseQBV parses the response of QBV.
func ParseQBV(resp []byte) (Battery, error) {
	const cmd = "QBV"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return Battery{}, err
	}
	f, err := splitFields(cmd, body, 5)
	if err != nil {
		return Battery{}, err
	}
	var b Battery
	if b.Voltage, err = quantity[unitVolt](cmd, 1, f[0], specBattQBV); err != nil {
		return Battery{}, err
	}
	for i, dst := range []*int{&b.SeriesCount, &b.ParallelCount} {
		valid, err := specCount2.check(cmd, i+2, f[i+1])
		if err != nil {
			return Battery{}, err
		}
		b.CountsValid[i] = valid
		if valid {
			n, _ := strconv.Atoi(f[i+1])
			*dst = n
		}
	}
	if b.Charge, err = quantity[unitPercent](cmd, 4, f[3], specPct3); err != nil {
		return Battery{}, err
	}
	if b.RuntimeEstimate, err = quantity[unitMinute](cmd, 5, f[4], specMin3); err != nil {
		return Battery{}, err
	}
	return b, nil
}

// QWSBitCount is the number of QWS bits in the family sample.
const QWSBitCount = 64

// Warnings is the response of QWS. The bits stay raw (meaning UNVERIFIED).
type Warnings struct {
	Bits StatusBits
}

// ParseQWS parses the response of QWS.
func ParseQWS(resp []byte) (Warnings, error) {
	const cmd = "QWS"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return Warnings{}, err
	}
	raw, err := bitsField(cmd, 1, body, QWSBitCount)
	if err != nil {
		return Warnings{}, err
	}
	return Warnings{Bits: StatusBits{Raw: raw}}, nil
}

// Flags is the response of QFLAG: letters after "E" are enabled, letters
// after "D" are disabled. Letter meanings are UNVERIFIED.
type Flags struct {
	Enabled  string
	Disabled string
}

// ParseQFLAG parses the response of QFLAG.
func ParseQFLAG(resp []byte) (Flags, error) {
	const cmd = "QFLAG"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return Flags{}, err
	}
	if len(body) == 0 || body[0] != 'E' {
		return Flags{}, fieldErr(cmd, 1, "no \"E\" at the start")
	}
	d := strings.IndexByte(body, 'D')
	if d < 0 {
		return Flags{}, fieldErr(cmd, 2, "no \"D\" part")
	}
	en, dis := body[1:d], body[d+1:]
	for i, part := range []string{en, dis} {
		for j := 0; j < len(part); j++ {
			if part[j] < 'a' || part[j] > 'z' {
				return Flags{}, fieldErr(cmd, i+1, "flag %q is not a lower-case letter", part[j])
			}
		}
	}
	return Flags{Enabled: en, Disabled: dis}, nil
}

// Rating is the response of QRI or F.
type Rating struct {
	Command        string
	Voltage        Volts
	Current        Amperes
	BatteryVoltage Volts
	Frequency      Hertz
}

func parseRating(cmd string, resp []byte, start byte) (Rating, error) {
	body, err := frame(cmd, resp, start)
	if err != nil {
		return Rating{}, err
	}
	f, err := splitFields(cmd, body, 4)
	if err != nil {
		return Rating{}, err
	}
	r := Rating{Command: cmd}
	if r.Voltage, err = quantity[unitVolt](cmd, 1, f[0], specVolt5); err != nil {
		return Rating{}, err
	}
	if r.Current, err = quantity[unitAmpere](cmd, 2, f[1], specPct3); err != nil {
		return Rating{}, err
	}
	if r.BatteryVoltage, err = quantity[unitVolt](cmd, 3, f[2], specBattR); err != nil {
		return Rating{}, err
	}
	if r.Frequency, err = quantity[unitHertz](cmd, 4, f[3], specHz4); err != nil {
		return Rating{}, err
	}
	return r, nil
}

// ParseQRI parses the response of QRI.
func ParseQRI(resp []byte) (Rating, error) { return parseRating("QRI", resp, '(') }

// ParseF parses the response of F.
func ParseF(resp []byte) (Rating, error) { return parseRating("F", resp, '#') }

// Information is the response of I. Spaces at the end of each field are
// removed. An empty field has no value.
type Information struct {
	Company string
	Model   string
	Version string
}

// Field widths of the I response, from the public Megatec document.
const (
	iCompanyWidth = 15
	iModelWidth   = 10
	iVersionWidth = 10
	iBodyLen      = iCompanyWidth + 1 + iModelWidth + 1 + iVersionWidth
)

// ParseI parses the response of I.
func ParseI(resp []byte) (Information, error) {
	const cmd = "I"
	body, err := frame(cmd, resp, '#')
	if err != nil {
		return Information{}, err
	}
	if len(body) != iBodyLen {
		return Information{}, frameErr(cmd, "length %d, expected %d", len(body), iBodyLen)
	}
	if body[iCompanyWidth] != ' ' {
		return Information{}, fieldErr(cmd, 2, "no space before the field")
	}
	if body[iCompanyWidth+1+iModelWidth] != ' ' {
		return Information{}, fieldErr(cmd, 3, "no space before the field")
	}
	c := body[:iCompanyWidth]
	m := body[iCompanyWidth+1 : iCompanyWidth+1+iModelWidth]
	v := body[iCompanyWidth+1+iModelWidth+1:]
	return Information{
		Company: strings.TrimSpace(c),
		Model:   strings.TrimSpace(m),
		Version: strings.TrimSpace(v),
	}, nil
}

// MaxQIDLen is the maximum length of the QID data that VLT accepts.
const MaxQIDLen = 32

// SerialNumber is the response of QID. It can be personal data. Tools must
// redact it before they record it.
type SerialNumber struct {
	Value string
}

// ParseQID parses the response of QID.
func ParseQID(resp []byte) (SerialNumber, error) {
	const cmd = "QID"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return SerialNumber{}, err
	}
	if len(body) == 0 || len(body) > MaxQIDLen {
		return SerialNumber{}, fieldErr(cmd, 1, "length %d is not 1 to %d", len(body), MaxQIDLen)
	}
	if strings.IndexByte(body, ' ') >= 0 {
		return SerialNumber{}, fieldErr(cmd, 1, "space in the field")
	}
	return SerialNumber{Value: body}, nil
}

// Firmware is the response of QVFW.
type Firmware struct {
	Version string
}

// ParseQVFW parses the response of QVFW.
func ParseQVFW(resp []byte) (Firmware, error) {
	const cmd = "QVFW"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return Firmware{}, err
	}
	const prefix = "VERFW:"
	if !strings.HasPrefix(body, prefix) {
		return Firmware{}, fieldErr(cmd, 1, "no %q prefix", prefix)
	}
	v := body[len(prefix):]
	if v == "" || strings.IndexByte(v, ' ') >= 0 {
		return Firmware{}, fieldErr(cmd, 1, "version %q is empty or has a space", v)
	}
	return Firmware{Version: v}, nil
}

// ModelData is the response of QMD. The field meaning comes from a family
// sample and is UNVERIFIED. Leading "#" characters are padding.
type ModelData struct {
	Model          string
	ModelValid     bool
	RatedVA        int
	RatedVAValid   bool
	PowerFactor    Percent
	Phases         string
	NominalInput   Volts
	NominalOutput  Volts
	BatteryCount   int
	BatteryCountOK bool
	BatteryVoltage Volts
}

func padded(cmd string, idx int, f string, width int, digits bool) (string, bool, error) {
	if len(f) != width {
		return "", false, fieldErr(cmd, idx, "width %d, expected %d", len(f), width)
	}
	v := strings.TrimLeft(f, "#")
	if v == "" {
		return "", false, nil
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if digits && (c < '0' || c > '9') {
			return "", false, fieldErr(cmd, idx, "value %q is not a number", f)
		}
		if c == '#' {
			return "", false, fieldErr(cmd, idx, "padding %q is not at the start", f)
		}
	}
	return v, true, nil
}

// ParseQMD parses the response of QMD.
func ParseQMD(resp []byte) (ModelData, error) {
	const cmd = "QMD"
	body, err := frame(cmd, resp, '(')
	if err != nil {
		return ModelData{}, err
	}
	f, err := splitFields(cmd, body, 8)
	if err != nil {
		return ModelData{}, err
	}
	var m ModelData
	if m.Model, m.ModelValid, err = padded(cmd, 1, f[0], 15, false); err != nil {
		return ModelData{}, err
	}
	va, ok, err := padded(cmd, 2, f[1], 7, true)
	if err != nil {
		return ModelData{}, err
	}
	if ok {
		m.RatedVA, _ = strconv.Atoi(va)
		m.RatedVAValid = true
	}
	if m.PowerFactor, err = quantity[unitPercent](cmd, 3, f[2], spec("dd")); err != nil {
		return ModelData{}, err
	}
	if _, err := spec("d/d").check(cmd, 4, f[3]); err != nil {
		return ModelData{}, err
	}
	m.Phases = f[3]
	if m.NominalInput, err = quantity[unitVolt](cmd, 5, f[4], spec("ddd")); err != nil {
		return ModelData{}, err
	}
	if m.NominalOutput, err = quantity[unitVolt](cmd, 6, f[5], spec("ddd")); err != nil {
		return ModelData{}, err
	}
	valid, err := specCount2.check(cmd, 7, f[6])
	if err != nil {
		return ModelData{}, err
	}
	if valid {
		m.BatteryCount, _ = strconv.Atoi(f[6])
		m.BatteryCountOK = true
	}
	if m.BatteryVoltage, err = quantity[unitVolt](cmd, 8, f[7], spec("dd.d")); err != nil {
		return ModelData{}, err
	}
	return m, nil
}

// Parse parses the response of a READ command in the registry.
func Parse(command string, resp []byte) (any, error) {
	e, ok := LookupRead(command)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotRead, command)
	}
	switch e.Response {
	case RespQ1:
		return ParseQ1(resp)
	case RespQS:
		return ParseQS(resp)
	case RespQGS:
		return ParseQGS(resp)
	case RespQMOD:
		return ParseQMOD(resp)
	case RespQBV:
		return ParseQBV(resp)
	case RespQWS:
		return ParseQWS(resp)
	case RespQFLAG:
		return ParseQFLAG(resp)
	case RespQRI:
		return ParseQRI(resp)
	case RespF:
		return ParseF(resp)
	case RespI:
		return ParseI(resp)
	case RespQID:
		return ParseQID(resp)
	case RespQVFW:
		return ParseQVFW(resp)
	case RespQMD:
		return ParseQMD(resp)
	}
	return nil, fmt.Errorf("dialect: no parser for %q", command)
}
