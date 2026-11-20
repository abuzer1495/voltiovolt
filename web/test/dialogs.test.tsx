// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-012 TASK 03-00-00-001 step 6: front-end tests for the dialog
// scroll (step 1, R-13), the controls (step 3, R-14), the text selection
// (step 4, R-15) and the connection text (step 5, R-16). The layout of
// each wizard step (step 2.4) is tested in a real browser by
// scripts/layout-check.mjs.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import en from '../i18n/en.json';
import tr from '../i18n/tr.json';
import { App } from '../src/app';
import { ConnState } from '../src/components';
import { pageLocked, wheelGuard } from '../src/dialog';
import { setLang, t } from '../src/i18n';
import { General } from '../src/screens/general';
import { Ports } from '../src/screens/device';
import { FirstStartPrompt, Wizard, wizardWanted } from '../src/screens/wizard';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

const css = readFileSync(join(__dirname, '../src/styles.css'), 'utf8');
let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  wizardWanted.set({ open: false, start: '' });
  fake.handlers.DevicePorts = () => ({ ports: [{ name: 'COM3', is_usb: true, vid: '067b', pid: '2303', product: 'Prolific USB-to-Serial Comm Port' }] });
  fake.handlers.AuthStatus = () => ({ pin_set: false, elevated: true });
  fake.handlers.Channels = () => ({ channels: [], email_presets: [] });
  fake.handlers.TelegramChats = () => ({ chats: [] });
  app.set({ auth: { pin_set: false, elevated: true }, conn: { state: 'connected', since: '' }, status: null, policy: null, countdown: null, events: [] });
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});

async function show(el: preact.JSX.Element) {
  await act(async () => render(el, root));
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
}

function wheel(target: Element): WheelEvent {
  const e = new WheelEvent('wheel', { deltaY: 100, bubbles: true, cancelable: true });
  target.dispatchEvent(e);
  return e;
}

describe('step 1: the page behind a dialog does not scroll (R-13)', () => {
  it('the wizard locks the page scroll while it is open', async () => {
    expect(pageLocked()).toBe(false);
    await show(<Wizard first portable />);
    expect(pageLocked()).toBe(true);
    expect(css).toContain('html.modal-open, html.modal-open body { overflow: hidden; }');
    await act(async () => render(null, root));
    expect(pageLocked()).toBe(false);
  });

  it('the first-start dialog locks the page scroll', async () => {
    await show(<FirstStartPrompt onStart={() => undefined} onLater={() => undefined} />);
    expect(pageLocked()).toBe(true);
    await act(async () => render(null, root));
    expect(pageLocked()).toBe(false);
  });

  it('the wheel over the dimmed area and over a part that cannot scroll does nothing', async () => {
    await show(<Wizard first portable />);
    const overlay = root.querySelector('.wizard-overlay') as HTMLElement;
    expect(wheel(overlay).defaultPrevented).toBe(true);
    expect(wheel(root.querySelector('.wizard-body h3, .wizard-body p') as HTMLElement).defaultPrevented).toBe(true);
  });

  it('the wheel over a part of the dialog that can scroll scrolls only that part', () => {
    const overlay = document.createElement('div');
    const body = document.createElement('div');
    const p = document.createElement('p');
    body.style.overflowY = 'auto';
    Object.defineProperty(body, 'scrollHeight', { value: 500 });
    Object.defineProperty(body, 'clientHeight', { value: 200 });
    body.scrollTop = 0;
    body.appendChild(p);
    overlay.appendChild(body);
    document.body.appendChild(overlay);
    const down = new WheelEvent('wheel', { deltaY: 100, bubbles: true, cancelable: true });
    Object.defineProperty(down, 'target', { value: p });
    wheelGuard(overlay, down);
    expect(down.defaultPrevented).toBe(false);
    const up = new WheelEvent('wheel', { deltaY: -100, bubbles: true, cancelable: true });
    Object.defineProperty(up, 'target', { value: p });
    wheelGuard(overlay, up); // already at the top: nothing to scroll
    expect(up.defaultPrevented).toBe(true);
    overlay.remove();
  });

  it('the dialog body and the inline dialogs contain the scroll', () => {
    expect(css).toMatch(/\.wizard-body \{[^}]*overscroll-behavior: contain;/);
    expect(css).toContain('.confirm, .help-panel { overscroll-behavior: contain; }');
  });
});

describe('step 3: no cut control (R-14)', () => {
  it('a button text stays on one line', () => {
    expect(css).toContain('button { white-space: nowrap; flex-shrink: 0; }');
  });

  it('the port table uses the short text "Use" with the full accessible name', async () => {
    for (const l of ['en', 'tr'] as const) {
      setLang(l);
      await show(<Ports part="select" elevated onUsed={() => undefined} />);
      const b = [...root.querySelectorAll('td button')][0] as HTMLButtonElement;
      expect(b.textContent).toBe(l === 'en' ? 'Use' : 'Kullan');
      expect(b.getAttribute('aria-label')).toBe(t('device.ports.use.label', { name: 'COM3' }));
      expect(root.textContent).not.toContain(t('device.scan.warning'));
      render(null, root);
    }
  });

  it('the scan part has the scan with its WARNING and no "Use" button', async () => {
    await show(<Ports part="scan" elevated onUsed={() => undefined} />);
    expect(root.textContent).toContain(t('device.scan.warning'));
    expect(root.querySelector('#scan-text')).toBeTruthy();
    expect([...root.querySelectorAll('button')].some((b) => b.textContent === t('device.ports.use.short'))).toBe(false);
  });
});

