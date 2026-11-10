// SPDX-License-Identifier: AGPL-3.0-or-later

package measure

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

const (
	minuteMs = int64(time.Minute / time.Millisecond)
	hourMs   = int64(time.Hour / time.Millisecond)
)

// Maintain makes the summaries of all complete minutes and hours before
// now, then deletes old values (step 4). It is safe to run it again: the
// summaries of a period are replaced.
func Maintain(ctx context.Context, db *sql.DB, now time.Time) error {
	nowMs := ms(now)
	curMinute := nowMs - nowMs%minuteMs
	curHour := nowMs - nowMs%hourMs
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1-minute values from the raw values. Start at the last summary (it
	// is made again), or at the oldest raw value.
	var from sql.NullInt64
	if err := tx.QueryRowContext(ctx, "SELECT max(ts) FROM measurements_1m").Scan(&from); err != nil {
		return err
	}
	start := int64(0)
	if from.Valid {
		start = from.Int64
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO measurements_1m (ts, name, unit, min, max, mean, count)
		SELECT ts - (ts % ?) AS b, name, unit, min(value), max(value), avg(value), count(*)
		FROM measurements WHERE ts >= ? AND ts < ? AND value IS NOT NULL
		GROUP BY b, name, unit`, minuteMs, start, curMinute); err != nil {
		return fmt.Errorf("measure: 1-minute summaries: %w", err)
	}

	// 1-hour values from the 1-minute values (mean weighted by count).
	if err := tx.QueryRowContext(ctx, "SELECT max(ts) FROM measurements_1h").Scan(&from); err != nil {
		return err
	}
	start = 0
	if from.Valid {
		start = from.Int64
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO measurements_1h (ts, name, unit, min, max, mean, count)
		SELECT ts - (ts % ?) AS b, name, unit, min(min), max(max), sum(mean * count) / sum(count), sum(count)
		FROM measurements_1m WHERE ts >= ? AND ts < ?
		GROUP BY b, name, unit`, hourMs, start, curHour); err != nil {
		return fmt.Errorf("measure: 1-hour summaries: %w", err)
	}

	// Retention. Raw values older than 7 days are in the summaries: the
	// job runs every hour.
	if _, err := tx.ExecContext(ctx, "DELETE FROM measurements WHERE ts < ?", ms(now.Add(-RawRetention))); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM measurements_1m WHERE ts < ?", ms(now.Add(-MinuteRetention))); err != nil {
		return err
	}
	return tx.Commit()
}

// Clock is the time source of the job.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

// RunJob runs Maintain at start and every hour until ctx is done.
func RunJob(ctx context.Context, db *sql.DB, clock Clock, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	for {
		if err := Maintain(ctx, db, clock.Now()); err != nil && ctx.Err() == nil {
			log.Error("measure: hourly job failed", "err", err)
		}
		if clock.Sleep(ctx, JobPeriod) != nil {
			return
		}
	}
}
