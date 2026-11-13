// SPDX-License-Identifier: AGPL-3.0-or-later
// Field help, official links and exact Owner texts (VLT-IS-009 TASK 03-00-00-004, OD-42).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { HELP_HOSTS, HELP_LINKS, HelpLine, isOfficialHelpURL } from '../src/help';
import { Device } from '../src/screens/device';
import { General } from '../src/screens/general';
import { Notifications } from '../src/screens/notifications';
import { Policy } from '../src/screens/policy';
import { SetPin } from '../src/screens/pin';
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

describe('isOfficialHelpURL', () => {
  it('allows only official https hosts', () => {
    expect(isOfficialHelpURL(HELP_LINKS.telegram)).toBe(true);
    expect(isOfficialHelpURL(HELP_LINKS.gmail)).toBe(true);
    expect(isOfficialHelpURL('https://support.microsoft.com/windows')).toBe(true);
    expect(isOfficialHelpURL('http://core.telegram.org/bots/tutorial')).toBe(false);
    expect(isOfficialHelpURL('https://example.com/')).toBe(false);
    expect(isOfficialHelpURL('https://evil.core.telegram.org/')).toBe(false);
    expect(isOfficialHelpURL('https://core.telegram.org.evil.com/')).toBe(false);
    expect(isOfficialHelpURL('https://user:pass@core.telegram.org/')).toBe(false);
    expect(isOfficialHelpURL('javascript:alert(1)')).toBe(false);
    expect(isOfficialHelpURL('')).toBe(false);
  });
  it('uses the exact official URLs', () => {
    expect(HELP_LINKS.telegram).toBe('https://core.telegram.org/bots/tutorial');
    expect(HELP_LINKS.gmail).toBe('https://support.google.com/accounts/answer/185833');
    expect(HELP_HOSTS).toEqual(['core.telegram.org', 'support.google.com', 'support.microsoft.com']);
  });
});

describe('HelpLine', () => {
  it('shows the one-sentence help and opens the panel and the official link', async () => {
    fake.handlers.OpenHelp = () => undefined;
    await show(<HelpLine text="Hint sentence." more="Longer steps." href={HELP_LINKS.telegram} />);
    expect(root.textContent).toContain('Hint sentence.');
    expect(root.textContent).not.toContain('Longer steps.');
    const q = root.querySelector('.help-q') as HTMLButtonElement;
    expect(q).toBeTruthy();
    expect(q.getAttribute('aria-label')).toBe(t('help.more'));
    await act(async () => { q.click(); });
    expect(root.textContent).toContain('Longer steps.');
    expect(root.textContent).toContain(t('help.open'));
    const open = [...root.querySelectorAll('button')].find((b) => b.textContent === t('help.open'));
    expect(open).toBeTruthy();
    await act(async () => { open!.click(); });
    const call = fake.calls.find((c) => c.method === 'OpenHelp');
    expect(call?.args).toEqual([HELP_LINKS.telegram]);
  });
  it('hides the "?" when there is no more text and no link', async () => {
    await show(<HelpLine text="Just a hint." />);
    expect(root.querySelector('.help-q')).toBeNull();
  });
});

describe('exact Owner help texts (EN)', () => {
  it('matches the required sentences', () => {
    expect(t('help.dtr')).toBe('Keep the default (unchanged) unless the UPS does not answer.');
    expect(t('help.policy.trigger.on_battery_seconds.more')).toBe('300 = shut down after 5 minutes on battery.');
    expect(t('help.policy.countdown_seconds')).toBe('Time to save your work before the shutdown.');
    expect(t('help.policy.ups_off.example')).toBe(
      'Power fails, after 5 min the computer shuts down, 2 min later the UPS turns off, 1 min later it turns on again.',
    );
    expect(t('help.battery')).toBe(
      'Used only when the UPS does not report the charge. Keep the defaults unless you know the battery data.',
    );
    expect(t('help.dataDir')).toContain('Make a copy of this folder to keep your history.');
    expect(t('help.pin')).toBe(
      'A PIN prevents other people at this computer from changing the shutdown settings.',
    );
    expect(t('help.telegram.token.example')).toBe('123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ');
    expect(t('help.serial.example')).toBe('COM3');
    expect(t('help.serial.example.linux')).toBe('/dev/ttyUSB0');
    expect(t('help.email.server.example')).toBe('smtp.example.com');
    expect(t('help.email.user.example')).toBe('name@gmail.com');
  });
});

