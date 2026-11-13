// SPDX-License-Identifier: AGPL-3.0-or-later
// Front-end component tests (TASK 03-00-00-008 step 4): PIN screen,
// countdown banner, policy form ranges, activation button rule. The
// bridge is a fake: no Wails runtime, no vltd.
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Countdown, PolicyInfo } from '../src/api';
import { CountdownBanner } from '../src/banners';
import { setLang, t } from '../src/i18n';
import { canActivate, FIELDS } from '../src/policyform';
import { Policy, PolicyForm } from '../src/screens/policy';
import { Login, PinRules, SetPin } from '../src/screens/pin';
import { Device, SCAN_TEXT } from '../src/screens/device';
import { UNTIL_LOW_TEXT, UPSControl } from '../src/screens/ups';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

let root: HTMLElement;

beforeEach(() => {
  fake.reset();
  setLang('en');
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});

async function show(el: preact.JSX.Element) {
  await act(async () => render(el, root));
}
function $(sel: string): HTMLElement {
  const e = root.querySelector(sel);
  if (!e) throw new Error('no ' + sel);
  return e as HTMLElement;
}
async function type(sel: string, v: string) {
  const e = $(sel) as HTMLInputElement;
  await act(async () => {
    e.value = v;
    e.dispatchEvent(new Event('input', { bubbles: true }));
  });
}
async function submit(sel = 'form') {
  await act(async () => {
    $(sel).dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  });
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}
async function click(el: HTMLElement) {
  await act(async () => {
    el.click();
    await new Promise((r) => setTimeout(r, 0));
  });
}
function byText(text: string): HTMLButtonElement {
  const b = [...root.querySelectorAll('button')].find((x) => x.textContent?.trim() === text);
  if (!b) throw new Error('no button ' + text);
  return b;
}

describe('PIN screen', () => {
  it('shows the rules', async () => {
    await show(<PinRules />);
    expect(root.textContent).toContain(t('pin.rules.length', { min: 6, max: 64 }));
    expect(root.querySelectorAll('li').length).toBe(4);
  });
  it('rejects a short PIN and a mismatch without a call', async () => {
    await show(<SetPin />);
    await type('#set-pin', '123');
    await type('#set-again', '123');
    await submit();
    expect($('[role=alert]').textContent).toBe(t('pin.err.length'));
    await type('#set-pin', 'abc12345');
    await type('#set-again', 'abc12346');
    await submit();
    expect($('[role=alert]').textContent).toBe(t('pin.err.mismatch'));
    expect(fake.calls.filter((c) => c.method === 'SetPIN')).toEqual([]);
  });
  it('sets a PIN and clears the fields', async () => {
    const done = vi.fn();
    await show(<SetPin onDone={done} />);
    await type('#set-pin', 'abc12345');
    await type('#set-again', 'abc12345');
    await submit();
    expect(fake.calls).toContainEqual({ method: 'SetPIN', args: ['abc12345'] });
    expect(($('#set-pin') as HTMLInputElement).value).toBe('');
    expect(done).toHaveBeenCalled();
  });
  it('shows the error of vltd (PIN rule)', async () => {
    fake.handlers.SetPIN = () => Promise.reject(fake.goError('invalid'));
    await show(<SetPin />);
    await type('#set-pin', 'abcdef');
    await type('#set-again', 'abcdef');
    await submit();
    expect($('[role=alert]').textContent).toBe(t('err.invalid'));
  });
  it('login shows the block time and disables the input', async () => {
    const until = new Date(Date.now() + 30_000).toISOString();
    await show(<Login blockUntil={until} />);
    expect(($('#login-pin') as HTMLInputElement).disabled).toBe(true);
    expect($('[role=status]').textContent).toMatch(/30|29/);
  });
  it('login with a wrong PIN shows the error and the new block time', async () => {
    fake.handlers.Login = () => Promise.reject(fake.goError('pin_incorrect'));
    fake.handlers.AuthStatus = () => ({ pin_set: true, elevated: false, block_until: new Date(Date.now() + 10_000).toISOString() });
    await show(<Login />);
    await type('#login-pin', 'abc12345');
    await submit();
    expect($('[role=alert]').textContent).toBe(t('err.pin_incorrect'));
    expect(($('#login-pin') as HTMLInputElement).value).toBe('');
    expect(($('#login-pin') as HTMLInputElement).disabled).toBe(true);
  });
});

