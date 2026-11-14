// SPDX-License-Identifier: AGPL-3.0-or-later
// Interface size (VLT-IS-011 TASK 03-00-00-004, OD-54): the "ui." Setting
// ui.scale is "auto" or a percent. The CSS sizes use one root value
// (--ui-scale in styles.css). Auto is the display scale of the OS: the
// WebView already uses it, so the factor is 1 (no double scaling). A
// percent is the size in percent of 96 DPI: the factor is percent / 100 /
// devicePixelRatio.
import { api } from './api';
import { createStore } from './store';

export const SCALES = [80, 90, 100, 110, 125, 150, 175, 200] as const;
export type ScaleSetting = 'auto' | (typeof SCALES)[number];
export const UI_SCALE_KEY = 'ui.scale';
const LOCAL_KEY = 'vlt.ui-scale';
const NOTICE_KEY = 'vlt.ui-scale-notice';

/** MIN_W and MIN_H are the minimum window size at 100 % (internal/gui/window.go). */
export const MIN_W = 1024;
export const MIN_H = 680;

export interface ScaleState {
  setting: ScaleSetting;
  /** factor is the CSS factor: 1 is the display scale of the OS. */
  factor: number;
  /** percent is the size that the user sees, in percent of 96 DPI. */
  percent: number;
  /** reduced: the screen is too small for the setting. */
  reduced: boolean;
  notice: boolean;
}

export const scale = createStore<ScaleState>({ setting: 'auto', factor: 1, percent: 100, reduced: false, notice: false });

export function parseScale(v: unknown): ScaleSetting | null {
  if (v === 'auto') return 'auto';
  const n = Number(v);
  return (SCALES as readonly number[]).includes(n) && String(v) === String(n) ? (n as ScaleSetting) : null;
}

/** factorFor is the CSS factor of a setting. */
export function factorFor(s: ScaleSetting, dpr: number): number {
  return s === 'auto' ? 1 : s / 100 / (dpr || 1);
}

/**
 * fit returns the factor that fits on the screen: the minimum window size
 * (1024 × 680 at 100 %) times the factor must fit in the available screen
 * size (CSS pixels). If not, it uses the largest size of SCALES that fits.
 */
export function fit(f: number, dpr: number, availW: number, availH: number): { factor: number; reduced: boolean } {
  if (!availW || !availH || (MIN_W * f <= availW && MIN_H * f <= availH)) return { factor: f, reduced: false };
  const ok = [...SCALES].reverse().map((p) => p / 100 / (dpr || 1)).find((x) => x < f && MIN_W * x <= availW && MIN_H * x <= availH);
  return { factor: ok ?? Math.min(availW / MIN_W, availH / MIN_H), reduced: true };
}

function env(): { dpr: number; w: number; h: number } {
  if (typeof window === 'undefined') return { dpr: 1, w: 0, h: 0 };
  return { dpr: window.devicePixelRatio || 1, w: window.screen?.availWidth ?? 0, h: window.screen?.availHeight ?? 0 };
}

/** apply sets --ui-scale at once (no restart) and the minimum window size. */
export function apply(setting: ScaleSetting = scale.get().setting): void {
  const { dpr, w, h } = env();
  const r = fit(factorFor(setting, dpr), dpr, w, h);
  const percent = Math.round(r.factor * dpr * 100);
  let notice = false;
  if (r.reduced) {
    try {
      const key = `${setting}@${w}x${h}@${dpr}`;
      if (localStorage.getItem(NOTICE_KEY) !== key) {
        localStorage.setItem(NOTICE_KEY, key);
        notice = true;
      }
    } catch {
      notice = true;
    }
  }
  scale.set({ setting, factor: r.factor, percent, reduced: r.reduced, notice: notice || (scale.get().notice && r.reduced) });
  if (typeof document !== 'undefined') document.documentElement.style.setProperty('--ui-scale', String(Math.round(r.factor * 10000) / 10000));
  void api.setInterfaceScale(Math.round(r.factor * 100)).catch(() => undefined);
}

export function closeNotice(): void {
  scale.set({ notice: false });
}

function local(): ScaleSetting | null {
  try {
    return parseScale(localStorage.getItem(LOCAL_KEY));
  } catch {
    return null;
  }
}

/** loadScale reads ui.scale and applies it. */
export async function loadScale(): Promise<ScaleSetting> {
  let s: ScaleSetting | null = null;
  let stored = false;
  try {
    const r = await api.settings();
    const x = (r?.settings ?? []).find((v) => v.key === UI_SCALE_KEY);
    if (x && !x.default) stored = true;
    s = parseScale(x?.value);
  } catch {
    /* no Settings */
  }
  if (!stored) s = local() ?? s;
  const v = s ?? 'auto';
  apply(v);
  return v;
}

/** setScale applies a size at once and stores it. */
export async function setScale(s: ScaleSetting): Promise<void> {
  apply(s);
  try {
    localStorage.setItem(LOCAL_KEY, String(s));
  } catch {
    /* no local store */
  }
  try {
    await api.setSetting(UI_SCALE_KEY, String(s));
  } catch {
    /* kept in localStorage */
  }
}

/** stepScale is the next (+1) or the previous (-1) size of SCALES. */
export function stepScale(cur: ScaleSetting, dir: 1 | -1, dpr: number): ScaleSetting {
  const p = cur === 'auto' ? Math.round(dpr * 100) : cur;
  if (dir > 0) return SCALES.find((x) => x > p) ?? SCALES[SCALES.length - 1];
  return [...SCALES].reverse().find((x) => x < p) ?? SCALES[0];
}

/** onKey handles Ctrl+Plus, Ctrl+Minus and Ctrl+0. It returns true if it used the key. */
export function onKey(e: KeyboardEvent): boolean {
  if (!e.ctrlKey || e.altKey || e.metaKey) return false;
  const { dpr } = env();
  const cur = scale.get().setting;
  let next: ScaleSetting | null = null;
  if (e.key === '+' || e.key === '=' || e.code === 'NumpadAdd') next = stepScale(cur, 1, dpr);
  else if (e.key === '-' || e.code === 'NumpadSubtract') next = stepScale(cur, -1, dpr);
  else if (e.key === '0' || e.code === 'Numpad0') next = 'auto';
  if (next === null) return false;
  e.preventDefault();
  void setScale(next);
  return true;
}

let watching = false;

/** watchScale adds the keyboard shortcuts and follows a DPI change (one time). */
export function watchScale(): void {
  if (watching || typeof window === 'undefined') return;
  watching = true;
  window.addEventListener('keydown', (e) => void onKey(e));
  window.addEventListener('resize', () => apply());
}
