// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"database/sql"
	"errors"
	"unicode/utf8"
)

// SecretRow is one row of the secrets table. The ciphertext is made by the
// Vault. The Store never sees a Secret value.
type SecretRow struct {
	ID         string
	Version    int64
	Nonce      []byte
	Ciphertext []byte
	Hint       string
	UpdatedAt  string
}

// HintMinLength is the minimum Secret length (in characters) for a hint.
const HintMinLength = 16

// Hint returns the last 4 characters of a Secret of 16 characters or more.
// For a shorter Secret, it returns "" (no hint). VLT-IS-003 TASK
// 03-00-00-003 step 4.
func Hint(secret []byte) string {
	if !utf8.Valid(secret) {
		// Count bytes for binary Secrets. Binary data gives no hint.
		return ""
	}
	n := utf8.RuneCount(secret)
	if n < HintMinLength {
		return ""
	}
	b := secret
	for i := 0; i < n-4; i++ {
		_, size := utf8.DecodeRune(b)
		b = b[size:]
	}
	return string(b)
}

// PutSecret writes a secrets row (insert or replace).
func (s *Store) PutSecret(ctx context.Context, r SecretRow) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO secrets (id, version, nonce, ciphertext, hint, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET version = excluded.version, nonce = excluded.nonce,
		ciphertext = excluded.ciphertext, hint = excluded.hint, updated_at = excluded.updated_at`,
		r.ID, r.Version, r.Nonce, r.Ciphertext, r.Hint, r.UpdatedAt)
	return err
}

// GetSecret reads a secrets row.
func (s *Store) GetSecret(ctx context.Context, id string) (SecretRow, error) {
	r := SecretRow{ID: id}
	err := s.db.QueryRowContext(ctx, "SELECT version, nonce, ciphertext, hint, updated_at FROM secrets WHERE id = ?", id).
		Scan(&r.Version, &r.Nonce, &r.Ciphertext, &r.Hint, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SecretRow{}, ErrNotFound
	}
	return r, err
}

// DeleteSecret removes a secrets row. It returns ErrNotFound if there is no row.
func (s *Store) DeleteSecret(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM secrets WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAllSecrets removes all secrets rows.
func (s *Store) DeleteAllSecrets(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM secrets")
	return err
}
