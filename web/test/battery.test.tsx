// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-012 TASK 03-00-00-004 steps 4 to 6: BATTERY_REPLACE in the Simple
// mode health, the battery card of the Device screen and the optional
// battery date of the wizard step "Canlı değerler".
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { StatusPush } from '../src/api';
import { setLang, t } from '../src/i18n';
import { BatteryDateField, BatteryReplace, today } from '../src/screens/battery';
import { health, plainEvent } from '../src/screens/simple';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

function status(battery?: StatusPush['ups']['battery']): StatusPush {
  return {
    ts: '', device: { state: 'connected' }, measurements: {}, countdown: { active: false } as never, policy_active: true,
    ups: { on_battery: false, low_battery: false, comm_lost: false, active: battery?.active ? ['BATTERY_REPLACE'] : [], battery },
  };
}

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  app.set({ auth: { pin_set: false, elevated: true } as never });
  fake.handlers.Settings = () => ({ settings: [{ key: 'battery.installed_on', value: '2022-01-10', version: 1, default: false }, { key: 'battery.replace_after_months', value: 36, version: 0, default: true }] });
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
function input(id: string): HTMLInputElement {
  return root.querySelector<HTMLInputElement>('#' + id)!;
}
async function type(el: HTMLInputElement, v: string) {
  await act(async () => {
    el.value = v;
    el.dispatchEvent(new Event('input', { bubbles: true }));
  });
}
async function submit() {
  await act(async () => { root.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
}
const sets = () => fake.calls.filter((c) => c.method === 'SetSetting').map((c) => c.args);

describe('Simple mode health (step 4)', () => {
  it('BATTERY_REPLACE gives "Problem" with the reason (age)', () => {
    const h = health(status({ active: true, reasons: ['age'], age_months: 58, replace_after_months: 36, installed_on: '2022-01-10' }), null);
    expect(h.level).toBe('problem');
    expect(t(h.reasons[0].key, h.reasons[0].vars)).toBe('The battery may need a replacement. The battery is 58 months old; the limit is 36 months.');
  });
  it('the Turkish text is "Akünün değişmesi gerekebilir." with the reason (test failed)', () => {
    setLang('tr');
    const h = health(status({ active: true, reasons: ['test_failed'], replace_after_months: 36 }), null);
    expect(h.level).toBe('problem');
    expect(t(h.reasons[0].key, h.reasons[0].vars)).toBe('Akünün değişmesi gerekebilir. Son akü testi başarısız oldu.');
  });
  it('no BATTERY_REPLACE: good', () => {
    expect(health(status(undefined), null).level).toBe('good');
  });
  it('the Event in plain language, and its end', () => {
    setLang('tr');
    expect(plainEvent({ type: 'BATTERY_REPLACE', severity: 'WARNING', data: { active: true } } as never)).toBe('Akü değişimi gerekebilir');
    expect(plainEvent({ type: 'BATTERY_REPLACE', severity: 'INFO', data: { active: false } } as never)).toContain('Akü değişimi gerekebilir');
  });
});

describe('Device screen battery card (step 5)', () => {
  it('shows the Settings with the help text and saves them', async () => {
    await show(<BatteryReplace elevated status={status(undefined)} />);
    expect(root.textContent).toContain('Most UPS batteries last 3 to 5 years. Write the date when you installed the battery.');
    expect(input('batt-installed').value).toBe('2022-01-10');
    expect(input('batt-installed').type).toBe('date');
    expect(input('batt-months').value).toBe('36');
    await type(input('batt-installed'), '2026-11-01');
    await type(input('batt-months'), '48');
    await submit();
    expect(sets()).toEqual([['battery.installed_on', '2026-11-01'], ['battery.replace_after_months', 48]]);
  });
  it('refuses a number out of 12 to 120', async () => {
    await show(<BatteryReplace elevated status={status(undefined)} />);
    await type(input('batt-months'), '6');
    await submit();
    expect(sets()).toEqual([]);
    expect(root.textContent).toContain('Write a number from 12 to 120.');
  });
  it('shows the active signal with its reason', async () => {
    await show(<BatteryReplace elevated status={status({ active: true, reasons: ['test_failed'], replace_after_months: 36 })} />);
    expect(root.textContent).toContain('The battery may need a replacement. The last battery test failed.');
  });
  it('the Turkish help text', () => {
    setLang('tr');
    expect(t('battery.help')).toBe('Çoğu UPS aküsü 3 ila 5 yıl dayanır. Aküyü taktığınız tarihi yazın.');
  });
});

describe('wizard battery date (step 6)', () => {
  it('is optional and saves only the date', async () => {
    setLang('tr');
    await show(<BatteryDateField elevated />);
    expect(root.textContent).toContain('Akü takılma tarihi');
    expect(input('wiz-batt-installed').required).toBe(false);
    expect(input('wiz-batt-installed').max).toBe(today());
    await type(input('wiz-batt-installed'), '2025-05-05');
    await submit();
    expect(sets()).toEqual([['battery.installed_on', '2025-05-05']]);
  });
});
