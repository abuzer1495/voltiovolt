// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/notify/email/oauth"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// memSecrets is an in-memory TokenStore for tests (no OS vault, no secrets in files).
type memSecrets struct {
	mu   sync.Mutex
	vals map[string][]byte
}

func newMemSecrets() *memSecrets { return &memSecrets{vals: map[string][]byte{}} }

func (m *memSecrets) Use(_ context.Context, id string, fn func([]byte) error) error {
	m.mu.Lock()
	b, ok := m.vals[id]
	cp := append([]byte(nil), b...)
	m.mu.Unlock()
	if !ok {
		return vault.ErrNotFound
	}
	return fn(cp)
}

func (m *memSecrets) Set(_ context.Context, id string, value []byte) error {
	m.mu.Lock()
	m.vals[id] = append([]byte(nil), value...)
	m.mu.Unlock()
	return nil
}

func (m *memSecrets) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	delete(m.vals, id)
	m.mu.Unlock()
	return nil
}

func (m *memSecrets) SecretStatus(_ context.Context, id string) (vault.SecretStatus, error) {
	m.mu.Lock()
	_, ok := m.vals[id]
	m.mu.Unlock()
	return vault.SecretStatus{ID: id, Set: ok}, nil
}

type gmailFake struct {
	mux                         *http.ServeMux
	srv                         *httptest.Server
	mu                          sync.Mutex
	refresh, access, nextAccess string
	sendStatus, revokeStatus    int
	revokeCalled                bool
	sent                        int
	lastAuth                    string
	tokenStatus                 int
}

func newGmailFake() *gmailFake {
	f := &gmailFake{
		mux:          http.NewServeMux(),
		refresh:      "refresh-token-test",
		access:       "access-token-test",
		nextAccess:   "access-token-refreshed",
		sendStatus:   http.StatusOK,
		revokeStatus: http.StatusOK,
		tokenStatus:  http.StatusOK,
	}
	f.mux.HandleFunc("/token", f.token)
	f.mux.HandleFunc("/revoke", f.revoke)
	f.mux.HandleFunc("/send", f.send)
	f.srv = httptest.NewTLSServer(f.mux)
	return f
}

func (f *gmailFake) close() { f.srv.Close() }

func (f *gmailFake) client() *oauth.Client {
	c := oauth.NewClient(oauth.Endpoints{
		Auth: f.srv.URL + "/auth", Token: f.srv.URL + "/token",
		Revoke: f.srv.URL + "/revoke", Send: f.srv.URL + "/send",
	})
	c.HTTP = f.srv.Client()
	return c
}

func (f *gmailFake) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tokenStatus != http.StatusOK {
		w.WriteHeader(f.tokenStatus)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
		return
	}
	if r.PostForm.Get("grant_type") == "authorization_code" {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": f.access, "refresh_token": f.refresh,
			"token_type": "Bearer", "expires_in": 3600, "scope": oauth.ScopeGmailSend,
		})
		return
	}
	if r.PostForm.Get("refresh_token") != f.refresh {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": f.nextAccess, "token_type": "Bearer", "expires_in": 3600, "scope": oauth.ScopeGmailSend,
	})
}

func (f *gmailFake) revoke(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.revokeCalled = true
	st := f.revokeStatus
	f.mu.Unlock()
	w.WriteHeader(st)
}

func (f *gmailFake) send(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.lastAuth = r.Header.Get("Authorization")
	st := f.sendStatus
	f.sent++
	f.mu.Unlock()
	_, _ = io.Copy(io.Discard, r.Body)
	w.WriteHeader(st)
	if st == http.StatusOK {
		_, _ = io.WriteString(w, `{"id":"m1"}`)
	}
}

func oauthMailer(t *testing.T, f *gmailFake, sec *memSecrets, from string) *Mailer {
	t.Helper()
	if from == "" {
		from = "sender@example.com"
	}
	return New(Config{
		Settings: func(context.Context) (Settings, error) {
			return Settings{
				Auth: AuthGmailOAuth, ClientID: "client-test.apps.googleusercontent.com",
				From: from, To: []string{"to@example.com"},
			}, nil
		},
		Secrets: sec,
		OAuth:   f.client(),
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Timeout: 5 * time.Second,
	})
}

