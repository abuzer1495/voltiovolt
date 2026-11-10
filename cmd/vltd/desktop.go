// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/notify"
)

// publisher is the part of ipc.Hub that the desktop Channel uses.
type publisher interface {
	Publish(kind string, data any) error
	Subscribers() int
}

// desktopChannel sends the Notifications to the VLT GUI as push messages
// (VLT-IS-007 TASK 03-00-00-004). The VLT GUI shows them as OS
// notifications. If no VLT GUI is connected, the Notification is not shown
// and not queued (step 3); the Event stays in the Events screen.
type desktopChannel struct{ hub publisher }

func (desktopChannel) Name() string { return notify.ChannelDesktop }

func (d desktopChannel) Send(_ context.Context, n notify.Notification) error {
	if d.hub.Subscribers() == 0 {
		return nil
	}
	return d.hub.Publish(wire.PushNotification, n)
}
