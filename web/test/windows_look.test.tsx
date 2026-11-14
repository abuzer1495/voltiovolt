// SPDX-License-Identifier: AGPL-3.0-or-later
// Windows look and native parts (VLT-IS-011 TASK 03-00-00-005, R-12, OD-55).
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../src/app';
import { Confirm } from '../src/components';
import { setLang, t } from '../src/i18n';
import { ui } from '../src/mode';
import { MENU_KEYS, THEME, accentColors, confirmNative, contrast, loadTheme, menuAction, menuState, native, onF1, onWizard, resetNative, watchNative } from '../src/native';
import { scale } from '../src/scale';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));
vi.mock('uplot', () => ({
  default: class {
    series: unknown[] = [];
    setData(): void {}
    setSeries(): void {}
    setScale(): void {}
    destroy(): void {}
  },
}));

const css = readFileSync(join(__dirname, '../src/styles.css'), 'utf8');
const en = JSON.parse(readFileSync(join(__dirname, '../i18n/en.json'), 'utf8')) as Record<string, string>;
const tr = JSON.parse(readFileSync(join(__dirname, '../i18n/tr.json'), 'utf8')) as Record<string, string>;
const tick = () => new Promise((r) => setTimeout(r, 0));

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  resetNative();
  setLang('en');
  localStorage.clear();
  ui.set({ mode: 'expert', loaded: true });
  scale.set({ setting: 'auto', factor: 1, percent: 100, reduced: false, notice: false });
  document.documentElement.style.removeProperty('--accent');
  delete document.documentElement.dataset.os;
  fake.handlers.SetSetting = () => undefined;
  fake.handlers.SetInterfaceScale = () => undefined;
  fake.handlers.SetMenu = () => undefined;
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});
async function show(el: preact.JSX.Element) {
  await act(async () => render(el, root));
  await act(async () => { await tick(); await tick(); });
}

describe('accent color (step 1.2)', () => {
  it('keeps a Windows accent with enough contrast and picks the text color on it', () => {
    const c = accentColors('#0067c0', false);
    expect(c).toEqual({ accent: '#0067c0', fg: '#ffffff' });
  });
  it('makes an accent lighter in the dark theme and darker in the light theme until 4.5:1', () => {
    for (const [hex, dark] of [['#0078d4', true], ['#ffd33d', false], ['#e81123', true], ['#00b7c3', false]] as const) {
      const c = accentColors(hex, dark);
      expect(c, hex).not.toBeNull();
      const th = dark ? THEME.dark : THEME.light;
      expect(contrast(c!.accent, th.bg)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(c!.accent, th.panel)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(c!.fg, c!.accent)).toBeGreaterThanOrEqual(4.5);
    }
  });
  it('refuses a value that is not #rrggbb', () => {
    expect(accentColors('red', false)).toBeNull();
    expect(accentColors('rgb(0,0,0)', false)).toBeNull();
  });
  it('THEME has the backgrounds of styles.css', () => {
    const light = css.slice(css.indexOf(':root'), css.indexOf('}'));
    const dark = css.slice(css.indexOf('prefers-color-scheme: dark'));
    expect(light).toContain(`--bg: ${THEME.light.bg};`);
    expect(light).toContain(`--panel: ${THEME.light.panel};`);
    expect(dark).toContain(`--bg: ${THEME.dark.bg};`);
    expect(dark).toContain(`--panel: ${THEME.dark.panel};`);
  });
  it('loadTheme sets the OS and the accent color of Windows', async () => {
    fake.handlers.Theme = () => ({ os: 'windows', accent: '#0067c0', native: true, menu: true });
    await loadTheme();
    expect(document.documentElement.dataset.os).toBe('windows');
    expect(document.documentElement.style.getPropertyValue('--accent')).toBe('#0067c0');
    expect(native.get()).toMatchObject({ dialogs: true, menu: true });
  });
  it('without a Go side the CSS colors stay', async () => {
    fake.handlers.Theme = () => { throw fake.goError('unavailable'); };
    await loadTheme();
    expect(document.documentElement.style.getPropertyValue('--accent')).toBe('');
    expect(native.get().dialogs).toBe(false);
  });
});

