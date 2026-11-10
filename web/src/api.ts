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

export interface DeviceStatus {
  state: string;
  since: string;
  config: { transport: string; serial_port: string; serial_dtr: boolean; serial_rts: boolean };
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

/** errorCode returns the error code of a rejected call. */
export function errorCode(e: unknown): string {
  const m = (e && typeof e === 'object' && 'message' in e ? String((e as { message: unknown }).message) : String(e)).trim();
  return /^[a-z_]+$/.test(m) ? m : 'ipc_error';
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
  setPolicy: (v: Record<string, unknown>) => call<PolicyInfo>('SetPolicy', v),
  simulate: () => call<SimResult>('Simulate'),
  activatePolicy: () => call<void>('ActivatePolicy'),
  deactivatePolicy: () => call<void>('DeactivatePolicy'),
  measurements: (name: string, fromMs: number, toMs: number, step: string) => call<Series>('Measurements', name, fromMs, toMs, step),
};

export { on };
