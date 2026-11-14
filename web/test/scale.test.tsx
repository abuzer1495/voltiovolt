// SPDX-License-Identifier: AGPL-3.0-or-later
// Interface size (VLT-IS-011 TASK 03-00-00-004, OD-54).
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { SCALES, UI_SCALE_KEY, apply, factorFor, fit, loadScale, onKey, scale, setScale, stepScale } from '../src/scale';
import { ScaleChoice, ScaleNotice, pct } from '../src/screens/scalechoice';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

let root: HTMLElement;
function screenSize(w: number, h: number, dpr = 1) {
  Object.defineProperty(window, 'devicePixelRatio', { value: dpr, configurable: true });
  Object.defineProperty(window.screen, 'availWidth', { value: w, configurable: true });
  Object.defineProperty(window.screen, 'availHeight', { value: h, configurable: true });
}
beforeEach(() => {
  fake.reset();
  setLang('en');
  localStorage.clear();
  screenSize(1920, 1040);
  scale.set({ setting: 'auto', factor: 1, percent: 100, reduced: false, notice: false });
  fake.handlers.SetSetting = () => undefined;
  fake.handlers.SetInterfaceScale = () => undefined;
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});
const rootScale = () => document.documentElement.style.getPropertyValue('--ui-scale');

describe('scale rules', () => {
  it('has the sizes of OD-54 and Auto as the default', () => {
    expect([...SCALES]).toEqual([80, 90, 100, 110, 125, 150, 175, 200]);
    expect(scale.get().setting).toBe('auto');
  });

  it('Auto uses the display scale of the WebView: no double scaling', () => {
    expect(factorFor('auto', 1.25)).toBe(1);
    expect(factorFor(125, 1.25)).toBe(1);
    expect(factorFor(150, 1)).toBe(1.5);
    expect(factorFor(100, 2)).toBe(0.5);
  });

  it('uses a smaller size when the screen is too small', () => {
    expect(fit(1, 1, 1024, 728)).toEqual({ factor: 1, reduced: false }); // 1024 × 768 with a taskbar
    expect(fit(1.25, 1, 1366, 728)).toEqual({ factor: 1, reduced: true }); // 125 % on 1366 × 768: 100 %
    expect(fit(2, 1, 1920, 1040).reduced).toBe(true);
    expect(fit(2, 1, 1920, 1040).factor).toBe(1.5);
    expect(fit(0.8, 1, 1024, 728).reduced).toBe(false);
  });

  it('applies at once, with no restart, and sets the minimum window size', async () => {
    await setScale(150);
    expect(rootScale()).toBe('1.5');
    expect(scale.get()).toMatchObject({ setting: 150, factor: 1.5, percent: 150, reduced: false });
    expect(fake.calls).toContainEqual({ method: 'SetInterfaceScale', args: [150] });
    expect(fake.calls).toContainEqual({ method: 'SetSetting', args: [UI_SCALE_KEY, '150'] });
    await setScale('auto');
    expect(rootScale()).toBe('1');
  });

  it('shows the "too small" message one time', async () => {
    screenSize(1366, 728);
    apply(150);
    // 1024 × 680 at 110 % is 748 px high: too high for 728 px; 100 % fits.
    expect(scale.get()).toMatchObject({ reduced: true, notice: true, percent: 100 });
    await act(async () => render(<ScaleNotice />, root));
    expect(root.textContent).toContain(t('scale.reduced', { want: '150 %', used: 100 }));
    scale.set({ notice: false });
    apply(150);
    expect(scale.get().notice).toBe(false);
  });

  it('Ctrl+Plus, Ctrl+Minus and Ctrl+0', () => {
    expect(stepScale('auto', 1, 1)).toBe(110);
    expect(stepScale('auto', 1, 1.25)).toBe(150);
    expect(stepScale(200, 1, 1)).toBe(200);
    expect(stepScale(80, -1, 1)).toBe(80);
    expect(stepScale(125, -1, 1)).toBe(110);
    const key = (k: string, code = '') => {
      const e = new KeyboardEvent('keydown', { key: k, code, ctrlKey: true, cancelable: true });
      return { used: onKey(e), prevented: e.defaultPrevented };
    };
    expect(key('+')).toEqual({ used: true, prevented: true });
    expect(scale.get().setting).toBe(110);
    expect(key('=').used).toBe(true);
    expect(scale.get().setting).toBe(125);
    expect(key('-').used).toBe(true);
    expect(scale.get().setting).toBe(110);
    expect(key('0').used).toBe(true);
    expect(scale.get().setting).toBe('auto');
    expect(onKey(new KeyboardEvent('keydown', { key: '+' }))).toBe(false); // no Ctrl
  });

  it('the stored size is used after a restart', async () => {
    screenSize(2560, 1400);
    fake.handlers.Settings = () => ({ settings: [{ key: UI_SCALE_KEY, value: '175', version: 3, default: false }] });
    expect(await loadScale()).toBe(175);
    expect(rootScale()).toBe('1.75');
  });

  it('the choice is a select with Auto and the sizes; Turkish writes %125', async () => {
    await act(async () => render(<ScaleChoice />, root));
    const sel = root.querySelector('select') as HTMLSelectElement;
    expect([...sel.options].map((o) => o.value)).toEqual(['auto', '80', '90', '100', '110', '125', '150', '175', '200']);
    await act(async () => {
      sel.value = '125';
      sel.dispatchEvent(new Event('change', { bubbles: true }));
    });
    expect(scale.get().setting).toBe(125);
    setLang('tr');
    expect(pct(125)).toBe('%125');
  });

  it('all CSS sizes come from one root value', () => {
    const css = readFileSync(join(__dirname, '../src/styles.css'), 'utf8');
    const px = [...css.split('\n').filter((l) => !l.trim().startsWith('@media')).join('\n').matchAll(/(?<![\w.-])(\d+(?:\.\d+)?)px\b/g)].map((m) => m[1]);
    // Only hairlines (1px), the focus ring (3px, 2px) and the root value
    // (14px) are fixed.
    expect([...new Set(px)].sort()).toEqual(['1', '14', '2', '3']);
  });
});
