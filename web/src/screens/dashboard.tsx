// SPDX-License-Identifier: AGPL-3.0-or-later
// Dashboard (TASK 03-00-00-004 step 1): mode, values, the last 10 Events.
import type { StatusPush, VltEvent } from '../api';
import { Badge } from '../components';
import { DASH, time, value } from '../format';
import { t } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';

export type Mode = 'mains' | 'battery' | 'bypass' | 'fault' | 'comm_lost' | 'unknown';

/** modeOf returns the UPS mode of a status push. */
export function modeOf(s: StatusPush | null): Mode {
  if (!s) return 'unknown';
  if (s.device?.state !== 'connected' || s.ups?.comm_lost) return 'comm_lost';
  const active = s.ups?.active ?? [];
  if (active.includes('UPS_FAULT')) return 'fault';
  if (active.includes('BYPASS_ACTIVE')) return 'bypass';
  if (s.ups?.on_battery) return 'battery';
  return 'mains';
}

const MODE_ICON: Record<Mode, string> = {
  mains: '⏻',
  battery: '▮',
  bypass: '↷',
  fault: '⚠',
  comm_lost: '⨯',
  unknown: '?',
};
const MODE_KIND: Record<Mode, 'ok' | 'warn' | 'crit' | 'info'> = {
  mains: 'ok',
  battery: 'warn',
  bypass: 'warn',
  fault: 'crit',
  comm_lost: 'crit',
  unknown: 'info',
};

export function ModeView({ mode }: { mode: Mode }) {
  return (
    <div class={'mode mode-' + MODE_KIND[mode]} role="status" aria-live="polite">
      <span class="mode-icon" aria-hidden="true">{MODE_ICON[mode]}</span>
      <span class="mode-text">{t('mode.' + mode)}</span>
    </div>
  );
}

// note: a text next to the value. battery.charge_estimate is calculated by
// VLT from the Q1 battery voltage (VLT-IS-006 TASK 03-00-00-003 step 8).
const ROWS: { key: string; name: string; digits: number; note?: string }[] = [
  { key: 'dash.input_voltage', name: 'input.voltage', digits: 1 },
  { key: 'dash.input_frequency', name: 'input.frequency', digits: 1 },
  { key: 'dash.output_voltage', name: 'output.voltage', digits: 1 },
  { key: 'dash.output_frequency', name: 'output.frequency', digits: 1 },
  { key: 'dash.output_current', name: 'output.current', digits: 1 },
  { key: 'dash.load', name: 'load.percent', digits: 0 },
  { key: 'dash.battery_voltage', name: 'battery.voltage', digits: 1 },
  { key: 'dash.battery_charge', name: 'battery.charge', digits: 0 },
  { key: 'dash.battery_charge_estimate', name: 'battery.charge_estimate', digits: 0, note: 'dash.estimate' },
  { key: 'dash.runtime', name: 'battery.runtime', digits: 0 },
  { key: 'dash.temperature', name: 'ups.temperature', digits: 1 },
];

export function Values({ s }: { s: StatusPush | null }) {
  const m = s?.measurements ?? {};
  return (
    <dl class="values">
      {ROWS.map((r) => (
        <div class="value">
          <dt>{t(r.key)}</dt>
          <dd>
            {value(m[r.name], r.digits, r.name === 'battery.runtime' ? t('unit.min') : undefined)}
            {r.note && m[r.name] ? <span class="small"> ({t(r.note)})</span> : null}
          </dd>
        </div>
      ))}
    </dl>
  );
}

export function sevIcon(sev: string): string {
  return sev === 'CRITICAL' ? '⛔' : sev === 'WARNING' ? '⚠' : 'ℹ';
}

export function EventRow({ e }: { e: VltEvent }) {
  const active = e.data?.active;
  return (
    <tr>
      <td>{time(e.ts)}</td>
      <td>
        <span aria-hidden="true">{sevIcon(e.severity)}</span> {t('sev.' + e.severity)}
      </td>
      <td>{e.type}{e.demo || e.data?.demo ? ' (' + t('event.demo') + ')' : ''}</td>
      <td>{active === true ? t('event.active') : active === false ? t('event.cleared') : DASH}</td>
    </tr>
  );
}

export function EventTable({ list, caption }: { list: VltEvent[]; caption: string }) {
  return (
    <table class="table">
      <caption>{caption}</caption>
      <thead>
        <tr>
          <th scope="col">{t('event.time')}</th>
          <th scope="col">{t('event.severity')}</th>
          <th scope="col">{t('event.type')}</th>
          <th scope="col">{t('event.state')}</th>
        </tr>
      </thead>
      <tbody>
        {list.length === 0 ? (
          <tr>
            <td colSpan={4}>{t('event.none')}</td>
          </tr>
        ) : (
          list.map((e) => <EventRow e={e} />)
        )}
      </tbody>
    </table>
  );
}

export function Dashboard() {
  const s = useStore(app);
  const mode = modeOf(s.status);
  return (
    <div class="screen">
      <h2>{t('nav.dashboard')}</h2>
      <ModeView mode={mode} />
      <section class="card" aria-labelledby="values-title">
        <h3 id="values-title">{t('dash.values')}</h3>
        <Values s={s.status} />
        <p class="small">
          {t('dash.policy')}{' '}
          {s.status ? (
            s.status.policy_active ? <Badge kind="ok" icon="✓" text={t('policy.active')} /> : <Badge kind="warn" icon="○" text={t('policy.inactive')} />
          ) : (
            DASH
          )}
        </p>
      </section>
      <section class="card">
        <EventTable list={s.events.slice(0, 10)} caption={t('dash.events')} />
      </section>
    </div>
  );
}
