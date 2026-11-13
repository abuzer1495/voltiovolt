-- SPDX-License-Identifier: AGPL-3.0-or-later
-- VLT-IS-010 TASK 03-00-00-004: origin of Events and raw Measurements
-- (real, demo, test). 1-minute and 1-hour summaries stay without demo
-- and test values (OD-48).

ALTER TABLE events ADD COLUMN origin TEXT NOT NULL DEFAULT 'real';
ALTER TABLE measurements ADD COLUMN origin TEXT NOT NULL DEFAULT 'real';
UPDATE events SET origin = 'demo' WHERE demo = 1;
UPDATE measurements SET origin = 'demo' WHERE demo = 1;
CREATE INDEX events_origin ON events (origin);
CREATE INDEX measurements_name_ts_origin ON measurements (name, ts, origin);
