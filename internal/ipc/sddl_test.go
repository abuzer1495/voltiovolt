// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"strings"
	"testing"
)

type ace struct{ typ, flags, rights, sid string }

// parseDACL parses "D:<flags>(ace)(ace)..." (SDDL, ACE strings with 6
// fields: type;flags;rights;object_guid;inherit_object_guid;account_sid).
func parseDACL(t *testing.T, s string) (string, []ace) {
	t.Helper()
	if !strings.HasPrefix(s, "D:") {
		t.Fatalf("no DACL: %s", s)
	}
	s = s[2:]
	i := strings.IndexByte(s, '(')
	if i < 0 {
		t.Fatalf("no ACE: %s", s)
	}
	flags, rest := s[:i], s[i:]
	var out []ace
	for rest != "" {
		if rest[0] != '(' {
			t.Fatalf("bad ACE list at %q", rest)
		}
		j := strings.IndexByte(rest, ')')
		if j < 0 {
			t.Fatalf("ACE not closed: %q", rest)
		}
		f := strings.Split(rest[1:j], ";")
		if len(f) != 6 {
			t.Fatalf("ACE %q has %d fields", rest[1:j], len(f))
		}
		if f[3] != "" || f[4] != "" {
			t.Fatalf("object ACE not expected: %q", rest[1:j])
		}
		out = append(out, ace{typ: f[0], flags: f[1], rights: f[2], sid: f[5]})
		rest = rest[j+1:]
	}
	return flags, out
}

// TestPipeSDDL is VLT-IS-004 TASK 03-00-00-001 step 2.
func TestPipeSDDL(t *testing.T) {
	const svc = "S-1-5-80-1111-2222-3333-4444-5555"
	const ops = "S-1-5-21-10-20-30-1001"
	flags, aces := parseDACL(t, PipeSDDL(svc, ops, nil))
	if flags != "P" {
		t.Fatalf("DACL flags %q, want P (protected)", flags)
	}
	want := []ace{
		{"A", "", "GA", "SY"},
		{"A", "", "GA", svc},
		{"A", "", "GRGW", "BA"},
		{"A", "", "GRGW", ops},
	}
	if len(aces) != len(want) {
		t.Fatalf("ACEs %v, want %v", aces, want)
	}
	for i := range want {
		if aces[i] != want[i] {
			t.Errorf("ACE %d = %+v, want %+v", i, aces[i], want[i])
		}
	}
	// No deny ACE, no inheritance flags, no Everyone, no Users.
	for _, a := range aces {
		if a.typ != "A" || a.flags != "" || a.sid == "WD" || a.sid == "BU" || a.sid == "AU" {
			t.Errorf("ACE not permitted: %+v", a)
		}
	}
	// Without the service account and without the group: only SYSTEM and
	// Administrators.
	_, aces = parseDACL(t, PipeSDDL("", "", nil))
	if len(aces) != 2 || aces[0] != (ace{"A", "", "GA", "SY"}) || aces[1] != (ace{"A", "", "GRGW", "BA"}) {
		t.Fatalf("minimum DACL: %+v", aces)
	}
	// Extra SIDs (tests) get read and write only.
	_, aces = parseDACL(t, PipeSDDL("", "", []string{"S-1-5-21-1-2-3-500"}))
	if aces[len(aces)-1] != (ace{"A", "", "GRGW", "S-1-5-21-1-2-3-500"}) {
		t.Fatalf("extra SID: %+v", aces)
	}
}
