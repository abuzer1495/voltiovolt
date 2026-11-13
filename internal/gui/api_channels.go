// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// Notification Channels (VLT-IS-007 TASK 03-00-00-007). The Secrets of the
// Channels (Telegram bot token, SMTP password) go only through SetSecret;
// no method gives them back.

// Channels calls channels.list.
func (a *API) Channels() (json.RawMessage, error) { return a.raw(wire.MethodChannelsList, nil) }

// SetChannel calls channels.settings.set (elevated): on or off, the
// minimum severity and, for the e-mail Channel, the SMTP Settings
// (server, port, security, user, from, to). A nil settings map changes no
// SMTP Setting.
func (a *API) SetChannel(channel string, enabled bool, minSeverity string, settings map[string]any) (json.RawMessage, error) {
	p := wire.ChannelSettingsParams{Channel: channel, Enabled: &enabled, MinSeverity: &minSeverity}
	if len(settings) > 0 {
		p.Settings = map[string]json.RawMessage{}
		for k, v := range settings {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, &Error{Code: wire.CodeInvalid}
			}
			p.Settings[k] = b
		}
	}
	return a.raw(wire.MethodChannelsSettingsSet, p)
}

// ChannelTest calls channels.test (elevated): a test message at once.
func (a *API) ChannelTest(channel string) (json.RawMessage, error) {
	return a.raw(wire.MethodChannelsTest, wire.ChannelParams{Channel: channel})
}

// TelegramPairStart calls telegram.pair.start (elevated): a new code of 6
// digits that expires after 10 minutes.
func (a *API) TelegramPairStart() (json.RawMessage, error) {
	return a.raw(wire.MethodTelegramPairStart, nil)
}

// TelegramChats calls telegram.chats.list.
func (a *API) TelegramChats() (json.RawMessage, error) {
	return a.raw(wire.MethodTelegramChatsList, nil)
}

// TelegramChatRemove calls telegram.chats.remove (elevated).
func (a *API) TelegramChatRemove(chatID int64) (json.RawMessage, error) {
	return a.raw(wire.MethodTelegramChatsRemove, wire.TelegramChatParams{ChatID: chatID})
}
