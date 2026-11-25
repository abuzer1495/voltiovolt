// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/notify/email/oauth"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// Vault IDs for Gmail OAuth (S-02, S-03). The refresh token never leaves
// the Vault except inside Use, and it is never logged. The access token
// stays in memory only.
const (
	RefreshSecretID      = "gmail.oauth.refresh_token"
	ClientSecretSecretID = "gmail.oauth.client_secret"
)

// Authentication modes (setting email.auth).
const (
	AuthPassword   = "password"
	AuthGmailOAuth = "gmail_oauth"
)

// TokenStore is the Vault for OAuth secrets (Set, Delete, Use, status).
type TokenStore interface {
	Secrets
	Set(ctx context.Context, id string, value []byte) error
	Delete(ctx context.Context, id string) error
	SecretStatus(ctx context.Context, id string) (vault.SecretStatus, error)
}

const oauthUserTimeout = 5 * time.Minute

type oauthFlow struct {
	lis          *oauth.Listener
	state        string
	verifier     string
	clientID     string
	clientSecret []byte
	cancel       context.CancelFunc
}

type oauthState struct {
	pending bool
	err     error
}

func (m *Mailer) oauthClient() *oauth.Client {
	if m.cfg.OAuth != nil {
		return m.cfg.OAuth
	}
	return oauth.NewClient(oauth.DefaultEndpoints())
}

func (m *Mailer) tokens() (TokenStore, bool) {
	ts, ok := m.cfg.Secrets.(TokenStore)
	return ts, ok
}

// OAuthStatus is the sign-in state for the GUI (no tokens).
type OAuthStatus struct {
	Auth        string
	Connected   bool
	ClientIDSet bool
	Pending     bool
	Error       string
}

// Status reports the Gmail OAuth state for channels.list.
func (m *Mailer) Status(ctx context.Context) (OAuthStatus, error) {
	s, err := m.cfg.Settings(ctx)
	if err != nil {
		return OAuthStatus{}, err
	}
	st := OAuthStatus{Auth: s.Auth}
	if st.Auth == "" {
		st.Auth = AuthPassword
	}
	st.ClientIDSet = s.ClientID != ""
	if ts, ok := m.tokens(); ok {
		ss, err := ts.SecretStatus(ctx, RefreshSecretID)
		if err != nil {
			return st, err
		}
		st.Connected = ss.Set
	}
	m.mu.Lock()
	st.Pending = m.oAuth.pending
	if m.oAuth.err != nil {
		st.Error = m.oAuth.err.Error()
	}
	m.mu.Unlock()
	return st, nil
}

// StartOAuth starts the RFC 8252 loopback flow and returns the
// authorization URL and the redirect URI. A background goroutine waits
// for the redirect (5 minutes) and stores the refresh token in the Vault.
func (m *Mailer) StartOAuth(ctx context.Context, clientID string, clientSecret []byte) (authURL, redirectURI string, err error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		if s, e := m.cfg.Settings(ctx); e == nil {
			clientID = strings.TrimSpace(s.ClientID)
		}
	}
	if clientID == "" {
		return "", "", errors.New("the Google client ID is not set")
	}
	ts, ok := m.tokens()
	if !ok {
		return "", "", errors.New("the vault is not available for OAuth")
	}
	if len(clientSecret) == 0 {
		_ = ts.Use(ctx, ClientSecretSecretID, func(b []byte) error {
			clientSecret = append([]byte(nil), b...)
			return nil
		})
	}
	m.CancelOAuth()
	lis, err := oauth.ListenLoopback()
	if err != nil {
		return "", "", err
	}
	state, err := oauth.NewState()
	if err != nil {
		_ = lis.Close()
		return "", "", err
	}
	verifier, challenge, err := oauth.NewPKCE()
	if err != nil {
		_ = lis.Close()
		return "", "", err
	}
	lis.Expect(state)
	url := oauth.AuthorizationURL(m.oauthClient().Endpoints, clientID, lis.RedirectURI, state, challenge)
	flow := &oauthFlow{lis: lis, state: state, verifier: verifier, clientID: clientID, clientSecret: append([]byte(nil), clientSecret...)}
	runCtx, cancel := context.WithTimeout(context.Background(), oauthUserTimeout)
	flow.cancel = cancel
	m.mu.Lock()
	m.flow = flow
	m.oAuth = oauthState{pending: true}
	m.mu.Unlock()
	go m.finishOAuth(runCtx, flow)
	return url, lis.RedirectURI, nil
}

// CancelOAuth stops a pending sign-in (a new StartOAuth also cancels).
func (m *Mailer) CancelOAuth() {
	m.mu.Lock()
	f := m.flow
	m.flow = nil
	m.mu.Unlock()
	if f == nil {
		return
	}
	if f.cancel != nil {
		f.cancel()
	}
	_ = f.lis.Close()
	clear(f.clientSecret)
	f.clientSecret = nil
	f.verifier = ""
	f.state = ""
}

