// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-009 TASK 03-00-00-002: Forgot PIN (OD-40).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { Login } from '../src/screens/pin';
import { Wizard, startWizard, wizardWanted } from '../src/screens/wizard';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  wizardWanted.set({ open: false, start: '' });
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
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
    el.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
}

const TEXT = 'This removes the PIN and deletes all saved passwords and tokens (Telegram, e-mail). The automatic shutdown becomes inactive. Your settings and your history stay.';

describe('Forgot PIN (VLT-IS-009 OD-40)', () => {
  it('hides the link when not in portable mode', async () => {
    await show(<Login />);
    expect(root.textContent).not.toContain('Forgot PIN');
  });

  it('shows the exact text and starts the wizard at the PIN step', async () => {
    expect(t('login.forgot.text')).toBe(TEXT);
    const onForgot = vi.fn();
    await show(<Login portable onForgot={onForgot} />);
    expect(root.textContent).toContain('Forgot PIN');
    await click($('button.link'));
    expect(root.textContent).toContain(TEXT);
    expect(root.textContent).toContain('Type RESET to confirm.');
    await type('#forgot-reset', 'reset');
    await submit('#forgot-form');
    expect(fake.calls.filter((c) => c.method === 'ForgotPIN')).toEqual([]);
    expect(onForgot).not.toHaveBeenCalled();

    await type('#forgot-reset', 'RESET');
    await submit('#forgot-form');
    expect(fake.calls).toContainEqual({ method: 'ForgotPIN', args: ['RESET'] });
    expect(onForgot).toHaveBeenCalledTimes(1);
  });

  it('sends RESET when the field is filled without an input event', async () => {
    const onForgot = vi.fn();
    await show(<Login portable onForgot={onForgot} />);
    await click($('button.link'));
    const input = $('#forgot-reset') as HTMLInputElement;
    input.value = 'RESET';
    await submit('#forgot-form');
    expect(fake.calls).toContainEqual({ method: 'ForgotPIN', args: ['RESET'] });
    expect(onForgot).toHaveBeenCalled();
  });

  it('startWizard("pin") opens the wizard at the PIN step', async () => {
    startWizard('pin');
    expect(wizardWanted.get()).toEqual({ open: true, start: 'pin' });
    app.set({ auth: { pin_set: false, elevated: true } as never, conn: { state: 'connected', since: '' } });
    await show(<Wizard first={false} portable startAt="pin" />);
    expect(root.textContent).toContain(t('wizard.step.pin'));
    expect(root.textContent).toContain(t('pin.set.title'));
  });
});
