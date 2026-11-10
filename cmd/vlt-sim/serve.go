// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/abuzer1495/voltiovolt/internal/sim"
)

func model(profile string, mainsOff bool, faults []string) (*sim.Model, error) {
	var p sim.Profile
	switch profile {
	case "q1":
		p = sim.ProfileQ1
	case "qs":
		p = sim.ProfileQS
	case "voltronic":
		p = sim.ProfileVoltronic
	default:
		return nil, fmt.Errorf("unknown profile %q", profile)
	}
	m := sim.NewModel(p)
	m.SetMains(!mainsOff)
	known := map[string]sim.Fault{}
	for _, f := range []sim.Fault{sim.FaultStale, sim.FaultSplit, sim.FaultNAK, sim.FaultEcho, sim.FaultNoResponse, sim.FaultWrongLength, sim.FaultUPSFailed} {
		known[string(f)] = f
	}
	for _, f := range faults {
		v, ok := known[f]
		if !ok {
			return nil, fmt.Errorf("unknown fault %q", f)
		}
		m.SetFault(v, true)
	}
	return m, nil
}
