// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/abuzer1495/voltiovolt/internal/sim"
)

// config is the parsed command line of vlt-simulator.
type config struct {
	lang     Lang
	headless bool
	scenario int // 0: none
	dialect  sim.Profile
}

func parseArgs(args []string, errw io.Writer) (config, error) {
	fs := flag.NewFlagSet("vlt-simulator", flag.ContinueOnError)
	fs.SetOutput(errw)
	lang := fs.String("lang", "", "language: tr or en (default: the OS language if it is Turkish, else English)")
	headless := fs.Bool("headless", false, "no screen, for CI")
	scenario := fs.Int("scenario", 0, "with --headless: run one scenario (1 to 9)")
	dialect := fs.String("dialect", "megatec", "dialect: megatec, voltronic-qs or voltronic")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if *lang != "" && !strings.EqualFold(*lang, "tr") && !strings.EqualFold(*lang, "en") {
		return config{}, fmt.Errorf("unknown --lang %q (want tr or en)", *lang)
	}
	code := *lang
	if code == "" {
		code = defaultLangCode()
	}
	if *scenario < 0 || *scenario > 9 {
		return config{}, fmt.Errorf("unknown --scenario %d (want 1 to 9)", *scenario)
	}
	p, err := sim.ParseProfile(*dialect)
	if err != nil {
		return config{}, err
	}
	return config{lang: lookupLang(code), headless: *headless, scenario: *scenario, dialect: p}, nil
}
