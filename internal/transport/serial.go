// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// Serial line settings: 2400 bit/s, 8 data bits, no parity, 1 stop bit.
const SerialBaud = 2400

// Line is the wanted state of one modem control line (DTR or RTS).
type Line string

// Line values (VLT-IS-006 TASK 03-00-00-004 step 1).
const (
	// LineUnchanged is the default. On Linux the Transport does not set
	// the line at open or after open. On Windows the serial library turns
	// DTR and RTS on when InitialStatusBits is nil (VLT-RPT-006), so
	// "unchanged" is applied as off at open: the lines are not asserted.
	LineUnchanged Line = "unchanged"
	LineOn        Line = "on"
	LineOff       Line = "off"
)

// ParseLine returns the Line for "unchanged", "on" or "off".
func ParseLine(s string) (Line, error) {
	switch l := Line(s); l {
	case LineUnchanged, LineOn, LineOff:
		return l, nil
	}
	return "", fmt.Errorf("transport: line value %q is not unchanged, on or off", s)
}

// SerialOptions are the modem control lines.
type SerialOptions struct {
	DTR Line
	RTS Line
	// Warn gets a message if the port cannot set a modem control line
	// (for example a pty). The port stays open. Can be nil.
	Warn func(msg string)
}

// DefaultSerialOptions returns DTR and RTS "unchanged".
func DefaultSerialOptions() SerialOptions {
	return SerialOptions{DTR: LineUnchanged, RTS: LineUnchanged}
}

// ErrPortBusy is returned when the serial port cannot open because another
// program uses it (VLT-IS-006 TASK 03-00-00-004 step 2).
var ErrPortBusy = errors.New("transport: port busy")

// SerialPortInfo is one serial port of the OS. It never contains the USB
// serial number (personal data).
type SerialPortInfo struct {
	Name    string `json:"name"`
	IsUSB   bool   `json:"is_usb"`
	VID     string `json:"vid,omitempty"`
	PID     string `json:"pid,omitempty"`
	Product string `json:"product,omitempty"`
	// ProbableUPS and Bridge are set by device.MarkPorts for a known
	// USB-to-serial bridge chip (VLT-IS-012 TASK 03-00-00-003).
	ProbableUPS bool   `json:"probable_ups,omitempty"`
	Bridge      string `json:"bridge,omitempty"`
}

// ListSerial lists the serial ports. It opens no port.
func ListSerial() ([]string, error) { return serial.GetPortsList() }

// ListSerialDetailed lists the serial ports with the USB data that the OS
// gives. It opens no port.
func ListSerialDetailed() ([]SerialPortInfo, error) {
	ps, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	out := make([]SerialPortInfo, 0, len(ps))
	for _, p := range ps {
		out = append(out, SerialPortInfo{Name: p.Name, IsUSB: p.IsUSB, VID: p.VID, PID: p.PID, Product: p.Product})
	}
	return out, nil
}

// initialBits gives the lines to set at open. go.bug.st/serial v1.8.0
// (serial_unix.go lines 249 to 268) does not touch the lines when the value
// is nil. On Windows (serial_windows.go lines 399 to 410, DCB fDtrControl
// and fRtsControl) a nil value turns DTR and RTS on, so VLT always passes
// InitialStatusBits on Windows. LineOn is on; LineOff and LineUnchanged
// (and the empty value) are off, so the default "DTR and RTS unchanged"
// does not assert the lines (VLT-RPT-006). On Linux the library fails the
// open if the device has no modem lines (a pty, error "getting modem bits
// status"), so on Linux VLT leaves InitialStatusBits nil and sets an
// explicit on or off line after open, with a warning if that fails.
func initialBits(goos string, so SerialOptions) *serial.ModemOutputBits {
	if goos != "windows" {
		return nil
	}
	return &serial.ModemOutputBits{DTR: lineAsserted(so.DTR), RTS: lineAsserted(so.RTS)}
}

// lineAsserted is the level written on Windows at open. Only LineOn is
// asserted; "unchanged" and "off" stay off.
func lineAsserted(l Line) bool { return l == LineOn }

// setModemLine is SetDTR or SetRTS after open. A failure is a warning; the
// port stays open.
func setModemLine(set func(bool) error, name string, on bool, so SerialOptions) {
	if err := set(on); err != nil && so.Warn != nil {
		want := LineOff
		if on {
			want = LineOn
		}
		so.Warn(fmt.Sprintf("cannot set %s %s: %v", name, want, err))
	}
}

// applyLinesAfterOpen sets DTR and RTS immediately after open. On Windows
// it always applies the resolved values (unchanged -> off), so the lines
// cannot stay on if the library ignored InitialStatusBits. On Linux it
// sets only an explicit on or off line (unchanged is left to the kernel).
func applyLinesAfterOpen(p interface {
	SetDTR(bool) error
	SetRTS(bool) error
}, goos string, so SerialOptions) {
	if goos == "windows" {
		setModemLine(p.SetDTR, "DTR", lineAsserted(so.DTR), so)
		setModemLine(p.SetRTS, "RTS", lineAsserted(so.RTS), so)
		return
	}
	if so.DTR == LineOn || so.DTR == LineOff {
		setModemLine(p.SetDTR, "DTR", so.DTR == LineOn, so)
	}
	if so.RTS == LineOn || so.RTS == LineOff {
		setModemLine(p.SetRTS, "RTS", so.RTS == LineOn, so)
	}
}

func isBusy(err error) bool {
	var pe *serial.PortError
	return errors.As(err, &pe) && pe.Code() == serial.PortBusy
}

// wrapSerialOpenError maps a serial open error to ErrPortBusy or, for a
// Linux USB serial port with permission denied, to ErrPermissionDenied.
func wrapSerialOpenError(name string, err error) error {
	if isBusy(err) {
		return fmt.Errorf("%w: %s: %v", ErrPortBusy, name, err)
	}
	if IsPermission(err) && IsLinuxUSBSerial(name) {
		return fmt.Errorf("%w: %s: %v", ErrPermissionDenied, name, err)
	}
	return err
}

type serialPort struct{ p serial.Port }

func (s *serialPort) Write(b []byte) (int, error) { return s.p.Write(b) }

func (s *serialPort) Read(b []byte, timeout time.Duration) (int, error) {
	if timeout < time.Millisecond {
		timeout = time.Millisecond
	}
	if err := s.p.SetReadTimeout(timeout); err != nil {
		return 0, err
	}
	return s.p.Read(b)
}

func (s *serialPort) Close() error { return s.p.Close() }

// NewSerial returns a Transport for the serial port name.
func NewSerial(name string, so SerialOptions, opts Options) *Stream {
	open := func(ctx context.Context) (Port, error) {
		if name == "" {
			return nil, errors.New("transport: no serial port name")
		}
		mode := &serial.Mode{
			BaudRate: SerialBaud,
			DataBits: 8,
			Parity:   serial.NoParity,
			StopBits: serial.OneStopBit,
		}
		mode.InitialStatusBits = initialBits(runtime.GOOS, so)
		p, err := serial.Open(name, mode)
		if err != nil {
			return nil, wrapSerialOpenError(name, err)
		}
		applyLinesAfterOpen(p, runtime.GOOS, so)
		return &serialPort{p: p}, nil
	}
	return NewStream(open, opts)
}
