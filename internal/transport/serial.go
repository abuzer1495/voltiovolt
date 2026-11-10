// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.bug.st/serial"
)

// Serial line settings: 2400 bit/s, 8 data bits, no parity, 1 stop bit.
const SerialBaud = 2400

// SerialOptions are the modem control lines after open.
// Default: DTR on, RTS off.
type SerialOptions struct {
	DTR bool
	RTS bool
	// Warn gets a message if the port cannot set a modem control line
	// (for example a pty). The port stays open. Can be nil.
	Warn func(msg string)
}

// DefaultSerialOptions returns DTR on and RTS off.
func DefaultSerialOptions() SerialOptions { return SerialOptions{DTR: true, RTS: false} }

// ListSerial lists the serial ports. It opens no port.
func ListSerial() ([]string, error) { return serial.GetPortsList() }

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
		p, err := serial.Open(name, mode)
		if err != nil {
			return nil, err
		}
		if err := p.SetDTR(so.DTR); err != nil && so.Warn != nil {
			so.Warn(fmt.Sprintf("cannot set DTR=%v: %v", so.DTR, err))
		}
		if err := p.SetRTS(so.RTS); err != nil && so.Warn != nil {
			so.Warn(fmt.Sprintf("cannot set RTS=%v: %v", so.RTS, err))
		}
		return &serialPort{p: p}, nil
	}
	return NewStream(open, opts)
}
