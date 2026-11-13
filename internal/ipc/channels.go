// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/notify"
	"github.com/abuzer1495/voltiovolt/internal/notify/email"
	"github.com/abuzer1495/voltiovolt/internal/notify/telegram"
	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// ChannelControl is the Notification part of vltd (VLT-IS-007 TASK
// 03-00-00-007): the Router and the Telegram Channel.
type ChannelControl interface {
	// Names returns the Channel names in the order of the Router.
	Names() []string
	// SendTest sends a test message on one Channel at once.
	SendTest(ctx context.Context, channel string) error
	StartPairing() (code string, expires time.Time, err error)
	Pairing() (active bool, expires time.Time)
	TelegramChats(ctx context.Context) ([]store.TelegramChat, error)
	RemoveTelegramChat(ctx context.Context, id int64) error
}

// emailKeys maps the names of channels.settings.set to the Setting keys of
// the e-mail Channel.
var emailKeys = map[string]string{
	"server":   store.SettingEmailServer,
	"port":     store.SettingEmailPort,
	"security": store.SettingEmailSecurity,
	"user":     store.SettingEmailUser,
	"from":     store.SettingEmailFrom,
	"to":       store.SettingEmailTo,
}

var emailOrder = []string{"server", "port", "security", "user", "from", "to"}

// channelSecret is the Vault ID of the Secret of a Channel ("" = none).
func channelSecret(name string) string {
	switch name {
	case notify.ChannelTelegram:
		return telegram.SecretID
	case notify.ChannelEmail:
		return email.SecretID
	}
	return ""
}

func (cn *conn) channelInfo(ctx context.Context, name string) (wire.ChannelInfo, error) {
	b := cn.s.b
	ci := wire.ChannelInfo{Name: name}
	ek, sk, ok := notify.SettingKeys(name)
	if !ok {
		return ci, errors.New("unknown channel")
	}
	if s, err := b.Store.GetSetting(ctx, ek); err == nil {
		_ = json.Unmarshal(s.Value, &ci.Enabled)
	}
	if s, err := b.Store.GetSetting(ctx, sk); err == nil {
		_ = json.Unmarshal(s.Value, &ci.MinSeverity)
	}
	if id := channelSecret(name); id != "" {
		st, err := b.Vault.SecretStatus(ctx, id)
		if err != nil {
			return ci, err
		}
		ci.Secret = &wire.SecretStatusResult{ID: st.ID, Set: st.Set, Hint: st.Hint, UpdatedAt: st.UpdatedAt}
	}
	switch name {
	case notify.ChannelEmail:
		ci.Settings = map[string]json.RawMessage{}
		for _, k := range emailOrder {
			s, err := b.Store.GetSetting(ctx, emailKeys[k])
			if err != nil {
				return ci, err
			}
			ci.Settings[k] = s.Value
		}
	case notify.ChannelTelegram:
		if on, exp := b.Channels.Pairing(); on {
			// The code is shown only once, in the result of
			// telegram.pair.start.
			ci.Pairing = &wire.TelegramPairResult{Expires: exp.UTC().Format(time.RFC3339)}
		}
		chats, err := b.Channels.TelegramChats(ctx)
		if err != nil {
			return ci, err
		}
		ci.Chats = len(chats)
	}
	return ci, nil
}

func knownChannel(b Backend, name string) bool {
	for _, n := range b.Channels.Names() {
		if n == name {
			return true
		}
	}
	return false
}

// testError maps a channels.test error to a wire error. The message has no
// Secret (redact.Scrub).
func testError(err error) *wire.Error {
	switch {
	case errors.Is(err, email.ErrNotConfigured):
		return invalid("the e-mail Settings are not complete")
	case errors.Is(err, errNoChat):
		return invalid("no paired Telegram chat")
	case errors.Is(err, notify.ErrNoChannel):
		return invalid("unknown channel")
	}
	return &wire.Error{Code: wire.CodeSendFailed, Message: redact.Scrub(err.Error())}
}

var errNoChat = errors.New("no paired chat")

