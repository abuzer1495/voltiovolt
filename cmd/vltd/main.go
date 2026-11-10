// SPDX-License-Identifier: AGPL-3.0-or-later

// Command vltd is the VLT service (VLT-IS-003 TASK 03-00-00-008). This is
// the service skeleton: logging, data directory check, Store, Vault, audit
// chain verification and IPC. It does not talk to a UPS yet.
//
// Flags: --version only.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/logging"
	"github.com/abuzer1495/voltiovolt/internal/platform"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	fs := flag.NewFlagSet("vltd", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "vltd: no arguments are permitted; the only flag is --version")
		os.Exit(2)
	}
	if *showVersion {
		fmt.Println("vltd", version)
		return
	}
	os.Exit(run())
}

func run() int {
	dir := store.DefaultDataDir()
	log, closer, err := logging.Open(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vltd: logging:", err)
		return 1
	}
	d := newDaemon(config{
		DataDir:    dir,
		CheckPerms: true,
		Provider:   vault.NewHostProvider,
		Listen:     func() (netListener, ipc.PeerCheck, error) { return ipc.DefaultEndpoint().Listen() },
		Log:        log,
		LogCloser:  closer,
		Version:    version,
	})
	if err := platform.Run("vltd", d, log); err != nil {
		log.Error("vltd stopped with an error", "err", err)
		fmt.Fprintln(os.Stderr, "vltd:", err)
		return 1
	}
	return 0
}
