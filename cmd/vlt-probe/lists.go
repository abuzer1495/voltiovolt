// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// The command lists are constants in the code. There is no flag for a raw
// command. Source: VLT-IS-002 03-50-01.

// probeList is the Probe List, in its fixed order.
var probeList = [...]string{"Q1", "QS", "QGS", "QMOD", "QBV", "QWS", "QFLAG", "QRI", "F", "I", "QID", "QVFW", "QMD"}

// watchList is the Watch List.
var watchList = [...]string{"Q1", "QS", "QGS", "QMOD", "QBV", "QWS"}

// Timing of vlt-probe.
const (
	commandGapMillis   = 500  // wait between commands
	timeoutMillis      = 3000 // timeout of one command
	watchPeriodMillis  = 2000 // start of a Watch List cycle
	notSupportedLimit  = 2    // "not supported" results before a command is stopped
	minWatchMinutes    = 1
	maxWatchMinutes    = 30
	defaultWatchMinute = 1
)
