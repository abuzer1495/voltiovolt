// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

type fakeChannels struct {
	mu      sync.Mutex
	tests   []string
	testErr error
	code    string
	exp     time.Time
	chats   []store.TelegramChat
}

func (f *fakeChannels) Names() []string {
	return []string{notify.ChannelDesktop, notify.ChannelTelegram, notify.ChannelEmail}
}
func (f *fakeChannels) SendTest(_ context.Context, ch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tests = append(f.tests, ch)
	return f.testErr
}
func (f *fakeChannels) StartPairing() (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.code, f.exp = "123456", time.Now().Add(10*time.Minute)
	return f.code, f.exp, nil
}
func (f *fakeChannels) Pairing() (bool, time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.code != "", f.exp
}
func (f *fakeChannels) TelegramChats(context.Context) ([]store.TelegramChat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.TelegramChat(nil), f.chats...), nil
}
func (f *fakeChannels) RemoveTelegramChat(_ context.Context, id int64) error {
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

func code(err error) string {
	var we *wire.Error
	if errors.As(err, &we) {
		return we.Code
	}
	return ""
}

// fakeTelegramToken makes a fake bot token at run time.
func fakeTelegramToken() string { return strings.Repeat("7", 10) + ":" + strings.Repeat("Ab3_x", 7) }

// TestChannelMethods is VLT-IS-007 TASK 03-00-00-007 step 1.
func TestChannelMethods(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	fc := &fakeChannels{chats: []store.TelegramChat{{ID: 100, Name: "Ayşe", PairedAt: time.Now()}}}
	e.srv.b.Channels = fc

	// Not elevated: list works, changes are refused.
	ro := typed.New(e.ln.dial(t))
	if err := ro.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	// OD-50 (VLT-IS-011): set_pin keeps the elevated state. Lock.
	if err := ro.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	var list wire.ChannelsListResult
	if err := ro.Call(ctx, wire.MethodChannelsList, nil, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Channels) != 3 || !list.Channels[0].Enabled || list.Channels[1].Enabled || list.Channels[1].MinSeverity != "WARNING" ||
		list.Channels[1].Chats != 1 || list.Channels[2].Settings["port"] == nil || len(list.Presets) != 1 || list.Presets[0].Server != "smtp.gmail.com" {
		t.Fatalf("list %+v", list)
	}
	for _, m := range []struct {
		method string
		params any
	}{
		{wire.MethodChannelsSettingsSet, wire.ChannelParams{Channel: "telegram"}},
		{wire.MethodChannelsTest, wire.ChannelParams{Channel: "desktop"}},
		{wire.MethodTelegramPairStart, nil},
		{wire.MethodTelegramChatsRemove, wire.TelegramChatParams{ChatID: 100}},
	} {
		if err := ro.Call(ctx, m.method, m.params, nil); code(err) != wire.CodeNotElevated {
			t.Errorf("%s not elevated: %v", m.method, err)
		}
	}

	c := typed.New(e.ln.dial(t))
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	on, sev := true, "CRITICAL"
	if err := c.Call(ctx, wire.MethodChannelsSettingsSet, wire.ChannelSettingsParams{Channel: "email", Enabled: &on, MinSeverity: &sev,
		Settings: map[string]json.RawMessage{"server": json.RawMessage(`"smtp.gmail.com"`), "to": json.RawMessage(`["a@example.com"]`)}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, wire.MethodChannelsSettingsSet, wire.ChannelSettingsParams{Channel: "email",
		Settings: map[string]json.RawMessage{"to": json.RawMessage(`["not an address"]`)}}, nil); code(err) != wire.CodeInvalid {
		t.Fatalf("bad address: %v", err)
	}
	if err := c.Call(ctx, wire.MethodChannelsSettingsSet, wire.ChannelSettingsParams{Channel: "telegram",
		Settings: map[string]json.RawMessage{"server": json.RawMessage(`"x"`)}}, nil); code(err) != wire.CodeInvalid {
		t.Fatalf("telegram settings: %v", err)
	}
	if err := c.Call(ctx, wire.MethodChannelsSettingsSet, wire.ChannelSettingsParams{Channel: "sms", Enabled: &on}, nil); code(err) != wire.CodeInvalid {
		t.Fatalf("unknown channel: %v", err)
	}
	c.Call(ctx, wire.MethodChannelsList, nil, &list)
	if em := list.Channels[2]; !em.Enabled || em.MinSeverity != "CRITICAL" || string(em.Settings["server"]) != `"smtp.gmail.com"` || em.Secret == nil || em.Secret.Set {
		t.Fatalf("email %+v", em)
	}

	// Secrets: write-only, the list shows only the hint.
	tok := fakeTelegramToken()
	if err := c.SetSecret(ctx, "telegram.token", "not a token"); code(err) != wire.CodeInvalid {
		t.Fatalf("bad token: %v", err)
	}
	if err := c.SetSecret(ctx, "telegram.token", tok); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage{}
	c.Call(ctx, wire.MethodChannelsList, nil, &raw)
	if strings.Contains(string(raw), tok) || strings.Contains(string(raw), tok[11:]) {
		t.Fatal("channels.list has the token")
	}
	json.Unmarshal(raw, &list)
	if tg := list.Channels[1]; tg.Secret == nil || !tg.Secret.Set {
		t.Fatalf("telegram %+v", tg)
	}

	// Test message.
	if err := c.Call(ctx, wire.MethodChannelsTest, wire.ChannelParams{Channel: "desktop"}, nil); err != nil {
		t.Fatal(err)
	}
	fc.testErr = errors.New("dial tcp: connection refused")
	if err := c.Call(ctx, wire.MethodChannelsTest, wire.ChannelParams{Channel: "email"}, nil); code(err) != wire.CodeSendFailed {
		t.Fatalf("send failed: %v", err)
	}

	// Pairing and chats.
	var pr wire.TelegramPairResult
	if err := c.Call(ctx, wire.MethodTelegramPairStart, nil, &pr); err != nil || pr.Code != "123456" || pr.Expires == "" {
		t.Fatalf("pair %+v %v", pr, err)
	}
	raw = json.RawMessage{}
	c.Call(ctx, wire.MethodChannelsList, nil, &raw)
	if strings.Contains(string(raw), "123456") || !strings.Contains(string(raw), `"pairing":{"expires"`) {
		t.Fatalf("list with pairing: %s", raw)
	}
	var chats wire.TelegramChatsResult
	if err := ro.Call(ctx, wire.MethodTelegramChatsList, nil, &chats); err != nil || len(chats.Chats) != 1 || chats.Chats[0].Name != "Ayşe" {
		t.Fatalf("chats %+v %v", chats, err)
	}
	if err := c.Call(ctx, wire.MethodTelegramChatsRemove, wire.TelegramChatParams{ChatID: 100}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, wire.MethodTelegramChatsRemove, wire.TelegramChatParams{ChatID: 100}, nil); code(err) != wire.CodeNotFound {
		t.Fatalf("second remove: %v", err)
	}
	// Telegram test with no paired chat.
	if err := c.Call(ctx, wire.MethodChannelsTest, wire.ChannelParams{Channel: "telegram"}, nil); code(err) != wire.CodeInvalid {
		t.Fatalf("telegram test without chat: %v", err)
	}

	// Audit entries without the code.
	rows, _ := e.au.List(ctx, 0, 500)
	found := map[string]bool{}
	for _, r := range rows {
		found[r.Action+" "+r.Result] = true
		if strings.Contains(string(r.Details), "123456") {
			t.Fatal("pairing code in the audit log")
		}
	}
	for _, w := range []string{"channel_test success", "channel_test failure", "telegram_pair_start success", "telegram_chat_remove success", "settings_set success", "secret_set failure"} {
		if !found[w] {
			t.Errorf("no audit entry %q", w)
		}
	}
}

func TestChannelsUnavailable(t *testing.T) {
	e := newEnv(t)
	c := typed.New(e.ln.dial(t))
	c.SetPIN(context.Background(), testPIN)
	if err := c.Call(context.Background(), wire.MethodChannelsList, nil, nil); code(err) != wire.CodeUnavailable {
		t.Fatalf("err %v", err)
	}
}