func TestSendGmailOAuth(t *testing.T) {
	f := newGmailFake()
	defer f.close()
	sec := newMemSecrets()
	_ = sec.Set(context.Background(), RefreshSecretID, []byte(f.refresh))
	m := oauthMailer(t, f, sec, "")
	err := m.Send(context.Background(), notify.Notification{Subject: "[VLT][info] Event", Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sent != 1 || !strings.HasPrefix(f.lastAuth, "Bearer ") {
		t.Fatalf("sent %d auth %q", f.sent, f.lastAuth)
	}
}

func TestSendGmailRetriesAfter401(t *testing.T) {
	f := newGmailFake()
	defer f.close()
	f.sendStatus = http.StatusUnauthorized
	sec := newMemSecrets()
	_ = sec.Set(context.Background(), RefreshSecretID, []byte(f.refresh))
	m := oauthMailer(t, f, sec, "")
	// First SendRaw 401, then refresh, then still 401 because sendStatus stays.
	if err := m.Send(context.Background(), notify.Notification{Subject: "s", Text: "b"}); err == nil {
		t.Fatal("expected error")
	}
	// Allow the retry to succeed.
	f.mu.Lock()
	f.sendStatus = http.StatusOK
	f.mu.Unlock()
	m.mu.Lock()
	m.access = "stale"
	m.accessExp = time.Now().Add(-time.Hour)
	m.mu.Unlock()
	if err := m.Send(context.Background(), notify.Notification{Subject: "s", Text: "b"}); err != nil {
		t.Fatal(err)
	}
}

func TestSendGmailNotSignedIn(t *testing.T) {
	f := newGmailFake()
	defer f.close()
	m := oauthMailer(t, f, newMemSecrets(), "")
	err := m.Send(context.Background(), notify.Notification{Subject: "s", Text: "b"})
	if err == nil || !strings.Contains(err.Error(), "not signed in") {
		t.Fatalf("err %v", err)
	}
}

func TestOAuthStatusAndDisconnect(t *testing.T) {
	f := newGmailFake()
	defer f.close()
	sec := newMemSecrets()
	_ = sec.Set(context.Background(), RefreshSecretID, []byte(f.refresh))
	_ = sec.Set(context.Background(), ClientSecretSecretID, []byte("client-secret-test"))
	m := oauthMailer(t, f, sec, "")
	st, err := m.Status(context.Background())
	if err != nil || !st.Connected || st.Auth != AuthGmailOAuth || !st.ClientIDSet {
		t.Fatalf("status %+v %v", st, err)
	}
	if err := m.DisconnectOAuth(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	called := f.revokeCalled
	f.mu.Unlock()
	if !called {
		t.Fatal("revoke not called")
	}
	st, _ = m.Status(context.Background())
	if st.Connected {
		t.Fatal("still connected")
	}
	if _, ok := sec.vals[RefreshSecretID]; ok {
		t.Fatal("refresh token not deleted")
	}
	if _, ok := sec.vals[ClientSecretSecretID]; ok {
		t.Fatal("client secret not deleted")
	}
}

func TestDisconnectRevokeFailureStillDeletes(t *testing.T) {
	f := newGmailFake()
	defer f.close()
	f.revokeStatus = http.StatusBadGateway
	sec := newMemSecrets()
	_ = sec.Set(context.Background(), RefreshSecretID, []byte(f.refresh))
	m := oauthMailer(t, f, sec, "")
	err := m.DisconnectOAuth(context.Background())
	if !errors.Is(err, oauth.ErrRevoke) {
		t.Fatalf("err %v", err)
	}
	if _, ok := sec.vals[RefreshSecretID]; ok {
		t.Fatal("refresh token should be deleted after a revoke failure")
	}
}

func TestStartOAuthLoopbackAndFinish(t *testing.T) {
	f := newGmailFake()
	defer f.close()
	sec := newMemSecrets()
	m := oauthMailer(t, f, sec, "")
	url, redirect, err := m.StartOAuth(context.Background(), "client-test.apps.googleusercontent.com", []byte("client-secret-test"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "code_challenge_method=S256") || !strings.Contains(url, "127.0.0.1") {
		t.Fatalf("url %s", url)
	}
	if !strings.HasPrefix(redirect, "http://127.0.0.1:") {
		t.Fatalf("redirect %s", redirect)
	}
	st, _ := m.Status(context.Background())
	if !st.Pending {
		t.Fatal("expected pending")
	}
	// Redirect with the state from the URL.
	u := mustURLQuery(t, url)
	res, err := http.Get(redirect + "?state=" + u.Get("state") + "&code=code-1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st, _ = m.Status(context.Background())
		if !st.Pending {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st.Pending {
		t.Fatalf("still pending: %+v", st)
	}
	if !st.Connected {
		t.Fatalf("not connected: %+v", st)
	}
	if _, ok := sec.vals[RefreshSecretID]; !ok {
		t.Fatal("refresh token not stored")
	}
}

func TestStartOAuthStateMismatch(t *testing.T) {
	f := newGmailFake()
	defer f.close()
	m := oauthMailer(t, f, newMemSecrets(), "")
	_, redirect, err := m.StartOAuth(context.Background(), "client-test.apps.googleusercontent.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(redirect + "?state=wrong&code=code-1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	deadline := time.Now().Add(5 * time.Second)
	var st OAuthStatus
	for time.Now().Before(deadline) {
		st, _ = m.Status(context.Background())
		if !st.Pending {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st.Connected || st.Error == "" || !strings.Contains(st.Error, "verified") {
		t.Fatalf("status %+v", st)
	}
}

func TestOAuthSecretsNotLogged(t *testing.T) {
	var buf strings.Builder
	f := newGmailFake()
	defer f.close()
	sec := newMemSecrets()
	_ = sec.Set(context.Background(), RefreshSecretID, []byte(f.refresh))
	m := New(Config{
		Settings: func(context.Context) (Settings, error) {
			return Settings{Auth: AuthGmailOAuth, ClientID: "cid", From: "a@example.com", To: []string{"b@example.com"}}, nil
		},
		Secrets: sec,
		OAuth:   f.client(),
		Log:     slog.New(slog.NewTextHandler(&buf, nil)),
		Timeout: 5 * time.Second,
	})
	if err := m.Send(context.Background(), notify.Notification{Subject: "s", Text: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := m.DisconnectOAuth(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, secret := range []string{f.refresh, f.access, f.nextAccess} {
		if strings.Contains(s, secret) {
			t.Fatalf("log leaked %q: %s", secret, s)
		}
	}
}

func TestCheckOAuthSkipsSMTPServer(t *testing.T) {
	s := Settings{Auth: AuthGmailOAuth, From: "a@example.com", To: []string{"b@example.com"}}
	if err := s.Check(); err != nil {
		t.Fatal(err)
	}
	s.From = ""
	if err := s.Check(); err == nil {
		t.Fatal("expected not configured")
	}
}

func mustURLQuery(t *testing.T, raw string) interface{ Get(string) string } {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}
