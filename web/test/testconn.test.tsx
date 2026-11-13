// SPDX-License-Identifier: AGPL-3.0-or-later
// Test connection UI (VLT-IS-010 TASK 03-00-00-004).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TestBanner, TestShutdownBanner } from '../src/banners';
import { setLang, t } from '../src/i18n';
import { Device } from '../src/screens/device';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  app.set({ auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' } });
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});

async function show(el: preact.JSX.Element, ticks = 3) {
  await act(async () => render(el, root));
  for (let i = 0; i < ticks; i++) {
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  }
}

describe('test connection UI', () => {
  it('has the exact English texts', () => {
    expect(t('device.transport.test')).toBe('Test connection (simulator)');
    expect(t('device.test.banner')).toBe('TEST CONNECTION');
    expect(t('device.test.notfound')).toBe('Start vlt-simulator.exe, then wait. VoltioVolt connects automatically.');
    expect(t('device.test.shutdown')).toBe('Test: the computer would shut down now.');
    expect(t('device.test.real_shutdown.warn')).toBe('With this setting on, a test can turn off this computer.');
    expect(t('device.state.simulator not found')).toBe('simulator not found');
  });

  it('shows the blue TEST CONNECTION banner and the dry-run shutdown text', async () => {
    await show(
      <>
        <TestBanner test />
        <TestShutdownBanner message="Test: the computer would shut down now." />
      </>,
    );
    expect(root.textContent).toContain('TEST CONNECTION');
    expect(root.textContent).toContain('Test: the computer would shut down now.');
    expect(root.querySelector('.banner-test')).toBeTruthy();
  });

  it('Device offers Test connection, the WARNING, and help when the simulator is not found', async () => {
    fake.handlers.DeviceStatus = () => ({
      state: 'simulator not found',
      since: '',
      config: { transport: 'test', serial_port: '', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] },
    });
    fake.handlers.DevicePorts = () => ({ ports: [] });
    fake.handlers.Settings = () => ({ settings: [{ key: 'test.real_shutdown', value: false }] });
    await show(<Device />, 6);
    expect(root.textContent).toContain(t('device.transport.test'));
    expect(root.textContent).toContain(t('device.test.notfound'));
    expect(root.textContent).toContain(t('device.test.real_shutdown.warn'));
    const sel = root.querySelector('#dev-transport') as HTMLSelectElement;
    expect([...sel.querySelectorAll('option')].some((o) => o.value === 'test')).toBe(true);
    const box = root.querySelector('#dev-real-shutdown') as HTMLInputElement;
    expect(box).toBeTruthy();
    expect(box.checked).toBe(false);
  });
});
