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
	// LineUnchanged: the Transport does not set the line at open and after
	// open. This is the default.
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
// is nil. On Windows (serial_windows.go lines 399 to 410) the library sets
// DTR and RTS on when the value is nil: there "unchanged" is not possible
// (VLT-RPT-006 section 7), so on Windows VLT gives the lines at open when
// both are set. On Linux the library fails the open if the device has no
// modem lines (a pty, error "getting modem bits status"), so on Linux VLT
// sets a line after open and gives a warning if that fails.
func initialBits(goos string, so SerialOptions) *serial.ModemOutputBits {
	if goos != "windows" {
		return nil
	}
	if so.DTR == LineUnchanged || so.RTS == LineUnchanged || so.DTR == "" || so.RTS == "" {
		return nil
	}
	return &serial.ModemOutputBits{DTR: so.DTR == LineOn, RTS: so.RTS == LineOn}
}

func isBusy(err error) bool {
	var pe *serial.PortError
	return errors.As(err, &pe) && pe.Code() == serial.PortBusy
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
			if isBusy(err) {
				return nil, fmt.Errorf("%w: %s: %v", ErrPortBusy, name, err)
			}
			return nil, err
		}
		// A line with "unchanged" is never set. With one line "unchanged"
		// and one line set, only the set line changes after open.
		if mode.InitialStatusBits == nil {
			if so.DTR == LineOn || so.DTR == LineOff {
				if err := p.SetDTR(so.DTR == LineOn); err != nil && so.Warn != nil {
					so.Warn(fmt.Sprintf("cannot set DTR %s: %v", so.DTR, err))
				}
			}
			if so.RTS == LineOn || so.RTS == LineOff {
				if err := p.SetRTS(so.RTS == LineOn); err != nil && so.Warn != nil {
					so.Warn(fmt.Sprintf("cannot set RTS %s: %v", so.RTS, err))
				}
			}
		}
		return &serialPort{p: p}, nil
	}
	return NewStream(open, opts)
}
