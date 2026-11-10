// SPDX-License-Identifier: AGPL-3.0-or-later
// Helpers of the source scans of the tests (TASK 03-00-00-008).
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

export const SRC = join(__dirname, '..', 'src');

export function sources(dir = SRC, ext = /\.(ts|tsx)$/): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? sources(join(dir, e.name), ext) : ext.test(e.name) ? [join(dir, e.name)] : [],
  );
}

/** stripComments removes the comments and keeps the line numbers. */
export function stripComments(s: string): string {
  return s
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:'"\\])\/\/.*$/gm, '$1');
}

export function read(f: string): string {
  return stripComments(readFileSync(f, 'utf8'));
}

const LETTER = /[A-Za-zÇĞİÖŞÜçğıöşü]/;

/**
 * hardcoded finds user text that is not from t(): JSX text between tags,
 * and literal values of aria-label, title, placeholder and alt.
 */
export function hardcoded(src: string): string[] {
  const out: string[] = [];
  // JSX text: >text< with letters. Not JSX: arrow functions (=>),
  // conditions (a > b ? <x/> : <y/>), template strings, and TS generic
  // types (Promise<T>: a word directly before a "<" that is not "</").
  for (const m of src.matchAll(/(.)>([^<>{}]*)<(.)/g)) {
    const raw = m[2];
    const txt = raw.trim();
    if (!LETTER.test(txt) || m[1] === '=') continue;
    if (/[?&|`$=;]/.test(txt)) continue;
    if (m[3] !== '/' && /[\w]$/.test(raw) && /^[\w.]+$/.test(txt)) continue;
    out.push(txt);
  }
  // A string literal as a JSX child: {'text'}.
  for (const m of src.matchAll(/\{\s*(["'`])([^"'`]*)\1\s*\}/g)) {
    if (LETTER.test(m[2]) && !m[2].includes('${')) out.push(m[2]);
  }
  for (const m of src.matchAll(/\b(aria-label|title|placeholder|alt)\s*=\s*(["'])([^"']*)\2/g)) {
    if (LETTER.test(m[3])) out.push(m[1] + '=' + m[3]);
  }
  return out;
}
