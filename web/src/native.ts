// SPDX-License-Identifier: AGPL-3.0-or-later
// The Windows look and the native parts (VLT-IS-011 TASK 03-00-00-005,
// R-12, OD-55): the accent color of Windows, the OS for the CSS, the
// native menu bar (texts from en.json and tr.json), F1 for the
// documentation and the native confirmation dialog.
import { api, events, type MenuState } from './api';
import { on } from './bridge';
import { lang, onLangChange, t } from './i18n';
import { setMode, ui } from './mode';
import { SCALES, parseScale, scale, setScale } from './scale';
import { createStore } from './store';

export const native = createStore<{ os: string; dialogs: boolean; menu: boolean; accent: string }>({ os: '', dialogs: false, menu: false, accent: '' });

/** The theme backgrounds of styles.css (the a11y test checks that they are the same). */
export const THEME = {
  light: { bg: '#f3f3f3', panel: '#fbfbfb' },
  dark: { bg: '#202020', panel: '#2b2b2b' },
} as const;

function rgb(hex: string): [number, number, number] {
  return [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)) as [number, number, number];
}
function hex(c: [number, number, number]): string {
  return '#' + c.map((v) => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, '0')).join('');
}
function lum(h: string): number {
  const c = rgb(h).map((v) => v / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}
export function contrast(a: string, b: string): number {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

/**
 * accentColors returns the accent and the text color on it for a theme:
 * the accent is used as a text color on the backgrounds too, so it is made
 * lighter (dark theme) or darker (light theme) until the contrast is
 * 4.5:1 or more. null: no valid accent.
 */
export function accentColors(accent: string, dark: boolean): { accent: string; fg: string } | null {
  if (!/^#[0-9a-f]{6}$/i.test(accent)) return null;
  const th = dark ? THEME.dark : THEME.light;
  const to: [number, number, number] = dark ? [255, 255, 255] : [0, 0, 0];
  const c0 = rgb(accent);
  let a = accent.toLowerCase();
  for (let k = 0; k <= 20 && (contrast(a, th.bg) < 4.5 || contrast(a, th.panel) < 4.5); k++) {
    const f = k / 20;
    a = hex([c0[0] + (to[0] - c0[0]) * f, c0[1] + (to[1] - c0[1]) * f, c0[2] + (to[2] - c0[2]) * f]);
  }
  if (contrast(a, th.bg) < 4.5 || contrast(a, th.panel) < 4.5) return null;
  const fg = contrast('#ffffff', a) >= contrast('#000000', a) ? '#ffffff' : '#000000';
  if (contrast(fg, a) < 4.5) return null;
  return { accent: a, fg };
}

function isDark(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches;
}

/** applyAccent sets --accent and --accent-fg from the Windows accent color. */
export function applyAccent(accent: string = native.get().accent): void {
  const st = document.documentElement.style;
  const c = accent ? accentColors(accent, isDark()) : null;
  if (!c) {
    st.removeProperty('--accent');
    st.removeProperty('--accent-fg');
    return;
  }
  st.setProperty('--accent', c.accent);
  st.setProperty('--accent-fg', c.fg);
}

/** loadTheme reads the OS, the accent color and the native parts. */
export async function loadTheme(): Promise<void> {
  try {
    const ti = await api.theme();
    if (!ti) return;
    native.set({ os: ti.os, dialogs: !!ti.native, menu: !!ti.menu, accent: ti.accent || '' });
    if (ti.os) document.documentElement.dataset.os = ti.os;
    applyAccent(ti.accent || '');
  } catch {
    /* no Go side: CSS defaults */
  }
}

export const MENU_KEYS = [
  'menu.file', 'menu.file.exit', 'menu.view', 'menu.view.simple', 'menu.view.expert', 'menu.view.scale',
  'menu.tools', 'menu.tools.wizard', 'menu.tools.simhelp', 'menu.help', 'menu.help.docs', 'menu.help.about', 'menu.about.text',
] as const;

function pctText(n: number): string {
  return lang() === 'tr' ? `%${n}` : `${n} %`;
}

/** menuState is the state of the menu bar for the Go side. */
export function menuState(): MenuState {
  const texts: Record<string, string> = {};
  for (const k of MENU_KEYS) texts[k] = t(k);
  return {
    texts,
    mode: ui.get().mode,
    scale: String(scale.get().setting),
    scales: [{ value: 'auto', label: t('scale.auto.short') }, ...SCALES.map((p) => ({ value: String(p), label: pctText(p) }))],
  };
}

let lastMenu = '';
export function sendMenu(): void {
  const st = menuState();
  const key = JSON.stringify(st);
  if (key === lastMenu) return;
  lastMenu = key;
  void api.setMenu(st).catch(() => {
    lastMenu = '';
  });
}

let wizard: (() => void) | null = null;
/** onWizard sets the function of Tools > Setup wizard (app.tsx). */
export function onWizard(f: () => void): void {
  wizard = f;
}

/** menuAction runs a command of the menu bar in the front end. */
export function menuAction(a: unknown): void {
  if (a === 'mode:simple' || a === 'mode:expert') {
    void setMode(a === 'mode:simple' ? 'simple' : 'expert');
  } else if (a === 'wizard') {
    wizard?.();
  } else if (typeof a === 'string' && a.startsWith('scale:')) {
    const v = parseScale(a.slice(6));
    if (v) void setScale(v);
  }
}

/** onF1 opens the documentation (step 4). */
export function onF1(e: KeyboardEvent): boolean {
  if (e.key !== 'F1' || e.ctrlKey || e.altKey || e.metaKey || e.shiftKey) return false;
  e.preventDefault();
  void api.openDocs().catch(() => undefined);
  return true;
}

let watching = false;
export function watchNative(): void {
  if (watching) return;
  watching = true;
  void loadTheme();
  lastMenu = '';
  sendMenu();
  onLangChange(sendMenu);
  ui.sub(sendMenu);
  scale.sub(sendMenu);
  on(events.menu, menuAction);
  if (typeof window !== 'undefined') {
    window.addEventListener('keydown', (e) => void onF1(e));
    if (typeof matchMedia === 'function') {
      const mq = matchMedia('(prefers-color-scheme: dark)');
      mq.addEventListener?.('change', () => applyAccent());
    }
  }
}

/** resetNative is for the tests. */
export function resetNative(): void {
  watching = false;
  lastMenu = '';
  wizard = null;
  native.set({ os: '', dialogs: false, menu: false, accent: '' });
}

/**
 * confirmNative asks with the native Yes / No / Cancel dialog. null: no
 * native dialog (the caller shows its own dialog).
 */
export async function confirmNative(title: string, text: string): Promise<'yes' | 'no' | 'cancel' | null> {
  if (!native.get().dialogs) return null;
  try {
    const r = await api.confirmNative({ title, text, yes: t('common.yes'), no: t('common.no'), cancel: t('common.cancel') });
    return r === 'yes' || r === 'no' || r === 'cancel' ? r : null;
  } catch {
    return null;
  }
}
