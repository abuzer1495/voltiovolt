// SPDX-License-Identifier: AGPL-3.0-or-later

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// fakeToken makes a fake bot token at run time, so that no token text is in
// the repository.
func fakeToken() string { return strings.Repeat("7", 10) + ":" + strings.Repeat("Ab3_x", 7) }

type fakeSecrets struct{ tok string }

func (f fakeSecrets) Use(_ context.Context, id string, fn func([]byte) error) error {
	if id != SecretID {
		return errors.New("wrong id")
	}
	if f.tok == "" {
		return vault.ErrNotFound
	}
	return fn([]byte(f.tok))
}

type fakeChats struct {
	mu    sync.Mutex
	chats []store.TelegramChat
}

func (f *fakeChats) ListTelegramChats(context.Context) ([]store.TelegramChat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.TelegramChat(nil), f.chats...), nil
}

func (f *fakeChats) AddTelegramChat(_ context.Context, c store.TelegramChat) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chats = append(f.chats, c)
	return nil
}

func (f *fakeChats) RemoveTelegramChat(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, c := range f.chats {
		if c.ID == id {
			f.chats = append(f.chats[:i], f.chats[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

type auditRow struct{ actor, action, target, result string }

type fakeAudit struct {
	mu   sync.Mutex
	rows []auditRow
}

func (f *fakeAudit) Record(_ context.Context, actor, action, target, result string, _ map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, auditRow{actor, action, target, result})
	return nil
}

// fakeAPI is a fake Telegram Bot API server (HTTPS).
type fakeAPI struct {
	t       *testing.T
	srv     *httptest.Server
	tok     string
	mu      sync.Mutex
	updates []Update
	sent    []map[string]any
	offsets []float64
	fail    bool
}

func newFakeAPI(t *testing.T, tok string) *fakeAPI {
	f := &fakeAPI{t: t, tok: tok}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/bot"+f.tok+"/") {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
		return
	}
	var req map[string]any
	_ = json.NewDecoder(r.Body).Decode(&req)
	if f.fail {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"ok":false,"error_code":500,"description":"Internal Server Error"}`)
		return
	}
	switch strings.TrimPrefix(r.URL.Path, "/bot"+f.tok+"/") {
	case "getUpdates":
		off, _ := req["offset"].(float64)
		f.offsets = append(f.offsets, off)
		var out []Update
		for _, u := range f.updates {
			if float64(u.ID) >= off {
				out = append(out, u)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": out})
	case "sendMessage":
		f.sent = append(f.sent, req)
		io.WriteString(w, `{"ok":true,"result":{}}`)
	default:
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"ok":false,"error_code":404,"description":"Not Found"}`)
	}
}

func (f *fakeAPI) add(id, chat int64, name, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, Update{ID: id, Message: &Message{Chat: Chat{ID: chat, Type: "private", FirstName: name}, Text: text}})
}

func (f *fakeAPI) sentTo() (chats []float64, texts []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.sent {
		c, _ := m["chat_id"].(float64)
		s, _ := m["text"].(string)
		chats = append(chats, c)
		texts = append(texts, s)
	}
	return
}

type env struct {
	api   *fakeAPI
	bot   *Bot
	chats *fakeChats
	audit *fakeAudit
	now   time.Time
	log   *bytes.Buffer
}

func newEnv(t *testing.T) *env {
	tok := fakeToken()
	e := &env{api: newFakeAPI(t, tok), chats: &fakeChats{}, audit: &fakeAudit{}, now: time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC), log: &bytes.Buffer{}}
	e.bot = New(Config{
		API: e.api.srv.URL, HTTP: e.api.srv.Client(), Secrets: fakeSecrets{tok}, Chats: e.chats, Audit: e.audit,
		Log:     slog.New(slog.NewTextHandler(e.log, nil)),
		Enabled: func(context.Context) bool { return true },
		Status: func(context.Context) notify.Status {
			return notify.Status{Snapshot: notify.Snapshot{Mode: notify.ModeMains}}
		},
		Language: func(context.Context) string { return "tr" },
		Now:      func() time.Time { return e.now },
	})
	return e
}

