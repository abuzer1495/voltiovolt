// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package vault

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// Portable Linux host binding (VLT-IS-008 TASK 03-00-00-002 step 3).
//
// If the Secret Service (D-Bus org.freedesktop.secrets) is available, the
// DEK is kept there. If not, the DEK is kept in a file with mode 0600 in
// the data folder and the Vault state is "reduced protection". The code
// talks to the Secret Service with github.com/godbus/dbus/v5 (already a
// module dependency). The service mode keeps CredentialsProvider
// (NewHostProvider) without changes.

// PortableDEKFile is the file name of the raw DEK when the Secret Service
// is not available.
const PortableDEKFile = "vault.dek"

// secretStore is the Secret Service side of the portable provider. Tests
// give a memory store; the live store talks to D-Bus.
type secretStore interface {
	Available(ctx context.Context) bool
	Get(ctx context.Context, installID []byte) ([]byte, error)
	Put(ctx context.Context, installID, dek []byte) error
}

// PortableProvider is the portable Linux host binding.
type PortableProvider struct {
	DataDir   string
	InstallID []byte
	ss        secretStore
}

// NewPortableProvider returns the portable Linux host binding.
func NewPortableProvider(dataDir string, installID []byte) KeyProvider {
	return &PortableProvider{
		DataDir:   dataDir,
		InstallID: append([]byte(nil), installID...),
		ss:        liveSecretService{},
	}
}

// CanCreate implements KeyProvider. Portable mode makes the DEK.
func (*PortableProvider) CanCreate() bool { return true }

func (p *PortableProvider) filePath() string {
	return filepath.Join(p.DataDir, PortableDEKFile)
}

// Load implements KeyProvider. A DEK file takes precedence, so that a
// later start of the Secret Service does not hide an existing file DEK.
func (p *PortableProvider) Load(ctx context.Context) ([]byte, string, error) {
	dek, err := readDEKFile(p.filePath())
	if err == nil {
		return dek, KeyModePortableFile, nil
	}
	if !errors.Is(err, ErrNotConfigured) {
		return nil, "", err
	}
	if p.ss != nil && p.ss.Available(ctx) {
		dek, err := p.ss.Get(ctx, p.InstallID)
		if err == nil {
			return dek, KeyModeSecretService, nil
		}
		if !errors.Is(err, ErrNotConfigured) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("%w: no portable DEK", ErrNotConfigured)
}

// Create implements KeyProvider: a new DEK from crypto/rand, stored in
// the Secret Service when it is available, otherwise in a 0600 file.
func (p *PortableProvider) Create(ctx context.Context) ([]byte, string, error) {
	dek := make([]byte, DEKSize)
	if _, err := rand.Read(dek); err != nil {
		return nil, "", err
	}
	if p.ss != nil && p.ss.Available(ctx) {
		if err := p.ss.Put(ctx, p.InstallID, dek); err == nil {
			return dek, KeyModeSecretService, nil
		}
	}
	if err := writeDEKFile(p.filePath(), dek); err != nil {
		wipe(dek)
		return nil, "", err
	}
	return dek, KeyModePortableFile, nil
}

func writeDEKFile(path string, dek []byte) error {
	if len(dek) != DEKSize {
		return fmt.Errorf("vault: DEK has %d bytes", len(dek))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".new"
	_ = os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if _, err := f.Write(dek); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	ok = true
	return nil
}

func readDEKFile(path string) ([]byte, error) {
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: no %s", ErrNotConfigured, PortableDEKFile)
	}
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("vault: DEK file is not a regular file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) != DEKSize {
		wipe(b)
		return nil, fmt.Errorf("vault: DEK file has %d bytes", len(b))
	}
	return b, nil
}

// liveSecretService talks to org.freedesktop.secrets on the session bus.
type liveSecretService struct{}

const (
	ssDest   = "org.freedesktop.secrets"
	ssPath   = dbus.ObjectPath("/org/freedesktop/secrets")
	ssSvc    = "org.freedesktop.Secret.Service"
	ssItem   = "org.freedesktop.Secret.Item"
	ssColl   = "org.freedesktop.Secret.Collection"
	ssSchema = "com.voltiovolt.Vault.DEK"
	ssLabel  = "VoltioVolt DEK"
	ssWait   = 3 * time.Second
)

type ssSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func withSSTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, ssWait)
}

func connectSS(ctx context.Context) (*dbus.Conn, error) {
	c, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	var has bool
	if err := c.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, ssDest).Store(&has); err != nil || !has {
		c.Close()
		if err == nil {
			err = errors.New("vault: Secret Service is not available")
		}
		return nil, err
	}
	return c, nil
}

