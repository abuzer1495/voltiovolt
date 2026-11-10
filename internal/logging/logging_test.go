// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/redact"
)

func TestNewLoggerRedacts(t *testing.T) {
	var buf bytes.Buffer
	v := "s3cr3t-" + strings.Repeat("Qz9", 6)
	l := NewLogger(&buf, nil)
	l.Info("x", "s", redact.NewString(v), "url", "https://example.invalid/?token="+v)
	if strings.Contains(buf.String(), v) {
		t.Fatalf("Secret in output: %s", buf.String())
	}
}

func TestRotatingFile(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRotatingFile(dir, "t.log", 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	line := []byte(strings.Repeat("x", 39) + "\n")
	for i := 0; i < 40; i++ {
		if _, err := r.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 5 {
		t.Fatalf("got %d files, want 5", len(ents))
	}
	for i := 0; i < 5; i++ {
		name := "t.log"
		if i > 0 {
			name = fmt.Sprintf("t.log.%d", i)
		}
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if st.Size() > 100 {
			t.Errorf("%s size %d > 100", name, st.Size())
		}
	}
	if _, err := r.Write(line); err == nil {
		t.Error("write after Close must fail")
	}
}

func TestRotationValues(t *testing.T) {
	if MaxFileBytes != 10*1024*1024 || MaxFiles != 5 {
		t.Fatal("rotation values differ from VLT-IS-003 TASK 03-00-00-002 step 6.2")
	}
}

func TestOpen(t *testing.T) {
	l, c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l.Info("open test")
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
