// SPDX-License-Identifier: AGPL-3.0-or-later
// Typed calls of internal/gui.API (Wails bindings) and the Wails events.
import { call, on } from './bridge';

export const events = {
  conn: 'vlt:conn',
  auth: 'vlt:auth',
  status: 'vlt:status',
  event: 'vlt:event',
  countdown: 'vlt:countdown',
  tray: 'vlt:tray',
} as const;

export interface ConnInfo {
  state: 'connecting' | 'connected' | 'disconnected';
  reason?: string;
  vltd_version?: string;
  protocol?: number;
  since: string;
  next_try?: string;
}

export interface AboutInfo {
  version: string;
  vltd_version: string;
  protocol: number;
  gui_protocol: number;
  license: string;
  source: string;
}

export interface VaultState {
  state: string;
  reason?: string;
  key_mode?: string;
}

export interface SystemStatus {
  vault: VaultState;
  store: { state: string; schema_version: number };
  uptime_seconds: number;
}

export interface AuthInfo {
  pin_set: boolean;
  elevated: boolean;
  block_until?: string;
}

export interface Point {
  name: string;
  unit: string;
  value: number;
  source: string;
}

export interface Countdown {
  active: boolean;
  trigger?: string;
  started?: string;
  ends?: string;
  remaining_seconds: number;
  action?: string;
  cancelable: boolean;
  requested: boolean;
}

export interface StatusPush {
  ts: string;
  device: { state: string };
  ups: { on_battery: boolean; low_battery: boolean; comm_lost: boolean; active: string[] };
  measurements: Record<string, Point>;
  countdown: Countdown;
  policy_active: boolean;
}

export interface VltEvent {
  id?: number;
  ts: string;
  type: string;
  severity: 'INFO' | 'WARNING' | 'CRITICAL';
  data: Record<string, unknown>;
}

export interface EventsPage {
  events: VltEvent[];
  next: number;
}

export interface PolicyConfig {
  action: string;
  'trigger.on_battery_seconds': number;
  'trigger.runtime_below_minutes': number;
  'trigger.charge_below_percent': number;
  'trigger.low_battery_flag': boolean;
  'trigger.comm_lost_on_battery_seconds': number;
  countdown_seconds: number;
  cancel_when_mains_returns: boolean;
  force_close_applications: boolean;
  'ups_off.enabled': boolean;
  'ups_off.delay_minutes': number;
  'ups_off.restore_minutes': number;
}

/** UPSTestResult is the result of the last battery test (VLT-IS-006). */
export interface UPSTestResult {
  result: 'TEST_PASSED' | 'TEST_FAILED';
  start: string;
  end: string;
  ups_failed: boolean;
  battery_low: boolean;
  cancelled: boolean;
}

/** UPSCaps is the ups.control.capabilities result. */
export interface UPSCaps {
  connected: boolean;
  functions: string[];
  last_test?: UPSTestResult;
}

/** UPSControlResult is the result of one UPS Control request. */
export interface UPSControlResult {
  function: string;
  bytes: string;
  sends: number;
  verified: boolean;
  no_send?: boolean;
}

export interface SimStep {
  t: number;
  kind: string;
  detail?: string;
}

export interface ScenarioResult {
  name: string;
  steps: SimStep[];
  outcome: string;
  expected?: string;
  deadline?: number;
  at?: number;
  pass: boolean;
}

export interface SimResult {
  version: string;
  passed: boolean;
  time: string;
  fallback?: boolean;
  scenarios: ScenarioResult[];
}

export interface PolicyInfo {
  settings: PolicyConfig;
  version: string;
  active: boolean;
  fallback: boolean;
  last_simulation?: { id: number; time: string; version: string; passed: boolean; result: SimResult } | null;
}

export interface TrayTexts {
  mains: string;
  battery: string;
  critical: string;
  offline: string;
  runtime: string;
  open: string;
  lock: string;
  quit: string;
}

/** Line is the wanted state of DTR or RTS (VLT-IS-006 TASK 03-00-00-004). */
export type Line = 'unchanged' | 'on' | 'off';

/** SerialPort is one serial port of the OS (device.ports). */
export interface SerialPort {
  name: string;
  is_usb: boolean;
  vid?: string;
  pid?: string;
  product?: string;
}

/** ScanResult is the device.scan result of one marked port. */
export interface ScanResult {
  name: string;
  result: 'response' | 'no response' | 'busy' | 'error';
  valid_q1: boolean;
}

export interface DeviceStatus {
  state: string;
  since: string;
  config: { transport: string; serial_port: string; serial_dtr: Line; serial_rts: Line; allowed_ports: string[] | null };
  profile?: {
    transport: string;
    port: string;
    dialect: string;
    supported: string[];
    nominal?: Record<string, unknown>;
    info?: Record<string, unknown>;
    model_data?: Record<string, unknown>;
    firmware?: Record<string, unknown>;
    serial_number?: string;
  };
}

export interface Row {
  ts: string;
  min: number;
  max: number;
  mean: number;
  count: number;
  source?: string;
}

export interface Series {
  name: string;
  unit: string;
  step: string;
  rows: Row[];
}

export interface AuditRow {
  seq: number;
  ts: string;
  actor: string;
  action: string;
  target: string;
  result: string;
  details: unknown;
}