func ssAttrs(id []byte) map[string]string {
	return map[string]string{
		"xdg:schema": ssSchema,
		"install-id": hex.EncodeToString(id),
	}
}

func (liveSecretService) Available(ctx context.Context) bool {
	ctx, cancel := withSSTimeout(ctx)
	defer cancel()
	c, err := connectSS(ctx)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func openSSSession(ctx context.Context, c *dbus.Conn) (dbus.ObjectPath, error) {
	var output dbus.Variant
	var path dbus.ObjectPath
	err := c.Object(ssDest, ssPath).CallWithContext(ctx, ssSvc+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&output, &path)
	if err != nil {
		return "", fmt.Errorf("vault: Secret Service OpenSession: %w", err)
	}
	return path, nil
}

func closeSSSession(ctx context.Context, c *dbus.Conn, path dbus.ObjectPath) {
	if path == "" || path == "/" {
		return
	}
	_ = c.Object(ssDest, path).CallWithContext(ctx, "org.freedesktop.Secret.Session.Close", 0)
}

func unlockSS(ctx context.Context, c *dbus.Conn, objects []dbus.ObjectPath) error {
	if len(objects) == 0 {
		return nil
	}
	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := c.Object(ssDest, ssPath).CallWithContext(ctx, ssSvc+".Unlock", 0, objects).Store(&unlocked, &prompt); err != nil {
		return fmt.Errorf("vault: Secret Service Unlock: %w", err)
	}
	if prompt != "" && prompt != "/" {
		return errors.New("vault: Secret Service Unlock needs a prompt")
	}
	return nil
}

func (liveSecretService) Get(ctx context.Context, installID []byte) ([]byte, error) {
	ctx, cancel := withSSTimeout(ctx)
	defer cancel()
	c, err := connectSS(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	session, err := openSSSession(ctx, c)
	if err != nil {
		return nil, err
	}
	defer closeSSSession(ctx, c, session)
	svc := c.Object(ssDest, ssPath)
	var unlocked, locked []dbus.ObjectPath
	if err := svc.CallWithContext(ctx, ssSvc+".SearchItems", 0, ssAttrs(installID)).Store(&unlocked, &locked); err != nil {
		return nil, fmt.Errorf("vault: Secret Service SearchItems: %w", err)
	}
	items := append(append([]dbus.ObjectPath{}, unlocked...), locked...)
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: no Secret Service item", ErrNotConfigured)
	}
	if len(locked) > 0 {
		if err := unlockSS(ctx, c, locked); err != nil {
			return nil, err
		}
	}
	var sec ssSecret
	if err := c.Object(ssDest, items[0]).CallWithContext(ctx, ssItem+".GetSecret", 0, session).Store(&sec); err != nil {
		return nil, fmt.Errorf("vault: Secret Service GetSecret: %w", err)
	}
	if len(sec.Value) != DEKSize {
		wipe(sec.Value)
		return nil, fmt.Errorf("vault: Secret Service DEK has %d bytes", len(sec.Value))
	}
	dek := append([]byte(nil), sec.Value...)
	wipe(sec.Value)
	return dek, nil
}

func (liveSecretService) Put(ctx context.Context, installID, dek []byte) error {
	ctx, cancel := withSSTimeout(ctx)
	defer cancel()
	c, err := connectSS(ctx)
	if err != nil {
		return err
	}
	defer c.Close()
	session, err := openSSSession(ctx, c)
	if err != nil {
		return err
	}
	defer closeSSSession(ctx, c, session)
	svc := c.Object(ssDest, ssPath)
	var coll dbus.ObjectPath
	if err := svc.CallWithContext(ctx, ssSvc+".ReadAlias", 0, "default").Store(&coll); err != nil {
		return fmt.Errorf("vault: Secret Service default collection: %w", err)
	}
	if coll == "" || coll == "/" {
		return errors.New("vault: Secret Service has no default collection")
	}
	if err := unlockSS(ctx, c, []dbus.ObjectPath{coll}); err != nil {
		return err
	}
	props := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Label":      dbus.MakeVariant(ssLabel),
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(ssAttrs(installID)),
	}
	secret := ssSecret{Session: session, Value: dek, ContentType: "application/octet-stream"}
	var item, prompt dbus.ObjectPath
	if err := c.Object(ssDest, coll).CallWithContext(ctx, ssColl+".CreateItem", 0, props, secret, true).Store(&item, &prompt); err != nil {
		return fmt.Errorf("vault: Secret Service CreateItem: %w", err)
	}
	if prompt != "" && prompt != "/" {
		return errors.New("vault: Secret Service CreateItem needs a prompt")
	}
	if item == "" || item == "/" {
		return errors.New("vault: Secret Service CreateItem returned no item")
	}
	return nil
}
