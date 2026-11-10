// SPDX-License-Identifier: AGPL-3.0-or-later

// Command vlt-setup is the Windows development setup tool (VLT-IS-004 TASK
// 03-00-00-002). It has two subcommands and no flags:
//
//	vlt-setup dev-install    make the group (and add the calling user), the
//	                         service, the user right,
//	                         the data directory, the recovery, then start
//	vlt-setup dev-uninstall  stop and remove the service and the user right;
//	                         ask before it deletes the data directory
//
// Run it as Administrator. vltd.exe must be in the same folder as
// vlt-setup.exe. The MVP installer uses the same code later (QI-5).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

// errWindowsOnly is returned on other systems.
var errWindowsOnly = errors.New("vlt-setup works on Windows only")

const usage = `usage: vlt-setup dev-install | dev-uninstall
Run as Administrator. vltd.exe must be in the same folder as vlt-setup.exe.
`

// env is the input and output of one run.
type env struct {
	in  *bufio.Reader
	out io.Writer
}

func (e env) printf(format string, a ...any) { fmt.Fprintf(e.out, format, a...) }

// ask prints a question and returns the answer line without the line end.
func (e env) ask(q string) string {
	e.printf("%s", q)
	s, _ := e.in.ReadString('\n')
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// parse returns the subcommand. There are no flags.
func parse(args []string) (string, error) {
	if len(args) != 1 {
		return "", errors.New("give exactly one subcommand")
	}
	switch args[0] {
	case cmdInstall, cmdUninstall:
		return args[0], nil
	}
	return "", fmt.Errorf("unknown subcommand %q", args[0])
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	cmd, err := parse(args)
	if err != nil {
		fmt.Fprintf(errOut, "vlt-setup: %v\n%s", err, usage)
		return 2
	}
	e := env{in: bufio.NewReader(in), out: out}
	switch cmd {
	case cmdInstall:
		err = devInstall(e)
	case cmdUninstall:
		err = devUninstall(e)
	}
	if err != nil {
		fmt.Fprintf(errOut, "vlt-setup %s: %v\n", cmd, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
