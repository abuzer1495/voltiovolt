// SPDX-License-Identifier: AGPL-3.0-or-later
// The Settings of the Shutdown Policy with the default values and the
// permitted ranges (VLT-IS-004 TASK 03-00-00-006 step 1, internal/store).
// WARNING: vltd checks the ranges again. This list must be the same.
import type { PolicyConfig, PolicyInfo } from './api';

export type FieldKey = keyof PolicyConfig;

export interface Field {
  key: FieldKey;
  kind: 'int' | 'bool' | 'action';
  def: number | boolean | string;
  min?: number;
  max?: number;
  unit?: string;
}

export const FIELDS: Field[] = [
  { key: 'action', kind: 'action', def: 'shutdown' },
  { key: 'trigger.on_battery_seconds', kind: 'int', def: 300, min: 0, max: 86400, unit: 's' },
  { key: 'trigger.runtime_below_minutes', kind: 'int', def: 5, min: 0, max: 600, unit: 'min' },
  { key: 'trigger.charge_below_percent', kind: 'int', def: 30, min: 0, max: 100, unit: '%' },
  { key: 'trigger.low_battery_flag', kind: 'bool', def: true },
  { key: 'trigger.comm_lost_on_battery_seconds', kind: 'int', def: 180, min: 0, max: 86400, unit: 's' },
  { key: 'countdown_seconds', kind: 'int', def: 60, min: 10, max: 600, unit: 's' },
  { key: 'cancel_when_mains_returns', kind: 'bool', def: true },
  { key: 'force_close_applications', kind: 'bool', def: true },
  // VLT-IS-006 TASK 03-00-00-006: the UPS output off step.
  { key: 'ups_off.enabled', kind: 'bool', def: false },
  { key: 'ups_off.delay_minutes', kind: 'int', def: 2, min: 1, max: 10, unit: 'min' },
  { key: 'ups_off.restore_minutes', kind: 'int', def: 1, min: 1, max: 9999, unit: 'min' },
];

/** checkField returns an i18n key of the problem, or null. */
export function checkField(f: Field, raw: string | boolean): string | null {
  if (f.kind === 'bool') return typeof raw === 'boolean' ? null : 'policy.err.bool';
  if (f.kind === 'action') return raw === 'shutdown' || raw === 'hibernate' ? null : 'policy.err.action';
  const s = String(raw).trim();
  if (!/^\d+$/.test(s)) return 'policy.err.int';
  const n = Number(s);
  if (n < (f.min ?? 0) || n > (f.max ?? Number.MAX_SAFE_INTEGER)) return 'policy.err.range';
  return null;
}

/** toValues checks all fields and gives the values for policy.set, or the problems. */
export function toValues(form: Record<string, string | boolean>): { values?: Record<string, unknown>; problems: Record<string, string> } {
  const problems: Record<string, string> = {};
  const values: Record<string, unknown> = {};
  for (const f of FIELDS) {
    const raw = form[f.key];
    const p = checkField(f, raw);
    if (p) problems[f.key] = p;
    else values[f.key] = f.kind === 'int' ? Number(String(raw).trim()) : raw;
  }
  return Object.keys(problems).length ? { problems } : { values, problems };
}

/**
 * canActivate is the activation rule (TASK 03-00-00-006 step 5): elevated,
 * not the Fallback Policy, not active, and a passed Simulation of the
 * current version.
 */
export function canActivate(p: PolicyInfo | null, elevated: boolean): boolean {
  if (!p || !elevated || p.fallback || p.active) return false;
  const s = p.last_simulation;
  return !!s && s.passed && s.version === p.version;
}
