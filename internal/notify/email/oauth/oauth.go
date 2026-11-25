// SPDX-License-Identifier: AGPL-3.0-or-later

// Package oauth is the Google OAuth 2.0 flow for native apps (RFC 8252,
// SEC-045): PKCE S256, a state check, and a loopback redirect on
// 127.0.0.1 with a random port. The only scope is gmail.send (Gmail API
// users.messages.send). SMTP XOAUTH2 would need the scope
// https://mail.google.com/ (full mailbox access). VoltioVolt does not
// use SMTP XOAUTH2.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ScopeGmailSend is the only scope VoltioVolt requests (SEC-045).
const ScopeGmailSend = "https://www.googleapis.com/auth/gmail.send"

const (
	defaultAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL  = "https://oauth2.googleapis.com/token"
	defaultRevokeURL = "https://oauth2.googleapis.com/revoke"
	defaultSendURL   = "https://gmail.googleapis.com/gmail/v1/users/me/messages/send"
	httpTimeout      = 30 * time.Second
	maxBody          = 1 << 20
)

// ErrStateMismatch, ErrDenied, ErrTimeout and ErrNoRefresh are clear
// results of the sign-in flow. They never include tokens or codes.
var (
	ErrStateMismatch = errors.New("the sign-in could not be verified; try again")
	ErrDenied        = errors.New("Google access was denied")
	ErrTimeout       = errors.New("the sign-in timed out")
	ErrNoRefresh     = errors.New("Google did not return a refresh token; use a Desktop app client and sign in again")
	ErrRevoke        = errors.New("local credentials were removed, but Google could not be reached to revoke the token")
)

// Endpoints are the Google OAuth and Gmail API URLs (tests override them).
type Endpoints struct {
	Auth   string
	Token  string
	Revoke string
	Send   string
}

// DefaultEndpoints returns the production Google endpoints.
func DefaultEndpoints() Endpoints {
	return Endpoints{Auth: defaultAuthURL, Token: defaultTokenURL, Revoke: defaultRevokeURL, Send: defaultSendURL}
}

// Token is an OAuth token set. Callers must not log it.
type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
	Expiry       time.Time
	Scope        string
}

// Client talks to the token, revoke and send endpoints.
type Client struct {
	HTTP      *http.Client
	Endpoints Endpoints
	Now       func() time.Time
}

// NewClient returns a Client with TLS 1.2 and a 30 s timeout.
func NewClient(ep Endpoints) *Client {
	if ep.Auth == "" {
		ep = DefaultEndpoints()
	}
	return &Client{
		HTTP: &http.Client{
			Timeout: httpTimeout,
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
		Endpoints: ep,
		Now:       time.Now,
	}
}

// NewPKCE returns a code_verifier and its S256 code_challenge (RFC 7636).
func NewPKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	return verifier, S256(verifier), nil
}

// S256 is the PKCE S256 challenge of verifier.
func S256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// NewState returns a 128-bit random state value (base64url).
func NewState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizationURL builds the Google authorization URL (PKCE S256, offline
// access, consent so a refresh token is issued).
func AuthorizationURL(ep Endpoints, clientID, redirectURI, state, challenge string) string {
	if ep.Auth == "" {
		ep = DefaultEndpoints()
	}
	q := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {ScopeGmailSend},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"access_type":           {"offline"},
		"prompt":                {"consent"},
	}
	return ep.Auth + "?" + q.Encode()
}

// AllowedAuthURL reports whether raw is an https authorization URL on
// accounts.google.com (the only URL the GUI may open for this flow).
func AllowedAuthURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "accounts.google.com" {
		return false
	}
	if u.Port() != "" && u.Port() != "443" {
		return false
	}
	return strings.HasPrefix(u.Path, "/o/oauth2/")
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) httpc() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return NewClient(c.Endpoints).HTTP
}

// Exchange trades an authorization code for tokens (PKCE verifier).
func (c *Client) Exchange(ctx context.Context, clientID, clientSecret, code, redirectURI, verifier string) (Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	return c.tokenPost(ctx, form)
}

// Refresh requests a new access token with a refresh token.
func (c *Client) Refresh(ctx context.Context, clientID, clientSecret, refreshToken string) (Token, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	return c.tokenPost(ctx, form)
}

// Revoke asks Google to revoke a refresh or access token.
func (c *Client) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return errors.New("no token to revoke")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoints.Revoke, strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.httpc().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxBody))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("revoke failed (HTTP %d)", res.StatusCode)
	}
	return nil
}

// SendRaw posts a raw RFC 5322 message to Gmail API users.messages.send.
func (c *Client) SendRaw(ctx context.Context, accessToken string, rfc822 []byte) error {
	if accessToken == "" {
		return errors.New("not signed in with Google")
	}
	raw := base64.RawURLEncoding.EncodeToString(rfc822)
	body, err := json.Marshal(map[string]string{"raw": raw})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoints.Send, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpc().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if res.StatusCode == http.StatusUnauthorized {
		clear(b)
		return errUnauthorized
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		clear(b)
		return fmt.Errorf("Gmail send failed (HTTP %d)", res.StatusCode)
	}
	clear(b)
	return nil
}