func (m *Mailer) finishOAuth(ctx context.Context, f *oauthFlow) {
	defer func() {
		_ = f.lis.Close()
		clear(f.clientSecret)
		f.clientSecret = nil
		f.verifier = ""
		f.state = ""
		f.cancel()
		m.mu.Lock()
		if m.flow == f {
			m.flow = nil
			m.oAuth.pending = false
		}
		m.mu.Unlock()
	}()
	code, err := f.lis.Wait(ctx, f.state)
	if err != nil {
		m.setOAuthErr(err)
		return
	}
	c := m.oauthClient()
	tok, err := c.Exchange(ctx, f.clientID, string(f.clientSecret), code, f.lis.RedirectURI, f.verifier)
	code = ""
	if err != nil {
		m.setOAuthErr(err)
		return
	}
	if tok.RefreshToken == "" {
		m.setOAuthErr(oauth.ErrNoRefresh)
		return
	}
	ts, ok := m.tokens()
	if !ok {
		m.setOAuthErr(errors.New("the vault is not available for OAuth"))
		return
	}
	if err := ts.Set(ctx, RefreshSecretID, []byte(tok.RefreshToken)); err != nil {
		m.setOAuthErr(err)
		return
	}
	if len(f.clientSecret) > 0 {
		if err := ts.Set(ctx, ClientSecretSecretID, f.clientSecret); err != nil {
			m.setOAuthErr(err)
			return
		}
	}
	m.mu.Lock()
	m.access = tok.AccessToken
	m.accessExp = tok.Expiry
	m.oAuth = oauthState{}
	m.mu.Unlock()
	m.cfg.Log.Info("gmail oauth signed in")
}

func (m *Mailer) setOAuthErr(err error) {
	m.mu.Lock()
	m.oAuth = oauthState{err: err}
	m.mu.Unlock()
	m.cfg.Log.Info("gmail oauth failed", "err", err.Error())
}

// DisconnectOAuth revokes the refresh token at Google, deletes the Vault
// secrets, and clears the in-memory access token.
func (m *Mailer) DisconnectOAuth(ctx context.Context) error {
	m.CancelOAuth()
	ts, ok := m.tokens()
	if !ok {
		return errors.New("the vault is not available for OAuth")
	}
	var refresh string
	_ = ts.Use(ctx, RefreshSecretID, func(b []byte) error {
		refresh = string(append([]byte(nil), b...))
		return nil
	})
	var revokeErr error
	if refresh != "" {
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		revokeErr = m.oauthClient().Revoke(rctx, refresh)
		cancel()
		refresh = ""
	}
	_ = ts.Delete(ctx, RefreshSecretID)
	_ = ts.Delete(ctx, ClientSecretSecretID)
	m.mu.Lock()
	m.access = ""
	m.accessExp = time.Time{}
	m.oAuth = oauthState{}
	m.mu.Unlock()
	if revokeErr != nil {
		return oauth.ErrRevoke
	}
	return nil
}

func (m *Mailer) sendGmail(ctx context.Context, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	token, err := m.accessToken(ctx)
	if err != nil {
		return err
	}
	err = m.oauthClient().SendRaw(ctx, token, msg)
	if oauth.Unauthorized(err) {
		m.mu.Lock()
		m.access = ""
		m.accessExp = time.Time{}
		m.mu.Unlock()
		token, err = m.accessToken(ctx)
		if err != nil {
			return err
		}
		err = m.oauthClient().SendRaw(ctx, token, msg)
	}
	return err
}

func (m *Mailer) accessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	if m.access != "" && (m.accessExp.IsZero() || m.cfg.Now().Add(30*time.Second).Before(m.accessExp)) {
		t := m.access
		m.mu.Unlock()
		return t, nil
	}
	m.mu.Unlock()
	s, err := m.cfg.Settings(ctx)
	if err != nil {
		return "", err
	}
	if s.ClientID == "" {
		return "", errors.New("the Google client ID is not set")
	}
	ts, ok := m.tokens()
	if !ok {
		return "", errors.New("the vault is not available for OAuth")
	}
	var refresh, secret string
	err = ts.Use(ctx, RefreshSecretID, func(b []byte) error {
		refresh = string(append([]byte(nil), b...))
		return nil
	})
	if errors.Is(err, vault.ErrNotFound) || refresh == "" {
		return "", errors.New("not signed in with Google")
	}
	if err != nil {
		return "", err
	}
	_ = ts.Use(ctx, ClientSecretSecretID, func(b []byte) error {
		secret = string(append([]byte(nil), b...))
		return nil
	})
	tok, err := m.oauthClient().Refresh(ctx, s.ClientID, secret, refresh)
	refresh = ""
	secret = ""
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.access = tok.AccessToken
	m.accessExp = tok.Expiry
	m.mu.Unlock()
	return tok.AccessToken, nil
}
