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

/**
 * fieldValue reads the live DOM value of an input in the submitted form.
 * Autofill, a password manager and some IME paths write the field without
 * an "input" event, so the Preact state can be empty (VLT-IS-009 R-02,
 * web/src/screens/pin.tsx).
 */
export function fieldValue(e: Event, id: string): string {
  const form = e.currentTarget as HTMLFormElement | null;
  if (!form || typeof form.querySelector !== 'function') return '';
  const el = form.querySelector('#' + id) as HTMLInputElement | null;
  return el?.value ?? '';
}

function onPINInput(set: (v: string) => void) {
  return (e: Event) => set((e.target as HTMLInputElement).value);
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
export function SetPin({ change = false, onDone, nested = false }: { change?: boolean; onDone?: () => void; nested?: boolean }) {
  const [pin, setPin] = useState('');
  const [again, setAgain] = useState('');
  const [msg, setMsg] = useState<{ kind: 'error' | 'ok'; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const id = change ? 'chg' : 'set';
  async function submit(e: Event) {
    e.preventDefault();
    const value = fieldValue(e, id + '-pin');
    const againVal = fieldValue(e, id + '-again');
    setPin(value);
    setAgain(againVal);
    const p = pinProblem(value);
    if (p) return setMsg({ kind: 'error', text: t(p) });
    if (value !== againVal) return setMsg({ kind: 'error', text: t('pin.err.mismatch') });
    setBusy(true);
    try {
      await api.setPIN(value);
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
      {change || nested ? (
        <h3 id={id + '-title'}>{t(change ? 'pin.change.title' : 'pin.set.title')}</h3>
      ) : (
        <h2 id={id + '-title'}>{t('pin.set.title')}</h2>
      )}
      {change ? null : <p>{t('pin.set.intro')}</p>}
      <PinRules />
      <form onSubmit={submit}>
        <label for={id + '-pin'}>{t(change ? 'pin.new' : 'pin.label')}</label>
        <input id={id + '-pin'} name="pin" type="password" autoComplete="new-password" value={pin} maxLength={PIN_MAX}
          onInput={onPINInput(setPin)} onChange={onPINInput(setPin)} />
        <label for={id + '-again'}>{t('pin.again')}</label>
        <input id={id + '-again'} name="pin-again" type="password" autoComplete="new-password" value={again} maxLength={PIN_MAX}
          onInput={onPINInput(setAgain)} onChange={onPINInput(setAgain)} />
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
    const value = fieldValue(e, 'login-pin');
    setPin(value);
    setMsg(null);
    if (value === '') {
      setMsg(t('pin.err.length'));
      return;
    }
    try {
      await api.login(value);
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
        <input id="login-pin" name="pin" type="password" autoComplete="current-password" value={pin} maxLength={PIN_MAX}
          disabled={left > 0} onInput={onPINInput(setPin)} onChange={onPINInput(setPin)} />
        {/* Do not disable on an empty Preact state: autofill can fill the
            field without an input event (VLT-IS-009 R-02). */}
        <button type="submit" disabled={left > 0}>{t('login.button')}</button>
      </form>
      {left > 0 ? <p role="status" class="msg msg-error">{t('login.blocked', { seconds: left })}</p> : null}
      {msg ? <p role="alert" class="msg msg-error">{msg}</p> : null}
    </section>
  );
}
