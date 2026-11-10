// SPDX-License-Identifier: AGPL-3.0-or-later
// Accessibility (TASK 03-00-00-008 step 2): contrast 4.5:1 or more in both
// themes, keyboard-only use (native controls only), accessible names, and
// state never by color alone.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { App } from '../src/app';
import { Badge } from '../src/components';
import { setLang } from '../src/i18n';
import { Dashboard, ModeView } from '../src/screens/dashboard';
import { General } from '../src/screens/general';
import { Policy } from '../src/screens/policy';
import { Security } from '../src/screens/security';
import { Events } from '../src/screens/events';
import { Device } from '../src/screens/device';
import { SetPin, Login } from '../src/screens/pin';
import { app } from '../src/state';
import * as fake from './fakebridge';
import { read, sources } from './source';

vi.mock('../src/bridge', () => import('./fakebridge'));

const css = readFileSync(join(__dirname, '..', 'src', 'styles.css'), 'utf8');

function vars(block: string): Record<string, string> {
  return Object.fromEntries([...block.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})/g)].map((m) => [m[1], m[2]]));
}
const light = vars(css.slice(css.indexOf(':root'), css.indexOf('}')));
const dark = vars(css.slice(css.indexOf('prefers-color-scheme: dark')));

function lum(hex: string): number {
  const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}
