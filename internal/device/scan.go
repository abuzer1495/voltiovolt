// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"
	"errors"
	"fmt"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// Port scan (VLT-IS-006 TASK 03-00-00-004 step 3.3): device.scan lists
// the serial ports of the OS and sends only "Q1" CR to each port that the
// user marks. It sends nothing to a port that is not marked (SEC-099).

// Scan results of one port.
const (
	ScanResponse   = "response"
	ScanNoResponse = "no response"
	ScanBusy       = "busy"
	ScanError      = "error"
)

// ScanCommand is the only command that the scan sends.
const ScanCommand = "Q1"

// MaxScanPorts is the maximum number of marked ports in one scan. With
// the response timeout of 1500 ms, a scan of 8 ports ends before the IPC
// request timeout of 30 seconds.
const MaxScanPorts = 8

// ScanResult is the result of one marked port.
type ScanResult struct {
	Name   string `json:"name"`
	Result string `json:"result"`
	// ValidQ1 is true if the response is a valid megatec Q1 response.
	ValidQ1 bool `json:"valid_q1"`
}

// ErrScanPort is returned for a marked port that is not in the port list
// of the OS, or for a bad list of marked ports.
var ErrScanPort = errors.New("device: marked port is not permitted")

// Scanner lists and scans the serial ports.
type Scanner struct {
	List      func() ([]transport.SerialPortInfo, error)
	NewSerial func(name string, so transport.SerialOptions, opts transport.Options) transport.Transport
	Options   transport.Options
}

// RealScanner returns the Scanner of the computer.
func RealScanner() *Scanner {
	return &Scanner{
		List: transport.ListSerialDetailed,
		NewSerial: func(name string, so transport.SerialOptions, opts transport.Options) transport.Transport {
			return transport.NewSerial(name, so, opts)
		},
		Options: transport.Options{Timeout: ResponseTimeout},
	}
}

// Ports lists the serial ports of the OS. It opens no port.
func (s *Scanner) Ports(context.Context) ([]transport.SerialPortInfo, error) {
	l, err := s.List()
	if l == nil {
		l = []transport.SerialPortInfo{}
	}
	return l, err
}

// Scan sends "Q1" CR to each marked port, one port after the other. The
// marked ports must be in the port list of the OS. inUse is the port that
// the Poller uses now ("" for none): it is not opened, its result is
// "busy".
func (s *Scanner) Scan(ctx context.Context, marked []string, so transport.SerialOptions, inUse string) ([]ScanResult, error) {
	if len(marked) == 0 || len(marked) > MaxScanPorts {
		return nil, fmt.Errorf("%w: mark 1 to %d ports", ErrScanPort, MaxScanPorts)
	}
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, p := range list {
		known[p.Name] = true
	}
	seen := map[string]bool{}
	for _, m := range marked {
		if !known[m] {
			return nil, fmt.Errorf("%w: %q is not a serial port of the OS", ErrScanPort, m)
		}
		if seen[m] {
			return nil, fmt.Errorf("%w: %q is marked two times", ErrScanPort, m)
		}
		seen[m] = true
	}
	req, err := dialect.EncodeRead(ScanCommand)
	if err != nil {
		return nil, err
	}
	out := make([]ScanResult, 0, len(marked))
	for _, name := range marked {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		out = append(out, s.scanOne(ctx, name, req, so, inUse))
	}
	return out, nil
}

func (s *Scanner) scanOne(ctx context.Context, name string, req []byte, so transport.SerialOptions, inUse string) ScanResult {
	r := ScanResult{Name: name}
	if name == inUse {
		r.Result = ScanBusy
		return r
	}
	tr := s.NewSerial(name, so, s.Options)
	if err := tr.Open(ctx); err != nil {
		if errors.Is(err, transport.ErrPortBusy) {
			r.Result = ScanBusy
		} else {
			r.Result = ScanError
		}
		return r
	}
	defer tr.Close()
	resp, err := tr.Exchange(ctx, req)
	switch {
	case errors.Is(err, transport.ErrTimeout):
		r.Result = ScanNoResponse
	case err != nil && len(resp.Raw) == 0:
		r.Result = ScanError
	default:
		r.Result = ScanResponse
		_, perr := dialect.ParseQ1(resp.Raw)
		r.ValidQ1 = perr == nil
	}
	return r
}
