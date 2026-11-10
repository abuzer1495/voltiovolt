// SPDX-License-Identifier: AGPL-3.0-or-later

// Command vlt-sim is the VLT UPS simulator for development and tests.
// On Linux it starts a pty endpoint and shows the pty path. Connect the
// serial Transport (for example vlt-probe --transport serial --port <path>)
// to this path. vlt-sim is a diagnostic tool, not vltd.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("vlt-sim", flag.ContinueOnError)
	profile := fs.String("profile", "voltronic", "Dialect profile: q1, qs or voltronic")
	mainsOff := fs.Bool("mains-off", false, "start with mains off (battery mode)")
	faults := fs.String("faults", "", "comma-separated faults: stale, split, nak, echo, no-response, wrong-length, ups-failed")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var fl []string
	if *faults != "" {
		fl = strings.Split(*faults, ",")
	}
	if err := serve(ctx, *profile, *mainsOff, fl, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "vlt-sim:", err)
		return 1
	}
	return 0
}
