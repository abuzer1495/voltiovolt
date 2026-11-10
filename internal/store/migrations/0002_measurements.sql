-- SPDX-License-Identifier: AGPL-3.0-or-later
-- VLT-IS-004 TASK 03-00-00-004: Measurements.
-- ts is Unix time in milliseconds (UTC). Raw values: 7 days. 1-minute
-- values: 400 days. 1-hour values: no limit.

CREATE INDEX measurements_name_ts ON measurements (name, ts);
CREATE INDEX measurements_ts ON measurements (ts);

CREATE TABLE measurements_1m (
	ts    INTEGER NOT NULL,
	name  TEXT NOT NULL,
	unit  TEXT NOT NULL,
	min   REAL NOT NULL,
	max   REAL NOT NULL,
	mean  REAL NOT NULL,
	count INTEGER NOT NULL,
	PRIMARY KEY (name, ts)
) STRICT;
CREATE INDEX measurements_1m_ts ON measurements_1m (ts);

CREATE TABLE measurements_1h (
	ts    INTEGER NOT NULL,
	name  TEXT NOT NULL,
	unit  TEXT NOT NULL,
	min   REAL NOT NULL,
	max   REAL NOT NULL,
	mean  REAL NOT NULL,
	count INTEGER NOT NULL,
	PRIMARY KEY (name, ts)
) STRICT;