describe('screens show field help', () => {
  const POLICY = {
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
  };

  it('Device shows serial, DTR and battery help', async () => {
    fake.handlers.DeviceStatus = () => ({
      state: 'connected',
      since: '',
      config: { transport: 'serial', serial_port: '', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] },
    });
    fake.handlers.DevicePorts = () => ({ ports: [] });
    fake.handlers.Settings = () => ({ settings: [] });
    await show(<Device />);
    expect(root.textContent).toContain(t('help.serial'));
    expect(root.textContent).toContain(t('help.dtr'));
    expect(root.textContent).toContain(t('help.battery'));
    expect(root.textContent).toContain(t('help.battery.title'));
    const port = root.querySelector('#dev-port') as HTMLInputElement;
    expect(port?.placeholder === t('help.serial.example') || port?.placeholder === t('help.serial.example.linux')).toBe(true);
  });

  it('Policy shows trigger, countdown and UPS-off timeline help', async () => {
    fake.handlers.Policy = () => structuredClone(POLICY);
    fake.handlers.ShutdownStatus = () => ({ state: 'idle' });
    app.set({ policy: POLICY as never, auth: { pin_set: true, elevated: true } as never, conn: { state: 'connected', since: '' } });
    await show(<Policy />);
    const edit = [...root.querySelectorAll('button')].find((b) => b.textContent === t('policy.edit'));
    expect(edit).toBeTruthy();
    await act(async () => { edit!.click(); });
    expect(root.textContent).toContain(t('help.policy.trigger.on_battery_seconds'));
    expect(root.textContent).toContain(t('help.policy.countdown_seconds'));
    expect(root.textContent).toContain(t('help.policy.ups_off.example'));
    const buttons = [...root.querySelectorAll('.help-q')] as HTMLButtonElement[];
    expect(buttons.length).toBeGreaterThan(0);
    await act(async () => { buttons[0].click(); });
    expect(root.textContent).toContain(t('help.policy.trigger.on_battery_seconds.more'));
  });

  it('Notifications shows Telegram, e-mail and severity help and placeholders', async () => {
    fake.handlers.Channels = () => ({
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
    });
    fake.handlers.TelegramChats = () => ({ chats: [] });
    await show(<Notifications />, 4);
    expect(root.textContent).toContain(t('help.telegram.token'));
    expect(root.textContent).toContain(t('help.telegram.pair'));
    expect(root.textContent).toContain(t('help.severity'));
    expect(root.textContent).toContain(t('help.email.server'));
    const token = root.querySelector('#secret-telegram-token') as HTMLInputElement;
    expect(token?.placeholder).toBe(t('help.telegram.token.example'));
    const server = root.querySelector('#email-server') as HTMLInputElement;
    expect(server?.placeholder).toBe(t('help.email.server.example'));
  });

  it('General shows the data-folder copy sentence and SetPin shows the PIN help', async () => {
    fake.handlers.About = () => ({
      version: '1', vltd_version: '1', protocol: 1, gui_protocol: 1, license: 'AGPL', source: 'x', mode: 'portable', data_dir: '/tmp/vlt',
    });
    fake.handlers.StartAtLogon = () => false;
    await show(<General />);
    expect(root.textContent).toContain(t('help.dataDir'));
    expect(root.textContent).toContain('Make a copy of this folder to keep your history.');
    render(null, root);
    await show(<SetPin />);
    expect(root.textContent).toContain(t('help.pin'));
  });
});