describe('countdown banner', () => {
  const cd: Countdown = { active: true, remaining_seconds: 42, action: 'shutdown', trigger: 'on_battery', cancelable: true } as Countdown;
  it('is not shown without a countdown', async () => {
    await show(<CountdownBanner cd={{ ...cd, active: false }} elevated lowBattery={false} />);
    expect(root.textContent).toBe('');
  });
  it('shows the seconds and the cancel button (elevated)', async () => {
    await show(<CountdownBanner cd={cd} elevated lowBattery={false} />);
    expect($('[role=alert]').textContent).toContain('42');
    const b = byText(t('cd.cancel'));
    expect(b.disabled).toBe(false);
    await click(b);
    expect(fake.calls.map((c) => c.method)).toContain('CancelShutdown');
  });
  it('the cancel button needs the elevated state', async () => {
    await show(<CountdownBanner cd={cd} elevated={false} lowBattery={false} />);
    expect(byText(t('cd.cancel')).disabled).toBe(true);
    expect(root.textContent).toContain(t('cd.unlock'));
  });
  it('no cancel at LOW_BATTERY', async () => {
    await show(<CountdownBanner cd={{ ...cd, cancelable: false }} elevated lowBattery />);
    expect(byText(t('cd.cancel')).disabled).toBe(true);
    expect(root.textContent).toContain(t('cd.lowbattery'));
  });
});

const settings = Object.fromEntries(FIELDS.map((f) => [f.key, f.def])) as unknown as PolicyInfo['settings'];
const pol = (o: Partial<PolicyInfo> = {}): PolicyInfo => ({
  settings,
  version: 'v1abcdef0123456789',
  active: false,
  fallback: false,
  last_simulation: null,
  ...o,
});
const passed = (version: string, ok = true) =>
  ({ id: 1, time: '2026-11-10T10:00:00Z', version, passed: ok, result: { version, passed: ok, time: '', scenarios: [] } });

describe('policy form ranges', () => {
  it('rejects values out of range and sends nothing', async () => {
    await show(<PolicyForm p={pol()} onSaved={() => undefined} />);
    await type('#pf-countdown_seconds', '5'); // min 10
    await type('#pf-trigger-charge_below_percent', '101'); // max 100
    await type('#pf-trigger-on_battery_seconds', '1.5');
    await submit();
    expect(root.querySelectorAll('[role=alert]').length).toBe(3);
    expect($('#pf-countdown_seconds').getAttribute('aria-invalid')).toBe('true');
    expect(fake.calls.filter((c) => c.method === 'SetPolicy')).toEqual([]);
  });
  it('accepts the limits and sends numbers', async () => {
    fake.handlers.SetPolicy = () => pol();
    const saved = vi.fn();
    await show(<PolicyForm p={pol()} onSaved={saved} />);
    await type('#pf-countdown_seconds', '600');
    await type('#pf-trigger-charge_below_percent', '0');
    await submit();
    const c = fake.calls.find((x) => x.method === 'SetPolicy');
    expect(c).toBeTruthy();
    const v = c!.args[0] as Record<string, unknown>;
    expect(v.countdown_seconds).toBe(600);
    expect(v['trigger.charge_below_percent']).toBe(0);
    expect(saved).toHaveBeenCalled();
  });
  it('shows the default and the range of each number field', async () => {
    await show(<PolicyForm p={pol()} onSaved={() => undefined} />);
    expect($('#pf-countdown_seconds-hint').textContent).toContain('10');
    expect($('#pf-countdown_seconds-hint').textContent).toContain('600');
  });
});

