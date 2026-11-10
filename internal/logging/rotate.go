// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RotatingFile is an io.WriteCloser. It writes to dir/name. When a write
// would make the file larger than maxBytes, it renames name to name.1,
// name.1 to name.2 and so on, and keeps maxFiles files in total.
type RotatingFile struct {
	mu       sync.Mutex
	dir      string
	name     string
	maxBytes int64
	maxFiles int
	f        *os.File
	size     int64
}

// NewRotatingFile opens (or makes) dir/name with mode 0600.
func NewRotatingFile(dir, name string, maxBytes int64, maxFiles int) (*RotatingFile, error) {
	if maxBytes <= 0 || maxFiles < 1 {
		return nil, errors.New("logging: bad rotation values")
	}
	r := &RotatingFile{dir: dir, name: name, maxBytes: maxBytes, maxFiles: maxFiles}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RotatingFile) path(i int) string {
	if i == 0 {
		return filepath.Join(r.dir, r.name)
	}
	return filepath.Join(r.dir, fmt.Sprintf("%s.%d", r.name, i))
}

func (r *RotatingFile) open() error {
	f, err := os.OpenFile(r.path(0), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	r.f, r.size = f, st.Size()
	return nil
}

func (r *RotatingFile) rotate() error {
	if err := r.f.Close(); err != nil {
		return err
	}
	r.f = nil
	_ = os.Remove(r.path(r.maxFiles - 1))
	for i := r.maxFiles - 2; i >= 0; i-- {
		if _, err := os.Stat(r.path(i)); err == nil {
			if err := os.Rename(r.path(i), r.path(i+1)); err != nil {
				return err
			}
		}
	}
	return r.open()
}

// Write implements io.Writer.
func (r *RotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return 0, os.ErrClosed
	}
	if r.size > 0 && r.size+int64(len(p)) > r.maxBytes {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := r.f.Write(p)
	r.size += int64(n)
	return n, err
}

// Close implements io.Closer.
func (r *RotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}
