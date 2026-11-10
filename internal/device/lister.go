// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"

	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// OSFuncs are the OS functions of the lister. Tests replace them with
// fakes, so that no test opens a real port.
type OSFuncs struct {
	ListHID   func() ([]transport.HIDDevice, error)
	NewHID    func(path string, opts transport.Options) transport.Transport
	NewSerial func(name string, so transport.SerialOptions, opts transport.Options) transport.Transport
}

// RealOS returns the OS functions of the computer.
func RealOS() OSFuncs {
	return OSFuncs{
		ListHID: transport.ListHID,
		NewHID: func(path string, opts transport.Options) transport.Transport {
			s, _ := transport.NewHID(path, opts)
			return s
		},
		NewSerial: func(name string, so transport.SerialOptions, opts transport.Options) transport.Transport {
			return transport.NewSerial(name, so, opts)
		},
	}
}

// OSLister returns the candidates of the computer.
func OSLister(opts transport.Options) Lister { return ListerFor(RealOS(), opts) }

// ListerFor returns the candidates for a Config (VLT-IS-006 TASK
// 03-00-00-004 step 3, OD-27, SEC-099):
//
//   - "not_configured": no candidate. vltd sends no byte.
//   - "hid": the HID devices with VID 0665 and PID 5161.
//   - "serial": only the serial port in device.serial_port. If it is
//     empty, no candidate.
//   - "auto": the HID devices with VID 0665 and PID 5161, then only the
//     serial ports in device.serial.allowed_ports (default empty).
//
// The lister never lists the serial ports of the OS for a candidate. A
// serial port gets bytes only if the user selected or allowed it.
func ListerFor(os OSFuncs, opts transport.Options) Lister {
	return func(ctx context.Context, c Config) ([]Candidate, error) {
		var out []Candidate
		if c.Transport == TransportAuto || c.Transport == TransportHID {
			devs, err := os.ListHID()
			if err == nil {
				for _, d := range devs {
					if d.VID != transport.VendorID || d.PID != transport.ProductID {
						continue
					}
					path := d.Path
					out = append(out, Candidate{Transport: TransportHID, Port: path, New: func() transport.Transport {
						return os.NewHID(path, opts)
					}})
				}
			}
		}
		var ports []string
		switch c.Transport {
		case TransportSerial:
			if c.SerialPort != "" {
				ports = []string{c.SerialPort}
			}
		case TransportAuto:
			ports = c.AllowedPorts
		}
		so := transport.SerialOptions{DTR: c.DTR, RTS: c.RTS}
		for _, name := range ports {
			name := name
			out = append(out, Candidate{Transport: TransportSerial, Port: name, New: func() transport.Transport {
				return os.NewSerial(name, so, opts)
			}})
		}
		return out, nil
	}
}