export function contrast(a: string, b: string): number {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

// [text, background] pairs of styles.css.
const TEXT: [string, string][] = [
  ['fg', 'bg'], ['fg', 'panel'], ['muted', 'bg'], ['muted', 'panel'], ['accent', 'bg'], ['accent', 'panel'],
  ['accent-fg', 'accent'], ['ok', 'bg'], ['ok', 'panel'], ['crit', 'bg'], ['crit', 'panel'], ['warn', 'warn-bg'],
  ['crit', 'crit-bg'], ['fg', 'crit-bg'], ['bg', 'crit'], ['fg', 'warn-bg'],
];
// Non-text (focus ring, borders of controls): 3:1 or more.
const UI: [string, string][] = [['focus', 'bg'], ['focus', 'panel'], ['border', 'bg']];

describe('contrast', () => {
  for (const [name, th] of [['light', light], ['dark', dark]] as const) {
    it(name + ' theme has all colors', () => {
      for (const [a, b] of [...TEXT, ...UI]) {
        expect(th[a], a).toMatch(/^#/);
        expect(th[b], b).toMatch(/^#/);
      }
    });
    for (const [a, b] of TEXT)
      it(`${name}: --${a} on --${b} ≥ 4.5`, () => expect(contrast(th[a], th[b])).toBeGreaterThanOrEqual(4.5));
    for (const [a, b] of UI)
      it(`${name}: --${a} on --${b} ≥ 3`, () => expect(contrast(th[a], th[b])).toBeGreaterThanOrEqual(3));
  }
  it('the contrast function is right', () => {
    expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrast('#767676', '#ffffff')).toBeGreaterThan(4.5);
  });
  it('a focus ring is visible', () => {
    expect(css).toMatch(/:focus-visible\s*\{\s*outline:\s*3px solid var\(--focus\)/);
    expect(css).not.toMatch(/outline:\s*(none|0)\b/);
  });
});

describe('keyboard', () => {
  it('click handlers only on native controls', () => {
    const bad: string[] = [];
    for (const f of sources(undefined, /\.tsx$/)) {
      for (const m of read(f).matchAll(/<(\w+)\b[^<>]*\bon(Click|MouseDown|KeyDown)=/g))
        if (!['button', 'a', 'input', 'select', 'Confirm'].includes(m[1])) bad.push(f.split('/src/')[1] + ': <' + m[1] + '>');
    }
    expect(bad).toEqual([]);
  });
  it('no positive tabindex', () => {
    for (const f of sources(undefined, /\.tsx$/)) expect(read(f)).not.toMatch(/tabIndex=\{?["']?[1-9]/);
  });
});

/** name is the accessible name of a control (simplified). */
function name(el: Element, doc: Document): string {
  const l = el.getAttribute('aria-label');
  if (l) return l;
  const by = el.getAttribute('aria-labelledby');
  if (by) return by.split(' ').map((id) => doc.getElementById(id)?.textContent ?? '').join(' ').trim();
  if (el.id) {
    const lab = doc.querySelector(`label[for="${el.id}"]`);
    if (lab?.textContent?.trim()) return lab.textContent.trim();
  }
  const wrap = el.closest('label');
  if (wrap?.textContent?.trim()) return wrap.textContent.trim();
  if (el.tagName === 'BUTTON' || el.tagName === 'A') return el.textContent?.trim() ?? '';
  return '';
}

function setup() {
  fake.reset();
  const p = {
    settings: { action: 'shutdown', 'trigger.on_battery_seconds': 300, 'trigger.runtime_below_minutes': 5, 'trigger.charge_below_percent': 30,
      'trigger.low_battery_flag': true, 'trigger.comm_lost_on_battery_seconds': 180, countdown_seconds: 60, cancel_when_mains_returns: true,
      force_close_applications: true },
    version: 'abc', active: false, fallback: false, last_simulation: null,
  };
  fake.handlers.Policy = () => p;
  fake.handlers.Connection = () => ({ state: 'connected', since: '' });
  fake.handlers.Last = () => ({});
  fake.handlers.AuthStatus = () => ({ pin_set: true, elevated: true });
  fake.handlers.Settings = () => ({ settings: [] });
  fake.handlers.ShutdownStatus = () => ({ active: false, remaining_seconds: 0, cancelable: true, requested: false });
  fake.handlers.SystemStatus = () => ({ vault: { state: 'reduced protection', reason: 'no TPM2' }, store: { state: 'ok', schema_version: 3 } });
  fake.handlers.Audit = () => ({ rows: [{ seq: 1, ts: '2026-11-10T10:00:00Z', actor: 'user', action: 'auth.login', target: '', result: 'ok' }], next: 0 });
  fake.handlers.Events = () => ({ events: [], next: 0 });
  fake.handlers.DeviceStatus = () => ({});
  fake.handlers.About = () => ({ version: 'test', vltd_version: '0.5.0', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: 'https://github.com/abuzer1495/voltiovolt' });
  app.set({ auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' }, policy: p as never });
}

describe('accessible names', () => {
  const screens: [string, () => preact.JSX.Element][] = [
    ['App', () => <App />], ['Dashboard', () => <Dashboard />], ['Policy', () => <Policy />], ['Security', () => <Security />], ['General', () => <General />],
    ['Events', () => <Events />], ['Device', () => <Device />], ['SetPin', () => <SetPin />], ['Login', () => <Login />],
  ];
  for (const lang of ['en', 'tr'] as const)
    for (const [n, f] of screens)
      it(`${n} (${lang}): each control has a name`, async () => {
        setup();
        setLang(lang);
        const root = document.createElement('div');
        document.body.appendChild(root);
        await act(async () => render(f(), root));
        await act(async () => {
          await new Promise((r) => setTimeout(r, 0));
        });
        const ctl = [...root.querySelectorAll('button, a[href], input, select, textarea')];
        if (n !== 'Dashboard') expect(ctl.length).toBeGreaterThan(0); // the Dashboard shows values only
        for (const el of ctl) expect(name(el, document), el.outerHTML.slice(0, 80)).not.toBe('');
        render(null, root);
        root.remove();
      });
});

describe('state not by color alone', () => {
  it('a badge has an icon and a text', async () => {
    const root = document.createElement('div');
    await act(async () => render(<Badge kind="crit" icon="⚠" text="locked" />, root));
    expect(root.textContent).toContain('⚠');
    expect(root.textContent).toContain('locked');
  });
  it('each mode has its own icon and text', async () => {
    const seen = new Set<string>();
    for (const m of ['mains', 'battery', 'bypass', 'fault', 'comm_lost', 'unknown'] as const) {
      const root = document.createElement('div');
      await act(async () => render(<ModeView mode={m} />, root));
      const txt = root.textContent ?? '';
      expect(txt.trim().length).toBeGreaterThan(2);
      seen.add(txt);
    }
    expect(seen.size).toBe(6);
  });
});

describe('headings', () => {
  it('Change PIN on the Security screen is a level-3 heading under the level-2 screen title', async () => {
    setup();
    setLang('en');
    const root = document.createElement('div');
    await act(async () => render(<Security />, root));
    const levels = [...root.querySelectorAll('h1,h2,h3,h4')].map((h) => Number(h.tagName[1]));
    expect(levels[0]).toBe(2);
    for (let i = 1; i < levels.length; i++) expect(levels[i] - levels[i - 1]).toBeLessThanOrEqual(1);
    expect(levels.slice(1).every((l) => l >= 3)).toBe(true);
  });
});
