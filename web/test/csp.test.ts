// SPDX-License-Identifier: AGPL-3.0-or-later
// SEC-095: the built index.html has the Content Security Policy, no inline
// script and no remote content. Run "npm run build" first.
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

const CSP =
  "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'";
const dist = join(__dirname, '..', '..', 'cmd', 'vlt-gui', 'frontend', 'dist');

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? files(join(dir, e.name)) : [join(dir, e.name)],
  );
}

describe('built index.html (SEC-095)', () => {
  it('exists', () => {
    expect(existsSync(join(dist, 'index.html'))).toBe(true);
  });
  const html = existsSync(join(dist, 'index.html')) ? readFileSync(join(dist, 'index.html'), 'utf8') : '';
  const doc = new DOMParser().parseFromString(html, 'text/html');

  it('has the CSP text of SEC-095', () => {
    const metas = [...doc.querySelectorAll('meta[http-equiv="Content-Security-Policy"]')];
    expect(metas.length).toBe(1);
    expect(metas[0].getAttribute('content')).toBe(CSP);
  });
  it('has no inline script and no inline style', () => {
    for (const s of doc.querySelectorAll('script')) {
      expect(s.getAttribute('src')).toBeTruthy();
      expect((s.textContent ?? '').trim()).toBe('');
    }
    expect(doc.querySelectorAll('style').length).toBe(0);
    expect(html).not.toMatch(/\son[a-z]+\s*=/i);
    expect(html).not.toMatch(/style\s*=/i);
  });
  it('loads no remote content', () => {
    for (const el of doc.querySelectorAll('[src],[href]')) {
      const u = el.getAttribute('src') ?? el.getAttribute('href') ?? '';
      expect(u.startsWith('/') || u.startsWith('./')).toBe(true);
    }
    for (const f of files(dist)) {
      if (!/\.(js|css|html)$/.test(f)) continue;
      const text = readFileSync(f, 'utf8');
      expect(text, f).not.toMatch(/(?:src|href|url\()\s*=?\s*["']?https?:\/\//i);
      expect(text, f).not.toMatch(/@import\s+url\(\s*["']?https?:/i);
    }
  });
  it('built JavaScript has no eval and no Function constructor', () => {
    for (const f of files(dist)) {
      if (!f.endsWith('.js')) continue;
      const text = readFileSync(f, 'utf8');
      expect(text, f).not.toMatch(/\beval\s*\(/);
      expect(text, f).not.toMatch(/\bnew\s+Function\s*\(/);
    }
  });
});
