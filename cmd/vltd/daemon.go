// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/auth"
	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

type netListener = net.Listener

// config is the daemon configuration. main gives the real values; the
// integration tests give a temporary directory, a test DEK provider and a
// test endpoint.
type config struct {
	DataDir    string
	CheckPerms bool
	Provider   func(dataDir string, installID []byte) vault.KeyProvider
	Listen     func() (net.Listener, ipc.PeerCheck, error)
	Log        *slog.Logger
	LogCloser  io.Closer
	Version    string
}

// daemon runs the start sequence and the stop sequence of step 1 and 2.
type daemon struct {
	cfg config
	log *slog.Logger

	st        *store.Store
	au        *audit.Log
	vt        *vault.Vault
	srv       *ipc.Server
	serveDone chan error
	stopOnce  sync.Once
	stopErr   error
}

func newDaemon(cfg config) *daemon {
	l := cfg.Log
	if l == nil {
		l = slog.Default()
	}
	return &daemon{cfg: cfg, log: l}
}

// Start: 1 logging (in cfg.Log), 2 data directory check, 3 Store, 4 Vault
// (continue on failure, SEC-090), 5 audit chain verification, 6 IPC.
// The ready signal (7) is sent by platform.Run after Start.
func (d *daemon) Start(ctx context.Context) (err error) {
	d.log.Info("vltd start", "version", d.cfg.Version, "data_dir", d.cfg.DataDir)
	defer func() {
		if err != nil {
			d.log.Error("vltd start failed; vltd stops", "err", err)
			d.closeAll()
		}
	}()
	if d.cfg.CheckPerms {
		if err := store.CheckDataDir(d.cfg.DataDir); err != nil {
			return fmt.Errorf("data directory check: %w", err)
		}
		d.log.Info("data directory check passed")
	}
	st, err := store.Open(ctx, filepath.Join(d.cfg.DataDir, store.DBFileName), store.Options{
		Logger: d.log,
		OnMACFailure: func(key string) {
			if d.au != nil {
				_ = d.au.Record(context.Background(), audit.ActorSystem, "settings_mac_failed", key, "failure", nil)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	d.st = st
	if d.cfg.CheckPerms {
		// The database files exist now; check them too.
		if err := store.CheckDataDir(d.cfg.DataDir); err != nil {
			return fmt.Errorf("data directory check after store open: %w", err)
		}
	}
	d.au = audit.New(st, d.log)
	installID, err := st.InstallID(ctx)
	if err != nil {
		return fmt.Errorf("store install ID: %w", err)
	}
	d.vt = vault.Open(ctx, st, d.cfg.Provider(d.cfg.DataDir, installID), d.au, d.log)
	vs := d.vt.Status()
	d.log.Info("vault state", "state", string(vs.State), "reason", vs.Reason, "key_mode", vs.KeyMode)
	vr := d.au.VerifyAtStart(ctx)
	d.log.Info("audit chain verification", "ok", vr.OK, "rows", vr.Rows)

	ln, check, err := d.cfg.Listen()
	if err != nil {
		return fmt.Errorf("ipc: %w", err)
	}
	d.srv = ipc.NewServer(ipc.Backend{Store: st, Vault: d.vt, Auth: auth.New(st, d.au, d.log), Audit: d.au,
		Version: d.cfg.Version, Log: d.log})
	d.serveDone = make(chan error, 1)
	go func() { d.serveDone <- d.srv.Serve(ln, check) }()
	d.log.Info("ipc server started", "addr", ln.Addr().String())
	_ = d.au.Record(ctx, audit.ActorSystem, "vltd_start", "vltd", "success", map[string]any{"version": d.cfg.Version, "vault": string(vs.State)})
	return nil
}

// Stop runs the stop sequence in the reverse order: IPC, Store, logging.
// It returns an error if ctx ends first (maximum 10 seconds).
func (d *daemon) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		d.stopOnce.Do(func() {
			d.log.Info("vltd stop")
			if d.au != nil {
				_ = d.au.Record(context.Background(), audit.ActorSystem, "vltd_stop", "vltd", "success", nil)
			}
			d.stopErr = d.closeAll()
		})
		close(done)
	}()
	select {
	case <-done:
		return d.stopErr
	case <-ctx.Done():
		return fmt.Errorf("stop sequence: %w", ctx.Err())
	}
}

func (d *daemon) closeAll() error {
	var errs []error
	if d.srv != nil {
		_ = d.srv.Close()
		select {
		case err := <-d.serveDone:
			if err != nil && !errors.Is(err, ipc.ErrClosed) {
				errs = append(errs, err)
			}
		case <-time.After(5 * time.Second):
			errs = append(errs, errors.New("ipc server did not stop"))
		}
		d.srv = nil
		d.log.Info("ipc server stopped")
	}
	if d.st != nil {
		if err := d.st.Close(); err != nil {
			errs = append(errs, err)
		}
		d.st = nil
		d.log.Info("store closed")
	}
	if d.cfg.LogCloser != nil {
		d.log.Info("vltd stopped")
		if err := d.cfg.LogCloser.Close(); err != nil {
			errs = append(errs, err)
		}
		d.cfg.LogCloser = nil
	}
	return errors.Join(errs...)
}
