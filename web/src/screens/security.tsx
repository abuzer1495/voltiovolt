// SPDX-License-Identifier: AGPL-3.0-or-later
// Security screen (TASK 03-00-00-003 step 5, TASK 03-00-00-007 step 1).
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type AuditRow, type SystemStatus } from '../api';
import { Badge, errText } from '../components';
import { DASH, time } from '../format';
import { t } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';
import { SetPin } from './pin';

export const RESET_TEXT = 'RESET VAULT';
const AUDIT_PAGE = 50;

const VAULT_KIND: Record<string, 'ok' | 'warn' | 'crit' | 'info'> = {
  available: 'ok',
  'reduced protection': 'warn',
  locked: 'crit',
  'not configured': 'warn',
};

export function VaultView({ st }: { st: SystemStatus | null }) {
  const v = st?.vault;
  return (
    <section class="card" aria-labelledby="vault-title">
      <h3 id="vault-title">{t('vault.title')}</h3>
      {!v ? (
        <p>{DASH}</p>
      ) : (
        <>
          <p>
            <Badge kind={VAULT_KIND[v.state] ?? 'info'} icon={v.state === 'available' ? '✓' : '⚠'} text={t('vault.state.' + v.state)} />
          </p>
          <p>{t('vault.reason', { reason: v.reason || DASH })}</p>
          {v.key_mode ? <p>{t('vault.keymode', { mode: v.key_mode })}</p> : null}
          {v.state === 'reduced protection' ? (
            <p class="banner banner-warn" role="note">
              <span aria-hidden="true">⚠</span> {t('vault.reduced')}
            </p>
          ) : null}
        </>
      )}
    </section>
  );
}

export function ResetVault({ elevated, onDone }: { elevated: boolean; onDone: () => void }) {
  const [text, setText] = useState('');
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  async function submit(e: Event) {
    e.preventDefault();
    if (text !== RESET_TEXT) return setMsg({ kind: 'error', text: t('reset.mismatch') });
    try {
      await api.resetVault(text);
      setText('');
      setMsg({ kind: 'ok', text: t('reset.done') });
      onDone();
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  return (
    <section class="card" aria-labelledby="reset-title">
      <h3 id="reset-title">{t('reset.title')}</h3>
      <p>{t('reset.explain')}</p>
      <form class="form" onSubmit={submit}>
        <label for="reset-text">{t('reset.label', { text: RESET_TEXT })}</label>
        <input id="reset-text" disabled={!elevated} value={text} autoComplete="off" onInput={(e) => setText((e.target as HTMLInputElement).value)} />
        <button type="submit" class="danger" disabled={!elevated || text !== RESET_TEXT}>{t('reset.button')}</button>
      </form>
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

export function AuditLog() {
  const [rows, setRows] = useState<AuditRow[]>([]);
  const [next, setNext] = useState(0);
  const [done, setDone] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  async function load(after: number, reset: boolean) {
    try {
      const r = await api.audit(after, AUDIT_PAGE);
      setRows((o) => (reset ? r.rows ?? [] : [...o, ...(r.rows ?? [])]));
      setNext(r.next);
      setDone(!r.next);
    } catch (e) {
      setErr(errText(errorCode(e)));
    }
  }
  useEffect(() => {
    void load(0, true);
  }, []);
  return (
    <section class="card">
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
      <table class="table">
        <caption>{t('audit.title')}</caption>
        <thead>
          <tr>
            <th scope="col">{t('audit.seq')}</th>
            <th scope="col">{t('event.time')}</th>
            <th scope="col">{t('audit.actor')}</th>
            <th scope="col">{t('audit.action')}</th>
            <th scope="col">{t('audit.target')}</th>
            <th scope="col">{t('audit.result')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr>
              <td>{r.seq}</td>
              <td>{time(r.ts)}</td>
              <td>{r.actor}</td>
              <td>{r.action}</td>
              <td>{r.target || DASH}</td>
              <td>{r.result}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <button type="button" disabled={done} onClick={() => void load(next, false)}>{t(done ? 'audit.end' : 'audit.more')}</button>
    </section>
  );
}

export function Security() {
  const s = useStore(app);
  const elevated = !!s.auth?.elevated;
  const [st, setSt] = useState<SystemStatus | null>(null);
  const load = () => api.systemStatus().then(setSt, () => undefined);
  useEffect(() => {
    void load();
  }, []);
  return (
    <div class="screen">
      <h2>{t('nav.security')}</h2>
      <VaultView st={st} />
      {elevated ? <SetPin change /> : (
        <section class="card"><h3>{t('pin.change.title')}</h3><p class="note">{t('locked.need')}</p></section>
      )}
      <ResetVault elevated={elevated} onDone={() => void load()} />
      {elevated ? <AuditLog /> : (
        <section class="card"><h3>{t('audit.title')}</h3><p class="note">{t('locked.need')}</p></section>
      )}
    </div>
  );
}
