// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-012 TASK 03-00-00-004: the battery Settings for the Event
// BATTERY_REPLACE. BatteryReplace is the card of the Device screen (step
// 5); BatteryDateField is the optional field of the wizard step "Canlı
// değerler" (step 6).
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type StatusPush } from '../api';
import { errText } from '../components';
import { HelpLine } from '../help';
import { t } from '../i18n';

export const INSTALLED_ON = 'battery.installed_on';
export const REPLACE_AFTER = 'battery.replace_after_months';
export const REPLACE_MIN = 12;
export const REPLACE_MAX = 120;
export const REPLACE_DEFAULT = 36;

type Msg = { kind: 'ok' | 'error'; text: string } | null;

/** today is the local date YYYY-MM-DD (the maximum of the date field). */
export function today(now = new Date()): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
}

/** batteryReason is the reason text of BATTERY_REPLACE. */
export function batteryReason(b: NonNullable<StatusPush['ups']['battery']> | undefined): string {
  const r = b?.reasons?.[0];
  if (r === 'test_failed') return t('health.battery_replace.test_failed');
  if (r === 'age') return t('health.battery_replace.age', { age: b?.age_months ?? 0, limit: b?.replace_after_months ?? REPLACE_DEFAULT });
  return '';
}

function useBatterySettings() {
  const [date, setDate] = useState('');
  const [months, setMonths] = useState(String(REPLACE_DEFAULT));
  useEffect(() => {
    void api.settings().then((r) => {
      const list = r?.settings ?? [];
      const d = list.find((x) => x.key === INSTALLED_ON)?.value;
      const m = list.find((x) => x.key === REPLACE_AFTER)?.value;
      if (typeof d === 'string') setDate(d);
      if (typeof m === 'number') setMonths(String(m));
    }, () => undefined);
  }, []);
  return { date, setDate, months, setMonths };
}

/** BatteryReplace is the battery card of the Device screen (step 5). */
export function BatteryReplace({ elevated, status }: { elevated: boolean; status: StatusPush | null }) {
  const { date, setDate, months, setMonths } = useBatterySettings();
  const [msg, setMsg] = useState<Msg>(null);
  const b = status?.ups?.battery;
  async function save(e: Event) {
    e.preventDefault();
    setMsg(null);
    const n = Number(months);
    if (!Number.isInteger(n) || n < REPLACE_MIN || n > REPLACE_MAX) {
      setMsg({ kind: 'error', text: t('battery.months.range', { min: REPLACE_MIN, max: REPLACE_MAX }) });
      return;
    }
    try {
      await api.setSetting(INSTALLED_ON, date);
      await api.setSetting(REPLACE_AFTER, n);
      setMsg({ kind: 'ok', text: t('battery.saved') });
    } catch (err) {
      setMsg({ kind: 'error', text: errText(errorCode(err)) });
    }
  }
  return (
    <section class="card" aria-labelledby="batt-repl-title">
      <h3 id="batt-repl-title">{t('battery.title')}</h3>
      <HelpLine text={t('battery.help')} />
      {b?.active ? (
        <div class="banner banner-warn" role="note">
          <span aria-hidden="true" class="banner-icon">⚠</span>
          <span>{t('health.battery_replace', { reason: batteryReason(b) })}</span>
        </div>
      ) : null}
      <form class="form form-2col" onSubmit={save}>
        <label for="batt-installed">{t('battery.installed_on')}</label>
        <input id="batt-installed" type="date" min="1990-01-01" max={today()} disabled={!elevated} value={date}
          onInput={(e) => setDate((e.target as HTMLInputElement).value)} />
        <label for="batt-months">{t('battery.months')}</label>
        <input id="batt-months" type="number" min={REPLACE_MIN} max={REPLACE_MAX} step={1} disabled={!elevated} value={months}
          onInput={(e) => setMonths((e.target as HTMLInputElement).value)} />
        <button type="submit" class="primary" disabled={!elevated}>{t('common.save')}</button>
      </form>
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

/** BatteryDateField is the optional battery date of the wizard (step 6). */
export function BatteryDateField({ elevated }: { elevated: boolean }) {
  const { date, setDate } = useBatterySettings();
  const [msg, setMsg] = useState<Msg>(null);
  async function save(e: Event) {
    e.preventDefault();
    setMsg(null);
    try {
      await api.setSetting(INSTALLED_ON, date);
      setMsg({ kind: 'ok', text: t('battery.saved') });
    } catch (err) {
      setMsg({ kind: 'error', text: errText(errorCode(err)) });
    }
  }
  return (
    <form class="row battery-date" onSubmit={save}>
      <label for="wiz-batt-installed">{t('battery.installed_on.optional')}</label>
      <input id="wiz-batt-installed" type="date" min="1990-01-01" max={today()} disabled={!elevated} value={date}
        title={t('battery.help')} onInput={(e) => setDate((e.target as HTMLInputElement).value)} />
      <button type="submit" disabled={!elevated}>{t('common.save')}</button>
      {msg ? <span role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</span> : null}
    </form>
  );
}
