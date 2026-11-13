// SPDX-License-Identifier: AGPL-3.0-or-later
// General screen: start at logon and data folder (VLT-IS-008 TASK 03-00-00-001 and 002).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { General } from '../src/screens/general';
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

async function show(el: preact.JSX.Element, ticks = 2) {
  await act(async () => render(el, root));
  for (let i = 0; i < ticks; i++) {
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  }
}

describe('General start at logon', () => {
  it('hides the switch when the mode is not portable', async () => {
    fake.handlers.About = () => ({ version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'service' });
    await show(<General />);
    expect(root.textContent).not.toContain(t('general.startAtLogon'));
  });

  it('shows the switch in portable mode and saves the setting', async () => {
    let on = false;
    fake.handlers.About = () => ({ version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'portable' });
    fake.handlers.StartAtLogon = () => on;
    fake.handlers.SetStartAtLogon = (v: unknown) => { on = !!v; };
    await show(<General />);
    expect(root.textContent).toContain(t('general.startAtLogon'));
    expect(root.textContent).toContain(t('general.startAtLogon.help'));
    const box = root.querySelector('input[type="checkbox"]') as HTMLInputElement;
    expect(box).toBeTruthy();
    expect(box.checked).toBe(false);
    await act(async () => {
      box.checked = true;
      box.dispatchEvent(new Event('change', { bubbles: true }));
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(fake.calls.some((c) => c.method === 'SetStartAtLogon' && c.args[0] === true)).toBe(true);
    expect(root.textContent).toContain(t('general.startAtLogon.saved'));
  });
});

describe('General data folder', () => {
  it('hides the path when About has no data_dir', async () => {
    fake.handlers.About = () => ({ version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'service' });
    await show(<General />);
    expect(root.textContent).not.toContain(t('general.dataDir'));
  });

  it('shows the data folder path in portable mode', async () => {
    fake.handlers.About = () => ({ version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'portable', data_dir: '/tmp/vv-data' });
    await show(<General />);
    expect(root.textContent).toContain(t('general.dataDir'));
    expect(root.textContent).toContain(t('general.dataDir.help'));
    expect(root.textContent).toContain('/tmp/vv-data');
  });
});

describe('General Set PIN and Remove PIN', () => {
  it('shows Set PIN in portable mode when no PIN is set', async () => {
    app.set({ auth: { pin_set: false, elevated: true } as never });
    fake.handlers.About = () => ({ version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'portable', data_dir: '/tmp/d' });
    await show(<General />);
    expect(root.textContent).toContain(t('pin.set.title'));
    expect(root.textContent).not.toContain(t('pin.remove.title'));
  });

  it('shows Remove PIN in portable mode when a PIN is set, and removes it', async () => {
    let pinSet = true;
    app.set({ auth: { pin_set: true, elevated: true } as never });
    fake.handlers.About = () => ({ version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'portable', data_dir: '/tmp/d' });
    fake.handlers.AuthStatus = () => ({ pin_set: pinSet, elevated: !pinSet });
    fake.handlers.RemovePIN = () => { pinSet = false; };
    await show(<General />);
    expect(root.textContent).toContain(t('pin.remove.title'));
    expect(root.textContent).toContain(t('pin.remove.intro'));
    const input = root.querySelector('#rm-pin') as HTMLInputElement;
    expect(input).toBeTruthy();
    await act(async () => {
      input.value = '7a3k9z';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      await new Promise((r) => setTimeout(r, 0));
    });
    const btn = [...root.querySelectorAll('button')].find((b) => b.textContent?.trim() === t('pin.remove.button')) as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(fake.calls.some((c) => c.method === 'RemovePIN' && c.args[0] === '7a3k9z')).toBe(true);
    // After a successful remove, refreshAuth shows Set PIN again.
    expect(root.textContent).toContain(t('pin.set.title'));
    expect(root.textContent).not.toContain(t('pin.remove.title'));
  });

  it('hides Set PIN and Remove PIN when the mode is not portable', async () => {
    app.set({ auth: { pin_set: false, elevated: false } as never });
    fake.handlers.About = () => ({ version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'service' });
    await show(<General />);
    expect(root.textContent).not.toContain(t('pin.set.title'));
    expect(root.textContent).not.toContain(t('pin.remove.title'));
  });
});
