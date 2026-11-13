// SPDX-License-Identifier: AGPL-3.0-or-later

// Package telegram is the Telegram Channel of the Notification Router
// (VLT-IS-007 TASK 03-00-00-005):
//
//  1. The Telegram Bot API over HTTPS with long polling (getUpdates). No
//     webhook.
//  2. The bot token is the Secret S-01 (Vault ID "telegram.token"). The token
//     is read from the Vault for each request and is never logged; errors
//     do not contain it.
//  3. Pairing: StartPairing makes a one-time code of 6 digits that expires
//     after 10 minutes (and after 5 wrong codes). The user sends
//     "/start <code>" to the bot. vltd records the chat ID with the name of
//     the chat.
//  4. vltd accepts commands only from paired chat IDs. It ignores other
//     chats and writes an audit entry (at most one for each chat in 10
//     minutes).
//  5. Commands: /status and /durum (the same function).
//  6. Send sends the Notifications of the Router to each paired chat.
package telegram

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// SecretID is the Vault ID of the bot token (S-01).
const SecretID = "telegram.token"

// DefaultAPI is the Telegram Bot API base URL.
const DefaultAPI = "https://api.telegram.org"

const (
	// CodeTTL is the life of a pairing code (step 3.1).
	CodeTTL = 10 * time.Minute
	// MaxCodeTries is the number of wrong codes after which the code
	// expires.
	MaxCodeTries = 5
	// PollTimeout is the long polling timeout of getUpdates in seconds.
	PollTimeout = 25
	// ignoreAuditPeriod is the minimum time between two audit entries for
	// the same not paired chat.
	ignoreAuditPeriod = 10 * time.Minute
	maxNameLen        = 64
	maxTextLen        = 4096
)

// Errors.
var (
	ErrBadToken  = errors.New("telegram: the bot token does not have the Telegram format")
	ErrNoPairing = errors.New("telegram: no pairing code")
)

// tokenFormat is the format of a bot token: "<bot ID>:<secret>".
var tokenFormat = regexp.MustCompile(`^[0-9]{1,20}:[A-Za-z0-9_-]{20,100}$`)

// ValidToken tells if b has the format of a bot token.
func ValidToken(b []byte) bool { return tokenFormat.Match(b) }

// Secrets gives the bot token (internal/vault.Vault).
type Secrets interface {
	Use(ctx context.Context, id string, fn func([]byte) error) error
}

// Chats stores the paired chats (internal/store.Store).
type Chats interface {
	ListTelegramChats(ctx context.Context) ([]store.TelegramChat, error)
	AddTelegramChat(ctx context.Context, c store.TelegramChat) error
	RemoveTelegramChat(ctx context.Context, id int64) error
}

// Auditor writes audit entries (internal/audit.Log).
type Auditor interface {
	Record(ctx context.Context, actor, action, target, result string, details map[string]any) error
}

// Config is the configuration of the Bot.
type Config struct {
	API     string       // default DefaultAPI
	HTTP    *http.Client // default: a client with a timeout of PollTimeout+15 s
	Secrets Secrets
	Chats   Chats
	Audit   Auditor
	Log     *slog.Logger
	// Enabled tells if the Telegram Channel is on. The Bot polls when it
	// is on or when a pairing code is active.
	Enabled  func(ctx context.Context) bool
	Status   func(ctx context.Context) notify.Status
	Language func(ctx context.Context) string
	Now      func() time.Time
	// Idle is the wait when the Bot is off or has no token (default 10 s).
	Idle time.Duration
}

// Bot is the Telegram Channel.
type Bot struct {
	cfg Config

	mu      sync.Mutex
	code    string
	codeExp time.Time
	tries   int
	offset  int64
	ignored map[int64]time.Time
}

