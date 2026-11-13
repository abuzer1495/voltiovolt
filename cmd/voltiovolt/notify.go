// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/abuzer1495/voltiovolt/internal/gui"
)

// osNotifier shows the desktop Notifications of vltd as OS notifications
// (VLT-IS-007 TASK 03-00-00-004): Windows toast notifications, and on Linux
// org.freedesktop.Notifications over D-Bus. It wraps the Wails
// notifications service so that a start failure (for example no D-Bus
// session) does not stop the VLT GUI: the GUI then shows no OS
// notifications and writes one log entry. It has no exported methods
// other than the service functions, so the front end cannot call it.
type osNotifier struct {
	ns  *notifications.NotificationService
	log *slog.Logger
	ok  atomic.Bool
	mu  sync.Mutex
	n   int
}

func newOSNotifier(log *slog.Logger) *osNotifier {
	return &osNotifier{ns: notifications.New(), log: log}
}

func (o *osNotifier) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	if err := o.ns.ServiceStartup(ctx, opts); err != nil {
		o.log.Warn("OS notifications are not available", "err", err)
		return nil
	}
	o.ok.Store(true)
	return nil
}

func (o *osNotifier) ServiceShutdown() error {
	if o.ok.Load() {
		return o.ns.ServiceShutdown()
	}
	return nil
}

// show shows one Notification.
func (o *osNotifier) show(n gui.OSNotification) {
	if !o.ok.Load() {
		return
	}
	o.mu.Lock()
	o.n++
	id := fmt.Sprintf("vlt-%d-%d", n.Time.UnixNano(), o.n)
	o.mu.Unlock()
	if err := o.ns.SendNotification(notifications.NotificationOptions{ID: id, Title: n.Title, Body: n.Body}); err != nil {
		o.log.Warn("OS notification not shown", "type", n.Type, "err", err)
	}
}
