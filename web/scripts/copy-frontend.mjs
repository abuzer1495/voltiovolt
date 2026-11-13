// SPDX-License-Identifier: AGPL-3.0-or-later
// Copy the built front end to cmd/voltiovolt/frontend/dist (VLT-IS-008).
import { cpSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const web = dirname(fileURLToPath(import.meta.url));
const root = join(web, '..', '..');
const src = join(root, 'cmd', 'vlt-gui', 'frontend', 'dist');
const dest = join(root, 'cmd', 'voltiovolt', 'frontend', 'dist');
mkdirSync(dirname(dest), { recursive: true });
cpSync(src, dest, { recursive: true });
