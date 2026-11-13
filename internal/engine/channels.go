// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// channelControl is ipc.ChannelControl for vltd (VLT-IS-007 TASK
// 03-00-00-007): the Router and the Telegram Channel of live.
type channelControl struct{ l *live }

func (c channelControl) Names() []string { return c.l.router.Channels() }

func (c channelControl) SendTest(ctx context.Context, channel string) error {
	return c.l.router.SendTest(ctx, channel, time.Now())
}

func (c channelControl) StartPairing() (string, time.Time, error) { return c.l.telegram.StartPairing() }

func (c channelControl) Pairing() (bool, time.Time) { return c.l.telegram.Pairing() }

func (c channelControl) TelegramChats(ctx context.Context) ([]store.TelegramChat, error) {
	return c.l.telegram.Chats(ctx)
}

func (c channelControl) RemoveTelegramChat(ctx context.Context, id int64) error {
	return c.l.telegram.RemoveChat(ctx, id)
}
