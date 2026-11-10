// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// USB identity of the Reference UPS interface (VID 0665, PID 5161).
const (
	VendorID  uint16 = 0x0665
	ProductID uint16 = 0x5161
)

// ReportLengths are the report lengths of a HID device, in bytes, without
// the report ID byte. They come from the report descriptor (Linux) or from
// HidP_GetCaps (Windows). VLT does not use fixed lengths.
type ReportLengths struct {
	Input        int  `json:"input"`
	Output       int  `json:"output"`
	Feature      int  `json:"feature"`
	HasReportIDs bool `json:"has_report_ids"`
	OutputID     byte `json:"output_report_id"`
}

// HIDDevice is the data of one HID device.
type HIDDevice struct {
	Path             string        `json:"path"`
	VID              uint16        `json:"vid"`
	PID              uint16        `json:"pid"`
	Product          string        `json:"product"`
	Serial           string        `json:"-"` // personal data: tools must redact it
	ReportDescriptor []byte        `json:"-"`
	Lengths          ReportLengths `json:"report_lengths"`
}

// ReportDevice is an open HID device that sends and receives reports.
//
// WriteReport writes one output report. The first byte is the report ID
// (0 if the device has no report IDs). ReadReport reads one input report and
// returns its data without the report ID byte. On timeout, ReadReport
// returns 0 and a nil error.
//
// NOTE: The report type for requests (output report or feature report) is
// UNVERIFIED for the Reference UPS. This code uses output reports.
type ReportDevice interface {
	WriteReport(r []byte) (int, error)
	ReadReport(p []byte, timeout time.Duration) (int, error)
	Close() error
}

// ErrNoOutputReport is returned when the device has no output report.
var ErrNoOutputReport = errors.New("transport: HID device has no output report")

// hidPort is a Port over a ReportDevice.
type hidPort struct {
	dev ReportDevice
	l   ReportLengths
	in  []byte
}

// NewHIDPort returns a Port that splits each request into output reports
// and joins input reports into a byte stream.
func NewHIDPort(dev ReportDevice, l ReportLengths) (Port, error) {
	if l.Output <= 0 {
		return nil, ErrNoOutputReport
	}
	if l.Input <= 0 {
		return nil, errors.New("transport: HID device has no input report")
	}
	return &hidPort{dev: dev, l: l, in: make([]byte, l.Input+1)}, nil
}

// Write splits p into reports. Each report starts with the report ID byte
// (0 if the device has no report IDs). The remaining bytes are zero. If a
// report write is partial or fails, Write stops and does not write the
// remaining reports. It returns the number of request bytes in complete
// reports.
func (h *hidPort) Write(p []byte) (int, error) {
	done := 0
	for done < len(p) {
		n := min(h.l.Output, len(p)-done)
		r := make([]byte, 1+h.l.Output)
		if h.l.HasReportIDs {
			r[0] = h.l.OutputID
		}
		copy(r[1:], p[done:done+n])
		w, err := h.dev.WriteReport(r)
		if err != nil {
			return done, err
		}
		if w != len(r) {
			return done, fmt.Errorf("transport: HID report write %d of %d bytes", w, len(r))
		}
		done += n
	}
	return done, nil
}

func (h *hidPort) Read(p []byte, timeout time.Duration) (int, error) {
	n, err := h.dev.ReadReport(h.in, timeout)
	if err != nil || n == 0 {
		return 0, err
	}
	return copy(p, h.in[:n]), nil
}

func (h *hidPort) Close() error { return h.dev.Close() }

// HIDOpener opens a HID device and returns a Port.
type HIDOpener func(ctx context.Context, path string) (ReportDevice, HIDDevice, error)

// NewHID returns a Transport for the HID device at path. If path is empty,
// it uses the first device with VID 0665 and PID 5161.
func NewHID(path string, opts Options) (*Stream, *HIDDevice) {
	info := &HIDDevice{}
	open := func(ctx context.Context) (Port, error) {
		p := path
		if p == "" {
			devs, err := ListHID()
			if err != nil {
				return nil, err
			}
			if len(devs) == 0 {
				return nil, errors.New("transport: no HID device with VID 0665 and PID 5161")
			}
			p = devs[0].Path
		}
		dev, d, err := openHID(p)
		if err != nil {
			return nil, err
		}
		*info = d
		port, err := NewHIDPort(dev, d.Lengths)
		if err != nil {
			_ = dev.Close()
			return nil, err
		}
		return port, nil
	}
	return NewStream(open, opts), info
}