func TestPairing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	code, exp, err := e.bot.StartPairing()
	if err != nil || len(code) != 6 || exp != e.now.Add(10*time.Minute) {
		t.Fatalf("code %q exp %v err %v", code, exp, err)
	}
	if on, _ := e.bot.Pairing(); !on {
		t.Fatal("pairing not active")
	}
	e.api.add(1, 100, "Ayşe", "/start "+code)
	if err := e.bot.PollOnce(ctx); err != nil {
		t.Fatal(err)
	}
	chats, _ := e.bot.Chats(ctx)
	if len(chats) != 1 || chats[0].ID != 100 || chats[0].Name != "Ayşe" {
		t.Fatalf("chats %+v", chats)
	}
	if on, _ := e.bot.Pairing(); on {
		t.Fatal("code not used once")
	}
	if r := e.audit.rows; len(r) != 1 || r[0] != (auditRow{"telegram", "telegram_pair", "100", "success"}) {
		t.Fatalf("audit %+v", r)
	}
	to, texts := e.api.sentTo()
	if len(to) != 1 || to[0] != 100 || !strings.Contains(texts[0], "eşleştirildi") {
		t.Fatalf("reply %v %v", to, texts)
	}
	// The next poll uses the next offset.
	if err := e.bot.PollOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if o := e.api.offsets; len(o) != 2 || o[1] != 2 {
		t.Fatalf("offsets %v", o)
	}
}

func TestPairingWrongAndExpiredCode(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	code, _, _ := e.bot.StartPairing()
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < MaxCodeTries; i++ {
		e.api.add(int64(i+1), 200, "Mallory", "/start "+wrong)
	}
	e.api.add(10, 200, "Mallory", "/start "+code) // after 5 wrong codes, the code is not valid
	if err := e.bot.PollOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if chats, _ := e.bot.Chats(ctx); len(chats) != 0 {
		t.Fatalf("paired after wrong codes: %+v", chats)
	}
	if n := len(e.audit.rows); n != MaxCodeTries+1 || e.audit.rows[0].result != "failure" {
		t.Fatalf("audit %+v", e.audit.rows)
	}
	if to, _ := e.api.sentTo(); len(to) != 0 {
		t.Fatalf("reply to a not paired chat: %v", to)
	}
	// Expired code.
	code, _, _ = e.bot.StartPairing()
	e.now = e.now.Add(CodeTTL)
	e.api.add(11, 300, "Late", "/start "+code)
	e.bot.PollOnce(ctx)
	if chats, _ := e.bot.Chats(ctx); len(chats) != 0 {
		t.Fatalf("paired with an expired code: %+v", chats)
	}
}

func TestCommandsOnlyFromPairedChats(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.chats.AddTelegramChat(ctx, store.TelegramChat{ID: 100, Name: "Ayşe"})
	e.api.add(1, 100, "Ayşe", "/durum")
	e.api.add(2, 100, "Ayşe", "/status@VltBot")
	e.api.add(3, 999, "Other", "/status")
	e.api.add(4, 999, "Other", "/durum") // second message of the same chat: no second audit entry
	e.api.add(5, 100, "Ayşe", "hello")
	if err := e.bot.PollOnce(ctx); err != nil {
		t.Fatal(err)
	}
	to, texts := e.api.sentTo()
	if len(to) != 3 || to[0] != 100 || to[1] != 100 || to[2] != 100 {
		t.Fatalf("replies to %v", to)
	}
	if !strings.Contains(texts[0], "VLT durumu") || !strings.Contains(texts[0], "Mod: Şebeke gücü") || texts[0] != texts[1] {
		t.Fatalf("status texts %q %q", texts[0], texts[1])
	}
	if !strings.Contains(texts[2], "/durum") {
		t.Fatalf("help %q", texts[2])
	}
	if r := e.audit.rows; len(r) != 1 || r[0] != (auditRow{"telegram", "telegram_ignored", "999", "denied"}) {
		t.Fatalf("audit %+v", r)
	}
	// After 10 minutes, a new audit entry.
	e.now = e.now.Add(ignoreAuditPeriod)
	e.api.add(6, 999, "Other", "/status")
	e.bot.PollOnce(ctx)
	if len(e.audit.rows) != 2 {
		t.Fatalf("audit %+v", e.audit.rows)
	}
}