export interface Setting {
  key: string;
  value: unknown;
  version: number;
  default: boolean;
}

/** errorCode returns the error code of a rejected call. */
export function errorCode(e: unknown): string {
  const m = (e && typeof e === 'object' && 'message' in e ? String((e as { message: unknown }).message) : String(e)).trim();
  return /^[a-z_]+$/.test(m) ? m : 'ipc_error';
}

/** SecretHint is the write-only Secret status (never the value). */
export interface SecretHint {
  id: string;
  set: boolean;
  hint?: string;
  updated_at?: string;
}

/** ChannelInfo is one Notification Channel (channels.list). */
export interface ChannelInfo {
  name: string;
  enabled: boolean;
  min_severity: string;
  secret?: SecretHint;
  settings?: Record<string, unknown>;
  pairing?: { code?: string; expires: string };
  chats: number;
}

/** EmailPreset is a set of SMTP server Settings (the preset "Gmail"). */
export interface EmailPreset {
  name: string;
  server: string;
  port: number;
  security: string;
}

/** ChannelsList is the result of channels.list. */
export interface ChannelsList {
  channels: ChannelInfo[];
  email_presets: EmailPreset[];
}

/** TelegramChat is one paired Telegram chat. */
export interface TelegramChat {
  chat_id: number;
  name: string;
  paired_at: string;
}

export const api = {
  connection: () => call<ConnInfo>('Connection'),
  about: () => call<AboutInfo>('About'),
  last: () => call<Record<string, unknown>>('Last'),
  systemStatus: () => call<SystemStatus>('SystemStatus'),
  authStatus: () => call<AuthInfo>('AuthStatus'),
  setPIN: (pin: string) => call<void>('SetPIN', pin),
  login: (pin: string) => call<void>('Login', pin),
  logout: () => call<void>('Logout'),
  events: (before: number, limit: number) => call<EventsPage>('Events', before, limit),
  policy: () => call<PolicyInfo>('Policy'),
  shutdownStatus: () => call<Countdown>('ShutdownStatus'),
  cancelShutdown: () => call<void>('CancelShutdown'),
  setTrayTexts: (tx: TrayTexts) => call<void>('SetTrayTexts', tx),
  deviceStatus: () => call<DeviceStatus>('DeviceStatus'),
  setDeviceSettings: (v: Record<string, unknown>) => call<void>('SetDeviceSettings', v),
  devicePorts: () => call<{ ports: SerialPort[] }>('DevicePorts'),
  /** deviceScan sends only "Q1" CR to each marked port (elevated). */
  deviceScan: (ports: string[], confirmation: string) => call<{ results: ScanResult[] }>('DeviceScan', ports, confirmation),
  // UPS Control (VLT-IS-006 TASK 03-00-00-008). There is no function for
  // the shutdown commands (SEC-098).
  upsCaps: () => call<UPSCaps>('UPSControlCapabilities'),
  upsTestStart: (type: '10s' | 'minutes' | 'until_low', minutes: number, confirmation: string) =>
    call<UPSControlResult>('UPSTestStart', type, minutes, confirmation),
  upsTestCancel: () => call<UPSControlResult>('UPSTestCancel'),
  upsBeeperSet: (on: boolean) => call<UPSControlResult>('UPSBeeperSet', on),
  setPolicy: (v: Record<string, unknown>) => call<PolicyInfo>('SetPolicy', v),
  simulate: () => call<SimResult>('Simulate'),
  activatePolicy: () => call<void>('ActivatePolicy'),
  deactivatePolicy: () => call<void>('DeactivatePolicy'),
  resetVault: (confirmation: string) => call<void>('ResetVault', confirmation),
  audit: (after: number, limit: number) => call<{ rows: AuditRow[]; next: number }>('Audit', after, limit),
  settings: () => call<{ settings: Setting[] }>('Settings'),
  setLanguage: (l: string) => call<void>('SetLanguage', l),
  openSource: () => call<void>('OpenSource'),
  // Notification Channels (VLT-IS-007 TASK 03-00-00-007).
  channels: () => call<ChannelsList>('Channels'),
  setChannel: (channel: string, enabled: boolean, minSeverity: string, settings: Record<string, unknown>) =>
    call<void>('SetChannel', channel, enabled, minSeverity, settings),
  channelTest: (channel: string) => call<void>('ChannelTest', channel),
  telegramPairStart: () => call<{ code: string; expires: string }>('TelegramPairStart'),
  telegramChats: () => call<{ chats: TelegramChat[] }>('TelegramChats'),
  telegramChatRemove: (chatID: number) => call<void>('TelegramChatRemove', chatID),
  /**
   * setSecret sends the text of an input field to the Go side (SEC-096).
   * The Go side sends it to vltd with secret.set and clears its buffer.
   * No method gives a Secret back; clear the input field after the call.
   */
  setSecret: (id: string, text: string) => {
    const b = new TextEncoder().encode(text);
    let bin = '';
    for (const c of b) bin += String.fromCharCode(c);
    b.fill(0);
    return call<void>('SetSecret', id, btoa(bin));
  },
  secretStatus: (id: string) => call<{ id: string; set: boolean; hint?: string; updated_at?: string }>('SecretStatus', id),
  measurements: (name: string, fromMs: number, toMs: number, step: string) => call<Series>('Measurements', name, fromMs, toMs, step),
};

export { on };
