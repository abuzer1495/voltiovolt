// SPDX-License-Identifier: AGPL-3.0-or-later
// Events screen (TASK 03-00-00-005 step 3): paging, filter by severity and type.
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type VltEvent } from '../api';
import { errText } from '../components';
import { t } from '../i18n';
import { EventTable } from './dashboard';

export const PAGE = 50;

export function filterEvents(list: VltEvent[], sev: string, type: string): VltEvent[] {
  return list.filter((e) => (sev === '' || e.severity === sev) && (type === '' || e.type === type));
}

export function Events() {
  const [list, setList] = useState<VltEvent[]>([]);
  const [next, setNext] = useState(0);
  const [done, setDone] = useState(false);
  const [sev, setSev] = useState('');
  const [type, setType] = useState('');
  const [err, setErr] = useState<string | null>(null);
  async function load(before: number, reset: boolean) {
    try {
      const p = await api.events(before, PAGE);
      const evs = p.events ?? [];
      setList((old) => (reset ? evs : [...old, ...evs]));
      setNext(p.next);
      setDone(!p.next);
    } catch (e) {
      setErr(errText(errorCode(e)));
    }
  }
  useEffect(() => {
    void load(0, true);
  }, []);
  const types = [...new Set(list.map((e) => e.type))].sort();
  const shown = filterEvents(list, sev, type);
  return (
    <div class="screen">
      <h2>{t('nav.events')}</h2>
      <div class="row filters">
        <label for="ev-sev">{t('event.severity')}</label>
        <select id="ev-sev" value={sev} onChange={(e) => setSev((e.target as HTMLSelectElement).value)}>
          <option value="">{t('common.all')}</option>
          <option value="INFO">{t('sev.INFO')}</option>
          <option value="WARNING">{t('sev.WARNING')}</option>
          <option value="CRITICAL">{t('sev.CRITICAL')}</option>
        </select>
        <label for="ev-type">{t('event.type')}</label>
        <select id="ev-type" value={type} onChange={(e) => setType((e.target as HTMLSelectElement).value)}>
          <option value="">{t('common.all')}</option>
          {types.map((x) => <option value={x}>{x}</option>)}
        </select>
        <button type="button" onClick={() => void load(0, true)}>{t('common.refresh')}</button>
      </div>
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
      <p role="status">{t('events.count', { shown: shown.length, loaded: list.length })}</p>
      <EventTable list={shown} caption={t('nav.events')} />
      <button type="button" disabled={done} onClick={() => void load(next, false)}>{t(done ? 'events.end' : 'events.more')}</button>
    </div>
  );
}
