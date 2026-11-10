// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/abuzer1495/voltiovolt/internal/sim"
)

func serve(ctx context.Context, profile string, mainsOff bool, faults []string, out io.Writer) error {
	m, err := model(profile, mainsOff, faults)
	if err != nil {
		return err
	}
	s := sim.New(m)
	p, err := sim.StartPTY(ctx, s)
	if err != nil {
		return err
	}
	defer p.Close()
	fmt.Fprintf(out, "vlt-sim: profile %s, pty path: %s\n", profile, p.Path)
	fmt.Fprintln(out, "vlt-sim: press Ctrl+C to stop")
	<-ctx.Done()
	fmt.Fprintf(out, "vlt-sim: received commands: %q\n", s.Received())
	if bad := s.NotIn(sim.ProbeList); len(bad) > 0 {
		fmt.Fprintf(out, "vlt-sim: WARNING: commands not in the Probe List: %q\n", bad)
	}
	return nil
}
