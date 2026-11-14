// SPDX-License-Identifier: AGPL-3.0-or-later
// Simple and Expert modes (VLT-IS-011 TASK 03-00-00-003, OD-53).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { StatusPush } from '../src/api';
import { App } from '../src/app';
import { setLang, t } from '../src/i18n';
import { loadMode, setMode, ui, UI_MODE_KEY } from '../src/mode';
import { Simple, health, plainEvent, triggerSentence } from '../src/screens/simple';
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

function st(over: Partial<StatusPush['ups']> = {}, m: Record<string, number> = {}, device = 'connected'): StatusPush {
  const measurements: StatusPush['measurements'] = {};
  for (const [k, v] of Object.entries({ 'load.percent': 30, 'ups.temperature': 30, 'input.voltage': 230, 'output.voltage': 230, 'battery.charge': 100, 'battery.runtime': 40, ...m })) {
    measurements[k] = { name: k, unit: k.includes('voltage') ? 'V' : k.includes('temperature') ? '°C' : k === 'battery.runtime' ? 'min' : '%', value: v, source: 'q1' };
  }
  return {
    ts: '2026-11-14T10:00:00Z',
    device: { state: device },
    ups: { on_battery: false, low_battery: false, comm_lost: false, active: [], ...over },
    measurements,
    countdown: { active: false, remaining_seconds: 0, cancelable: true, requested: false },
    policy_active: true,
  };
}
const cdOn = { active: true, remaining_seconds: 50, cancelable: true, requested: false, trigger: 'on_battery_seconds' };

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  localStorage.clear();
  ui.set({ mode: 'simple', loaded: false });
  fake.handlers.Settings = () => ({ settings: [] });
  fake.handlers.SetSetting = () => undefined;
  fake.handlers.Measurements = (name: unknown) => ({ name: String(name), unit: 'V', step: 'raw', rows: [] });
  fake.handlers.Connection = () => ({ state: 'connected', since: '' });
  fake.handlers.AuthStatus = () => ({ pin_set: true, elevated: true });
  fake.handlers.About = () => ({ version: 'test', vltd_version: '', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: '', mode: 'portable' });
  fake.handlers.WizardCompleted = () => true;
  fake.handlers.WizardLaterCount = () => 2;
  fake.handlers.Last = () => ({});
  app.set({
    auth: { pin_set: true, elevated: true },
    conn: { state: 'connected', since: '' },
    status: st(),
    policy: null,
    countdown: null,
    events: [],
  });
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});

const tick = () => new Promise((r) => setTimeout(r, 0));
async function show(el: preact.JSX.Element) {
  await act(async () => render(el, root));
  await act(async () => { await tick(); await tick(); });
}
function btn(text: string): HTMLButtonElement {
  const b = [...root.querySelectorAll('button')].find((x) => x.textContent?.trim() === text);
  if (!b) throw new Error('no button ' + text);
  return b as HTMLButtonElement;
}

describe('health rules', () => {
  it('Good: mains and no WARNING or CRITICAL condition', () => {
    expect(health(st(), null)).toEqual({ level: 'good', reasons: [{ key: 'health.good.sentence' }] });
  });

  it.each([
    ['on battery', st({ on_battery: true, active: ['ON_BATTERY'] }), 'health.on_battery'],
    ['AVR', st({ active: ['AVR_ACTIVE'] }), 'health.avr'],
    ['bypass', st({ active: ['BYPASS_ACTIVE'] }), 'health.bypass'],
    ['load 90 %', st({}, { 'load.percent': 90 }), 'health.load_high'],
    ['temperature 45 °C', st({}, { 'ups.temperature': 45 }), 'health.temp_high'],
    ['communication lost', st({ comm_lost: true }), 'health.comm_lost'],
    ['no connection', st({}, {}, 'disconnected'), 'health.comm_lost'],
    ['runtime low (WARNING)', st({ active: ['RUNTIME_LOW'] }), 'health.runtime_low'],
  ])('Attention: %s', (_n, s, key) => {
    const h = health(s, null);
    expect(h.level).toBe('attention');
    expect(h.reasons.map((r) => r.key)).toContain(key);
  });

  it('Attention limits: 89 % and 44 °C are Good', () => {
    expect(health(st({}, { 'load.percent': 89, 'ups.temperature': 44 }), null).level).toBe('good');
  });

  it.each([
    ['battery low', st({ low_battery: true, on_battery: true }), null, 'health.low_battery'],
    ['UPS fault', st({ active: ['UPS_FAULT'] }), null, 'health.fault'],
    ['overload 100 %', st({}, { 'load.percent': 100 }), null, 'health.overload'],
    ['OVERLOAD Event', st({ active: ['OVERLOAD'] }, { 'load.percent': 95 }), null, 'health.overload'],
    ['over temperature', st({ active: ['OVER_TEMPERATURE'] }, { 'ups.temperature': 52 }), null, 'health.over_temp'],
    ['countdown', st({ on_battery: true }), cdOn, 'health.countdown'],
    ['UPS shutdown', st({ active: ['UPS_SHUTDOWN_ACTIVE'] }), null, 'health.ups_shutdown'],
  ])('Problem: %s', (_n, s, cd, key) => {
    const h = health(s, cd);
    expect(h.level).toBe('problem');
    expect(h.reasons[0].key === key || h.reasons.some((r) => r.key === key)).toBe(true);
  });

  it('the Simple card shows text, icon and colour for each state', async () => {
    for (const [s, cd, level, icon] of [
      [st(), null, 'good', '✓'],
      [st({ on_battery: true }), null, 'attention', '⚠'],
      [st({ low_battery: true }), null, 'problem', '⛔'],
    ] as const) {
      app.set({ status: s, countdown: cd });
      await show(<Simple />);
      const card = root.querySelector('.status-card') as HTMLElement;
      expect(card.dataset.health).toBe(level);
      expect(card.className).toContain(level === 'good' ? 'status-ok' : level === 'attention' ? 'status-warn' : 'status-crit');
      expect(card.querySelector('.status-icon')?.textContent).toBe(icon);
      expect(card.querySelector('h3')?.textContent).toBe(t('health.' + level));
      expect(card.querySelector('.status-sentence')?.textContent?.length).toBeGreaterThan(5);
    }
  });
});