describe('visual style (step 1)', () => {
  it('Segoe UI Variable, then Segoe UI, then the system font', () => {
    expect(css).toMatch(/--font: "Segoe UI Variable Text", "Segoe UI Variable", "Segoe UI", system-ui/);
    expect(css).toContain('font-family: var(--font); }');
  });
  it('4 px corners, 32 px controls, soft dialog shadows', () => {
    expect(css).toContain('--radius: calc(4 * var(--px));');
    expect(css).toContain('--control-height: calc(32 * var(--px));');
    expect(css).not.toMatch(/border-radius: calc\((6|8) \* var\(--px\)\)/);
    expect(css).toMatch(/\.wizard-dialog \{[^}]*box-shadow: var\(--shadow\)/);
    expect(css).toMatch(/\.prompt-dialog \{[^}]*box-shadow: var\(--shadow\)/);
  });
  it('Linux uses the system font and the GTK selection colors (step 5)', () => {
    expect(css).toContain('html[data-os="linux"] { --font: system-ui');
    expect(css).toContain('html[data-os="linux"] button.primary { background: Highlight; color: HighlightText;');
  });
});

describe('menu bar (step 2.1)', () => {
  it('sends the menu texts of the language with the access keys', () => {
    setLang('tr');
    ui.set({ mode: 'simple' });
    scale.set({ setting: 125 });
    const st = menuState();
    expect(Object.keys(st.texts).sort()).toEqual([...MENU_KEYS].sort());
    expect(st.texts['menu.file']).toBe('&Dosya');
    expect(st.texts['menu.view']).toBe('&Görünüm');
    expect(st.texts['menu.tools.simhelp']).toBe('&Simülatör yardımı');
    expect(st.texts['menu.help.about']).toBe('VoltioVolt &hakkında');
    expect(st.mode).toBe('simple');
    expect(st.scale).toBe('125');
    expect(st.scales[0]).toEqual({ value: 'auto', label: t('scale.auto.short') });
    expect(st.scales).toContainEqual({ value: '125', label: '%125' });
  });
  it('each menu has one access key per item and no access key twice', () => {
    for (const d of [en, tr]) {
      const groups = [
        ['menu.file', 'menu.view', 'menu.tools', 'menu.help'],
        ['menu.view.simple', 'menu.view.expert', 'menu.view.scale'],
        ['menu.tools.wizard', 'menu.tools.simhelp'],
        ['menu.help.docs', 'menu.help.about'],
      ];
      for (const g of groups) {
        const keys = g.map((k) => {
          const m = /&(.)/u.exec(d[k]);
          expect(m, k).not.toBeNull();
          return m![1].toLocaleLowerCase('tr');
        });
        expect(new Set(keys).size, g.join()).toBe(keys.length);
      }
    }
  });
  it('watchNative sends the menu and again after a change of the language and the mode', async () => {
    fake.handlers.Theme = () => ({ os: 'windows', accent: '', native: true, menu: true });
    watchNative();
    await tick();
    const n = () => fake.calls.filter((c) => c.method === 'SetMenu').length;
    expect(n()).toBe(1);
    setLang('tr');
    expect(n()).toBe(2);
    ui.set({ mode: 'simple' });
    expect(n()).toBe(3);
    ui.set({ mode: 'simple' }); // no change: no new call
    expect(n()).toBe(3);
  });
  it('menu commands change the mode, the size and open the wizard', async () => {
    const wiz = vi.fn();
    onWizard(wiz);
    watchNative();
    fake.emit('vlt:menu', 'mode:simple');
    await tick();
    expect(ui.get().mode).toBe('simple');
    menuAction('mode:expert');
    await tick();
    expect(ui.get().mode).toBe('expert');
    fake.emit('vlt:menu', 'scale:150');
    await tick();
    expect(scale.get().setting).toBe(150);
    menuAction('scale:999');
    expect(scale.get().setting).toBe(150);
    menuAction('wizard');
    expect(wiz).toHaveBeenCalledTimes(1);
  });
});

describe('keyboard (step 4)', () => {
  it('F1 opens the documentation; Ctrl+F1 does not', () => {
    const ev = (init: KeyboardEventInit) => new KeyboardEvent('keydown', { cancelable: true, ...init });
    const e = ev({ key: 'F1' });
    expect(onF1(e)).toBe(true);
    expect(e.defaultPrevented).toBe(true);
    expect(fake.calls).toContainEqual({ method: 'OpenDocs', args: [] });
    expect(onF1(ev({ key: 'F1', ctrlKey: true }))).toBe(false);
    expect(onF1(ev({ key: 'F2' }))).toBe(false);
  });
});

