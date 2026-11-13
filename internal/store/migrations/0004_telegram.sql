-- SPDX-License-Identifier: AGPL-3.0-or-later
-- VLT-IS-007 TASK 03-00-00-005 step 3.3: the paired Telegram chats.
-- vltd accepts Telegram commands only from these chat IDs.

CREATE TABLE telegram_chats (
	chat_id   INTEGER PRIMARY KEY,
	name      TEXT NOT NULL,
	paired_at TEXT NOT NULL -- RFC 3339, UTC
) STRICT;
