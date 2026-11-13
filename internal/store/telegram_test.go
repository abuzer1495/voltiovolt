// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTelegramChats(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir(), Options{})
	t0 := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	if err := s.AddTelegramChat(ctx, TelegramChat{ID: 100, Name: "Ayşe", PairedAt: t0}); err != nil {
		t.Fatal(err)
	}
	s.AddTelegramChat(ctx, TelegramChat{ID: -200, Name: "Family", PairedAt: t0.Add(time.Minute)})
	s.AddTelegramChat(ctx, TelegramChat{ID: 100, Name: "Ayşe Y.", PairedAt: t0.Add(2 * time.Minute)})
	cs, err := s.ListTelegramChats(ctx)
	if err != nil || len(cs) != 2 || cs[0].ID != -200 || cs[1].Name != "Ayşe Y." || !cs[1].PairedAt.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("chats %+v %v", cs, err)
	}
	if err := s.RemoveTelegramChat(ctx, -200); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveTelegramChat(ctx, -200); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second remove: %v", err)
	}
	if cs, _ := s.ListTelegramChats(ctx); len(cs) != 1 {
		t.Fatalf("chats %+v", cs)
	}
}
