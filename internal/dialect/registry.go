// SPDX-License-Identifier: AGPL-3.0-or-later

// Package dialect contains the command registry, the READ request encoder
// and the response parsers for the Megatec Q protocol family.
//
// Each protocol fact in this package has a source line in docs/protocol.md.
// This package has no encoder for CONTROL or CONFIG commands (SEC-037).
package dialect

// Class is the safety class of a command.
type Class string

// Command classes.
const (
	ClassRead    Class = "READ"
	ClassControl Class = "CONTROL"
	ClassConfig  Class = "CONFIG"
)

// Dialect is one variant of the Megatec Q protocol family.
type Dialect string

// Known Dialects. Names of VLT-IS-001 Rev 05, 00-10-00 (VLT-IS-006 01-30-00).
const (
	// DialectMegatec: Q1, F and I (VLT-REF-001 sections 5 and 6). This is
	// the path of the Reference UPS (OD-26, FIELD).
	DialectMegatec Dialect = "megatec"
	// DialectVoltronicQS: the QS family.
	DialectVoltronicQS Dialect = "voltronic-qs"
	// DialectVoltronic: the QGS family.
	DialectVoltronic Dialect = "voltronic"
)

// ResponseType names the parser for the response of a READ command.
type ResponseType string

// Response types. A CONTROL or CONFIG row has no response type.
const (
	RespQ1    ResponseType = "status-q1"
	RespQS    ResponseType = "status-qs"
	RespQGS   ResponseType = "status-qgs"
	RespQMOD  ResponseType = "mode"
	RespQBV   ResponseType = "battery"
	RespQWS   ResponseType = "warning-bits"
	RespQFLAG ResponseType = "flags"
	RespQRI   ResponseType = "rating-qri"
	RespF     ResponseType = "rating-f"
	RespI     ResponseType = "information"
	RespQID   ResponseType = "serial-number"
	RespQVFW  ResponseType = "firmware"
	RespQMD   ResponseType = "model-data"
)

// Entry is one row of the command registry.
//
// A CONTROL or CONFIG row contains only the name and the class. The name of
// such a row can be a pattern (for example "SnnRmmmm"). The encoder never
// makes bytes for these rows.
type Entry struct {
	Name     string
	Class    Class
	Dialects []Dialect
	Response ResponseType
}

var allDialects = []Dialect{DialectMegatec, DialectVoltronicQS, DialectVoltronic}

// registry is the one command table of VLT.
//
// "D" is in no class. It is not in this table, and VLT never sends it.
var registry = []Entry{
	// READ commands. The order is the Probe List order of VLT-IS-006
	// TASK 03-00-00-003 step 4.
	{Name: "Q1", Class: ClassRead, Dialects: allDialects, Response: RespQ1},
	{Name: "F", Class: ClassRead, Dialects: allDialects, Response: RespF},
	{Name: "I", Class: ClassRead, Dialects: allDialects, Response: RespI},
	{Name: "QS", Class: ClassRead, Dialects: []Dialect{DialectVoltronicQS, DialectVoltronic}, Response: RespQS},
	{Name: "QGS", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQGS},
	{Name: "QMOD", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQMOD},
	{Name: "QBV", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQBV},
	{Name: "QWS", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQWS},
	{Name: "QFLAG", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQFLAG},
	{Name: "QRI", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQRI},
	{Name: "QID", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQID},
	{Name: "QVFW", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQVFW},
	{Name: "QMD", Class: ClassRead, Dialects: []Dialect{DialectVoltronic}, Response: RespQMD},

	// CONTROL commands: name and class only.
	{Name: "Q", Class: ClassControl},
	{Name: "T", Class: ClassControl},
	{Name: "TL", Class: ClassControl},
	{Name: "Tnn", Class: ClassControl},
	{Name: "Snn", Class: ClassControl},
	{Name: "SnnRmmmm", Class: ClassControl},
	{Name: "C", Class: ClassControl},
	{Name: "CS", Class: ClassControl},
	{Name: "CT", Class: ClassControl},
	{Name: "SOFF", Class: ClassControl},
	{Name: "SON", Class: ClassControl},
	{Name: "BZOFF", Class: ClassControl},
	{Name: "BZON", Class: ClassControl},

	// CONFIG commands: name and class only.
	{Name: "PEx", Class: ClassConfig},
	{Name: "PDx", Class: ClassConfig},
}

// Registry returns a copy of the command registry.
func Registry() []Entry {
	out := make([]Entry, len(registry))
	for i, e := range registry {
		e.Dialects = append([]Dialect(nil), e.Dialects...)
		out[i] = e
	}
	return out
}

// LookupRead returns the READ row for an exact command name.
// It returns false for all other strings, including CONTROL and CONFIG rows.
func LookupRead(name string) (Entry, bool) {
	for _, e := range registry {
		if e.Class == ClassRead && e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}
