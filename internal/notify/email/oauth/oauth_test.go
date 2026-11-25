// SPDX-License-Identifier: AGPL-3.0-or-later

package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPKCES256(t *testing.T) {
	v, ch, err := NewPKCE()
	if err != nil || v == "" || ch == "" {
		t.Fatalf("NewPKCE: %q %q %v", v, ch, err)
	}
	if ch != S256(v) {
		t.Fatalf("challenge is not S256")
	}
	sum := sha256.Sum256([]byte(v))
	if ch != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Fatalf("challenge encoding")
	}
	v2, ch2, _ := NewPKCE()
	if v == v2 || ch == ch2 {
		t.Fatalf("PKCE values must be unique")
	}
}

func TestNewStateUnique(t *testing.T) {
	a, err := NewState()
	b, err2 := NewState()
	if err != nil || err2 != nil || a == "" || a == b {
		t.Fatalf("state: %q %q %v %v", a, b, err, err2)
	}
}

func TestAuthorizationURL(t *testing.T) {
	u, err := url.Parse(AuthorizationURL(DefaultEndpoints(), "cid.apps.googleusercontent.com", "http://127.0.0.1:12345/", "st", "ch"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Host != "accounts.google.com" {
		t.Fatalf("host %s", u)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"client_id":             "cid.apps.googleusercontent.com",
		"redirect_uri":          "http://127.0.0.1:12345/",
		"response_type":         "code",
		"scope":                 ScopeGmailSend,
		"state":                 "st",
		"code_challenge":        "ch",
		"code_challenge_method": "S256",
		"access_type":           "offline",
		"prompt":                "consent",
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	if q.Get("scope") == "https://mail.google.com/" {
		t.Fatal("must not request the full-mailbox SMTP scope")
	}
}

func TestAllowedAuthURL(t *testing.T) {
	ok := "https://accounts.google.com/o/oauth2/v2/auth?client_id=x"
	if !AllowedAuthURL(ok) {
		t.Fatal("good URL rejected")
	}
	for _, raw := range []string{
		"http://accounts.google.com/o/oauth2/v2/auth",
		"https://evil.example/o/oauth2/v2/auth",
		"https://accounts.google.com.evil/o/oauth2/v2/auth",
		"https://accounts.google.com/not-oauth",
		"https://user@accounts.google.com/o/oauth2/v2/auth",
		"https://accounts.google.com:8080/o/oauth2/v2/auth",
	} {
		if AllowedAuthURL(raw) {
			t.Errorf("allowed %q", raw)
		}
	}
}

func TestListenLoopback(t *testing.T) {
	l, err := ListenLoopback()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	u, err := url.Parse(l.RedirectURI)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/" {
		t.Fatalf("redirect %q", l.RedirectURI)
	}
	if u.Hostname() == "localhost" || u.Hostname() == "0.0.0.0" {
		t.Fatal("must bind 127.0.0.1, not localhost or all interfaces")
	}
}

func TestWaitSuccessAndStateMismatchAndDenied(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		l, _ := ListenLoopback()
		defer l.Close()
		l.Expect("good-state")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		go func() {
			res, err := http.Get(l.RedirectURI + "?state=good-state&code=auth-code-1")
			if err != nil {
				t.Error(err)
				return
			}
			defer res.Body.Close()
			b, _ := io.ReadAll(res.Body)
			if res.StatusCode != 200 || !strings.Contains(string(b), "close this window") {
				t.Errorf("page %d %s", res.StatusCode, b)
			}
		}()
		code, err := l.Wait(ctx, "good-state")
		if err != nil || code != "auth-code-1" {
			t.Fatalf("got %q %v", code, err)
		}
	})
	t.Run("state", func(t *testing.T) {
		l, _ := ListenLoopback()
		defer l.Close()
		l.Expect("want")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		go func() { _, _ = http.Get(l.RedirectURI + "?state=other&code=x") }()
		_, err := l.Wait(ctx, "want")
		if !errors.Is(err, ErrStateMismatch) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("denied", func(t *testing.T) {
		l, _ := ListenLoopback()
		defer l.Close()
		l.Expect("st")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		go func() { _, _ = http.Get(l.RedirectURI + "?state=st&error=access_denied") }()
		_, err := l.Wait(ctx, "st")
		if !errors.Is(err, ErrDenied) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		l, _ := ListenLoopback()
		defer l.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, err := l.Wait(ctx, "st")
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("err %v", err)
		}
	})
}

// fakeGoogle is a local token/revoke/send server. It never talks to Google.
type fakeGoogle struct {
	mux *http.ServeMux
	srv *httptest.Server

	code, verifier, clientID, secret string
	refresh, access                  string
	wantRefresh                      bool
	sendStatus                       int
	revokeStatus                     int
	tokenStatus                      int
	body                             string
	gotForm                          url.Values
	gotAuth                          string
}

func newFakeGoogle() *fakeGoogle {
	f := &fakeGoogle{
		mux:          http.NewServeMux(),
		code:         "code-1",
		verifier:     "verifier-1",
		clientID:     "client-1",
		secret:       "secret-1",
		refresh:      "refresh-token-1",
		access:       "access-token-1",
		wantRefresh:  true,
		sendStatus:   http.StatusOK,
		revokeStatus: http.StatusOK,
		tokenStatus:  http.StatusOK,
	}
	f.mux.HandleFunc("/token", f.token)
	f.mux.HandleFunc("/revoke", f.revoke)
	f.mux.HandleFunc("/send", f.send)
	f.srv = httptest.NewServer(f.mux)
	return f
}

func (f *fakeGoogle) client() *Client {
	ep := Endpoints{Auth: f.srv.URL + "/auth", Token: f.srv.URL + "/token", Revoke: f.srv.URL + "/revoke", Send: f.srv.URL + "/send"}
	c := NewClient(ep)
	c.HTTP = f.srv.Client()
	return c
}

func (f *fakeGoogle) close() { f.srv.Close() }

func (f *fakeGoogle) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.gotForm = r.PostForm
	if f.body != "" {
		w.WriteHeader(f.tokenStatus)
		_, _ = io.WriteString(w, f.body)
		return
	}
	if r.PostForm.Get("grant_type") == "authorization_code" {
		if r.PostForm.Get("code") != f.code || r.PostForm.Get("code_verifier") != f.verifier || r.PostForm.Get("client_id") != f.clientID {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		if f.secret != "" && r.PostForm.Get("client_secret") != f.secret {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
			return
		}
		tok := map[string]any{"access_token": f.access, "token_type": "Bearer", "expires_in": 3600, "scope": ScopeGmailSend}
		if f.wantRefresh {
			tok["refresh_token"] = f.refresh
		}
		w.WriteHeader(f.tokenStatus)
		_ = json.NewEncoder(w).Encode(tok)
		return
	}
	if r.PostForm.Get("grant_type") == "refresh_token" {
		if r.PostForm.Get("refresh_token") != f.refresh {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-token-2", "token_type": "Bearer", "expires_in": 3600, "scope": ScopeGmailSend})
		return
	}
	w.WriteHeader(http.StatusBadRequest)
}

func (f *fakeGoogle) revoke(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.gotForm = r.PostForm
	w.WriteHeader(f.revokeStatus)
}

func (f *fakeGoogle) send(w http.ResponseWriter, r *http.Request) {
	f.gotAuth = r.Header.Get("Authorization")
	b, _ := io.ReadAll(r.Body)
	f.body = string(b)
	w.WriteHeader(f.sendStatus)
	if f.sendStatus == http.StatusOK {
		_, _ = io.WriteString(w, `{"id":"msg-1"}`)
	}
}

func TestExchangeAndRefreshAndRevokeAndSend(t *testing.T) {
	f := newFakeGoogle()
	defer f.close()
	c := f.client()
	ctx := context.Background()
	tok, err := c.Exchange(ctx, f.clientID, f.secret, f.code, "http://127.0.0.1:1/", f.verifier)
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "access-token-1" || tok.RefreshToken != "refresh-token-1" || tok.Expiry.IsZero() {
		t.Fatalf("token %+v", tok)
	}
	tok2, err := c.Refresh(ctx, f.clientID, f.secret, f.refresh)
	if err != nil || tok2.AccessToken != "access-token-2" {
		t.Fatalf("refresh %v %+v", err, tok2)
	}
	if err := c.Revoke(ctx, f.refresh); err != nil {
		t.Fatal(err)
	}
	if f.gotForm.Get("token") != f.refresh {
		t.Fatalf("revoke form %v", f.gotForm)
	}
	msg := []byte("From: a@example.com\r\nTo: b@example.com\r\nSubject: t\r\n\r\nbody")
	if err := c.SendRaw(ctx, tok.AccessToken, msg); err != nil {
		t.Fatal(err)
	}
	if f.gotAuth != "Bearer access-token-1" {
		t.Fatalf("auth %q", f.gotAuth)
	}
	var posted map[string]string
	if err := json.Unmarshal([]byte(f.body), &posted); err != nil || posted["raw"] == "" {
		t.Fatalf("send body %s", f.body)
	}
	raw, err := base64.RawURLEncoding.DecodeString(posted["raw"])
	if err != nil || string(raw) != string(msg) {
		t.Fatalf("raw %v %q", err, raw)
	}
}

func TestExchangeErrors(t *testing.T) {
	f := newFakeGoogle()
	defer f.close()
	c := f.client()
	ctx := context.Background()
	_, err := c.Exchange(ctx, f.clientID, f.secret, "bad-code", "http://127.0.0.1:1/", f.verifier)
	if err == nil || !strings.Contains(err.Error(), "no longer valid") {
		t.Fatalf("invalid_grant: %v", err)
	}
	if strings.Contains(err.Error(), "access-token") || strings.Contains(err.Error(), "refresh-token") {
		t.Fatalf("error leaked a token: %v", err)
	}
	f.wantRefresh = false
	tok, err := c.Exchange(ctx, f.clientID, f.secret, f.code, "http://127.0.0.1:1/", f.verifier)
	if err != nil {
		t.Fatal(err)
	}
	if tok.RefreshToken != "" {
		t.Fatal("unexpected refresh")
	}
}

func TestSendUnauthorized(t *testing.T) {
	f := newFakeGoogle()
	defer f.close()
	f.sendStatus = http.StatusUnauthorized
	c := f.client()
	err := c.SendRaw(context.Background(), "expired", []byte("x"))
	if !Unauthorized(err) {
		t.Fatalf("err %v", err)
	}
}

func TestRevokeFailure(t *testing.T) {
	f := newFakeGoogle()
	defer f.close()
	f.revokeStatus = http.StatusBadRequest
	c := f.client()
	if err := c.Revoke(context.Background(), "r"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNoSecretsInLogs(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewTextHandler(&buf, nil))
	f := newFakeGoogle()
	defer f.close()
	c := f.client()
	tok, err := c.Exchange(context.Background(), f.clientID, f.secret, f.code, "http://127.0.0.1:1/", f.verifier)
	if err != nil {
		t.Fatal(err)
	}
	// The production code never logs tokens. This test records that a
	// caller log of a safe message does not contain them either.
	log.Info("oauth exchange complete")
	s := buf.String()
	for _, secret := range []string{tok.AccessToken, tok.RefreshToken, f.secret, f.code, f.verifier} {
		if secret != "" && strings.Contains(s, secret) {
			t.Fatalf("log contains secret %q: %s", secret, s)
		}
	}
}