describe('activation button rule', () => {
  it('canActivate', () => {
    const v = 'v1abcdef0123456789';
    expect(canActivate(null, true)).toBe(false);
    expect(canActivate(pol(), true)).toBe(false); // no Simulation
    expect(canActivate(pol({ last_simulation: passed(v) }), false)).toBe(false); // locked
    expect(canActivate(pol({ last_simulation: passed(v, false) }), true)).toBe(false); // failed
    expect(canActivate(pol({ last_simulation: passed('other') }), true)).toBe(false); // old version
    expect(canActivate(pol({ last_simulation: passed(v), active: true }), true)).toBe(false); // active
    expect(canActivate(pol({ last_simulation: passed(v), fallback: true }), true)).toBe(false); // Fallback
    expect(canActivate(pol({ last_simulation: passed(v) }), true)).toBe(true);
  });
  async function policyScreen(p: PolicyInfo, elevated: boolean) {
    fake.handlers.Policy = () => p;
    fake.handlers.ShutdownStatus = () => ({ active: false });
    app.set({ auth: { pin_set: true, elevated } as never, policy: p });
    await show(<Policy />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
  it('the button is off without a passed Simulation', async () => {
    await policyScreen(pol(), true);
    expect(byText(t('policy.activate')).disabled).toBe(true);
    expect(root.textContent).toContain(t('policy.activate.rule'));
  });
  it('the button is on after a passed Simulation and asks first', async () => {
    const p = pol({ last_simulation: passed('v1abcdef0123456789') });
    await policyScreen(p, true);
    const b = byText(t('policy.activate'));
    expect(b.disabled).toBe(false);
    await click(b);
    expect(fake.calls.map((c) => c.method)).not.toContain('ActivatePolicy');
    expect(root.textContent).toContain(t('policy.activate.confirm'));
  });
  it('the button is off when locked', async () => {
    await policyScreen(pol({ last_simulation: passed('v1abcdef0123456789') }), false);
    expect(byText(t('policy.activate')).disabled).toBe(true);
  });
});

// VLT-IS-006 TASK 03-00-00-004 step 3.4 and TASK 03-00-00-008: the port
// list, the scan of the marked ports only, "Use this port", the port busy
// help. The bridge is a fake: no port is opened.
describe('Device screen ports', () => {
  function setupDevice(state: string) {
    app.set({ auth: { pin_set: true, elevated: true } as never });
    fake.handlers.DeviceStatus = () => ({ state, since: '', config: { transport: 'not_configured', serial_port: '', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] } });
    fake.handlers.DevicePorts = () => ({ ports: [{ name: 'COM3', is_usb: true, vid: '1a86', pid: '7523', product: 'USB Serial' }, { name: 'COM4', is_usb: false }] });
    fake.handlers.DeviceScan = () => ({ results: [{ name: 'COM3', result: 'response', valid_q1: true }] });
  }
  async function showDevice() {
    await show(<Device />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
  it('shows the port busy help and the scan warning', async () => {
    setupDevice('port busy');
    await showDevice();
    expect(root.textContent).toContain('Another program uses this port. Stop UPSilon 2000 and the RupsMon service, or the other program, then try again.');
    expect(root.textContent).toContain('VLT sends the text Q1 to each marked port. Mark only ports with a UPS.');
    expect(root.textContent).toContain('USB Serial 1a86:7523');
  });
  it('shows the exact Linux serial permission help and does not run a command', async () => {
    expect(t('device.port.permission.help')).toBe('Your user cannot open this port. Run this command one time, then log out and log in: sudo usermod -aG dialout <user name>');
    setupDevice('port permission denied');
    await showDevice();
    expect(root.textContent).toContain(t('device.port.permission.help'));
    expect(root.textContent).not.toContain(t('device.hid.permission.help'));
  });
  it('shows the HID permission help with a udev rule as an example', async () => {
    setupDevice('hid permission denied');
    await showDevice();
    expect(root.textContent).toContain(t('device.hid.permission.help'));
    expect(t('device.hid.permission.help')).toContain('KERNEL=="hidraw*"');
    expect(root.textContent).not.toContain(t('device.port.permission.help'));
  });
  it('shows both permission help texts when both opens fail', async () => {
    setupDevice('permission denied');
    await showDevice();
    expect(root.textContent).toContain(t('device.port.permission.help'));
    expect(root.textContent).toContain(t('device.hid.permission.help'));
  });
  it('scans only the marked ports after the confirmation text', async () => {
    setupDevice('not configured');
    await showDevice();
    const scanBtn = byText(t('device.scan.button'));
    expect(scanBtn.disabled).toBe(true);
    const box = root.querySelector('input[aria-label="' + t('device.ports.mark.label', { name: 'COM3' }) + '"]') as HTMLInputElement;
    await act(async () => {
      box.click();
    });
    await type('#scan-text', 'scan ports');
    expect(byText(t('device.scan.button')).disabled).toBe(true);
    await type('#scan-text', SCAN_TEXT);
    expect(byText(t('device.scan.button')).disabled).toBe(false);
    await click(byText(t('device.scan.button')));
    const scans = fake.calls.filter((c) => c.method === 'DeviceScan');
    expect(scans).toEqual([{ method: 'DeviceScan', args: [['COM3'], SCAN_TEXT] }]);
    expect(root.textContent).toContain(t('device.scan.result.response.q1'));
  });
  it('uses a port after the confirmation', async () => {
    setupDevice('not configured');
    await showDevice();
    const use = root.querySelector('button[aria-label="' + t('device.ports.use.label', { name: 'COM4' }) + '"]') as HTMLButtonElement;
    await click(use);
    expect(fake.calls.some((c) => c.method === 'SetDeviceSettings')).toBe(false);
    const confirm = [...root.querySelectorAll('button')].filter((x) => x.textContent?.trim() === t('device.ports.use.button'));
    await click(confirm[confirm.length - 1]);
    const set = fake.calls.filter((c) => c.method === 'SetDeviceSettings');
    expect(set.length).toBe(1);
    expect(set[0].args[0]).toEqual({ 'device.transport': 'serial', 'device.serial_port': 'COM4' });
    expect(fake.calls.some((c) => c.method === 'DeviceScan')).toBe(false);
  });
});

// VLT-IS-006 TASK 03-00-00-008 step 1: UPS Control screen. The bridge is a
// fake: no command goes to a UPS.
describe('UPS Control screen', () => {
  const all = ['test.10s', 'test.minutes', 'test.until_low', 'test.cancel', 'beeper'];
  async function upsScreen(elevated: boolean, functions = all, last?: unknown) {
    app.set({ auth: { pin_set: true, elevated } as never });
    fake.handlers.UPSControlCapabilities = () => ({ connected: functions.length > 0, functions, last_test: last });
    fake.handlers.UPSTestStart = () => ({ function: 'test.10s', bytes: 'T', sends: 1, verified: true });
    fake.handlers.UPSBeeperSet = () => ({ function: 'beeper', bytes: 'Q', sends: 1, verified: true });
    fake.handlers.UPSTestCancel = () => ({ function: 'test.cancel', bytes: 'CT', sends: 1, verified: true });
    await show(<UPSControl />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
  const names = ['ups.test.10s', 'ups.test.minutes', 'ups.test.low', 'ups.test.cancel', 'ups.beeper.on', 'ups.beeper.off'];
  it('each button is off without the elevated state', async () => {
    await upsScreen(false);
    for (const n of names) expect(byText(t(n)).disabled, n).toBe(true);
  });
  it('each button is off without the capability', async () => {
    await upsScreen(true, []);
    for (const n of names) expect(byText(t(n)).disabled, n).toBe(true);
    expect(root.textContent).toContain(t('ups.noups'));
  });
  it('has no shutdown button', async () => {
    await upsScreen(true);
    const labels = [...root.querySelectorAll('button')].map((b) => b.textContent ?? '');
    expect(labels.some((l) => /shut ?down|output off/i.test(l))).toBe(false);
  });
  it('test 10 seconds asks first, then sends START TEST', async () => {
    await upsScreen(true);
    await click(byText(t('ups.test.10s')));
    expect(fake.calls.some((c) => c.method === 'UPSTestStart')).toBe(false);
    await click(byText(t('ups.test.start')));
    const c = fake.calls.filter((x) => x.method === 'UPSTestStart');
    expect(c).toEqual([{ method: 'UPSTestStart', args: ['10s', 0, 'START TEST'] }]);
  });
  it('test for minutes checks the range 1 to 99', async () => {
    await upsScreen(true);
    await type('#ups-minutes', '100');
    expect(byText(t('ups.test.minutes')).disabled).toBe(true);
    await type('#ups-minutes', '15');
    await click(byText(t('ups.test.minutes')));
    await click(byText(t('ups.test.start')));
    expect(fake.calls.filter((x) => x.method === 'UPSTestStart')[0].args).toEqual(['minutes', 15, 'START TEST']);
  });
  it('test until low shows the WARNING and needs the typed text', async () => {
    await upsScreen(true);
    await click(byText(t('ups.test.low')));
    expect(root.textContent).toContain('This test discharges the battery until it is low. The protected equipment has less battery time after the test until the battery is charged again.');
    await type('#ups-low-text', 'test until low');
    expect(byText(t('ups.test.low.start')).disabled).toBe(true);
    await type('#ups-low-text', UNTIL_LOW_TEXT);
    await submit('form');
    expect(fake.calls.filter((x) => x.method === 'UPSTestStart')).toEqual([{ method: 'UPSTestStart', args: ['until_low', 0, 'TEST UNTIL LOW'] }]);
  });
  it('beeper off and cancel test send their requests', async () => {
    await upsScreen(true);
    await click(byText(t('ups.beeper.off')));
    await click(byText(t('ups.test.cancel')));
    expect(fake.calls.filter((x) => x.method === 'UPSBeeperSet')[0].args).toEqual([false]);
    expect(fake.calls.some((x) => x.method === 'UPSTestCancel')).toBe(true);
  });
  it('shows the last test result and its time', async () => {
    await upsScreen(true, all, { result: 'TEST_FAILED', start: '2026-11-10T10:00:00Z', end: '2026-11-10T10:00:10Z', ups_failed: false, battery_low: true, cancelled: false });
    expect(root.textContent).toContain('TEST_FAILED');
    expect(root.textContent).toContain(t('ups.battery_low'));
  });
});

// VLT-IS-006 TASK 03-00-00-008 step 2: the UPS output off Settings.
describe('Policy UPS output off', () => {
  it('shows the WARNING, the text with the values and the ranges', async () => {
    await show(<PolicyForm p={pol()} onSaved={() => undefined} />);
    expect(root.textContent).toContain('An incorrect delay can turn off the power before the computer has shut down. Data can be lost.');
    expect(root.textContent).toContain('The UPS turns off its output 2 minutes after the shutdown starts. The computer must shut down in less time. The UPS turns on its output 1 minutes later.');
    expect($('#pf-ups_off-delay_minutes-hint').textContent).toContain('10');
    expect($('#pf-ups_off-restore_minutes-hint').textContent).toContain('9999');
  });
  it('refuses a delay out of range', async () => {
    fake.handlers.SetPolicy = () => pol();
    await show(<PolicyForm p={pol()} onSaved={() => undefined} />);
    await type('#pf-ups_off-delay_minutes', '11');
    await submit('form');
    expect(fake.calls.some((c) => c.method === 'SetPolicy')).toBe(false);
    await type('#pf-ups_off-delay_minutes', '10');
    await submit('form');
    const c = fake.calls.find((x) => x.method === 'SetPolicy');
    expect((c!.args[0] as Record<string, unknown>)['ups_off.delay_minutes']).toBe(10);
  });
});
