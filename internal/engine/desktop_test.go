// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/notify"
)

type fakeHub struct {
	subs  int
	kinds []string
	data  []any
}

func (f *fakeHub) Publish(kind string, data any) error {
	f.kinds = append(f.kinds, kind)
	f.data = append(f.data, data)
	return nil
}
func (f *fakeHub) Subscribers() int { return f.subs }

func TestDesktopChannel(t *testing.T) {
	h := &fakeHub{}
	c := desktopChannel{h}
	if c.Name() != notify.ChannelDesktop {
		t.Fatalf("name %q", c.Name())
	}
	n := notify.Notification{Subject: "[VLT][CRITICAL] On battery", Text: "x", Severity: "CRITICAL", Type: "ON_BATTERY", Time: time.Now()}
	// No VLT GUI: no push, no error (no retry).
	if err := c.Send(context.Background(), n); err != nil || len(h.kinds) != 0 {
		t.Fatalf("no GUI: err %v, pushes %v", err, h.kinds)
	}
	h.subs = 1
	if err := c.Send(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	if len(h.kinds) != 1 || h.kinds[0] != wire.PushNotification || h.data[0].(notify.Notification).Subject != n.Subject {
		t.Fatalf("pushes %v %v", h.kinds, h.data)
	}
}