describe('step 4: interface text is not selectable (R-15)', () => {
  it('labels, buttons, menus, tables, tiles and navigation are not selectable', () => {
    expect(css).toContain('body { -webkit-user-select: none; user-select: none; }');
  });

  it('values that a user can copy stay selectable', async () => {
    expect(css).toMatch(/input, textarea, select, \[contenteditable="true"\], \.selectable, \.selectable \*, \.selectable-values dd \{ -webkit-user-select: text; user-select: text; \}/);
    fake.handlers.About = () => ({ version: '0.9.0-rc.1', vltd_version: '', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: '', mode: 'portable', data_dir: 'C:\\VoltioVolt\\data' });
    await show(<General />);
    const path = [...root.querySelectorAll('.selectable')].map((e) => e.textContent);
    expect(path).toContain('C:\\VoltioVolt\\data');
    expect(path).toContain('0.9.0-rc.1');
  });

  it('a text selection is cleared when a dialog opens and when it closes', async () => {
    const sel = window.getSelection()!;
    const spy = vi.spyOn(sel, 'removeAllRanges');
    await show(<Wizard first portable />);
    expect(spy).toHaveBeenCalledTimes(1);
    await act(async () => render(null, root));
    expect(spy).toHaveBeenCalledTimes(2);
    spy.mockRestore();
  });
});

describe('step 5: the connection text of portable mode (R-16)', () => {
  const dev = (state: string, transport: string, port = 'COM3') => ({ ts: '', device: { state, config: { transport, serial_port: port }, profile: { port } } }) as never;
  const conn = { state: 'connected', since: '' } as const;
  const text = () => root.querySelector('.conn > span:last-child')?.textContent;

  it('shows the state of the UPS link in Turkish and English', async () => {
    const cases: [unknown, string, string][] = [
      [dev('connected', 'serial'), "UPS'e bağlı (COM3)", 'Connected to the UPS (COM3)'],
      [dev('comm lost', 'serial'), 'UPS bağlı değil', 'UPS not connected'],
      [dev('not configured', ''), 'UPS bağlı değil', 'UPS not connected'],
      [dev('connected', 'test'), 'Test bağlantısı', 'Test connection'],
      [dev('connected', 'demo'), 'Demo', 'Demo'],
    ];
    for (const [status, want, wantEn] of cases) {
      setLang('tr');
      await show(<ConnState conn={conn} status={status as never} />);
      expect(text()).toBe(want);
      setLang('en');
      await show(<ConnState conn={conn} status={status as never} />);
      expect(text()).toBe(wantEn);
    }
  });

  it('the vltd text appears only in the service mode', async () => {
    setLang('tr');
    await show(<ConnState conn={conn} service status={null} />);
    expect(text()).toBe("vltd'ye bağlı");
    await show(<ConnState conn={conn} status={null} />);
    expect(root.textContent).not.toContain('vltd');
  });

  it('portable mode never shows "vltd\'ye bağlı" in the header', async () => {
    setLang('tr');
    fake.handlers.Settings = () => ({ settings: [{ key: 'ui.mode', value: 'expert', default: false }] });
    fake.handlers.Connection = () => ({ state: 'connected', since: '' });
    fake.handlers.AuthStatus = () => ({ pin_set: true, elevated: true });
    fake.handlers.About = () => ({ version: 'test', vltd_version: '', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: '', mode: 'portable' });
    fake.handlers.WizardCompleted = () => true;
    fake.handlers.WizardLaterCount = () => 2;
    fake.handlers.Last = () => ({ status: dev('connected', 'serial') });
    app.set({ auth: { pin_set: true, elevated: true }, conn: { state: 'connected', since: '' }, status: dev('connected', 'serial') });
    await show(<App />);
    const header = root.querySelector('header.top') as HTMLElement;
    for (const l of ['tr', 'en'] as const) {
      await act(async () => setLang(l));
      expect(header.textContent).not.toContain("vltd'ye bağlı");
      expect(header.textContent).not.toContain('vltd');
      expect(header.textContent).toContain(t('conn.ups.connected', { port: 'COM3' }));
    }
  });

  it('no portable text of the header names vltd', () => {
    const E = en as Record<string, string>;
    const T = tr as Record<string, string>;
    for (const k of Object.keys(E).filter((x) => x.startsWith('conn.ups.') || x.startsWith('conn.portable.'))) {
      expect(E[k], k).not.toContain('vltd');
      expect(T[k], k).not.toContain('vltd');
    }
  });
});
