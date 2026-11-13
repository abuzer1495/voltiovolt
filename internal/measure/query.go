// SPDX-License-Identifier: AGPL-3.0-or-later

package measure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Steps of a query.
const (
	StepRaw    = "raw"
	StepMinute = "1m"
	StepHour   = "1h"
)

// MaxRows is the maximum number of rows of one query.
const MaxRows = 10000

// Row is one value of a query. For raw values, Min, Max and Mean are the
// value and Count is 1.
type Row struct {
	TS     time.Time `json:"ts"`
	Min    float64   `json:"min"`
	Max    float64   `json:"max"`
	Mean   float64   `json:"mean"`
	Count  int64     `json:"count"`
	Source string    `json:"source,omitempty"`
	Demo   bool      `json:"demo,omitempty"`
	Origin string    `json:"origin,omitempty"`
}

// ErrBadQuery is returned for a query that is not valid.
var ErrBadQuery = errors.New("measure: query is not valid")

// Query returns the values of name in [from, to) with a step.
func Query(ctx context.Context, db *sql.DB, name string, from, to time.Time, step string) (string, []Row, error) {
	return QueryFilter(ctx, db, name, from, to, step, false)
}

// QueryFilter is Query with an option to include Measurements of other
// origins. The default (includeDemo false) shows only the "real" origin.
func QueryFilter(ctx context.Context, db *sql.DB, name string, from, to time.Time, step string, includeDemo bool) (string, []Row, error) {
	return QueryByOrigin(ctx, db, name, from, to, step, "real", includeDemo)
}

// QueryByOrigin is Query with an origin filter (VLT-IS-010 OD-48).
// 1-minute and 1-hour summaries contain only real values; when the current
// origin is not "real" and includeOthers is false, those steps are empty.
func QueryByOrigin(ctx context.Context, db *sql.DB, name string, from, to time.Time, step, current string, includeOthers bool) (string, []Row, error) {
	unit, ok := Units[name]
	if !ok {
		return "", nil, fmt.Errorf("%w: unknown name %q", ErrBadQuery, name)
	}
	if !to.After(from) {
		return "", nil, fmt.Errorf("%w: the range is empty", ErrBadQuery)
	}
	if current == "" {
		current = "real"
	}
	var q string
	args := []any{name, ms(from), ms(to)}
	switch step {
	case StepRaw:
		q = "SELECT ts, value, value, value, 1, source, demo, origin FROM measurements WHERE name = ? AND ts >= ? AND ts < ? AND value IS NOT NULL"
		if !includeOthers {
			q += " AND origin = ?"
			args = append(args, current)
		}
		q += " ORDER BY ts LIMIT ?"
		args = append(args, MaxRows)
	case StepMinute, StepHour:
		if !includeOthers && current != "real" {
			return unit, []Row{}, nil
		}
		table := "measurements_1m"
		if step == StepHour {
			table = "measurements_1h"
		}
		q = "SELECT ts, min, max, mean, count, '', 0, 'real' FROM " + table + " WHERE name = ? AND ts >= ? AND ts < ? ORDER BY ts LIMIT ?"
		args = append(args, MaxRows)
	default:
		return "", nil, fmt.Errorf("%w: step must be raw, 1m or 1h", ErrBadQuery)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		var r Row
		var ts int64
		var demo int
		if err := rows.Scan(&ts, &r.Min, &r.Max, &r.Mean, &r.Count, &r.Source, &demo, &r.Origin); err != nil {
			return "", nil, err
		}
		r.TS = time.UnixMilli(ts).UTC()
		r.Demo = demo != 0
		if r.Origin == "" {
			if r.Demo {
				r.Origin = "demo"
			} else {
				r.Origin = "real"
			}
		}
		out = append(out, r)
	}
	return unit, out, rows.Err()
}
