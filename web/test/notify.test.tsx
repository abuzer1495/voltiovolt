// SPDX-License-Identifier: AGPL-3.0-or-later
// Notifications screen (VLT-IS-007 TASK 03-00-00-007).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChannelsList } from '../src/api';
import { setLang, t } from '../src/i18n';
import { Notifications } from '../src/screens/notifications';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

const LIST: ChannelsList = {
  channels: [
    { name: 'desktop', enabled: true, min_severity: 'WARNING', chats: 0 },
    { name: 'telegram', enabled: false, min_severity: 'WARNING', secret: { id: 'telegram.token', set: false }, chats: 0 },
    {
      name: 'email',
      enabled: false,
      min_severity: 'WARNING',
      secret: { id: 'smtp.password', set: false },
      settings: { server: '', port: 465, security: 'tls', user: '', from: '', to: [] },
      chats: 0,
    },
  ],
  email_presets: [{ name: 'gmail', server: 'smtp.gmail.com', port: 465, security: 'tls' }],
};

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  fake.handlers.Channels = () => structuredClone(LIST);
  fake.handlers.TelegramChats = () => ({ chats: [] });
  app.set({ auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' } });
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
});

async function show(ticks = 2) {
  await act(async () => render(<Notifications />, root));
  for (let i = 0; i < ticks; i++) {
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  }
}
function $(sel: string): HTMLElement {
  const e = root.querySelector(sel);
  if (!e) throw new Error('no ' + sel + ' in ' + root.innerHTML.slice(0, 200));
  return e as HTMLElement;
}
function byText(text: string): HTMLButtonElement {
  const b = [...root.querySelectorAll('button')].find((x) => x.textContent?.trim() === text);
  if (!b) throw new Error('no button ' + text);
  return b;
}
async function click(el: HTMLElement) {
  await act(async () => {
    el.click();
    await new Promise((r) => setTimeout(r, 0));
  });
}
async function type(sel: string, v: string) {
  const e = $(sel) as HTMLInputElement;
  await act(async () => {
    e.value = v;
    e.dispatchEvent(new Event('input', { bubbles: true }));
  });
}
async function submit(sel: string) {
  await act(async () => {
    $(sel).dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await new Promise((r) => setTimeout(r, 0));
  });
}

describe('Notifications screen', () => {
  it('shows one section for each Channel', async () => {
    await show();
    expect(root.textContent).toContain(t('notify.channel.desktop'));
    expect(root.textContent).toContain(t('notify.channel.telegram'));
    expect(root.textContent).toContain(t('notify.channel.email'));
    expect(root.querySelectorAll('section').length).toBe(3);
  });

  it('disables changes when locked', async () => {
    app.set({ auth: { pin_set: true, elevated: false } as never, conn: { state: 'connected', since: '' } });
    await show();
    expect(root.textContent).toContain(t('locked.need'));
    const buttons = [...root.querySelectorAll('button')] as HTMLButtonElement[];
    expect(buttons.length).toBeGreaterThan(0);
    expect(buttons.every((b) => b.disabled)).toBe(true);
    const inputs = [...root.querySelectorAll('input, select, textarea')] as HTMLInputElement[];
    expect(inputs.every((i) => i.disabled)).toBe(true);
  });

  it('sends a test message', async () => {
    fake.handlers.ChannelTest = () => ({ ok: true });
    await show();
    await click(byText(t('notify.test')));
    expect(fake.calls.some((c) => c.method === 'ChannelTest' && c.args[0] === 'desktop')).toBe(true);
    expect(root.textContent).toContain(t('notify.test.sent'));
  });

  it('the Secret field is write-only: type password, cleared after save, then only the hint', async () => {
    let set = false;
    fake.handlers.SetSecret = (id: unknown, _b64: unknown) => {
      expect(id).toBe('telegram.token');
      set = true;
    };
    fake.handlers.Channels = () => {
      const l = structuredClone(LIST);
      if (set) l.channels[1].secret = { id: 'telegram.token', set: true, hint: 'Ab3x' };
      return l;
    };
    await show();
    const input = $('#secret-telegram-token') as HTMLInputElement;
    expect(input.type).toBe('password');
    await type('#secret-telegram-token', '1234567890:AAAAAAAAAAAAAAAAAAAA');
    expect(input.value.length).toBeGreaterThan(0);
    // the nearest form
    await submit('#secret-telegram-token');
    // after save the field is empty and the hint is shown (not the value)
    const after = $('#secret-telegram-token') as HTMLInputElement;
    expect(after.value).toBe('');
    expect(root.textContent).toContain('Ab3x');
    expect(root.textContent).not.toContain('1234567890:AAAAAAAAAAAAAAAAAAAA');
    expect(root.textContent).toContain(t('notify.secret.hint', { hint: 'Ab3x' }).slice(0, 20));
  });

  it('the Gmail preset fills the Settings and shows the help text', async () => {
    await show();
    expect(root.textContent).not.toContain(t('notify.email.gmail.help'));
    await click(byText(t('notify.email.preset.gmail')));
    expect(($('#email-server') as HTMLInputElement).value).toBe('smtp.gmail.com');
    expect(($('#email-port') as HTMLInputElement).value).toBe('465');
    expect(($('#email-security') as HTMLSelectElement).value).toBe('tls');
    expect(root.textContent).toContain(t('notify.email.gmail.help'));
    expect(t('notify.email.gmail.help')).toBe(
      'Use a Google app password, not your normal password. Google account, Security, 2-Step Verification, App passwords.',
    );
  });

  it('saves the e-mail Settings', async () => {
    fake.handlers.SetChannel = () => ({ ok: true });
    await show();
    await type('#email-server', 'smtp.example.com');
    await type('#email-from', 'vlt@example.com');
    const to = $('#email-to') as HTMLTextAreaElement;
    await act(async () => {
      to.value = 'a@example.com\nb@example.com';
      to.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await submit('#email-server');
    const call = fake.calls.find((c) => c.method === 'SetChannel' && c.args[0] === 'email');
    expect(call).toBeTruthy();
    const settings = call!.args[3] as Record<string, unknown>;
    expect(settings.server).toBe('smtp.example.com');
    expect(settings.from).toBe('vlt@example.com');
    expect(settings.to).toEqual(['a@example.com', 'b@example.com']);
  });

  it('shows the pairing code and the steps, and removes a chat', async () => {
    fake.handlers.TelegramPairStart = () => ({ code: '654321', expires: new Date(Date.now() + 10 * 60 * 1000).toISOString() });
    fake.handlers.TelegramChats = () => ({ chats: [{ chat_id: 100, name: 'Ayşe', paired_at: '2026-11-10T10:00:00Z' }] });
    fake.handlers.TelegramChatRemove = () => ({ ok: true });
    await show();
    expect(root.textContent).toContain(t('notify.telegram.steps'));
    expect(root.textContent).toContain('Ayşe');
    await click(byText(t('notify.telegram.pair.button')));
    expect(root.textContent).toContain('654321');
    expect(root.textContent).toContain('/start 654321');
    await click(byText(t('notify.telegram.remove')));
    expect(fake.calls.some((c) => c.method === 'TelegramChatRemove' && c.args[0] === 100)).toBe(true);
    expect(root.textContent).toContain(t('notify.telegram.removed'));
  });
});
