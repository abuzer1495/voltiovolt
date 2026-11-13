// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"log/slog"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// fakePassword makes a fake password at run time.
func fakePassword() string { return "abcd" + strings.Repeat("wxyz", 3) }

type fakeSecrets struct{ pw string }

func (f fakeSecrets) Use(_ context.Context, id string, fn func([]byte) error) error {
	if id != SecretID {
		return errors.New("wrong id")
	}
	if f.pw == "" {
		return vault.ErrNotFound
	}
	return fn([]byte(f.pw))
}

// newCert makes a self-signed certificate for 127.0.0.1.
func newCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fake smtp"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(c)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

// fakeSMTP is a fake SMTP server: implicit TLS or STARTTLS.
type fakeSMTP struct {
	ln       net.Listener
	cert     tls.Certificate
	implicit bool
	startTLS bool // offer STARTTLS (plain mode)

	mu    sync.Mutex
	auth  string
	from  string
	rcpts []string
	data  string
	cmds  []string
}

func newFakeSMTP(t *testing.T, cert tls.Certificate, implicit, startTLS bool) *fakeSMTP {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln, cert: cert, implicit: implicit, startTLS: startTLS}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func (f *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	c.SetDeadline(time.Now().Add(10 * time.Second))
	tlsOn := false
	if f.implicit {
		tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{f.cert}})
		if tc.Handshake() != nil {
			return
		}
		c, tlsOn = tc, true
	}
	r := bufio.NewReader(c)
	w := func(s string) { c.Write([]byte(s + "\r\n")) }
	w("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		f.mu.Lock()
		f.cmds = append(f.cmds, strings.SplitN(line, " ", 2)[0])
		f.mu.Unlock()
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "EHLO"):
			if f.startTLS && !tlsOn {
				w("250-fake")
				w("250 STARTTLS")
			} else {
				w("250-fake")
				w("250 AUTH PLAIN")
			}
		case up == "STARTTLS":
			w("220 go ahead")
			tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{f.cert}})
			if tc.Handshake() != nil {
				return
			}
			c, tlsOn = tc, true
			r = bufio.NewReader(c)
		case strings.HasPrefix(up, "AUTH PLAIN"):
			f.mu.Lock()
			f.auth = strings.TrimSpace(line[len("AUTH PLAIN"):])
			f.mu.Unlock()
			w("235 ok")
		case strings.HasPrefix(up, "MAIL FROM:"):
			f.mu.Lock()
			f.from = line[len("MAIL FROM:"):]
			f.mu.Unlock()
			w("250 ok")
		case strings.HasPrefix(up, "RCPT TO:"):
			f.mu.Lock()
			f.rcpts = append(f.rcpts, line[len("RCPT TO:"):])
			f.mu.Unlock()
			w("250 ok")
		case up == "DATA":
			w("354 go")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.data = b.String()
			f.mu.Unlock()
			w("250 queued")
		case up == "QUIT":
			w("221 bye")
			return
		default:
			w("502 no")
		}
	}
}

func (f *fakeSMTP) has(cmd string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.cmds {
		if strings.EqualFold(c, cmd) {
			return true
		}
	}
	return false
}

func settings(port int, sec string) Settings {
	return Settings{Server: "127.0.0.1", Port: port, Security: sec, User: "vlt@example.com", From: "vlt@example.com", To: []string{"a@example.com", "b@example.org"}}
}

func mailer(s Settings, pool *x509.CertPool, log *bytes.Buffer) *Mailer {
	return New(Config{Settings: func(context.Context) (Settings, error) { return s, nil }, Secrets: fakeSecrets{fakePassword()},
		TLS: &tls.Config{RootCAs: pool}, Timeout: 5 * time.Second, Log: slog.New(slog.NewTextHandler(log, nil))})
}

var note = notify.Notification{Subject: "[VLT][CRITICAL] On battery", Text: "[VLT][KRİTİK] Akü ile çalışıyor\nMod: Akü ile çalışıyor", Severity: "CRITICAL", Type: "ON_BATTERY"}

func checkSent(t *testing.T, f *fakeSMTP) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	want := base64.StdEncoding.EncodeToString([]byte("\x00vlt@example.com\x00" + fakePassword()))
	if f.auth != want {
		t.Errorf("auth %q", f.auth)
	}
	if f.from != "<vlt@example.com>" || len(f.rcpts) != 2 || f.rcpts[1] != "<b@example.org>" {
		t.Errorf("from %q rcpts %v", f.from, f.rcpts)
	}
	for _, w := range []string{"Subject: [VLT][CRITICAL] On battery\r\n", "To: a@example.com, b@example.org\r\n", "Content-Type: text/plain; charset=utf-8", "Content-Transfer-Encoding: quoted-printable"} {
		if !strings.Contains(f.data, w) {
			t.Errorf("no %q in\n%s", w, f.data)
		}
	}
}

