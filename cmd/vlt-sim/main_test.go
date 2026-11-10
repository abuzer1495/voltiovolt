// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "testing"

func TestModelFlags(t *testing.T) {
	if _, err := model("voltronic", true, []string{"nak", "split"}); err != nil {
		t.Fatal(err)
	}
	if _, err := model("bad", false, nil); err == nil {
		t.Error("unknown profile must fail")
	}
	if _, err := model("q1", false, []string{"bad"}); err == nil {
		t.Error("unknown fault must fail")
	}
}
