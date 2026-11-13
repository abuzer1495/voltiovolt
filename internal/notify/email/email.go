// SPDX-License-Identifier: AGPL-3.0-or-later

// Package email is the e-mail Channel (SMTP) of the Notification Router
// (VLT-IS-007 TASK 03-00-00-006):
//
//  1. Settings: server, port, security (TLS on port 465, or STARTTLS on port
//     587), user name, sender address, receiver addresses (1 to 5).
//  2. The password is the Secret S-04 (Vault ID "smtp.password"). It is read
//     from the Vault only for the authentication of a send.
//  3. The preset "Gmail": smtp.gmail.com, port 465, TLS (Gmail).
//  4. The server certificate is always verified (TLS 1.2 or later). If the
//     server does not offer STARTTLS, or STARTTLS fails, nothing is sent and
//     the password is not used.
//  5. The subject has the format "[VLT][severity] Event".
package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// SecretID is the Vault ID of the SMTP password (S-04).
const SecretID = "smtp.password"

// Security modes.
const (
	SecurityTLS      = "tls"      // implicit TLS, port 465
	SecurityStartTLS = "starttls" // STARTTLS, port 587
)

// Settings are the SMTP Settings.
type Settings struct {
	Server   string   `json:"server"`
	Port     int      `json:"port"`
	Security string   `json:"security"`
	User     string   `json:"user"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

// Preset is a set of server Settings.
type Preset struct {
	Name     string `json:"name"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Security string `json:"security"`
}

// Gmail is the preset "Gmail" (step 3).
var Gmail = Preset{Name: "gmail", Server: "smtp.gmail.com", Port: 465, Security: SecurityTLS}

// ErrNotConfigured: the server, the sender or the receivers are not set.
var ErrNotConfigured = errors.New("email: the SMTP Settings are not complete")

// ErrNoStartTLS: the server does not offer STARTTLS.
var ErrNoStartTLS = errors.New("email: the server does not offer STARTTLS; nothing is sent")

// Secrets gives the password (internal/vault.Vault).
type Secrets interface {
	Use(ctx context.Context, id string, fn func([]byte) error) error
}

// Config is the configuration of the Mailer.
type Config struct {
	Settings func(ctx context.Context) (Settings, error)
	Secrets  Secrets
	Log      *slog.Logger
	// TLS is the base TLS configuration (tests). It is cloned; ServerName
	// and MinVersion are always set by the Mailer.
	TLS     *tls.Config
	Timeout time.Duration // default 30 s
	Now     func() time.Time
}

// Mailer is the e-mail Channel.
type Mailer struct{ cfg Config }

// New returns a Mailer.
func New(cfg Config) *Mailer {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Mailer{cfg: cfg}
}

// Name is the Channel name.
func (m *Mailer) Name() string { return notify.ChannelEmail }

// Check tells if the Settings are complete and correct.
func (s Settings) Check() error {
	if s.Server == "" || s.From == "" || len(s.To) == 0 {
		return ErrNotConfigured
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("email: port %d is not correct", s.Port)
	}
	if s.Security != SecurityTLS && s.Security != SecurityStartTLS {
		return fmt.Errorf("email: security %q is not correct", s.Security)
	}
	if len(s.To) > store.MaxEmailReceivers {
		return fmt.Errorf("email: more than %d receivers", store.MaxEmailReceivers)
	}
	for _, a := range append([]string{s.From}, s.To...) {
		if !store.ValidEmailAddress(a) {
			return fmt.Errorf("email: address %q is not correct", a)
		}
	}
	if strings.ContainsAny(s.User, "\r\n") {
		return errors.New("email: the user name is not correct")
	}
	return nil
}

func (m *Mailer) tlsConfig(server string) *tls.Config {
	var c *tls.Config
	if m.cfg.TLS != nil {
		c = m.cfg.TLS.Clone()
	} else {
		c = &tls.Config{}
	}
	c.ServerName = server
	c.MinVersion = tls.VersionTLS12
	c.InsecureSkipVerify = false
	return c
}

// Send sends the Notification to the receivers.
func (m *Mailer) Send(ctx context.Context, n notify.Notification) error {
	s, err := m.cfg.Settings(ctx)
	if err != nil {
		return err
	}
	if err := s.Check(); err != nil {
		return err
	}
	msg := m.message(s, n)
	if err := m.send(ctx, s, msg); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("email: %s", redact.Scrub(err.Error()))
	}
	return nil
}

func (m *Mailer) send(ctx context.Context, s Settings, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	addr := net.JoinHostPort(s.Server, strconv.Itoa(s.Port))
	d := &net.Dialer{}
	var conn net.Conn
	var err error
	if s.Security == SecurityTLS {
		td := &tls.Dialer{NetDialer: d, Config: m.tlsConfig(s.Server)}
		conn, err = td.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	c, err := smtp.NewClient(conn, s.Server)
	if err != nil {
		return err
	}
	defer c.Close()
	if s.Security == SecurityStartTLS {
		if err := c.Hello("localhost"); err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return ErrNoStartTLS
		}
		if err := c.StartTLS(m.tlsConfig(s.Server)); err != nil {
			return fmt.Errorf("STARTTLS failed; nothing is sent: %w", err)
		}
	}
	if st, ok := c.TLSConnectionState(); !ok || !st.HandshakeComplete {
		return errors.New("no TLS connection; nothing is sent")
	}
	if s.User != "" {
		err := m.cfg.Secrets.Use(ctx, SecretID, func(pw []byte) error {
			return c.Auth(smtp.PlainAuth("", s.User, string(pw), s.Server))
		})
		if errors.Is(err, vault.ErrNotFound) {
			return errors.New("the SMTP password is not set")
		}
		if err != nil {
			return fmt.Errorf("authentication: %w", err)
		}
	}
	if err := c.Mail(s.From); err != nil {
		return err
	}
	for _, to := range s.To {
		if err := c.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// oneLine removes CR, LF and other control characters (no header
// injection).
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// message makes the RFC 5322 message: UTF-8 text, quoted-printable.
func (m *Mailer) message(s Settings, n notify.Notification) []byte {
	var b bytes.Buffer
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	host := s.From[strings.LastIndexByte(s.From, '@')+1:]
	subject := oneLine(n.Subject)
	fmt.Fprintf(&b, "From: %s\r\n", s.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(s.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", m.cfg.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), host)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	b.WriteString("X-Mailer: VoltioVolt\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	text := strings.ReplaceAll(strings.ReplaceAll(n.Text, "\r\n", "\n"), "\n", "\r\n")
	_, _ = qp.Write([]byte(text + "\r\n"))
	_ = qp.Close()
	return b.Bytes()
}

// StoreSettings reads the SMTP Settings from the Store.
func StoreSettings(st *store.Store) func(ctx context.Context) (Settings, error) {
	return func(ctx context.Context) (Settings, error) {
		var s Settings
		for _, f := range []struct {
			key string
			out any
		}{
			{store.SettingEmailServer, &s.Server}, {store.SettingEmailPort, &s.Port}, {store.SettingEmailSecurity, &s.Security},
			{store.SettingEmailUser, &s.User}, {store.SettingEmailFrom, &s.From}, {store.SettingEmailTo, &s.To},
		} {
			v, err := st.GetSetting(ctx, f.key)
			if err != nil {
				return Settings{}, err
			}
			if err := json.Unmarshal(v.Value, f.out); err != nil {
				return Settings{}, err
			}
		}
		return s, nil
	}
}
