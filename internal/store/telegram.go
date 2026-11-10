// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"time"
)

// TelegramChat is one paired Telegram chat (VLT-IS-007 TASK 03-00-00-005).
type TelegramChat struct {
	ID       int64     `json:"chat_id"`
	Name     string    `json:"name"`
	PairedAt time.Time `json:"paired_at"`
}

// ListTelegramChats returns the paired chats in the order of pairing.
func (s *Store) ListTelegramChats(ctx context.Context) ([]TelegramChat, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT chat_id, name, paired_at FROM telegram_chats ORDER BY paired_at, chat_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TelegramChat
	for rows.Next() {
		var c TelegramChat
		var ts string
		if err := rows.Scan(&c.ID, &c.Name, &ts); err != nil {
			return nil, err
		}
		c.PairedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddTelegramChat adds a paired chat or updates its name and time.
func (s *Store) AddTelegramChat(ctx context.Context, c TelegramChat) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO telegram_chats (chat_id, name, paired_at) VALUES (?, ?, ?)
		ON CONFLICT (chat_id) DO UPDATE SET name = excluded.name, paired_at = excluded.paired_at`,
		c.ID, c.Name, c.PairedAt.UTC().Format(time.RFC3339))
	return err
}

// RemoveTelegramChat removes a paired chat. It returns ErrNotFound when the
// chat is not paired.
func (s *Store) RemoveTelegramChat(ctx context.Context, id int64) error {
	r, err := s.db.ExecContext(ctx, "DELETE FROM telegram_chats WHERE chat_id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
