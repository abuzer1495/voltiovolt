// SPDX-License-Identifier: AGPL-3.0-or-later
// Demo mode UI (VLT-IS-009 TASK 03-00-00-005, OD-43).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DemoBanner, DemoShutdownBanner } from '../src/banners';
import { setLang, t } from '../src/i18n';
import { Device } from '../src/screens/device';
import { Events } from '../src/screens/events';
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

describe('demo mode UI', () => {
  it('has the exact English texts', () => {
    expect(t('device.transport.demo')).toBe('Demo (simulator)');
    expect(t('device.demo.banner')).toBe('DEMO');
    expect(t('device.demo.powerfail')).toBe('Simulate power failure');
    expect(t('demo.shutdown')).toBe('Demo: the computer would shut down now.');
  });

  it('shows the yellow DEMO banner and the dry-run shutdown text', async () => {
    await show(
      <>
        <DemoBanner demo />
        <DemoShutdownBanner message="Demo: the computer would shut down now." />
      </>,
    );
    expect(root.textContent).toContain('DEMO');
    expect(root.textContent).toContain('Demo: the computer would shut down now.');
    expect(root.querySelector('.banner-demo')).toBeTruthy();
  });

  it('Device offers Demo (simulator) and Simulate power failure', async () => {
    fake.handlers.DeviceStatus = () => ({
      state: 'connected',
      since: '',
      config: { transport: 'demo', serial_port: '', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] },
    });
    fake.handlers.DevicePorts = () => ({ ports: [] });
    fake.handlers.Settings = () => ({ settings: [] });
    fake.handlers.DemoPowerFail = () => undefined;
    await show(<Device />);
    expect(root.textContent).toContain(t('device.transport.demo'));
    expect(root.textContent).toContain(t('device.demo.powerfail'));
    const sel = root.querySelector('#dev-transport') as HTMLSelectElement;
    expect([...sel.querySelectorAll('option')].some((o) => o.value === 'demo')).toBe(true);
    const btn = [...root.querySelectorAll('button')].find((b) => b.textContent === t('device.demo.powerfail'));
    expect(btn).toBeTruthy();
    await act(async () => { btn!.click(); });
    expect(fake.calls.some((c) => c.method === 'DemoPowerFail')).toBe(true);
  });

  it('Events hides the demo checkbox off by default', async () => {
    fake.handlers.Events = () => ({ events: [], next: 0 });
    await show(<Events />);
    const box = root.querySelector('#ev-demo') as HTMLInputElement;
    expect(box).toBeTruthy();
    expect(box.checked).toBe(false);
    expect(root.textContent).toContain(t('events.show_demo'));
    const call = fake.calls.find((c) => c.method === 'Events');
    expect(call).toBeTruthy();
    expect(call?.args.length).toBe(2);
  });
});
