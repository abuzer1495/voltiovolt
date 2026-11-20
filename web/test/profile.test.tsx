// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-012 TASK 03-00-00-003 step 5: the mark "Probable UPS" and the UPS
// profile choice for a port with the Reference UPS bridge chip (Prolific
// 067B:2303). The bridge is a fake: no port is opened, no byte is sent.
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { Ports } from '../src/screens/device';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

const PROFILE = { id: 'tuncmatik-newtech-pro-ii', name: 'Tunçmatik Newtech Pro II (Megatec)' };
let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  app.set({ auth: { pin_set: false, elevated: true } as never });
  fake.handlers.DevicePorts = () => ({
    ports: [
      { name: 'COM3', is_usb: true, vid: '067b', pid: '2303', product: 'Prolific USB-to-Serial Comm Port', probable_ups: true, bridge: 'Prolific PL2303', profiles: [PROFILE] },
      { name: 'COM4', is_usb: true, vid: '1a86', pid: '7523', product: 'USB-SERIAL CH340', probable_ups: true, bridge: 'WCH CH340' },
      { name: 'COM1', is_usb: false },
    ],
  });
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});

async function settle() {
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
}
async function showPorts() {
  await act(async () => render(<Ports elevated part="select" onUsed={() => undefined} />, root));
  await settle();
}
function row(name: string): HTMLTableRowElement {
  const r = Array.from(root.querySelectorAll('tbody tr')).find((x) => x.textContent?.includes(name));
  if (!r) throw new Error('no row ' + name);
  return r as HTMLTableRowElement;
}
async function click(el: Element) {
  await act(async () => (el as HTMLElement).click());
  await settle();
}
function confirmButton(): HTMLButtonElement {
  const b = Array.from(root.querySelectorAll('button')).find((x) => x.textContent === t('device.ports.use.button'));
  if (!b) throw new Error('no confirm button');
  return b;
}
function saved(): Record<string, unknown> {
  const c = fake.calls.filter((x) => x.method === 'SetDeviceSettings');
  expect(c.length).toBe(1);
  return c[0].args[0] as Record<string, unknown>;
}

describe('Probable UPS mark (step 2)', () => {
  it('marks the ports with a known bridge chip and nothing else', async () => {
    await showPorts();
    expect(row('COM3').textContent).toContain(t('device.ports.probable'));
    expect(row('COM4').textContent).toContain(t('device.ports.probable'));
    expect(row('COM1').textContent).not.toContain(t('device.ports.probable'));
    expect(row('COM3').querySelector('.badge')?.getAttribute('title')).toBe('Prolific PL2303');
    // The mark sends nothing and selects nothing (SEC-099).
    expect(fake.calls.map((c) => c.method)).toEqual(['DevicePorts']);
    expect(root.querySelector('.profile-choice')).toBeNull();
  });
  it('has the Turkish text', () => {
    setLang('tr');
    expect(t('device.ports.probable')).toBe('Muhtemel UPS');
    expect(t('device.profile.other')).toBe('Başka UPS (otomatik algılama)');
  });
});

describe('UPS profile (step 3)', () => {
  it('offers the profile for 067B:2303 and saves it with DTR and RTS unchanged', async () => {
    await showPorts();
    await click(row('COM3').querySelector('button')!);
    const radios = Array.from(root.querySelectorAll<HTMLInputElement>('.profile-choice input[type=radio]'));
    expect(radios.map((r) => r.value)).toEqual([PROFILE.id, '']);
    expect(radios[0].checked).toBe(true);
    expect(root.querySelector('.profile-choice')?.textContent).toContain(PROFILE.name);
    expect(root.querySelector('.profile-choice')?.textContent).toContain(t('device.profile.other'));
    await click(confirmButton());
    expect(saved()).toEqual({ 'device.transport': 'serial', 'device.serial_port': 'COM3', 'device.profile': PROFILE.id, 'device.serial_dtr': 'unchanged', 'device.serial_rts': 'unchanged' });
  });
  it('"Other UPS" selects the automatic detection', async () => {
    await showPorts();
    await click(row('COM3').querySelector('button')!);
    const other = root.querySelector<HTMLInputElement>('.profile-choice input[value=""]')!;
    await click(other);
    await click(confirmButton());
    expect(saved()).toEqual({ 'device.transport': 'serial', 'device.serial_port': 'COM3', 'device.profile': '' });
  });
  it('offers no profile for another chip', async () => {
    await showPorts();
    await click(row('COM4').querySelector('button')!);
    expect(root.querySelector('.profile-choice')).toBeNull();
    await click(confirmButton());
    expect(saved()).toEqual({ 'device.transport': 'serial', 'device.serial_port': 'COM4', 'device.profile': '' });
  });
});