var errUnauthorized = errors.New("gmail: access token rejected")

// Unauthorized reports whether err is an HTTP 401 from SendRaw.
func Unauthorized(err error) bool { return errors.Is(err, errUnauthorized) }

type tokenJSON struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (c *Client) tokenPost(ctx context.Context, form url.Values) (Token, error) {
	var zero Token
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoints.Token, strings.NewReader(form.Encode()))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.httpc().Do(req)
	if err != nil {
		return zero, fmt.Errorf("token request failed: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return zero, err
	}
	defer clear(body)
	var tj tokenJSON
	if err := json.Unmarshal(body, &tj); err != nil {
		return zero, fmt.Errorf("token response is not valid (HTTP %d)", res.StatusCode)
	}
	if tj.Error != "" || res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := tokenError(tj.Error, tj.ErrorDesc, res.StatusCode)
		return zero, errors.New(msg)
	}
	if tj.AccessToken == "" {
		return zero, errors.New("Google did not return an access token")
	}
	tok := Token{
		AccessToken:  tj.AccessToken,
		RefreshToken: tj.RefreshToken,
		TokenType:    tj.TokenType,
		ExpiresIn:    tj.ExpiresIn,
		Scope:        tj.Scope,
	}
	if tj.ExpiresIn > 0 {
		tok.Expiry = c.now().Add(time.Duration(tj.ExpiresIn) * time.Second)
	}
	return tok, nil
}

func tokenError(code, desc string, status int) string {
	code = oneLine(code)
	desc = oneLine(desc)
	switch code {
	case "access_denied":
		return ErrDenied.Error()
	case "invalid_grant":
		return "the Google sign-in is no longer valid; sign in again"
	case "invalid_client":
		return "the Google client ID or client secret is not correct"
	}
	if desc != "" && len(desc) < 200 {
		return "Google sign-in failed: " + desc
	}
	if code != "" {
		return "Google sign-in failed (" + code + ")"
	}
	return fmt.Sprintf("Google sign-in failed (HTTP %d)", status)
}

func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// Listener is a loopback HTTP server on 127.0.0.1 with a random port.
type Listener struct {
	ln          net.Listener
	RedirectURI string
	mu          sync.Mutex
	want        string
	once        sync.Once
	ch          chan loopResult
	srv         *http.Server
}

type loopResult struct {
	code string
	err  error
}

// ListenLoopback binds 127.0.0.1:0 and returns a listener whose redirect
// URI is http://127.0.0.1:<port>/.
func ListenLoopback() (*Listener, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.IP.IsUnspecified() || !addr.IP.IsLoopback() {
		_ = ln.Close()
		return nil, errors.New("loopback listen did not bind 127.0.0.1")
	}
	l := &Listener{
		ln:          ln,
		RedirectURI: fmt.Sprintf("http://127.0.0.1:%d/", addr.Port),
		ch:          make(chan loopResult, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handle)
	l.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return context.Background() },
	}
	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

// Expect records the state that the redirect must present.
func (l *Listener) Expect(state string) {
	l.mu.Lock()
	l.want = state
	l.mu.Unlock()
}

// Wait blocks until the redirect arrives, the context ends, or Close is
// called. The state in the redirect must match wantState.
func (l *Listener) Wait(ctx context.Context, wantState string) (string, error) {
	l.mu.Lock()
	l.want = wantState
	l.mu.Unlock()
	select {
	case r := <-l.ch:
		return r.code, r.err
	case <-ctx.Done():
		_ = l.Close()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", ErrTimeout
		}
		return "", ctx.Err()
	}
}

// Close stops the loopback server.
func (l *Listener) Close() error {
	if l == nil || l.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := l.srv.Shutdown(ctx)
	_ = l.ln.Close()
	return err
}

func (l *Listener) finish(r loopResult) {
	l.once.Do(func() { l.ch <- r })
}

func (l *Listener) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	// Never log the request URL (it may contain the code).
	if errCode := q.Get("error"); errCode != "" {
		msg := ErrDenied
		if errCode != "access_denied" {
			msg = errors.New("Google sign-in failed")
		}
		writePage(w, false, msg.Error())
		l.finish(loopResult{err: msg})
		return
	}
	l.mu.Lock()
	want := l.want
	l.mu.Unlock()
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || state != want {
		writePage(w, false, ErrStateMismatch.Error())
		l.finish(loopResult{err: ErrStateMismatch})
		return
	}
	if code == "" {
		writePage(w, false, "Google did not return an authorization code")
		l.finish(loopResult{err: errors.New("Google did not return an authorization code")})
		return
	}
	writePage(w, true, "You can close this window and return to VoltioVolt.")
	l.finish(loopResult{code: code})
}

func writePage(w http.ResponseWriter, ok bool, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	}
	title := "VoltioVolt"
	if ok {
		title = "VoltioVolt — signed in"
	} else {
		title = "VoltioVolt — sign-in did not complete"
	}
	_, _ = fmt.Fprintf(w, "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>%s</title></head><body><p>%s</p></body></html>", title, htmlEscape(msg))
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
