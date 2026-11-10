// SPDX-License-Identifier: AGPL-3.0-or-later

// Command vlt-probe is a READ-only capture tool for the VLT project.
//
// WARNING: vlt-probe connects to the UPS. It sends only the commands in the
// Probe List and the Watch List. These lists are constants in the code.
//
// Usage:
//
//	vlt-probe version
//	vlt-probe scan
//	vlt-probe probe [--transport hid|serial] [--port NAME] [--dtr unchanged|on|off] [--rts unchanged|on|off]
//	vlt-probe watch [--transport hid|serial] [--port NAME] [--minutes 1..30] [--dtr unchanged|on|off] [--rts unchanged|on|off]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/transport"
)

type config struct {
	mode      string
	transport string
	port      string
	minutes   int
	dtr       string
	rts       string
}

const usage = `Usage:
  vlt-probe version
  vlt-probe scan
  vlt-probe probe [--transport hid|serial] [--port NAME] [--dtr unchanged|on|off] [--rts unchanged|on|off]
  vlt-probe watch [--transport hid|serial] [--port NAME] [--minutes 1..30] [--dtr unchanged|on|off] [--rts unchanged|on|off]
`

func parseArgs(args []string, stderr io.Writer) (config, error) {
	var c config
	if len(args) == 0 {
		return c, errors.New("no mode: use version, scan, probe or watch")
	}
	c.mode = args[0]
	if c.mode == "version" {
		if len(args) != 1 {
			return c, errors.New("version has no flags")
		}
		return c, nil
	}
	switch c.mode {
	case "scan", "probe", "watch":
	default:
		return c, fmt.Errorf("unknown mode %q: use scan, probe or watch", c.mode)
	}
	fs := flag.NewFlagSet("vlt-probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.transport, "transport", "hid", "Transport: hid or serial")
	fs.StringVar(&c.port, "port", "", "serial port name (serial only)")
	fs.IntVar(&c.minutes, "minutes", defaultWatchMinute, "watch time in minutes, 1 to 30 (watch only)")
	// VLT-IS-006 TASK 03-00-00-004 step 1: default "unchanged".
	fs.StringVar(&c.dtr, "dtr", string(transport.LineUnchanged), "DTR line: unchanged, on or off (serial only)")
	fs.StringVar(&c.rts, "rts", string(transport.LineUnchanged), "RTS line: unchanged, on or off (serial only)")
	if err := fs.Parse(args[1:]); err != nil {
		return c, err
	}
	if fs.NArg() != 0 {
		return c, fmt.Errorf("unexpected arguments: %q", fs.Args())
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if c.transport != "hid" && c.transport != "serial" {
		return c, fmt.Errorf("--transport must be hid or serial")
	}
	if _, err := transport.ParseLine(c.dtr); err != nil {
		return c, errors.New("--dtr must be unchanged, on or off")
	}
	if _, err := transport.ParseLine(c.rts); err != nil {
		return c, errors.New("--rts must be unchanged, on or off")
	}
	if c.mode == "scan" && len(set) > 0 {
		return c, errors.New("scan has no flags")
	}
	if c.transport != "serial" && (set["port"] || set["dtr"] || set["rts"]) {
		return c, errors.New("--port, --dtr and --rts are for --transport serial only")
	}
	if c.transport == "serial" && c.port == "" && c.mode != "scan" {
		return c, errors.New("--transport serial needs --port")
	}
	if c.mode != "watch" && set["minutes"] {
		return c, errors.New("--minutes is for watch only")
	}
	if c.minutes < minWatchMinutes || c.minutes > maxWatchMinutes {
		return c, fmt.Errorf("--minutes must be %d to %d", minWatchMinutes, maxWatchMinutes)
	}
	return c, nil
}

func main() {
	c, err := parseArgs(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vlt-probe:", err)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if c.mode == "version" {
		fmt.Println("vlt-probe", toolVersion)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, c, os.Stdout, "."); err != nil {
		fmt.Fprintln(os.Stderr, "vlt-probe:", err)
		os.Exit(1)
	}
}

func scan(w io.Writer) error {
	devs, herr := transport.ListHID()
	fmt.Fprintln(w, "HID devices with VID 0665 and PID 5161:")
	if herr != nil {
		fmt.Fprintln(w, "  error:", herr)
	}
	if len(devs) == 0 && herr == nil {
		fmt.Fprintln(w, "  none")
	}
	for _, d := range devs {
		serial := ""
		if d.Serial != "" {
			serial = redacted(len(d.Serial))
		}
		fmt.Fprintf(w, "  %s  %04X:%04X  product %q  serial %s  report lengths in/out/feature %d/%d/%d  descriptor %d bytes\n",
			d.Path, d.VID, d.PID, d.Product, serial, d.Lengths.Input, d.Lengths.Output, d.Lengths.Feature, len(d.ReportDescriptor))
	}
	ports, perr := transport.ListSerial()
	fmt.Fprintln(w, "Serial ports:")
	if perr != nil {
		fmt.Fprintln(w, "  error:", perr)
	}
	if len(ports) == 0 && perr == nil {
		fmt.Fprintln(w, "  none")
	}
	for _, p := range ports {
		fmt.Fprintln(w, " ", p)
	}
	fmt.Fprintln(w, "Summary: scan mode sent no command.")
	return nil
}

func openTransport(c config, w io.Writer, warnings *[]string) (transport.Transport, func() *transport.HIDDevice) {
	opts := transport.Options{Timeout: timeoutMillis * time.Millisecond, MinInterval: commandGapMillis * time.Millisecond}
	if c.transport == "serial" {
		so := transport.SerialOptions{DTR: transport.Line(c.dtr), RTS: transport.Line(c.rts), Warn: func(m string) {
			fmt.Fprintln(w, "Warning:", m)
			*warnings = append(*warnings, m)
		}}
		return transport.NewSerial(c.port, so, opts), func() *transport.HIDDevice { return nil }
	}
	t, info := transport.NewHID("", opts)
	return t, func() *transport.HIDDevice { return info }
}

func run(ctx context.Context, c config, w io.Writer, dir string) error {
	if c.mode == "scan" {
		return scan(w)
	}
	var warnings []string
	t, info := openTransport(c, w, &warnings)
	return session(ctx, c, t, info, w, dir, newRunner(t), &warnings)
}

func session(ctx context.Context, c config, t transport.Transport, info func() *transport.HIDDevice, w io.Writer, dir string, r *runner, warnings *[]string) error {
	start := r.now()
	if err := t.Open(ctx); err != nil {
		return fmt.Errorf("open %s: %w", c.transport, err)
	}
	defer t.Close()
	capt := newCapture(c.transport, c.mode, start)
	if c.transport == "serial" {
		capt.SerialCfg = fmt.Sprintf("%d bit/s, 8 data bits, no parity, 1 stop bit, DTR %s, RTS %s", transport.SerialBaud, c.dtr, c.rts)
	}
	capt.Device = deviceData(info())
	if warnings != nil {
		capt.Warnings = *warnings
	}
	var err error
	switch c.mode {
	case "probe":
		capt.Entries, err = r.probe(ctx)
	case "watch":
		fmt.Fprintf(w, "Watch mode: %d minutes. Press Ctrl+C to stop.\n", c.minutes)
		capt.Entries, err = r.watch(ctx, c.minutes)
	}
	if errors.Is(err, context.Canceled) {
		err = nil // stopped by the user; keep the Capture
	}
	path, werr := writeCapture(dir, capt, start)
	if werr != nil {
		return werr
	}
	summary(w, capt, path)
	return err
}
