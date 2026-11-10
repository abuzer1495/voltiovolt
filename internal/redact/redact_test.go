// SPDX-License-Identifier: AGPL-3.0-or-later

package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeToken makes a fake Telegram bot token at run time, so that no token
// text is in the repository.
func fakeToken() string {
	return strings.Repeat("7", 10) + ":" + strings.Repeat("Ab3_x", 7)
}

// fakeValue makes a fake Secret value at run time.
func fakeValue() string {
	return "s3cr3t-" + strings.Repeat("Qz9", 6)
}

type withSecret struct {
	Name  string
	Token Secret
	inner Secret
	Ptr   *Secret
}

func values() (string, []any) {
	v := fakeValue()
	s := NewString(v)
	return v, []any{
		s,
		&s,
		withSecret{Name: "a", Token: s, inner: s, Ptr: &s},
		&withSecret{Name: "b", Token: s, inner: s, Ptr: &s},
		[]Secret{s, s},
		map[string]Secret{"k": s},
	}
}

func TestFmtVerbs(t *testing.T) {
	v, vals := values()
	for i, x := range vals {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
			out := fmt.Sprintf(verb, x)
			if strings.Contains(out, v) || strings.Contains(out, fmt.Sprintf("%x", v)) {
				t.Errorf("value %d verb %s: Secret in output %q", i, verb, out)
			}
		}
		if out := fmt.Sprint(x) + fmt.Sprintln(x); strings.Contains(out, v) {
			t.Errorf("value %d: Secret in Sprint output", i)
		}
	}
	if got := fmt.Sprintf("%v", NewString(v)); got != Redacted {
		t.Errorf("%%v = %q, want %q", got, Redacted)
	}
}

func TestJSON(t *testing.T) {
	v, vals := values()
	for i, x := range vals {
		b, err := json.Marshal(x)
		if err != nil {
			t.Fatalf("value %d: %v", i, err)
		}
		if bytes.Contains(b, []byte(v)) {
			t.Errorf("value %d: Secret in JSON %s", i, b)
		}
		if !bytes.Contains(b, []byte(Redacted)) {
			t.Errorf("value %d: no %q in JSON %s", i, Redacted, b)
		}
	}
	tb, _ := NewString(v).MarshalText()
	if string(tb) != Redacted {
		t.Errorf("MarshalText = %q", tb)
	}
}

func TestExpose(t *testing.T) {
	v := fakeValue()
	s := NewString(v)
	if string(s.Expose()) != v || s.Len() != len(v) {
		t.Fatal("Expose does not give the bytes")
	}
	s.Wipe()
	if !bytes.Equal(s.Expose(), make([]byte, len(v))) {
		t.Fatal("Wipe does not clear the bytes")
	}
	var z Secret
	if z.Expose() != nil || !z.IsZero() || z.String() != Redacted {
		t.Fatal("zero Secret")
	}
}

func logBoth(t *testing.T, f func(l *slog.Logger)) string {
	t.Helper()
	var buf bytes.Buffer
	f(slog.New(NewHandler(slog.NewTextHandler(&buf, nil))))
	f(slog.New(NewHandler(slog.NewJSONHandler(&buf, nil))))
	return buf.String()
}

func TestSlogHandlers(t *testing.T) {
	v, vals := values()
	out := logBoth(t, func(l *slog.Logger) {
		for i, x := range vals {
			l.Info("value", "i", i, "x", x)
		}
		l.Info("attr", slog.Any("s", NewString(v)))
		l.Info("group", slog.Group("g", slog.Any("s", NewString(v)), slog.String("n", "x")))
		l.With("pre", NewString(v)).WithGroup("wg").Info("with", "s", NewString(v))
	})
	if strings.Contains(out, v) {
		t.Fatalf("Secret in slog output:\n%s", out)
	}
	if !strings.Contains(out, Redacted) {
		t.Fatalf("no %q in slog output", Redacted)
	}
}

func TestScrubPatterns(t *testing.T) {
	tok := fakeToken()
	v := fakeValue()
	cases := []string{
		"https://api.telegram.org/bot" + tok + "/getMe",
		"token " + tok + " end",
		"Authorization: Basic " + v,
		"authorization:" + v,
		"Bearer " + v,
		"https://example.invalid/x?token=" + v + "&y=1",
		"https://example.invalid/x?a=1&key=" + v,
		"https://example.invalid/x?secret=" + v,
		"https://example.invalid/x?password=" + v + "#f",
		"https://example.invalid/x?access_token=" + v,
		"https://example.invalid/bot" + v + "/sendMessage",
	}
	for _, c := range cases {
		got := Scrub(c)
		if strings.Contains(got, v) || strings.Contains(got, tok) || strings.Contains(got, tok[11:]) {
			t.Errorf("Scrub(%q) = %q", c, got)
		}
		if !strings.Contains(got, Redacted) {
			t.Errorf("Scrub(%q) = %q, no %q", c, got, Redacted)
		}
	}
	// Normal text does not change.
	for _, c := range []string{"Q1 response 209.7", "https://example.invalid/x?name=a", "the key is not here"} {
		if got := Scrub(c); got != c {
			t.Errorf("Scrub(%q) = %q", c, got)
		}
	}
}

// TestHTTPErrorToken is VLT-IS-003 TASK 03-00-00-002 step 7.4.
func TestHTTPErrorToken(t *testing.T) {
	tok := fakeToken()
	c := &http.Client{Timeout: 2 * time.Second}
	// Port 1 on the loopback address: the request fails without a network.
	_, err := c.Get("http://127.0.0.1:1/bot" + tok + "/getMe")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), tok) {
		t.Fatalf("test is not valid: the error does not contain the token")
	}
	wrapped := fmt.Errorf("telegram: %w", err)
	out := logBoth(t, func(l *slog.Logger) {
		l.Error("request failed", "err", err)
		l.Error("request failed", "err", wrapped)
		l.Error("request failed: " + err.Error())
		l.Error("request failed", "text", err.Error())
		l.Error("request failed", "errs", []error{err, errors.New("x")})
	})
	if strings.Contains(out, tok) || strings.Contains(out, tok[11:]) {
		t.Fatalf("token in log output:\n%s", out)
	}
}

func FuzzScrub(f *testing.F) {
	f.Add("https://api.telegram.org/bot" + fakeToken() + "/getMe")
	f.Add("Authorization: Bearer x")
	f.Fuzz(func(t *testing.T, s string) {
		out := Scrub(s)
		if Scrub(out) != out {
			t.Fatalf("Scrub is not stable: %q", s)
		}
	})
}