describe('Simple screen', () => {
  it('has the tiles, the automatic shutdown, the chart, the last 3 Events and the buttons', async () => {
    app.set({
      policy: { settings: { action: 'shutdown', 'trigger.on_battery_seconds': 300, 'trigger.runtime_below_minutes': 5, 'trigger.charge_below_percent': 30 } as never, version: 'a', active: true, fallback: false },
      events: [
        { ts: '2026-11-14T10:03:00Z', type: 'ONLINE', severity: 'INFO', data: {} },
        { ts: '2026-11-14T10:02:00Z', type: 'ON_BATTERY', severity: 'WARNING', data: { active: false } },
        { ts: '2026-11-14T10:01:00Z', type: 'ON_BATTERY', severity: 'WARNING', data: { active: true } },
        { ts: '2026-11-14T10:00:00Z', type: 'COMM_LOST', severity: 'WARNING', data: {} },
      ],
    });
    await show(<Simple />);
    expect(root.querySelectorAll('.tile')).toHaveLength(6);
    expect(root.textContent).toContain(t('simple.auto.on'));
    expect(root.textContent).toContain(triggerSentence(app.get().policy));
    expect(triggerSentence(app.get().policy)).toBe('The computer shuts down after 5 min on battery, or when the runtime is below 5 min or the charge is below 30 %.');
    expect(root.querySelector('[data-chart="simple"]')).toBeTruthy();
    expect(root.querySelectorAll('.simple-events li')).toHaveLength(3);
    expect(root.textContent).toContain('Power failure: on battery: ended');
    expect(root.textContent).not.toContain(t('plain.COMM_LOST'));
    expect(btn(t('mode.ui.to_expert'))).toBeTruthy();
    await act(async () => btn(t('simple.settings')).click());
    const panel = root.querySelector('.simple-settings') as HTMLElement;
    expect(panel.textContent).toContain(t('general.lang'));
    expect(panel.textContent).toContain(t('mode.ui.title'));
    expect(panel.textContent).toContain(t('wizard.general.start'));
    // no elevated settings in Simple mode
    expect(root.textContent).not.toContain(t('nav.policy'));
    expect(root.textContent).not.toContain(t('pin.set.title'));
  });

  it('plain Events use plain language in Turkish too', () => {
    setLang('tr');
    expect(plainEvent({ ts: '', type: 'ON_BATTERY', severity: 'WARNING', data: { active: true } })).toBe('Elektrik kesintisi: aküde');
    expect(plainEvent({ ts: '', type: 'ON_BATTERY', severity: 'WARNING', data: { active: false } })).toBe('Elektrik kesintisi: aküde: sona erdi');
  });
});

describe('mode switch', () => {
  it('the default is Simple; the buttons switch without a PIN and store ui.mode', async () => {
    app.set({ auth: { pin_set: true, elevated: false } });
    await show(<App />);
    expect(ui.get().mode).toBe('simple');
    expect(root.querySelector('[data-mode="simple"]')).toBeTruthy();
    expect(root.querySelector('nav.nav')).toBeNull();
    expect(root.querySelector('.status-card')).toBeTruthy();
    await act(async () => { btn(t('mode.ui.to_expert')).click(); await tick(); });
    expect(ui.get().mode).toBe('expert');
    expect(root.querySelector('nav.nav')).toBeTruthy();
    expect(fake.calls).toContainEqual({ method: 'SetSetting', args: [UI_MODE_KEY, 'expert'] });
    expect(fake.calls.some((c) => c.method === 'Login')).toBe(false);
    await act(async () => { btn(t('mode.ui.to_simple')).click(); await tick(); });
    expect(ui.get().mode).toBe('simple');
    expect(fake.calls).toContainEqual({ method: 'SetSetting', args: [UI_MODE_KEY, 'simple'] });
  });

  it('the countdown banner with Cancel is also in Simple mode', async () => {
    app.set({ countdown: cdOn as never });
    await show(<App />);
    expect(root.querySelector('[data-mode="simple"]')).toBeTruthy();
    expect(btn(t('cd.cancel'))).toBeTruthy();
  });

  it('the stored mode is used after a restart', async () => {
    fake.handlers.Settings = () => ({ settings: [{ key: UI_MODE_KEY, value: 'expert', version: 2, default: false }] });
    expect(await loadMode()).toBe('expert');
    fake.handlers.Settings = () => ({ settings: [{ key: UI_MODE_KEY, value: 'simple', version: 1, default: true }] });
    expect(await loadMode()).toBe('simple');
  });

  it('without the store (no PIN, service mode) the choice stays in localStorage', async () => {
    fake.handlers.SetSetting = () => { throw fake.goError('pin_not_set'); };
    fake.handlers.Settings = () => { throw fake.goError('pin_not_set'); };
    await setMode('expert');
    ui.set({ mode: 'simple', loaded: false });
    expect(await loadMode()).toBe('expert');
  });
});
