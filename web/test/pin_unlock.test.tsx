// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-009 TASK 03-00-00-001 step 1.3: the value that the PIN field sends.
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Login, SetPin } from '../src/screens/pin';
import { setLang } from '../src/i18n';
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

describe('PIN field value (VLT-IS-009 R-02)', () => {
  it('sends the exact Set PIN value, including a leading zero', async () => {
    await show(<SetPin />);
    const input = $('#set-pin') as HTMLInputElement;
    expect(input.type).toBe('password');
    expect(input.inputMode || '').not.toBe('numeric');
    await type('#set-pin', '08174629');
    await type('#set-again', '08174629');
    await submit();
    expect(fake.calls).toContainEqual({ method: 'SetPIN', args: ['08174629'] });
  });

  it('sends the exact Unlock value, including a leading zero', async () => {
    await show(<Login />);
    const input = $('#login-pin') as HTMLInputElement;
    expect(input.type).toBe('password');
    expect(input.inputMode || '').not.toBe('numeric');
    await type('#login-pin', '08174629');
    await submit();
    expect(fake.calls).toContainEqual({ method: 'Login', args: ['08174629'] });
  });

  it('sends a letters PIN without a change of case', async () => {
    await show(<SetPin />);
    await type('#set-pin', 'AbCdEf7x');
    await type('#set-again', 'AbCdEf7x');
    await submit();
    expect(fake.calls).toContainEqual({ method: 'SetPIN', args: ['AbCdEf7x'] });
    fake.calls.length = 0;
    await show(<Login />);
    await type('#login-pin', 'AbCdEf7x');
    await submit();
    expect(fake.calls).toContainEqual({ method: 'Login', args: ['AbCdEf7x'] });
  });

  it('does not trim spaces: a trailing space is in the value, and Set PIN refuses it', async () => {
    await show(<SetPin />);
    await type('#set-pin', 'k7Pq2Lm9 ');
    await type('#set-again', 'k7Pq2Lm9 ');
    await submit();
    expect(fake.calls.filter((c) => c.method === 'SetPIN')).toEqual([]);
    await show(<Login />);
    await type('#login-pin', 'k7Pq2Lm9 ');
    await submit();
    expect(fake.calls).toContainEqual({ method: 'Login', args: ['k7Pq2Lm9 '] });
  });

  it('Unlock sends the current DOM value even if no input event was fired (autofill, IME)', async () => {
    const seen: unknown[] = [];
    fake.handlers.Login = (pin: unknown) => {
      seen.push(pin);
    };
    await show(<Login />);
    const e = $('#login-pin') as HTMLInputElement;
    // WebView2 / a password manager can write the field without an
    // 'input' event. Preact state then stays empty.
    e.value = 'k7Pq2Lm9';
    await submit();
    expect(seen).toEqual(['k7Pq2Lm9']);
  });

  it('Set PIN sends the current DOM value even if no input event was fired', async () => {
    const seen: unknown[] = [];
    fake.handlers.SetPIN = (pin: unknown) => {
      seen.push(pin);
    };
    await show(<SetPin />);
    ($('#set-pin') as HTMLInputElement).value = 'k7Pq2Lm9';
    ($('#set-again') as HTMLInputElement).value = 'k7Pq2Lm9';
    await submit();
    expect(seen).toEqual(['k7Pq2Lm9']);
  });
});