describe('native confirmation (step 2.2)', () => {
  it('asks with Yes, No and Cancel in the language of the user', async () => {
    setLang('tr');
    native.set({ dialogs: true });
    fake.handlers.ConfirmNative = () => 'yes';
    expect(await confirmNative('Etkinleştir', 'Emin misiniz?')).toBe('yes');
    expect(fake.calls).toContainEqual({ method: 'ConfirmNative', args: [{ title: 'Etkinleştir', text: 'Emin misiniz?', yes: 'Evet', no: 'Hayır', cancel: 'Vazgeç' }] });
  });
  for (const [answer, done] of [['yes', 'confirm'], ['no', 'cancel'], ['cancel', 'cancel']] as const) {
    it(`Confirm: native "${answer}" → ${done}`, async () => {
      native.set({ dialogs: true });
      fake.handlers.ConfirmNative = () => answer;
      const onConfirm = vi.fn();
      const onCancel = vi.fn();
      await show(<Confirm text="Sure?" confirmLabel="Activate" onConfirm={onConfirm} onCancel={onCancel} />);
      expect(onConfirm).toHaveBeenCalledTimes(done === 'confirm' ? 1 : 0);
      expect(onCancel).toHaveBeenCalledTimes(done === 'cancel' ? 1 : 0);
      expect(root.querySelector('.confirm')).toBeNull();
    });
  }
  it('Confirm: the inline dialog when the native dialog fails', async () => {
    native.set({ dialogs: true });
    fake.handlers.ConfirmNative = () => { throw fake.goError('unavailable'); };
    await show(<Confirm text="Sure?" confirmLabel="Activate" onConfirm={() => undefined} onCancel={() => undefined} />);
    expect(root.querySelector('.confirm')?.textContent).toContain('Sure?');
  });
  it('Confirm: the inline dialog at once without native dialogs (browser, tests)', async () => {
    await show(<Confirm text="Sure?" confirmLabel="Activate" onConfirm={() => undefined} onCancel={() => undefined} />);
    expect(root.querySelector('.confirm')).toBeTruthy();
    expect(fake.calls.some((c) => c.method === 'ConfirmNative')).toBe(false);
  });
});

describe('left navigation pane (step 1.4)', () => {
  beforeEach(() => {
    fake.handlers.Settings = () => ({ settings: [{ key: 'ui.mode', value: 'expert', default: false }] });
    fake.handlers.Connection = () => ({ state: 'connected', since: '' });
    fake.handlers.AuthStatus = () => ({ pin_set: true, elevated: true });
    fake.handlers.About = () => ({ version: 'test', vltd_version: '', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: '', mode: 'portable' });
    fake.handlers.WizardCompleted = () => true;
    fake.handlers.WizardLaterCount = () => 2;
    fake.handlers.Last = () => ({});
    app.set({ auth: { pin_set: true, elevated: true }, conn: { state: 'connected', since: '' } });
    Object.defineProperty(window, 'innerWidth', { value: 1366, configurable: true });
  });
  it('has an icon and a text for each screen, can be collapsed to the icons and keeps the choice', async () => {
    await show(<App />);
    const nav = root.querySelector('nav.nav') as HTMLElement;
    expect(nav).toBeTruthy();
    const links = [...nav.querySelectorAll('a')];
    expect(links).toHaveLength(9);
    for (const a of links) {
      expect(a.querySelector('svg.nav-icon[aria-hidden="true"]')).toBeTruthy();
      expect(a.querySelector('.nav-text')?.textContent).toBeTruthy();
    }
    const toggle = nav.querySelector('button.nav-toggle') as HTMLButtonElement;
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    expect(toggle.getAttribute('aria-label')).toBe(t('nav.collapse'));
    await act(async () => toggle.click());
    expect(nav.classList.contains('nav-collapsed')).toBe(true);
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(links[0].getAttribute('title')).toBe(t('nav.dashboard'));
    // the text stays for screen readers
    expect(links[0].textContent).toBe(t('nav.dashboard'));
    expect(localStorage.getItem('vlt.nav-collapsed')).toBe('1');
  });
  it('starts collapsed in a narrow window', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    await show(<App />);
    expect(root.querySelector('nav.nav')?.classList.contains('nav-collapsed')).toBe(true);
  });
  it('the tab order follows the screen: header, navigation, content', async () => {
    await show(<App />);
    const layout = root.querySelector('.layout') as HTMLElement;
    const order = [...layout.children].map((e) => e.tagName.toLowerCase() + (e.className ? '.' + e.className.split(' ')[0] : ''));
    expect(order.indexOf('header.top')).toBeLessThan(order.indexOf('nav.nav'));
    expect(order.indexOf('nav.nav')).toBeLessThan(order.indexOf('main'));
    expect(root.querySelector('[tabindex]:not([tabindex="0"]):not([tabindex="-1"])')).toBeNull();
  });
});