func TestSend(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	n := notify.Notification{Subject: "[VLT][CRITICAL] On battery", Text: "[VLT][CRITICAL] On battery\nMode: On battery"}
	// No paired chat: nothing to send, no error.
	if err := e.bot.Send(ctx, n); err != nil {
		t.Fatal(err)
	}
	e.chats.AddTelegramChat(ctx, store.TelegramChat{ID: 100})
	e.chats.AddTelegramChat(ctx, store.TelegramChat{ID: -200})
	if err := e.bot.Send(ctx, n); err != nil {
		t.Fatal(err)
	}
	to, texts := e.api.sentTo()
	if len(to) != 2 || to[0] != 100 || to[1] != -200 || texts[0] != n.Text {
		t.Fatalf("sent %v %v", to, texts)
	}
	e.api.mu.Lock()
	e.api.fail = true
	e.api.mu.Unlock()
	if err := e.bot.Send(ctx, n); err == nil {
		t.Fatal("no error when no chat got the message")
	}
}

// TestNoTokenInErrorsAndLogs: the token is not in an error or a log entry.
func TestNoTokenInErrorsAndLogs(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.chats.AddTelegramChat(ctx, store.TelegramChat{ID: 100})
	e.api.srv.Close() // connection errors contain the URL
	err := e.bot.Send(ctx, notify.Notification{Text: "x"})
	if err == nil {
		t.Fatal("no error")
	}
	if strings.Contains(err.Error(), fakeToken()) || strings.Contains(e.log.String(), fakeToken()) {
		t.Fatalf("token in the error or the log: %v\n%s", err, e.log.String())
	}
	if !strings.Contains(err.Error(), "/bot[REDACTED]") {
		t.Fatalf("error %v", err)
	}
}

func TestTokenFormatAndNoToken(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.bot.cfg.Secrets = fakeSecrets{"not a token/../x"}
	if err := e.bot.PollOnce(ctx); !errors.Is(err, ErrBadToken) {
		t.Fatalf("err %v", err)
	}
	e.bot.cfg.Secrets = fakeSecrets{""}
	if err := e.bot.PollOnce(ctx); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("err %v", err)
	}
	if len(e.api.offsets) != 0 {
		t.Fatal("request without a valid token")
	}
	if !ValidToken([]byte(fakeToken())) {
		t.Fatal("fake token not valid")
	}
}

func TestRunPollsAndStops(t *testing.T) {
	e := newEnv(t)
	e.chats.AddTelegramChat(context.Background(), store.TelegramChat{ID: 100})
	e.api.add(1, 100, "Ayşe", "/status")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.bot.Run(ctx); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if to, _ := e.api.sentTo(); len(to) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no reply")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestChatName(t *testing.T) {
	for _, c := range []struct {
		chat Chat
		want string
	}{
		{Chat{ID: 1, Title: "Family"}, "Family"},
		{Chat{ID: 1, FirstName: "Ali", LastName: "Veli"}, "Ali Veli"},
		{Chat{ID: 1, Username: "ali"}, "@ali"},
		{Chat{ID: 42}, "42"},
		{Chat{ID: 1, Title: strings.Repeat("ş", 100)}, strings.Repeat("ş", 64)},
	} {
		if got := c.chat.name(); got != c.want {
			t.Errorf("name %q, want %q", got, c.want)
		}
	}
}
