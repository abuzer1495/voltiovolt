// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package server

import (
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

// ListenExclusive listens on the endpoint of OD-44 and takes a file lock
// so that a second vlt-simulator process stops with ErrAlreadyRunning.
// The lock is released when the listener is closed or the process ends.
func ListenExclusive() (net.Listener, error) {
	return listenExclusive(Path(), lockPath(Path()))
}

func lockPath(sock string) string {
	return strings.TrimSuffix(sock, ".sock") + ".lock"
}

func listenExclusive(path, lock string) (net.Listener, error) {
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil && !os.IsExist(err) {
		if dirOf(path) != "/tmp" {
			return nil, err
		}
	}
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if err == unix.EWOULDBLOCK || err == unix.EAGAIN {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	ln, err := ListenAt(path)
	if err != nil {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
		return nil, err
	}
	return &lockedListener{Listener: ln, lock: f}, nil
}

func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i <= 0 {
		return "."
	}
	return path[:i]
}

type lockedListener struct {
	net.Listener
	lock *os.File
	once sync.Once
	err  error
}

func (l *lockedListener) Close() error {
	l.once.Do(func() {
		l.err = l.Listener.Close()
		if l.lock != nil {
			_ = unix.Flock(int(l.lock.Fd()), unix.LOCK_UN)
			_ = l.lock.Close()
			l.lock = nil
		}
	})
	return l.err
}
