// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"

	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// OSLister returns the candidates of the computer: with "auto", first the
// HID devices with VID 0665 and PID 5161, then the serial ports (step 2).
// If the Setting device.serial_port has a name, only that serial port.
func OSLister(opts transport.Options) Lister {
	return func(ctx context.Context, c Config) ([]Candidate, error) {
		var out []Candidate
		if c.Transport == TransportAuto || c.Transport == TransportHID {
			devs, err := transport.ListHID()
			if err == nil {
				for _, d := range devs {
					if d.VID != transport.VendorID || d.PID != transport.ProductID {
						continue
					}
					path := d.Path
					out = append(out, Candidate{Transport: TransportHID, Port: path, New: func() transport.Transport {
						s, _ := transport.NewHID(path, opts)
						return s
					}})
				}
			}
		}
		if c.Transport == TransportAuto || c.Transport == TransportSerial {
			var ports []string
			if c.SerialPort != "" {
				ports = []string{c.SerialPort}
			} else if ps, err := transport.ListSerial(); err == nil {
				ports = ps
			}
			so := transport.SerialOptions{DTR: c.DTR, RTS: c.RTS}
			for _, name := range ports {
				name := name
				out = append(out, Candidate{Transport: TransportSerial, Port: name, New: func() transport.Transport {
					return transport.NewSerial(name, so, opts)
				}})
			}
		}
		return out, nil
	}
}
