-- SPDX-License-Identifier: AGPL-3.0-or-later
-- VLT-IS-004 TASK 03-00-00-006: Shutdown Policy state and Simulation
-- results. The policy Settings are in the settings table.

CREATE TABLE policy_simulations (
	id      INTEGER PRIMARY KEY,
	ts      INTEGER NOT NULL, -- Unix time in milliseconds (UTC)
	version TEXT NOT NULL,    -- SHA-256 of the policy Settings
	passed  INTEGER NOT NULL CHECK (passed IN (0, 1)),
	result  TEXT NOT NULL     -- JSON
) STRICT;
CREATE INDEX policy_simulations_version ON policy_simulations (version, passed);

CREATE TABLE policy_state (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
) STRICT;
