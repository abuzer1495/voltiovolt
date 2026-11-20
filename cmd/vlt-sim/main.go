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

	"github.com/abuzer1495/voltiovolt/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// buildVersion is set by the build with -ldflags "-X main.buildVersion=..."
// (VLT-IS-012 TASK 03-00-00-006).
var buildVersion = "0.0.0-dev"

func run(args []string) int {
	fs := flag.NewFlagSet("vlt-sim", flag.ContinueOnError)
	profile := fs.String("profile", "voltronic", "Dialect profile: q1, qs or voltronic")
	mainsOff := fs.Bool("mains-off", false, "start with mains off (battery mode)")
	faults := fs.String("faults", "", "comma-separated faults: stale, split, nak, echo, no-response, wrong-length, ups-failed")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println(version.Line("vlt-sim", buildVersion))
		return 0
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
