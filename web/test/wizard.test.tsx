// SPDX-License-Identifier: AGPL-3.0-or-later
// First-run wizard (VLT-IS-007 TASK 03-00-00-008).
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { General } from '../src/screens/general';
import { FirstStartPrompt, STEPS, Wizard, wizardWanted } from '../src/screens/wizard';
import { app } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  wizardWanted.set({ open: false });
  fake.handlers.DevicePorts = () => ({ ports: [{ name: 'COM3', is_usb: true }] });
  fake.handlers.DeviceStatus = () => ({ state: 'connected', since: '', config: { transport: 'serial', serial_port: 'COM3', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] } });
  fake.handlers.Policy = () => ({
    settings: {
      action: 'shutdown',
      'trigger.on_battery_seconds': 300,
      'trigger.runtime_below_minutes': 5,
      'trigger.charge_below_percent': 30,
      'trigger.low_battery_flag': true,
      'trigger.comm_lost_on_battery_seconds': 180,
      countdown_seconds: 60,
      cancel_when_mains_returns: true,
      force_close_applications: true,
      'ups_off.enabled': false,
      'ups_off.delay_minutes': 2,
      'ups_off.restore_minutes': 1,
    },
    version: 'abc',
    active: false,
    fallback: false,
  });
  fake.handlers.Simulate = () => ({ version: 'abc', passed: true, time: '2026-11-13T10:00:00Z', scenarios: [] });
  fake.handlers.ActivatePolicy = () => undefined;
  fake.handlers.SetPIN = () => undefined;
  fake.handlers.AuthStatus = () => ({ pin_set: true, elevated: true });
  fake.handlers.Channels = () => ({ channels: [], email_presets: [] });
  fake.handlers.TelegramChats = () => ({ chats: [] });
  app.set({
    auth: { pin_set: false, elevated: false },
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

async function show(el: preact.JSX.Element) {
  await act(async () => render(el, root));
  await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
}
function byText(text: string): HTMLButtonElement {
  const b = [...root.querySelectorAll('button')].find((x) => x.textContent?.trim() === text);
  if (!b) throw new Error('no button ' + text + ' in ' + (root.textContent ?? '').slice(0, 200));
  return b;
}
async function click(el: HTMLElement) {
  await act(async () => {
    el.click();
    await new Promise((r) => setTimeout(r, 0));
  });
}

describe('first-run wizard', () => {
  it('starts at the PIN step, with Skip and Back disabled', async () => {
    await show(<Wizard first />);
    expect(root.textContent).toContain(t('pin.set.title'));
    expect(root.textContent).toContain(t('wizard.progress', { n: 1, total: STEPS.length, title: t('wizard.step.pin') }));
    expect(byText(t('wizard.skip')).disabled).toBe(true);
    expect(byText(t('wizard.back')).disabled).toBe(true);
    const dlg = root.querySelector('.wizard-dialog') as HTMLElement;
    expect(dlg).toBeTruthy();
    expect(getComputedStyle(dlg).width === '760px' || dlg.classList.contains('wizard-dialog')).toBe(true);
    const steps = [...root.querySelectorAll('.wizard-steps li')].map((li) => li.textContent);
    expect(steps).toEqual(STEPS.map((s) => t('wizard.step.' + s)));
    expect(root.querySelector('.wizard-steps li[aria-current="step"]')?.textContent).toBe(t('wizard.step.pin'));
    expect(root.querySelector('.wizard-footer')).toBeTruthy();
  });

  it('makes the PIN optional in portable mode, with Skip and the exact help text', async () => {
    await show(<Wizard first portable />);
    expect(root.textContent).toContain(t('wizard.pin.help'));
    expect(t('wizard.pin.help')).toBe('A PIN prevents other people at this computer from changing the shutdown settings.');
    expect(byText(t('wizard.skip')).disabled).toBe(false);
    expect(byText(t('wizard.back')).disabled).toBe(true);
    await click(byText(t('wizard.skip')));
    expect(root.textContent).toContain(t('wizard.step.port'));
  });

  it('Skip and Back move between the steps after the PIN', async () => {
    app.set({ auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' } });
    await show(<Wizard first={false} />);
    expect(root.textContent).toContain(t('wizard.step.port'));
    await click(byText(t('wizard.skip')));
    expect(root.textContent).toContain(t('wizard.step.values'));
    await click(byText(t('wizard.back')));
    expect(root.textContent).toContain(t('wizard.step.port'));
  });

  it('the modem question uses the required text, and No shows the warning', async () => {
    setLang('en');
    expect(t('wizard.modem.question')).toBe('Are the modem and the router connected to the UPS?');
    expect(t('wizard.modem.warn')).toBe('During a power failure, the internet stops. Telegram and e-mail cannot send messages.');
    app.set({ auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' } });
    await show(<Wizard first={false} />);
    // skip to modem: port, values, policy, telegram, email, modem = 5 skips
    for (const title of ['port', 'values', 'policy', 'telegram', 'email']) {
      expect(root.textContent).toContain(t('wizard.step.' + title));
      await click(byText(t('wizard.skip')));
    }
    expect(root.textContent).toContain(t('wizard.modem.question'));
    expect(root.textContent).not.toContain(t('wizard.modem.warn'));
    const no = [...root.querySelectorAll('label')].find((l) => l.textContent?.includes(t('wizard.modem.no')));
    expect(no).toBeTruthy();
    await act(async () => {
      no!.querySelector('input')!.click();
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(root.textContent).toContain(t('wizard.modem.warn'));
    await click(byText(t('wizard.next')));
    expect(root.textContent).toContain(t('wizard.step.summary'));
    expect(root.textContent).toContain(t('wizard.modem.no'));
    expect(root.textContent).toContain(t('wizard.modem.warn'));
  });

  it('the General screen starts the wizard', async () => {
    app.set({ auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' } });
    fake.handlers.About = () => ({ version: 'test', vltd_version: '', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: '' });
    await show(<General />);
    expect(root.textContent).toContain(t('wizard.general.start'));
    await click(byText(t('wizard.general.start')));
    expect(wizardWanted.get().open).toBe(true);
  });

  it('the Shutdown Policy step shows the exact error when the user cannot shut down', async () => {
    expect(t('sim.os_shutdown.error')).toBe('This user cannot shut down the computer.');
    const base = fake.handlers.Policy as () => object;
    const pol = base();
    fake.handlers.Policy = () => ({ ...pol, os_shutdown: { name: 'OS shutdown', pass: false, error: 'This user cannot shut down the computer.' } });
    app.set({ auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' } });
    await show(<Wizard first={false} />);
    await click(byText(t('wizard.skip'))); // port
    await click(byText(t('wizard.skip'))); // values
    expect(root.textContent).toContain(t('wizard.step.policy'));
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
    expect(root.textContent).toContain('This user cannot shut down the computer.');
    expect(root.querySelector('[role="alert"]')?.textContent).toBe(t('sim.os_shutdown.error'));
  });

    it('is a 760 by 560 dialog with the navigation at the bottom right', async () => {
    await show(<Wizard first portable />);
    const dlg = root.querySelector('.wizard-dialog') as HTMLElement;
    expect(dlg).toBeTruthy();
    const footer = root.querySelector('.wizard-footer') as HTMLElement;
    expect(footer).toBeTruthy();
    expect(footer.compareDocumentPosition(root.querySelector('.wizard-body')!) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy();
    const labels = [...footer.querySelectorAll('button')].map((b) => b.textContent?.trim());
    expect(labels).toEqual([t('wizard.back'), t('wizard.skip'), t('wizard.next')]);
  });
});

describe('first-start dialog (OD-41)', () => {
  it('has the exact English text', () => {
    expect(t('wizard.prompt.title')).toBe('Start the setup wizard?');
    expect(t('wizard.prompt.start')).toBe('Start');
    expect(t('wizard.prompt.later')).toBe('Later');
  });

  it('Start opens the wizard and Later calls the Later handler', async () => {
    const onStart = vi.fn();
    const onLater = vi.fn();
    await show(<FirstStartPrompt onStart={onStart} onLater={onLater} />);
    expect(root.textContent).toContain('Start the setup wizard?');
    await click(byText('Start'));
    expect(onStart).toHaveBeenCalledTimes(1);
    await click(byText('Later'));
    expect(onLater).toHaveBeenCalledTimes(1);
    expect(root.querySelector('.prompt-dialog')).toBeTruthy();
  });
});

describe('compact CSS (VLT-IS-009 003)', () => {
  it('sets the base sizes as CSS variables', () => {
    const css = readFileSync(join(__dirname, '../src/styles.css'), 'utf8');
    expect(css).toContain('--font-size: 14px');
    expect(css).toContain('--title-size: 20px');
    expect(css).toContain('--section-size: 16px');
    expect(css).toContain('--control-height: 32px');
    expect(css).toContain('--wizard-w: 760px');
    expect(css).toContain('--wizard-h: 560px');
  });
});
