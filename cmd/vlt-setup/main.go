// SPDX-License-Identifier: AGPL-3.0-or-later

// Command vlt-setup is the Windows development setup tool (VLT-IS-004 TASK
// 03-00-00-002). It has two subcommands and no flags:
//
//	vlt-setup version        print the version
//	vlt-setup dev-install [--dir FOLDER]
//	                         make the group (and add the calling user), the
//	                         service, the user right,
//	                         the data directory, the recovery, then start
//	vlt-setup dev-uninstall [--dir FOLDER]
//	                         stop and remove the service and the user right;
//	                         ask before it deletes the data directory
//
// Run it as Administrator. vltd.exe must be in the same folder as
// vlt-setup.exe. --dir is the install folder of vltd.exe (an absolute
// path); the default is %ProgramFiles%\VoltioVolt. If vltd.exe is already
// in that folder, it is not copied (VLT-IS-007 TASK 03-00-00-001 step 3.3). The MVP installer uses the same code later (QI-5).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// errWindowsOnly is returned on other systems.
var errWindowsOnly = errors.New("vlt-setup works on Windows only")

// version is set at build time with -ldflags "-X main.version=...".
var version = "0.0.0-dev"

const usage = `usage: vlt-setup version | dev-install [--dir FOLDER] | dev-uninstall [--dir FOLDER]
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

// opts are the options of one run.
type opts struct {
	cmd string
	dir string // install folder; "" is the default
}

// parse returns the subcommand and the --dir value.
func parse(args []string) (opts, error) {
	if len(args) == 0 {
		return opts{}, errors.New("give exactly one subcommand")
	}
	o := opts{cmd: args[0]}
	switch o.cmd {
	case cmdVersion:
		if len(args) != 1 {
			return opts{}, errors.New("version has no flags")
		}
		return o, nil
	case cmdInstall, cmdUninstall:
	default:
		return opts{}, fmt.Errorf("unknown subcommand %q", args[0])
	}
	rest := args[1:]
	switch {
	case len(rest) == 0:
	case len(rest) == 2 && rest[0] == "--dir" && rest[1] != "":
		o.dir = rest[1]
	case len(rest) == 1 && strings.HasPrefix(rest[0], "--dir=") && len(rest[0]) > len("--dir="):
		o.dir = strings.TrimPrefix(rest[0], "--dir=")
	default:
		return opts{}, fmt.Errorf("unknown arguments %q", rest)
	}
	return o, nil
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	o, err := parse(args)
	if err != nil {
		fmt.Fprintf(errOut, "vlt-setup: %v\n%s", err, usage)
		return 2
	}
	e := env{in: bufio.NewReader(in), out: out}
	switch o.cmd {
	case cmdVersion:
		e.printf("vlt-setup %s\n", version)
		return 0
	case cmdInstall:
		err = devInstall(e, o.dir)
	case cmdUninstall:
		err = devUninstall(e, o.dir)
	}
	if err != nil {
		fmt.Fprintf(errOut, "vlt-setup %s: %v\n", o.cmd, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
