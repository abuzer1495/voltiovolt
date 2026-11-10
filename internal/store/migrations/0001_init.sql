-- SPDX-License-Identifier: AGPL-3.0-or-later
-- VLT-IS-003 TASK 03-00-00-003 step 3. Forward only.

CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value BLOB NOT NULL
) STRICT;

CREATE TABLE settings (
	key        TEXT PRIMARY KEY,
	value      TEXT NOT NULL,
	version    INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	mac        BLOB NOT NULL
) STRICT;

CREATE TABLE secrets (
	id         TEXT PRIMARY KEY,
	version    INTEGER NOT NULL,
	nonce      BLOB NOT NULL,
	ciphertext BLOB NOT NULL,
	hint       TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
) STRICT;

-- Later instruction. Schema only.
CREATE TABLE measurements (
	ts     INTEGER NOT NULL,
	name   TEXT NOT NULL,
	value  REAL,
	unit   TEXT NOT NULL,
	source TEXT NOT NULL
) STRICT;

-- Later instruction. Schema only.
CREATE TABLE events (
	ts       INTEGER NOT NULL,
	type     TEXT NOT NULL,
	severity TEXT NOT NULL,
	data     TEXT NOT NULL
) STRICT;

-- VLT-IS-003 TASK 03-00-00-006. Append-only.
CREATE TABLE audit (
	seq       INTEGER PRIMARY KEY,
	ts        TEXT NOT NULL,
	actor     TEXT NOT NULL CHECK (actor IN ('gui', 'telegram', 'system')),
	action    TEXT NOT NULL,
	target    TEXT NOT NULL,
	result    TEXT NOT NULL,
	details   TEXT NOT NULL,
	prev_hash TEXT NOT NULL,
	hash      TEXT NOT NULL
) STRICT;

CREATE TRIGGER audit_no_update BEFORE UPDATE ON audit
BEGIN
	SELECT RAISE(ABORT, 'audit is append-only');
END;

CREATE TRIGGER audit_no_delete BEFORE DELETE ON audit
BEGIN
	SELECT RAISE(ABORT, 'audit is append-only');
END;

CREATE TABLE auth_state (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
) STRICT;
