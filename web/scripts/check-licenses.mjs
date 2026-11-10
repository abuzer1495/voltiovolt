// SPDX-License-Identifier: AGPL-3.0-or-later
// npm license check with the SEC-061 allow list. It reads each package of
// package-lock.json from node_modules (after "npm ci --ignore-scripts") and
// fails if a license is not on the list. A package that is not installed
// on this platform (an optional package of another platform) is checked
// with the license field of package-lock.json.
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const allowed = (process.env.ALLOWED_LICENSES || 'MIT,BSD-2-Clause,BSD-3-Clause,Apache-2.0,ISC,MPL-2.0')
  .split(',')
  .map((s) => s.trim());

/** ok checks a simple SPDX expression: "A", "(A OR B)", "A AND B". */
export function ok(expr) {
  if (typeof expr !== 'string' || expr === '') return false;
  const e = expr.replace(/[()]/g, ' ').trim();
  if (/\sAND\s/.test(e) && /\sOR\s/.test(e)) return false; // not used; check by hand
  if (/\sOR\s/.test(e)) return e.split(/\sOR\s/).some((p) => allowed.includes(p.trim()));
  if (/\sAND\s/.test(e)) return e.split(/\sAND\s/).every((p) => allowed.includes(p.trim()));
  return allowed.includes(e);
}

const lock = JSON.parse(readFileSync('package-lock.json', 'utf8'));
let bad = 0;
let n = 0;
for (const [path, meta] of Object.entries(lock.packages)) {
  if (path === '') continue;
  n++;
  const pj = join(path, 'package.json');
  let license = meta.license;
  if (existsSync(pj)) {
    const p = JSON.parse(readFileSync(pj, 'utf8'));
    license = p.license ?? (Array.isArray(p.licenses) ? p.licenses.map((l) => l.type).join(' OR ') : undefined);
  }
  const name = path.replace(/^.*node_modules\//, '');
  const good = ok(license);
  console.log(`${good ? 'ok ' : 'BAD'} ${name}@${meta.version} ${license}`);
  if (!good) bad++;
}
console.log(`${n} packages, ${bad} not permitted (allow list: ${allowed.join(', ')})`);
if (bad > 0) process.exit(1);
