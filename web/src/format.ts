// SPDX-License-Identifier: AGPL-3.0-or-later
import type { Point } from './api';
import { lang } from './i18n';

/** DASH is the text of a value with no data (not zero). */
export const DASH = '—';

export function num(v: number | undefined | null, digits = 1): string {
  if (v === undefined || v === null || Number.isNaN(v)) return DASH;
  return v.toLocaleString(lang() === 'tr' ? 'tr-TR' : 'en-GB', { maximumFractionDigits: digits, minimumFractionDigits: 0 });
}

/** value formats a Measurement with its unit, or DASH if there is no data. */
export function value(p: Point | undefined, digits = 1, unit?: string): string {
  if (!p) return DASH;
  const n = num(p.value, digits);
  return n === DASH ? DASH : `${n} ${unit ?? p.unit}`;
}

export function time(ts: string | undefined): string {
  if (!ts) return DASH;
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return DASH;
  return d.toLocaleString(lang() === 'tr' ? 'tr-TR' : 'en-GB');
}
