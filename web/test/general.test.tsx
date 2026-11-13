// SPDX-License-Identifier: AGPL-3.0-or-later
// General screen: start at logon (VLT-IS-008 TASK 03-00-00-001 step 6).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { General } from '../src/screens/general';
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
