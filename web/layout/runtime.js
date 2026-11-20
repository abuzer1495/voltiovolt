// SPDX-License-Identifier: AGPL-3.0-or-later
// Fake Wails runtime for the layout test (VLT-IS-012 TASK 03-00-00-001
// step 2.4). It is served as /wails/runtime.js only by
// scripts/layout-check.mjs. It is not in the product.
const q = new URLSearchParams(location.search);
const MODE = q.get('mode') || 'expert';
const SCALE = q.get('scale') || 'auto';
const LANG = q.get('lang') || 'en';
const now = Date.now();
const m = (name, unit, value) => ({ name, unit, value, source: 'q1' });
const status = {
  ts: new Date(now).toISOString(),
  device: { state: 'connected', config: { transport: 'serial', serial_port: 'COM3' }, profile: { port: 'COM3' } },
  ups: { on_battery: false, low_battery: false, comm_lost: false, active: [] },
  measurements: {
    'input.voltage': m('input.voltage', 'V', 229.8), 'input.frequency': m('input.frequency', 'Hz', 50), 'output.voltage': m('output.voltage', 'V', 220.1),
    'output.frequency': m('output.frequency', 'Hz', 50), 'output.current': m('output.current', 'A', 1.2), 'load.percent': m('load.percent', '%', 31),
    'battery.voltage': m('battery.voltage', 'V', 27.1), 'battery.charge': m('battery.charge', '%', 100), 'battery.charge_estimate': m('battery.charge_estimate', '%', 98),
    'battery.runtime': m('battery.runtime', 'min', 42), 'ups.temperature': m('ups.temperature', '°C', 31),
  },
  countdown: { active: false, remaining_seconds: 0, cancelable: true, requested: false }, policy_active: true,
};
const pol = {
  settings: {
    action: 'shutdown', 'trigger.on_battery_seconds': 300, 'trigger.runtime_below_minutes': 5, 'trigger.charge_below_percent': 30,
    'trigger.low_battery_flag': true, 'trigger.comm_lost_on_battery_seconds': 180, countdown_seconds: 60, cancel_when_mains_returns: true,
    force_close_applications: true, 'ups_off.enabled': false, 'ups_off.delay_minutes': 2, 'ups_off.restore_minutes': 1,
  },
  version: 'a', active: false, fallback: false,
};
const ev = (i, type, sev, active) => ({ id: i, ts: new Date(now - i * 60000).toISOString(), type, severity: sev, data: { active } });
const events = [ev(1, 'ONLINE', 'INFO', true), ev(2, 'ON_BATTERY', 'WARNING', false), ev(3, 'ON_BATTERY', 'WARNING', true), ev(4, 'COMM_RESTORED', 'INFO', true)];
const series = (name, from, to) => {
  const rows = [];
  for (let x = from; x <= to; x += 60000) rows.push({ ts: new Date(x).toISOString(), min: 220, max: 232, mean: 225 + 5 * Math.sin(x / 6e5), count: 1 });
  return { name, unit: 'V', step: 'raw', rows };
};
// Three ports: the longest real USB description of the Owner tests (R-17).
const ports = [
  { name: 'COM1', is_usb: false },
  { name: 'COM3', is_usb: true, vid: '067b', pid: '2303', product: 'Prolific USB-to-Serial Comm Port' },
  { name: 'COM4', is_usb: true, vid: '1a86', pid: '7523', product: 'USB-SERIAL CH340' },
];
const H = {
  Connection: () => ({ state: 'connected', since: '' }),
  About: () => ({ version: 'layout', vltd_version: '', protocol: 1, gui_protocol: 1, license: 'AGPL-3.0-or-later', source: '', mode: 'portable', data_dir: 'C:\\Users\\user\\AppData\\Local\\Programs\\VoltioVolt\\data' }),
  AuthStatus: () => ({ pin_set: false, elevated: true }),
  Last: () => ({ status, countdown: status.countdown }),
  Settings: () => ({ settings: [{ key: 'ui.mode', value: MODE, version: 1, default: false }, { key: 'ui.scale', value: SCALE, version: 1, default: false }, { key: 'ui.language', value: LANG, version: 1, default: false }] }),
  Events: () => ({ events, next: 0 }), Policy: () => pol,
  WizardCompleted: () => q.get('first') !== '1', WizardLaterCount: () => 0,
  Measurements: (n, f, t) => series(n, Math.max(f, t - 3600e3), t),
  DeviceStatus: () => ({ state: 'connected', since: '', config: { transport: 'serial', serial_port: 'COM3', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] }, profile: { transport: 'serial', port: 'COM3', dialect: 'megatec', supported: ['Q1', 'F', 'I'] } }),
  DevicePorts: () => ({ ports }),
  ShutdownStatus: () => ({ active: false, remaining_seconds: 0, cancelable: true, requested: false }),
  SystemStatus: () => ({ vault: { state: 'ok', reason: '' }, store: { state: 'ok', schema_version: 3 } }),
  Channels: () => ({ channels: [], email_presets: [] }), TelegramChats: () => ({ chats: [] }),
  Audit: () => ({ rows: [], next: 0 }), UPSControlCapabilities: () => ({ connected: true, functions: ['test.10s', 'beeper'] }),
  StartAtLogon: () => false,
};
export const Call = { ByName: async (full, ...a) => { const n = full.split('.').pop(); const h = H[n]; return h ? h(...a) : undefined; } };
export const Events = { On: () => () => {} };