// New returns a Bot.
func New(cfg Config) *Bot {
	if cfg.API == "" {
		cfg.API = DefaultAPI
	}
	cfg.API = strings.TrimRight(cfg.API, "/")
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: (PollTimeout + 15) * time.Second}
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Idle == 0 {
		cfg.Idle = 10 * time.Second
	}
	if cfg.Language == nil {
		cfg.Language = func(context.Context) string { return "en" }
	}
	return &Bot{cfg: cfg, ignored: map[int64]time.Time{}}
}

// Name is the Channel name.
func (b *Bot) Name() string { return notify.ChannelTelegram }

// StartPairing makes a new one-time code of 6 digits. A new code replaces
// the old code.
func (b *Bot) StartPairing() (code string, expires time.Time, err error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", time.Time{}, err
	}
	code = fmt.Sprintf("%06d", n.Int64())
	b.mu.Lock()
	defer b.mu.Unlock()
	b.code, b.codeExp, b.tries = code, b.cfg.Now().Add(CodeTTL), 0
	return code, b.codeExp, nil
}

// Pairing tells if a pairing code is active and when it expires.
func (b *Bot) Pairing() (active bool, expires time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.code == "" || !b.cfg.Now().Before(b.codeExp) {
		return false, time.Time{}
	}
	return true, b.codeExp
}

// checkCode compares the code in constant time. A correct code is used
// once. MaxCodeTries wrong codes end the code.
func (b *Bot) checkCode(code string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.code == "" || !b.cfg.Now().Before(b.codeExp) {
		b.code = ""
		return ErrNoPairing
	}
	if len(code) == len(b.code) && subtle.ConstantTimeCompare([]byte(code), []byte(b.code)) == 1 {
		b.code = ""
		return nil
	}
	b.tries++
	if b.tries >= MaxCodeTries {
		b.code = ""
	}
	return errors.New("wrong code")
}

// Chats returns the paired chats.
func (b *Bot) Chats(ctx context.Context) ([]store.TelegramChat, error) {
	return b.cfg.Chats.ListTelegramChats(ctx)
}

// RemoveChat removes a paired chat.
func (b *Bot) RemoveChat(ctx context.Context, id int64) error {
	return b.cfg.Chats.RemoveTelegramChat(ctx, id)
}

// apiError is an error response of the Bot API.
type apiError struct {
	Code        int
	Description string
	RetryAfter  int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("telegram: API error %d: %s", e.Code, e.Description)
}

type response struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

// call sends one Bot API request. The token is read from the Vault only for
// the request. An error never contains the token.
func (b *Bot) call(ctx context.Context, method string, req, out any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return b.cfg.Secrets.Use(ctx, SecretID, func(tok []byte) error {
		if !ValidToken(tok) {
			return ErrBadToken
		}
		err := b.do(ctx, b.cfg.API+"/bot"+string(tok)+"/"+method, body, out)
		if err == nil {
			return nil
		}
		var ae *apiError
		if errors.As(err, &ae) {
			ae.Description = redact.Scrub(strings.ReplaceAll(ae.Description, string(tok), redact.Redacted))
			return ae
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// net/http errors contain the URL; the URL contains the token.
		return fmt.Errorf("telegram: %s: %s", method, redact.Scrub(strings.ReplaceAll(err.Error(), string(tok), redact.Redacted)))
	})
}

func (b *Bot) do(ctx context.Context, url string, body []byte, out any) error {
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := b.cfg.HTTP.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var res response
	if err := json.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("HTTP status %d, response is not JSON", resp.StatusCode)
	}
	if !res.OK {
		return &apiError{Code: res.ErrorCode, Description: res.Description, RetryAfter: res.Parameters.RetryAfter}
	}
	if out != nil {
		return json.Unmarshal(res.Result, out)
	}
	return nil
}

