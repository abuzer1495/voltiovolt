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
}

// ErrBadQuery is returned for a query that is not valid.
var ErrBadQuery = errors.New("measure: query is not valid")

// Query returns the values of name in [from, to) with a step.
func Query(ctx context.Context, db *sql.DB, name string, from, to time.Time, step string) (string, []Row, error) {
	unit, ok := Units[name]
	if !ok {
		return "", nil, fmt.Errorf("%w: unknown name %q", ErrBadQuery, name)
	}
	if !to.After(from) {
		return "", nil, fmt.Errorf("%w: the range is empty", ErrBadQuery)
	}
	var q string
	switch step {
	case StepRaw:
		q = "SELECT ts, value, value, value, 1, source FROM measurements WHERE name = ? AND ts >= ? AND ts < ? AND value IS NOT NULL ORDER BY ts LIMIT ?"
	case StepMinute:
		q = "SELECT ts, min, max, mean, count, '' FROM measurements_1m WHERE name = ? AND ts >= ? AND ts < ? ORDER BY ts LIMIT ?"
	case StepHour:
		q = "SELECT ts, min, max, mean, count, '' FROM measurements_1h WHERE name = ? AND ts >= ? AND ts < ? ORDER BY ts LIMIT ?"
	default:
		return "", nil, fmt.Errorf("%w: step must be raw, 1m or 1h", ErrBadQuery)
	}
	rows, err := db.QueryContext(ctx, q, name, ms(from), ms(to), MaxRows)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		var r Row
		var ts int64
		if err := rows.Scan(&ts, &r.Min, &r.Max, &r.Mean, &r.Count, &r.Source); err != nil {
			return "", nil, err
		}
		r.TS = time.UnixMilli(ts).UTC()
		out = append(out, r)
	}
	return unit, out, rows.Err()
}
