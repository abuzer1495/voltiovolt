// SPDX-License-Identifier: AGPL-3.0-or-later
// TASK 03-00-00-008 step 1: all user text is in i18n/en.json and
// i18n/tr.json. A missing key fails; hardcoded text in a component fails.
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import en from '../i18n/en.json';
import tr from '../i18n/tr.json';
import { hardcoded, read, sources } from './source';

const EN = en as Record<string, string>;
const TR = tr as Record<string, string>;

function vars(s: string): string[] {
  return [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
}

describe('i18n files', () => {
  it('have the same keys', () => {
    expect(Object.keys(TR).filter((k) => !(k in EN))).toEqual([]);
    expect(Object.keys(EN).filter((k) => !(k in TR))).toEqual([]);
  });
  it('have no empty text', () => {
    for (const d of [EN, TR]) for (const [k, v] of Object.entries(d)) expect(v.trim(), k).not.toBe('');
  });
  it('have the same {name} values in each language', () => {
    for (const k of Object.keys(EN)) expect(vars(TR[k] ?? ''), k).toEqual(vars(EN[k]));
  });
  it('Turkish text has no broken characters and no ASCII spellings', () => {
    const all = Object.values(TR).join('\n');
    expect(all).toMatch(/[çğıöşüÇĞİÖŞÜ]/);
    expect(all).not.toMatch(/Ã|Å|Ä°|Ä±|Ã¼|\uFFFD/); // UTF-8 read as Latin-1
    // Frequent ASCII spellings of Turkish words.
    const words = ['baglanti', 'baglan', 'guc', 'sifre', 'gecmis', 'guvenlik', 'cikis', 'giris', 'dusuk', 'yuksek',
      'surum', 'degistir', 'degisti', 'kullanici', 'ayarlari', 'olaylari', 'islem', 'kaynak kodu acik', 'yanlis', 'dogru',
      'calisiyor', 'sebeke', 'ozellik', 'sifirla', 'onayla', 'gun', 'saat dilimi', 'acil', 'kisa', 'uzun sure'];
    const ascii = new RegExp('(?<!\\p{L})(' + words.join('|') + ')(?!\\p{L})', 'iu');
    for (const [k, v] of Object.entries(TR)) expect(ascii.test(v), k + ': ' + v).toBe(false);
  });
});

/** keyRefs returns the literal keys of t('...') and the prefixes of t('x.' + y). */
function keyRefs(src: string): { keys: string[]; prefixes: string[] } {
  const keys = [...src.matchAll(/\bt\(\s*'([^']+)'\s*[,)]/g)].map((m) => m[1]);
  const prefixes = [...src.matchAll(/\bt\(\s*'([^']+\.)'\s*\+/g)].map((m) => m[1]);
  // t(cond ? 'a' : 'b')
  for (const m of src.matchAll(/\bt\(\s*[^'()]+\?\s*'([^']+)'\s*:\s*'([^']+)'/g)) keys.push(m[1], m[2]);
  return { keys, prefixes };
}

describe('components', () => {
  const files = sources();
  it('use only keys that exist', () => {
    const missing: string[] = [];
    for (const f of files) {
      const { keys, prefixes } = keyRefs(read(f));
      for (const k of keys) if (!(k in EN) || !(k in TR)) missing.push(f.split('/src/')[1] + ': ' + k);
      for (const p of prefixes)
        if (!Object.keys(EN).some((k) => k.startsWith(p))) missing.push(f.split('/src/')[1] + ': ' + p + '*');
    }
    expect(missing).toEqual([]);
  });
  it('have no hardcoded user text', () => {
    const found: string[] = [];
    for (const f of sources(undefined, /\.tsx$/)) for (const h of hardcoded(read(f))) found.push(f.split('/src/')[1] + ': ' + h);
    expect(found).toEqual([]);
  });
  it('the scan finds hardcoded text (self test)', () => {
    expect(hardcoded('<p>Hello</p>')).toEqual(['Hello']);
    expect(hardcoded("<b>{'Merhaba dünya'}</b>")).toEqual(['Merhaba dünya']);
    expect(hardcoded('<button aria-label="Close">x</button>')).toContain('aria-label=Close');
    expect(hardcoded("<p>{t('a.b')}</p><span>{' · '}</span>")).toEqual([]);
  });
  it('the key check finds a missing key (self test)', () => {
    expect(keyRefs("t('no.such.key')").keys).toEqual(['no.such.key']);
    expect('no.such.key' in EN).toBe(false);
  });
});

describe('error codes', () => {
  it('each error code of internal/ipc/wire has a text', () => {
    const dir = join(__dirname, '..', '..', 'internal', 'ipc', 'wire');
    const codes = readdirSync(dir)
      .filter((f) => f.endsWith('.go') && !f.endsWith('_test.go'))
      .flatMap((f) => [...readFileSync(join(dir, f), 'utf8').matchAll(/\bCode\w+\s*=\s*"([a-z_]+)"/g)].map((m) => m[1]));
    expect(codes.length).toBeGreaterThan(10);
    for (const c of codes) expect('err.' + c in EN && 'err.' + c in TR, c).toBe(true);
  });
});
