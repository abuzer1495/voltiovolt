// SPDX-License-Identifier: AGPL-3.0-or-later
// PIN screens: first start "Set PIN", login (unlock) and PIN change
// (TASK 03-00-00-003).
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode } from '../api';
import { t } from '../i18n';

export const PIN_MIN = 6;
export const PIN_MAX = 64;

/** pinProblem checks the length and the characters of a PIN. vltd checks the other rules. */
export function pinProblem(pin: string): string | null {
  if (pin.length < PIN_MIN || pin.length > PIN_MAX) return 'pin.err.length';
  if (!/^[A-Za-z0-9]+$/.test(pin)) return 'pin.err.chars';
  return null;
}

export function PinRules() {
  return (
    <ul class="rules" aria-label={t('pin.rules.title')}>
      <li>{t('pin.rules.length', { min: PIN_MIN, max: PIN_MAX })}</li>
      <li>{t('pin.rules.chars')}</li>
      <li>{t('pin.rules.simple')}</li>
      <li>{t('pin.rules.common')}</li>
    </ul>
  );
}

function errText(code: string): string {
  const k = 'err.' + code;
  const s = t(k);
  return s === k ? t('err.ipc_error') : s;
}

/** SetPin is the only screen at first start. With change, it is the PIN change of the Security screen. */
export function SetPin({ change = false, onDone }: { change?: boolean; onDone?: () => void }) {
  const [pin, setPin] = useState('');
  const [again, setAgain] = useState('');
  const [msg, setMsg] = useState<{ kind: 'error' | 'ok'; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const id = change ? 'chg' : 'set';
  async function submit(e: Event) {
    e.preventDefault();
    const p = pinProblem(pin);
    if (p) return setMsg({ kind: 'error', text: t(p) });
    if (pin !== again) return setMsg({ kind: 'error', text: t('pin.err.mismatch') });
    setBusy(true);
    try {
      await api.setPIN(pin);
      setPin('');
      setAgain('');
      setMsg({ kind: 'ok', text: t(change ? 'pin.changed' : 'pin.set.done') });
      onDone?.();
    } catch (err) {
      setMsg({ kind: 'error', text: errText(errorCode(err)) });
    } finally {
      setBusy(false);
    }
  }
  return (
    <section class="card pin" aria-labelledby={id + '-title'}>
      <h2 id={id + '-title'}>{t(change ? 'pin.change.title' : 'pin.set.title')}</h2>
      {change ? null : <p>{t('pin.set.intro')}</p>}
      <PinRules />
      <form onSubmit={submit}>
        <label for={id + '-pin'}>{t(change ? 'pin.new' : 'pin.label')}</label>
        <input id={id + '-pin'} type="password" autoComplete="new-password" value={pin} maxLength={PIN_MAX}
          onInput={(e) => setPin((e.target as HTMLInputElement).value)} />
        <label for={id + '-again'}>{t('pin.again')}</label>
        <input id={id + '-again'} type="password" autoComplete="new-password" value={again} maxLength={PIN_MAX}
          onInput={(e) => setAgain((e.target as HTMLInputElement).value)} />
        <button type="submit" disabled={busy}>{t(change ? 'pin.change.button' : 'pin.set.button')}</button>
      </form>
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

/** remaining returns the seconds until an RFC 3339 time (0 if past or empty). */
export function remaining(until: string | undefined, now: number = Date.now()): number {
  if (!until) return 0;
  const t0 = Date.parse(until);
  if (Number.isNaN(t0)) return 0;
  return Math.max(0, Math.ceil((t0 - now) / 1000));
}

/** Login is the PIN entry to unlock (auth.login). It shows the block time. */
export function Login({ blockUntil, onDone }: { blockUntil?: string; onDone?: () => void }) {
  const [pin, setPin] = useState('');
  const [msg, setMsg] = useState<string | null>(null);
  const [until, setUntil] = useState(blockUntil);
  const [, tick] = useState(0);
  useEffect(() => setUntil(blockUntil), [blockUntil]);
  const left = remaining(until);
  useEffect(() => {
    if (left <= 0) return;
    const h = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(h);
  }, [left > 0]);
  async function submit(e: Event) {
    e.preventDefault();
    setMsg(null);
    try {
      await api.login(pin);
      setPin('');
      onDone?.();
    } catch (err) {
      setPin('');
      const code = errorCode(err);
      setMsg(errText(code));
      try {
        const a = await api.authStatus();
        setUntil(a.block_until);
      } catch {
        /* the connection state shows the problem */
      }
    }
  }
  return (
    <section class="card pin" aria-labelledby="login-title">
      <h2 id="login-title">{t('login.title')}</h2>
      <form onSubmit={submit}>
        <label for="login-pin">{t('pin.label')}</label>
        <input id="login-pin" type="password" autoComplete="current-password" value={pin} maxLength={PIN_MAX}
          disabled={left > 0} onInput={(e) => setPin((e.target as HTMLInputElement).value)} />
        <button type="submit" disabled={left > 0 || pin === ''}>{t('login.button')}</button>
      </form>
      {left > 0 ? <p role="status" class="msg msg-error">{t('login.blocked', { seconds: left })}</p> : null}
      {msg ? <p role="alert" class="msg msg-error">{msg}</p> : null}
    </section>
  );
}
