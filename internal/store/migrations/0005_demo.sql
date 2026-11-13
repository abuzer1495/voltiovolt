-- SPDX-License-Identifier: AGPL-3.0-or-later
-- VLT-IS-009 TASK 03-00-00-005: mark Events and raw Measurements of the
-- demo mode. 1-minute and 1-hour summaries stay without demo values.

ALTER TABLE events ADD COLUMN demo INTEGER NOT NULL DEFAULT 0;
ALTER TABLE measurements ADD COLUMN demo INTEGER NOT NULL DEFAULT 0;
CREATE INDEX events_demo ON events (demo);
CREATE INDEX measurements_name_ts_demo ON measurements (name, ts, demo);
