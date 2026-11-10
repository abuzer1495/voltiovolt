// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func unitLines(t *testing.T, name string) map[string][]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "packaging", "linux", name))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	sec := ""
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "[") {
			sec = l
			continue
		}
		out[sec] = append(out[sec], l)
	}
	return out
}

func has(lines []string, l string) bool {
	for _, x := range lines {
		if x == l {
			return true
		}
	}
	return false
}

// TestSystemdUnits is VLT-IS-004 TASK 03-00-00-001 step 3 (QI-1).
func TestSystemdUnits(t *testing.T) {
	p := unitLines(t, "vltd.service")
	f := unitLines(t, "vltd-fallback.service")
	for _, l := range []string{"OnFailure=vltd-fallback.service", "StartLimitBurst=3", "StartLimitIntervalSec=60"} {
		if !has(p["[Unit]"], l) {
			t.Errorf("vltd.service [Unit]: no %q", l)
		}
	}
	for _, l := range []string{"Restart=on-failure", "RestartSec=5", "RestartPreventExitStatus=243"} {
		if !has(p["[Service]"], l) {
			t.Errorf("vltd.service [Service]: no %q", l)
		}
	}
	// VLT-IS-006 TASK 03-00-00-004 step 4: group dialout for the serial
	// ports (both units; the loop below checks the fallback unit).
	if !has(p["[Service]"], "SupplementaryGroups=dialout") {
		t.Error("vltd.service: no SupplementaryGroups=dialout")
	}
	if !has(f["[Unit]"], "Conflicts=vltd.service") {
		t.Error("fallback: no Conflicts=vltd.service")
	}
	if !has(f["[Service]"], "Environment=VLT_FALLBACK_MODE=1") {
		t.Error("fallback: no Environment=VLT_FALLBACK_MODE=1")
	}
	if _, ok := f["[Install]"]; ok {
		t.Error("fallback: has an [Install] section")
	}
	// Same ExecStart, user, groups and hardening; no credential, no restart.
	for _, l := range p["[Service]"] {
		switch {
		case strings.HasPrefix(l, "LoadCredentialEncrypted="), strings.HasPrefix(l, "Restart"):
			if has(f["[Service]"], l) {
				t.Errorf("fallback has %q", l)
			}
		default:
			if !has(f["[Service]"], l) {
				t.Errorf("fallback: no %q", l)
			}
		}
	}
	for _, l := range f["[Service]"] {
		if strings.HasPrefix(l, "LoadCredential") || strings.HasPrefix(l, "SetCredential") {
			t.Errorf("fallback has %q", l)
		}
	}
}

// TestPolkitRule is VLT-IS-004 TASK 03-00-00-001 step 4 (QI-3).
func TestPolkitRule(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "packaging", "linux", "50-vltd.rules"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `subject.user != "vltd"`) {
		t.Error("the rule is not only for the user vltd")
	}
	want := []string{"power-off", "power-off-multiple-sessions", "power-off-ignore-inhibit",
		"hibernate", "hibernate-multiple-sessions", "hibernate-ignore-inhibit"}
	n := strings.Count(s, `case "org.freedesktop.login1.`)
	if n != len(want) {
		t.Errorf("%d actions, want %d", n, len(want))
	}
	for _, a := range want {
		if !strings.Contains(s, `case "org.freedesktop.login1.`+a+`":`) {
			t.Errorf("no action %s", a)
		}
	}
	sh, err := os.ReadFile(filepath.Join("..", "..", "packaging", "linux", "vlt-dev-init.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sh), `install -m 0644 -o root -g root "$here/50-vltd.rules" /etc/polkit-1/rules.d/50-vltd.rules`) {
		t.Error("vlt-dev-init.sh does not install the rule with root:root 0644")
	}
}
