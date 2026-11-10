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
};

export { on };
