// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-011 TASK 03-00-00-001 (R-08): the Owner sequence in the GUI.
// A PIN set in the wizard keeps the session elevated (OD-50), the lock
// screen never covers the wizard (OD-51), user input sends a throttled
// keep-alive (OD-52) and the lock screen opens only on the step 5 events.
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../src/app';
import { setLang, t } from '../src/i18n';
import { Wizard, wizardWanted } from '../src/screens/wizard';
import { TOUCH_MS, app, resetTouch, userInput } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

const PIN = '482619';
// engine is the auth state of the fake engine, with the OD-50 rule.
let engine = { pin_set: false, elevated: true };

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  resetTouch();
  setLang('en');
  wizardWanted.set({ open: false });
  engine = { pin_set: false, elevated: true }; // portable, no PIN: elevated (OD-37)
  const pol = {
    settings: { action: 'shutdown', 'trigger.on_battery_seconds': 300, 'trigger.runtime_below_minutes': 5, 'trigger.charge_below_percent': 30,
      'trigger.low_battery_flag': true, 'trigger.comm_lost_on_battery_seconds': 180, countdown_seconds: 60, cancel_when_mains_returns: true,
      force_close_applications: true, 'ups_off.enabled': false, 'ups_off.delay_minutes': 2, 'ups_off.restore_minutes': 1 },
    version: 'abc', active: false, fallback: false,
    last_simulation: null as null | { version: string; passed: boolean; time: string; scenarios: [] },
  };
  const needElev = () => {
    if (engine.pin_set && !engine.elevated) throw fake.goError('not_elevated');
  };
  fake.handlers.AuthStatus = () => ({ ...engine });
  fake.handlers.SetPIN = () => { engine = { pin_set: true, elevated: true }; };
  fake.handlers.Login = (pin: unknown) => {
    if (pin !== PIN) throw fake.goError('pin_incorrect');
    engine.elevated = true;
  };
  fake.handlers.Logout = () => { engine.elevated = false; };
  fake.handlers.SetDeviceSettings = () => { needElev(); };
  fake.handlers.Policy = () => pol;
  fake.handlers.Simulate = () => {
    needElev();
    pol.last_simulation = { version: 'abc', passed: true, time: '2026-11-14T10:00:00Z', scenarios: [] };
    return pol.last_simulation;
  };
  fake.handlers.ActivatePolicy = () => { needElev(); pol.active = true; };
  fake.handlers.DevicePorts = () => ({ ports: [] });
  fake.handlers.DeviceStatus = () => ({ state: 'connected', since: '', config: { transport: 'test', serial_port: '', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] } });
  fake.handlers.Channels = () => ({ channels: [], email_presets: [] });
  fake.handlers.TelegramChats = () => ({ chats: [] });
  fake.handlers.Connection = () => ({ state: 'connected', since: '' });
  fake.handlers.Last = () => ({});
  fake.handlers.Settings = () => ({ settings: [] });
  fake.handlers.ShutdownStatus = () => ({ active: false, remaining_seconds: 0, cancelable: true, requested: false });
  fake.handlers.SystemStatus = () => ({ vault: { state: 'ok', reason: '' }, store: { state: 'ok', schema_version: 3 } });
  fake.handlers.Events = () => ({ events: [], next: 0 });
  fake.handlers.About = () => ({ version: 'test', vltd_version: '', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: '', mode: 'portable' });
  fake.handlers.WizardCompleted = () => true;
  fake.handlers.WizardLaterCount = () => 2;
  app.set({
    auth: { ...engine },
    conn: { state: 'connected', since: '' },
    status: null,
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
function byText(text: string): HTMLButtonElement {
  const b = [...root.querySelectorAll('button')].find((x) => x.textContent?.trim() === text);
  if (!b) throw new Error('no button ' + text + ' in ' + (root.textContent ?? '').slice(0, 300));
  return b;
}
async function click(el: HTMLElement) {
  await act(async () => { el.click(); await tick(); await tick(); });
}
async function typeIn(el: HTMLInputElement, v: string) {
  await act(async () => {
    el.value = v;
    el.dispatchEvent(new Event('input', { bubbles: true }));
  });
}
async function submit(form: HTMLFormElement) {
  await act(async () => {
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await tick(); await tick(); await tick();
  });
}

describe('Owner sequence T-14 1.1-1.7 (VLT-IS-011 R-08)', () => {
  it('a PIN set in the wizard does not lock the next steps', async () => {
    const done = vi.fn();
    // 1.1 a new data folder: the wizard starts at the PIN step
    await show(<Wizard first portable onDone={done} />);
    expect(root.textContent).toContain(t('pin.set.title'));
    // 1.2 set the PIN
    const [p1, p2] = [...root.querySelectorAll('input[type="password"]')] as HTMLInputElement[];
    await typeIn(p1, PIN);
    await typeIn(p2, PIN);
    await submit(p1.form!);
    expect(fake.calls).toContainEqual({ method: 'SetPIN', args: [PIN] });
    // 1.3 the port step opens without the lock
    expect(root.textContent).toContain(t('wizard.port.intro'));
    expect(app.get().auth).toMatchObject({ pin_set: true, elevated: true });
    expect(root.querySelector('.wizard-unlock')).toBeNull();
    expect(root.textContent).not.toContain(t('locked.need'));
    // 1.4 Test connection (simulator) saves with device.settings.set and no PIN request
    const sim = byText(t('device.transport.test'));
    expect(sim.disabled).toBe(false);
    await click(sim);
    expect(fake.calls).toContainEqual({ method: 'SetDeviceSettings', args: [{ 'device.transport': 'test' }] });
    expect(root.textContent).toContain(t('wizard.port.selected', { port: t('device.transport.test') }));
    expect(root.querySelector('.wizard-unlock')).toBeNull();
    // 1.5 the policy: Simulation and activation, no PIN request
    await click(byText(t('wizard.next'))); // values
    await click(byText(t('wizard.next'))); // policy
    expect(root.textContent).toContain(t('wizard.policy.intro'));
    await click(byText(t('policy.simulate')));
    const act1 = byText(t('policy.activate'));
    expect(act1.disabled).toBe(false);
    await click(act1);
    const confirm = [...root.querySelectorAll('button')].filter((b) => b.textContent?.trim() === t('policy.activate')).pop()!;
    await click(confirm);
    expect(fake.calls.some((c) => c.method === 'ActivatePolicy')).toBe(true);
    expect(root.querySelector('.wizard-unlock')).toBeNull();
    expect(fake.calls.some((c) => c.method === 'Login')).toBe(false);
    // 1.6 complete: the wizard ends, unlocked
    for (let i = 0; i < 6 && !root.querySelector('.wizard-footer')?.textContent?.includes(t('wizard.finish')); i++) {
      expect(root.querySelector('.wizard-unlock')).toBeNull();
      await click(byText(t('wizard.skip')));
    }
    await click(byText(t('wizard.finish')));
    expect(done).toHaveBeenCalled();
    expect(app.get().auth?.elevated).toBe(true);
  });

  it('a locked wizard step shows the PIN field inside the dialog (OD-51)', async () => {
    engine = { pin_set: true, elevated: false };
    app.set({ ...app.get(), auth: { ...engine } });
    await show(<Wizard first={false} />);
    expect(root.querySelector('.wizard-dialog .wizard-unlock')).toBeTruthy();
    const pin = root.querySelector('.wizard-unlock input[type="password"]') as HTMLInputElement;
    await typeIn(pin, PIN);
    await submit(pin.form!);
    expect(app.get().auth?.elevated).toBe(true);
    expect(root.querySelector('.wizard-unlock')).toBeNull();
    expect(byText(t('device.transport.test')).disabled).toBe(false);
  });
});

describe('lock screen (VLT-IS-011 001 step 5)', () => {
  it('opens at the start with a PIN, never over the wizard, and after Lock (1.7)', async () => {
    engine = { pin_set: true, elevated: false };
    app.set({ ...app.get(), auth: { ...engine } });
    await show(<App />);
    expect(root.querySelector('.lock-screen')).toBeTruthy();
    // never over the wizard
    await act(async () => { wizardWanted.set({ open: true, start: '' }); await tick(); });
    expect(root.querySelector('.lock-screen')).toBeNull();
    expect(root.querySelector('.wizard-unlock')).toBeTruthy();
    await act(async () => { wizardWanted.set({ open: false, start: '' }); await tick(); });
    // unlock with the PIN
    const pin = root.querySelector('.lock-screen input[type="password"]') as HTMLInputElement;
    await typeIn(pin, PIN);
    await submit(pin.form!);
    expect(app.get().auth?.elevated).toBe(true);
    expect(root.querySelector('.lock-screen')).toBeNull();
    // 1.7 Lock, then the lock screen again, then unlock with the PIN
    await click(byText(t('lock.button')));
    expect(root.querySelector('.lock-screen')).toBeTruthy();
    const pin2 = root.querySelector('.lock-screen input[type="password"]') as HTMLInputElement;
    await typeIn(pin2, PIN);
    await submit(pin2.form!);
    expect(app.get().auth?.elevated).toBe(true);
  });
});

describe('keep-alive after user input (OD-52)', () => {
  it('sends Touch at most one time in 30 s, only when elevated', () => {
    app.set({ ...app.get(), auth: { pin_set: true, elevated: false } });
    userInput(1_000);
    expect(fake.calls.filter((c) => c.method === 'Touch')).toHaveLength(0);
    app.set({ ...app.get(), auth: { pin_set: true, elevated: true } });
    userInput(2_000);
    userInput(2_000 + TOUCH_MS - 1);
    expect(fake.calls.filter((c) => c.method === 'Touch')).toHaveLength(1);
    userInput(2_000 + TOUCH_MS + 1);
    expect(fake.calls.filter((c) => c.method === 'Touch')).toHaveLength(2);
  });

  it('keyboard and mouse input call Touch; polling does not', async () => {
    engine = { pin_set: true, elevated: true };
    app.set({ ...app.get(), auth: { ...engine } });
    await show(<App />);
    const before = fake.calls.filter((c) => c.method === 'Touch').length;
    expect(before).toBe(0);
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' })); await tick(); });
    expect(fake.calls.filter((c) => c.method === 'Touch')).toHaveLength(1);
  });
});