// sendText sends one message to one chat.
func (b *Bot) sendText(ctx context.Context, chat int64, text string) error {
	if len(text) > maxTextLen {
		text = text[:maxTextLen]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	return b.call(ctx, "sendMessage", map[string]any{"chat_id": chat, "text": text}, nil)
}

// Send sends the Notification to each paired chat (step 6). With no paired
// chat, it does nothing. It returns an error only when no chat got the
// message, so that the Router tries again; a retry does not send the
// message again to a chat that got it.
func (b *Bot) Send(ctx context.Context, n notify.Notification) error {
	chats, err := b.cfg.Chats.ListTelegramChats(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range chats {
		if err := b.sendText(ctx, c.ID, n.Text); err != nil {
			errs = append(errs, err)
			b.cfg.Log.Warn("telegram message not sent", "chat", c.ID, "err", err)
		}
	}
	if len(chats) > 0 && len(errs) == len(chats) {
		return errors.Join(errs...)
	}
	return nil
}

// Update is the part of a Telegram update that the Bot uses.
type Update struct {
	ID      int64    `json:"update_id"`
	Message *Message `json:"message"`
}

// Message is a Telegram message.
type Message struct {
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
}

// Chat is a Telegram chat.
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// name is the name of the chat for the paired chat list.
func (c Chat) name() string {
	n := c.Title
	if n == "" {
		n = strings.TrimSpace(c.FirstName + " " + c.LastName)
	}
	if n == "" && c.Username != "" {
		n = "@" + c.Username
	}
	if n == "" {
		n = strconv.FormatInt(c.ID, 10)
	}
	if utf8.RuneCountInString(n) > maxNameLen {
		n = string([]rune(n)[:maxNameLen])
	}
	return n
}

// active tells if the Bot polls.
func (b *Bot) active(ctx context.Context) bool {
	if on, _ := b.Pairing(); on {
		return true
	}
	return b.cfg.Enabled != nil && b.cfg.Enabled(ctx)
}

// Run polls the Bot API until ctx is done.
func (b *Bot) Run(ctx context.Context) {
	backoff := 5 * time.Second
	var lastErr string
	for ctx.Err() == nil {
		wait := time.Duration(0)
		if !b.active(ctx) {
			wait = b.cfg.Idle
		} else if err := b.PollOnce(ctx); err != nil {
			var ae *apiError
			switch {
			case ctx.Err() != nil:
				return
			case errors.Is(err, vault.ErrNotFound), errors.Is(err, vault.ErrUnavailable), errors.Is(err, vault.ErrNotConfigured):
				wait = b.cfg.Idle
			case errors.As(err, &ae) && ae.RetryAfter > 0:
				wait = time.Duration(ae.RetryAfter) * time.Second
			default:
				wait = backoff
				backoff = min(backoff*2, time.Minute)
			}
			if err.Error() != lastErr {
				lastErr = err.Error()
				b.cfg.Log.Warn("telegram polling failed", "err", err)
			}
		} else {
			backoff, lastErr = 5*time.Second, ""
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}
}

// PollOnce sends one getUpdates request and handles the updates.
func (b *Bot) PollOnce(ctx context.Context) error {
	b.mu.Lock()
	off := b.offset
	b.mu.Unlock()
	var ups []Update
	if err := b.call(ctx, "getUpdates", map[string]any{"offset": off, "timeout": PollTimeout, "allowed_updates": []string{"message"}}, &ups); err != nil {
		return err
	}
	for _, u := range ups {
		b.mu.Lock()
		if u.ID >= b.offset {
			b.offset = u.ID + 1
		}
		b.mu.Unlock()
		if u.Message != nil {
			b.handle(ctx, *u.Message)
		}
	}
	return nil
}

func (b *Bot) record(ctx context.Context, action string, chat int64, result string, details map[string]any) {
	if b.cfg.Audit == nil {
		return
	}
	if err := b.cfg.Audit.Record(ctx, audit.ActorTelegram, action, strconv.FormatInt(chat, 10), result, details); err != nil {
		b.cfg.Log.Error("audit entry not written", "action", action, "err", err)
	}
}

func (b *Bot) paired(ctx context.Context, id int64) (bool, error) {
	chats, err := b.cfg.Chats.ListTelegramChats(ctx)
	if err != nil {
		return false, err
	}
	for _, c := range chats {
		if c.ID == id {
			return true, nil
		}
	}
	return false, nil
}

// command returns the command ("/status") and the argument. It removes a
// "@botname" suffix.
func command(text string) (cmd, arg string) {
	f := strings.Fields(text)
	if len(f) == 0 {
		return "", ""
	}
	cmd = strings.ToLower(f[0])
	if i := strings.IndexByte(cmd, '@'); i > 0 {
		cmd = cmd[:i]
	}
	if len(f) > 1 {
		arg = f[1]
	}
	return cmd, arg
}

var replies = map[string]struct{ paired, already, help string }{
	"en": {paired: "This chat is paired with VoltioVolt. Commands: /status", already: "This chat is already paired. Commands: /status", help: "Commands: /status"},
	"tr": {paired: "Bu sohbet VoltioVolt ile eşleştirildi. Komutlar: /durum", already: "Bu sohbet zaten eşleştirildi. Komutlar: /durum", help: "Komutlar: /durum"},
}

func (b *Bot) reply(ctx context.Context, chat int64, text string) {
	if err := b.sendText(ctx, chat, text); err != nil {
		b.cfg.Log.Warn("telegram reply not sent", "chat", chat, "err", err)
	}
}

func (b *Bot) handle(ctx context.Context, m Message) {
	id := m.Chat.ID
	cmd, arg := command(m.Text)
	isPaired, err := b.paired(ctx, id)
	if err != nil {
		b.cfg.Log.Error("telegram: paired chats not read", "err", err)
		return
	}
	lang := b.cfg.Language(ctx)
	rp, ok := replies[lang]
	if !ok {
		rp = replies["en"]
	}
	switch {
	case cmd == "/start" && isPaired:
		b.reply(ctx, id, rp.already)
	case cmd == "/start" && arg != "":
		if err := b.checkCode(arg); err != nil {
			b.record(ctx, "telegram_pair", id, "failure", map[string]any{"reason": err.Error()})
			return
		}
		c := store.TelegramChat{ID: id, Name: m.Chat.name(), PairedAt: b.cfg.Now().UTC()}
		if err := b.cfg.Chats.AddTelegramChat(ctx, c); err != nil {
			b.cfg.Log.Error("telegram chat not stored", "err", err)
			b.record(ctx, "telegram_pair", id, "failure", map[string]any{"reason": "store"})
			return
		}
		b.record(ctx, "telegram_pair", id, "success", map[string]any{"name": c.Name})
		b.cfg.Log.Info("telegram chat paired", "chat", id)
		b.reply(ctx, id, rp.paired)
	case !isPaired:
		b.ignore(ctx, id)
	case cmd == "/status" || cmd == "/durum":
		var st notify.Status
		if b.cfg.Status != nil {
			st = b.cfg.Status(ctx)
		}
		b.reply(ctx, id, notify.RenderStatus(lang, st, b.cfg.Now(), nil))
	default:
		b.reply(ctx, id, rp.help)
	}
}

// ignore writes an audit entry for a message of a chat that is not paired,
// at most once in ignoreAuditPeriod for each chat. It does not reply.
func (b *Bot) ignore(ctx context.Context, id int64) {
	now := b.cfg.Now()
	b.mu.Lock()
	last, seen := b.ignored[id]
	if seen && now.Sub(last) < ignoreAuditPeriod {
		b.mu.Unlock()
		return
	}
	b.ignored[id] = now
	if len(b.ignored) > 1000 {
		for k, t := range b.ignored {
			if now.Sub(t) >= ignoreAuditPeriod {
				delete(b.ignored, k)
			}
		}
	}
	b.mu.Unlock()
	b.record(ctx, "telegram_ignored", id, "denied", map[string]any{"reason": "chat not paired"})
}
