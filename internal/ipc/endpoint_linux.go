// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// DefaultSocket is the vltd socket path (step 6.1).
const DefaultSocket = "/run/vltd/vltd.sock"

// DefaultGroup is the group that can use the socket.
const DefaultGroup = "vlt"

// Endpoint is the Linux IPC endpoint: a Unix socket, mode 0660, group
// "vlt", in a directory with mode 0750. A peer is accepted if its uid is 0
// or if it is a member of the group (SO_PEERCRED).
type Endpoint struct {
	Path string // default DefaultSocket
	// Group is the group name (default "vlt"). GID, if not negative, is
	// used and Group is not looked up (tests).
	Group string
	GID   int
	// SetDirMode: set mode 0750 and the group on the socket directory. The
	// systemd RuntimeDirectory= makes the directory.
	SetDirMode bool
	// lookupGroups returns the group IDs of a uid. Tests can change it.
	lookupGroups func(uid uint32) ([]string, error)
}

// DefaultEndpoint returns the vltd endpoint.
func DefaultEndpoint() Endpoint {
	return Endpoint{Path: DefaultSocket, Group: DefaultGroup, GID: -1, SetDirMode: true}
}

func (e Endpoint) gid() (int, error) {
	if e.GID >= 0 {
		return e.GID, nil
	}
	name := e.Group
	if name == "" {
		name = DefaultGroup
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("ipc: group %q: %w", name, err)
	}
	return strconv.Atoi(g.Gid)
}

// Listen makes the socket. It removes an old socket file at the path.
func (e Endpoint) Listen() (net.Listener, PeerCheck, error) {
	if e.Path == "" {
		e.Path = DefaultSocket
	}
	gid, err := e.gid()
	if err != nil {
		return nil, nil, err
	}
	dir := filepath.Dir(e.Path)
	if e.SetDirMode {
		if err := os.Chmod(dir, 0o750); err != nil {
			return nil, nil, fmt.Errorf("ipc: socket directory: %w", err)
		}
		if err := os.Chown(dir, -1, gid); err != nil {
			return nil, nil, fmt.Errorf("ipc: socket directory group: %w", err)
		}
	}
	if fi, err := os.Lstat(e.Path); err == nil {
		if fi.Mode()&os.ModeSocket == 0 {
			return nil, nil, fmt.Errorf("ipc: %s exists and is not a socket", e.Path)
		}
		if err := os.Remove(e.Path); err != nil {
			return nil, nil, err
		}
	}
	old := syscall.Umask(0o177)
	ln, err := net.Listen("unix", e.Path)
	syscall.Umask(old)
	if err != nil {
		return nil, nil, err
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(true)
	if err := os.Chown(e.Path, -1, gid); err != nil {
		ln.Close()
		return nil, nil, fmt.Errorf("ipc: socket group: %w", err)
	}
	if err := os.Chmod(e.Path, 0o660); err != nil {
		ln.Close()
		return nil, nil, err
	}
	lg := e.lookupGroups
	if lg == nil {
		lg = userGroups
	}
	return ln, peerCheck(uint32(gid), lg), nil
}

func userGroups(uid uint32) ([]string, error) {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return nil, err
	}
	return u.GroupIds()
}

// PeerCred returns the SO_PEERCRED values of a Unix socket connection.
func PeerCred(c net.Conn) (*unix.Ucred, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return nil, errors.New("not a Unix socket connection")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return nil, err
	}
	var cred *unix.Ucred
	var cerr error
	if err := raw.Control(func(fd uintptr) {
		cred, cerr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return nil, err
	}
	return cred, cerr
}

func peerCheck(gid uint32, groups func(uint32) ([]string, error)) PeerCheck {
	return func(c net.Conn) error {
		cred, err := PeerCred(c)
		if err != nil {
			return fmt.Errorf("SO_PEERCRED: %w", err)
		}
		if cred.Uid == 0 || cred.Gid == gid {
			return nil
		}
		ids, err := groups(cred.Uid)
		if err != nil {
			return fmt.Errorf("uid %d: groups: %w", cred.Uid, err)
		}
		if slices.Contains(ids, strconv.FormatUint(uint64(gid), 10)) {
			return nil
		}
		return fmt.Errorf("uid %d is not root and not a member of gid %d", cred.Uid, gid)
	}
}
