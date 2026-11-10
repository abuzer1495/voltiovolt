// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"context"
	"encoding/json"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// Channel names.
const (
	ChannelDesktop  = "desktop"
	ChannelTelegram = "telegram"
	ChannelEmail    = "email"
)

// settingKeys are the Setting keys of each Channel (enabled, minimum
// severity).
var settingKeys = map[string][2]string{
	ChannelDesktop:  {store.SettingNotifyDesktopEnabled, store.SettingNotifyDesktopSeverity},
	ChannelTelegram: {store.SettingNotifyTelegramEnabled, store.SettingNotifyTelegramSeverity},
	ChannelEmail:    {store.SettingNotifyEmailEnabled, store.SettingNotifyEmailSeverity},
}

// SettingKeys returns the enabled and the minimum severity Setting keys of
// a Channel, and false for an unknown Channel.
func SettingKeys(channel string) (enabled, minSeverity string, ok bool) {
	k, ok := settingKeys[channel]
	return k[0], k[1], ok
}

// StoreSettings reads the Channel Settings from the Store. An error or an
// unknown Channel gives "off".
func StoreSettings(st *store.Store) func(ctx context.Context, channel string) ChannelSettings {
	return func(ctx context.Context, channel string) ChannelSettings {
		k, ok := settingKeys[channel]
		if !ok {
			return ChannelSettings{}
		}
		var cs ChannelSettings
		s, err := st.GetSetting(ctx, k[0])
		if err != nil || json.Unmarshal(s.Value, &cs.Enabled) != nil {
			return ChannelSettings{}
		}
		s, err = st.GetSetting(ctx, k[1])
		if err != nil || json.Unmarshal(s.Value, &cs.MinSeverity) != nil {
			cs.MinSeverity = Warning
		}
		return cs
	}
}

// StoreLanguage reads the Setting ui.language. An error gives "en".
func StoreLanguage(st *store.Store) func(ctx context.Context) string {
	return func(ctx context.Context) string {
		s, err := st.GetSetting(ctx, store.SettingLanguage)
		var l string
		if err != nil || json.Unmarshal(s.Value, &l) != nil {
			return "en"
		}
		return l
	}
}
