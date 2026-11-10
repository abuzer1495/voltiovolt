// SPDX-License-Identifier: AGPL-3.0-or-later

package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

func setup(t *testing.T) (*store.Store, *Log, *bytes.Buffer) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), store.DBFileName), store.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	var buf bytes.Buffer
	return st, New(st, slog.New(slog.NewTextHandler(&buf, nil))), &buf
}

func fill(t *testing.T, l *Log, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if err := l.Record(ctx, ActorGUI, "settings_set", "ui.language", "success", map[string]any{"i": i, "b": "x", "a": []int{1, 2}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCorrectChain(t *testing.T) {
	ctx := context.Background()
	_, l, _ := setup(t)
	if res, _ := l.Verify(ctx); !res.OK || res.Rows != 0 {
		t.Fatalf("empty: %+v", res)
	}
	fill(t, l, 20)
	res, err := l.Verify(ctx)
	if err != nil || !res.OK || res.Rows != 20 {
		t.Fatalf("verify: %+v %v", res, err)
	}
	rows, _ := l.List(ctx, 0, 5)
	if len(rows) != 5 || rows[0].Seq != 1 || rows[0].PrevHash != GenesisHash || rows[1].PrevHash != rows[0].Hash {
		t.Fatalf("rows: %+v", rows)
	}
	// Paging.
	next, _ := l.List(ctx, rows[4].Seq, 100)
	if len(next) != 15 || next[0].Seq != 6 {
		t.Fatalf("page 2: %d rows", len(next))
	}
	// Canonical details: sorted keys.
	if string(rows[0].Details) != `{"a":[1,2],"b":"x","i":0}` {
		t.Fatalf("details %s", rows[0].Details)
	}
	if res := l.VerifyAtStart(ctx); !res.OK {
		t.Fatal("VerifyAtStart")
	}
	if after, _ := l.List(ctx, 20, 10); len(after) != 0 {
		t.Fatal("VerifyAtStart wrote an entry for a correct chain")
	}
}

func dropTriggers(t *testing.T, st *store.Store) {
	t.Helper()
	for _, q := range []string{"DROP TRIGGER audit_no_update", "DROP TRIGGER audit_no_delete"} {
		if _, err := st.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChangedRow(t *testing.T) {
	ctx := context.Background()
	st, l, logBuf := setup(t)
	fill(t, l, 10)
	dropTriggers(t, st)
	st.DB().Exec("UPDATE audit SET result = 'failure' WHERE seq = 4")
	res, _ := l.Verify(ctx)
	if res.OK || res.BrokenAt != 4 || !strings.Contains(res.Reason, "changed") {
		t.Fatalf("changed row: %+v", res)
	}
	res = l.VerifyAtStart(ctx)
	if res.OK {
		t.Fatal("VerifyAtStart OK")
	}
	last, _ := l.List(ctx, 10, 10)
	if len(last) != 1 || last[0].Action != ActionChainBroken || last[0].Actor != ActorSystem {
		t.Fatalf("no audit_chain_broken entry: %+v", last)
	}
	if !strings.Contains(logBuf.String(), "audit chain broken") {
		t.Fatal("no log entry")
	}
	// A changed details value and a changed hash are also found.
	st2, l2, _ := setup(t)
	fill(t, l2, 3)
	dropTriggers(t, st2)
	st2.DB().Exec(`UPDATE audit SET details = '{"i":9}' WHERE seq = 2`)
	if res, _ := l2.Verify(ctx); res.OK || res.BrokenAt != 2 {
		t.Fatalf("changed details: %+v", res)
	}
}

func TestDeletedRow(t *testing.T) {
	ctx := context.Background()
	st, l, _ := setup(t)
	fill(t, l, 10)
	dropTriggers(t, st)
	st.DB().Exec("DELETE FROM audit WHERE seq = 5")
	res, _ := l.Verify(ctx)
	if res.OK || res.BrokenAt != 6 {
		t.Fatalf("deleted row: %+v", res)
	}
	// The first row deleted.
	st2, l2, _ := setup(t)
	fill(t, l2, 3)
	dropTriggers(t, st2)
	st2.DB().Exec("DELETE FROM audit WHERE seq = 1")
	if res, _ := l2.Verify(ctx); res.OK || res.BrokenAt != 2 {
		t.Fatalf("deleted first row: %+v", res)
	}
}

func TestAppendOnly(t *testing.T) {
	st, l, _ := setup(t)
	fill(t, l, 2)
	if _, err := st.DB().Exec("UPDATE audit SET result = 'x'"); err == nil {
		t.Fatal("UPDATE accepted")
	}
	if _, err := st.DB().Exec("DELETE FROM audit"); err == nil {
		t.Fatal("DELETE accepted")
	}
}

func TestNoSecretInDetails(t *testing.T) {
	ctx := context.Background()
	_, l, _ := setup(t)
	v := "fake-" + strings.Repeat("Zq8", 8)
	tok := strings.Repeat("4", 9) + ":" + strings.Repeat("Ab3_x", 7)
	l.Record(ctx, ActorSystem, "x", "y", "ok", map[string]any{
		"s": redact.NewString(v), "url": "https://example.invalid/?token=" + v, "msg": "bot" + tok,
	})
	rows, _ := l.List(ctx, 0, 10)
	b, _ := json.Marshal(rows)
	if bytes.Contains(b, []byte(v)) || bytes.Contains(b, []byte(tok)) {
		t.Fatalf("Secret in audit row: %s", b)
	}
	if err := l.Record(ctx, "root", "x", "y", "ok", nil); err == nil {
		t.Fatal("unknown actor accepted")
	}
}
