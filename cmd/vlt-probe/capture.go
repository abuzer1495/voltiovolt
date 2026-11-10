// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// toolVersion is the version of vlt-probe.
const toolVersion = "0.1.0-dev"

// Device is the device data in the Capture.
type Device struct {
	Path             string                  `json:"path,omitempty"`
	VID              string                  `json:"vid,omitempty"`
	PID              string                  `json:"pid,omitempty"`
	Product          string                  `json:"product,omitempty"`
	Serial           string                  `json:"usb_serial,omitempty"`
	ReportDescriptor string                  `json:"report_descriptor_hex"`
	DescriptorNote   string                  `json:"report_descriptor_note,omitempty"`
	ReportLengths    transport.ReportLengths `json:"report_lengths"`
}

// Capture is the content of a Capture file. It contains no host name and
// no user name.
type Capture struct {
	Tool      string   `json:"tool"`
	Version   string   `json:"tool_version"`
	OS        string   `json:"os"`
	Transport string   `json:"transport"`
	Mode      string   `json:"mode"`
	SerialCfg string   `json:"serial_settings,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Started   string   `json:"started_utc"`
	Device    *Device  `json:"device,omitempty"`
	Entries   []Entry  `json:"commands"`
}

func deviceData(d *transport.HIDDevice) *Device {
	if d == nil || d.Path == "" {
		return nil
	}
	out := &Device{
		Path:             d.Path,
		VID:              fmt.Sprintf("%04X", d.VID),
		PID:              fmt.Sprintf("%04X", d.PID),
		Product:          d.Product,
		ReportDescriptor: hex.EncodeToString(d.ReportDescriptor),
		ReportLengths:    d.Lengths,
	}
	if d.Serial != "" {
		out.Serial = redacted(len(d.Serial))
	}
	if runtime.GOOS == "windows" {
		out.DescriptorNote = "Windows hid.dll gives no raw report descriptor. Report lengths are from HidP_GetCaps."
	}
	return out
}

func captureName(t time.Time) string {
	return "vlt-capture-" + t.UTC().Format("20060102T150405Z") + ".vltcap.json"
}

func newCapture(tr, mode string, start time.Time) *Capture {
	return &Capture{
		Tool:      "vlt-probe",
		Version:   toolVersion,
		OS:        runtime.GOOS + "/" + runtime.GOARCH,
		Transport: tr,
		Mode:      mode,
		Started:   start.UTC().Format(time.RFC3339),
	}
}

func writeCapture(dir string, c *Capture, start time.Time) (string, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, captureName(start))
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func summary(w io.Writer, c *Capture, path string) {
	counts := map[string]map[string]int{}
	order := []string{}
	for _, e := range c.Entries {
		if counts[e.Command] == nil {
			counts[e.Command] = map[string]int{}
			order = append(order, e.Command)
		}
		counts[e.Command][e.Result]++
	}
	fmt.Fprintf(w, "Summary: %d commands sent in %s mode over %s.\n", len(c.Entries), c.Mode, c.Transport)
	for _, cmd := range order {
		var parts []string
		for r, n := range counts[cmd] {
			parts = append(parts, fmt.Sprintf("%s %d", r, n))
		}
		sort.Strings(parts)
		fmt.Fprintf(w, "  %-6s %v\n", cmd, parts)
	}
	if path != "" {
		fmt.Fprintf(w, "Capture file: %s\n", path)
		fmt.Fprintln(w, "Check the file before you send it to the Owner channel.")
	}
}