func TestSendTLS(t *testing.T) {
	cert, pool := newCert(t)
	f := newFakeSMTP(t, cert, true, false)
	var log bytes.Buffer
	if err := mailer(settings(f.port(), SecurityTLS), pool, &log).Send(context.Background(), note); err != nil {
		t.Fatal(err)
	}
	checkSent(t, f)
}

func TestSendStartTLS(t *testing.T) {
	cert, pool := newCert(t)
	f := newFakeSMTP(t, cert, false, true)
	var log bytes.Buffer
	if err := mailer(settings(f.port(), SecurityStartTLS), pool, &log).Send(context.Background(), note); err != nil {
		t.Fatal(err)
	}
	checkSent(t, f)
}

// TestNoStartTLSNoSend: no STARTTLS offer, or a STARTTLS failure (a
// certificate that is not trusted): no AUTH, no MAIL.
func TestNoStartTLSNoSend(t *testing.T) {
	cert, pool := newCert(t)
	f := newFakeSMTP(t, cert, false, false)
	var log bytes.Buffer
	err := mailer(settings(f.port(), SecurityStartTLS), pool, &log).Send(context.Background(), note)
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("err %v", err)
	}
	f2 := newFakeSMTP(t, cert, false, true)
	err = mailer(settings(f2.port(), SecurityStartTLS), x509.NewCertPool(), &log).Send(context.Background(), note)
	if err == nil || !strings.Contains(err.Error(), "STARTTLS failed") {
		t.Fatalf("err %v", err)
	}
	for _, s := range []*fakeSMTP{f, f2} {
		if s.has("AUTH") || s.has("MAIL") {
			t.Fatal("AUTH or MAIL without TLS")
		}
	}
}

// TestCertificateVerified: implicit TLS with a certificate that is not
// trusted fails.
func TestCertificateVerified(t *testing.T) {
	cert, _ := newCert(t)
	f := newFakeSMTP(t, cert, true, false)
	var log bytes.Buffer
	if err := mailer(settings(f.port(), SecurityTLS), x509.NewCertPool(), &log).Send(context.Background(), note); err == nil {
		t.Fatal("no error with a certificate that is not trusted")
	}
	if f.has("MAIL") {
		t.Fatal("MAIL sent")
	}
}

func TestNotConfiguredAndChecks(t *testing.T) {
	m := mailer(Settings{Port: 465, Security: SecurityTLS}, nil, &bytes.Buffer{})
	if err := m.Send(context.Background(), note); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err %v", err)
	}
	s := settings(465, SecurityTLS)
	s.To = []string{"a@example.com", "b@example.com", "c@example.com", "d@example.com", "e@example.com", "f@example.com"}
	if s.Check() == nil {
		t.Fatal("6 receivers accepted")
	}
	s = settings(465, SecurityTLS)
	s.To = []string{"x@example.com\r\nBcc: y@example.com"}
	if s.Check() == nil {
		t.Fatal("header injection accepted")
	}
	if Gmail.Server != "smtp.gmail.com" || Gmail.Port != 465 || Gmail.Security != SecurityTLS {
		t.Fatalf("Gmail preset %+v", Gmail)
	}
}

func TestSubjectOneLine(t *testing.T) {
	m := mailer(settings(465, SecurityTLS), nil, &bytes.Buffer{})
	b := string(m.message(settings(465, SecurityTLS), notify.Notification{Subject: "[VLT][INFO] x\r\nBcc: evil@example.com", Text: "t"}))
	if strings.Contains(b, "\r\nBcc:") {
		t.Fatalf("header injection:\n%s", b)
	}
}

func TestNoPasswordInErrorsAndLogs(t *testing.T) {
	cert, pool := newCert(t)
	f := newFakeSMTP(t, cert, true, false)
	f.ln.Close()
	var log bytes.Buffer
	err := mailer(settings(f.port(), SecurityTLS), pool, &log).Send(context.Background(), note)
	if err == nil {
		t.Fatal("no error")
	}
	if strings.Contains(err.Error(), fakePassword()) || strings.Contains(log.String(), fakePassword()) {
		t.Fatal("password in the error or the log")
	}
	// No password in the Vault: no MAIL.
	f2 := newFakeSMTP(t, cert, true, false)
	s := settings(f2.port(), SecurityTLS)
	m := New(Config{Settings: func(context.Context) (Settings, error) { return s, nil }, Secrets: fakeSecrets{}, TLS: &tls.Config{RootCAs: pool}})
	if err := m.Send(context.Background(), note); err == nil || !strings.Contains(err.Error(), "password is not set") {
		t.Fatalf("err %v", err)
	}
	if f2.has("MAIL") {
		t.Fatal("MAIL without a password")
	}
}