// dispatchChannels serves the Channel methods (VLT-IS-007 TASK
// 03-00-00-007). The access rules (elevated for all changes) are in
// methodAccess.
func (cn *conn) dispatchChannels(ctx context.Context, req wire.Request) (any, *wire.Error) {
	b := cn.s.b
	if b.Channels == nil {
		return nil, unavailable("notification channels")
	}
	switch req.Method {
	case wire.MethodChannelsList:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		res := wire.ChannelsListResult{Presets: []wire.EmailPreset{{Name: email.Gmail.Name, Server: email.Gmail.Server, Port: email.Gmail.Port, Security: email.Gmail.Security}}}
		for _, n := range b.Channels.Names() {
			ci, err := cn.channelInfo(ctx, n)
			if err != nil {
				return nil, internal()
			}
			res.Channels = append(res.Channels, ci)
		}
		return res, nil

	case wire.MethodChannelsSettingsSet:
		var p wire.ChannelSettingsParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if !knownChannel(b, p.Channel) {
			return nil, invalid("unknown channel")
		}
		ek, sk, _ := notify.SettingKeys(p.Channel)
		values := map[string]json.RawMessage{}
		order := []string{}
		add := func(k string, v json.RawMessage) { values[k] = v; order = append(order, k) }
		if p.Enabled != nil {
			v, _ := json.Marshal(*p.Enabled)
			add(ek, v)
		}
		if p.MinSeverity != nil {
			v, _ := json.Marshal(*p.MinSeverity)
			add(sk, v)
		}
		if len(p.Settings) > 0 {
			if p.Channel != notify.ChannelEmail {
				return nil, invalid("this channel has no other Settings")
			}
			for _, k := range emailOrder {
				if v, ok := p.Settings[k]; ok {
					add(emailKeys[k], v)
				}
			}
			for k := range p.Settings {
				if _, ok := emailKeys[k]; !ok {
					return nil, invalid("not an e-mail Setting: " + k)
				}
			}
		}
		if len(values) == 0 {
			return nil, invalid("no Setting to change")
		}
		if err := checkAll(values); err != nil {
			return nil, invalid(err.Error())
		}
		for _, k := range order {
			ver, err := b.Store.SetSetting(ctx, k, values[k], audit.ActorGUI)
			if err != nil {
				cn.record(ctx, "settings_set", k, "failure", map[string]any{"reason": err.Error()})
				if errors.Is(err, store.ErrVaultUnavailable) {
					return nil, &wire.Error{Code: wire.CodeVaultUnavailable, Message: "vault not available"}
				}
				return nil, invalid(err.Error())
			}
			cn.record(ctx, "settings_set", k, "success", map[string]any{"version": ver, "value": values[k]})
		}
		return wire.OKResult{OK: true}, nil

	case wire.MethodChannelsTest:
		var p wire.ChannelParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if !knownChannel(b, p.Channel) {
			return nil, invalid("unknown channel")
		}
		err := error(nil)
		if p.Channel == notify.ChannelTelegram {
			if chats, e := b.Channels.TelegramChats(ctx); e == nil && len(chats) == 0 {
				err = errNoChat
			}
		}
		if err == nil {
			err = b.Channels.SendTest(ctx, p.Channel)
		}
		if err != nil {
			cn.record(ctx, "channel_test", p.Channel, "failure", map[string]any{"reason": err.Error()})
			return nil, testError(err)
		}
		cn.record(ctx, "channel_test", p.Channel, "success", nil)
		return wire.OKResult{OK: true}, nil

	case wire.MethodTelegramPairStart:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		code, exp, err := b.Channels.StartPairing()
		if err != nil {
			return nil, internal()
		}
		// The code is not written to the audit log.
		cn.record(ctx, "telegram_pair_start", "telegram", "success", map[string]any{"expires": exp.UTC().Format(time.RFC3339)})
		return wire.TelegramPairResult{Code: code, Expires: exp.UTC().Format(time.RFC3339)}, nil

	case wire.MethodTelegramChatsList:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		chats, err := b.Channels.TelegramChats(ctx)
		if err != nil {
			return nil, internal()
		}
		res := wire.TelegramChatsResult{Chats: make([]wire.TelegramChat, 0, len(chats))}
		for _, c := range chats {
			res.Chats = append(res.Chats, wire.TelegramChat{ChatID: c.ID, Name: c.Name, PairedAt: c.PairedAt.UTC().Format(time.RFC3339)})
		}
		return res, nil

	case wire.MethodTelegramChatsRemove:
		var p wire.TelegramChatParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		target := strconv.FormatInt(p.ChatID, 10)
		if err := b.Channels.RemoveTelegramChat(ctx, p.ChatID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, &wire.Error{Code: wire.CodeNotFound, Message: "chat not paired"}
			}
			cn.record(ctx, "telegram_chat_remove", target, "failure", map[string]any{"reason": err.Error()})
			return nil, internal()
		}
		cn.record(ctx, "telegram_chat_remove", target, "success", nil)
		return wire.OKResult{OK: true}, nil
	}
	return nil, &wire.Error{Code: wire.CodeUnknownMethod, Message: "unknown method"}
}
