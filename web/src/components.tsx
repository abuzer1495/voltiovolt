// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'preact/hooks';
import type { ConnInfo } from './api';
import { t } from './i18n';
import { confirmNative, native } from './native';
import { useStore } from './store';

/** ConnState shows the connection state with an icon and a text, always. */
export function ConnState({ conn }: { conn: ConnInfo }) {
  const icon = conn.state === 'connected' ? '●' : conn.state === 'connecting' ? '◌' : '✕';
  return (
    <div class={`conn conn-${conn.state}`} role="status" aria-live="polite">
      <span aria-hidden="true" class="conn-icon">{icon}</span>
      <span>{t('conn.' + conn.state)}</span>
    </div>
  );
}

export function isWindows(): boolean {
  return typeof navigator !== 'undefined' && /windows/i.test(navigator.userAgent);
}

/** ConnProblem shows the reason of a failed connection and the help text. */
export function ConnProblem({ conn }: { conn: ConnInfo }) {
  return (
    <section class="card" aria-labelledby="conn-title">
      <h2 id="conn-title">{t('conn.disconnected')}</h2>
      {conn.reason ? <p role="alert">{t('conn.reason.' + conn.reason)}</p> : null}
      {conn.reason === 'access_denied' ? (
        <p class="help">{t(isWindows() ? 'conn.help.windows' : 'conn.help.linux')}</p>
      ) : null}
      <p>{t('conn.retry')}</p>
    </section>
  );
}

/** Badge is a state with an icon and a text (never color only). */
export function Badge({ kind, icon, text }: { kind: 'ok' | 'warn' | 'crit' | 'info'; icon: string; text: string }) {
  return (
    <span class={'badge badge-' + kind}>
      <span aria-hidden="true">{icon}</span> {text}
    </span>
  );
}

/**
 * Confirm asks before an action. VLT-IS-011 TASK 03-00-00-005 step 2.2: in
 * the VoltioVolt window it is the native question dialog with Yes, No and
 * Cancel (No and Cancel do not do the action). Without a native dialog
 * (or if it fails) it is an inline confirmation with a text and two
 * buttons; the focus goes to it. A confirmation with a typed word is not
 * this component: it stays in the page.
 */
export function Confirm({ text, confirmLabel, onConfirm, onCancel }: { text: string; confirmLabel: string; onConfirm: () => void; onCancel: () => void }) {
  const nat = useStore(native).dialogs;
  const [inline, setInline] = useState(!nat);
  useEffect(() => {
    if (!nat) return;
    let live = true;
    void confirmNative(confirmLabel, text).then((r) => {
      if (!live) return;
      if (r === 'yes') onConfirm();
      else if (r === null) setInline(true);
      else onCancel();
    });
    return () => {
      live = false;
    };
  }, []);
  if (!inline) return <p class="small" role="status">{t('confirm.native.wait')}</p>;
  return (
    <div class="confirm" role="alertdialog" aria-modal="false" aria-label={confirmLabel} aria-describedby="confirm-text">
      <p id="confirm-text">{text}</p>
      <div class="row">
        <button type="button" class="danger" ref={(el) => el?.focus()} onClick={onConfirm}>{confirmLabel}</button>
        <button type="button" onClick={onCancel}>{t('common.cancel')}</button>
      </div>
    </div>
  );
}

/** errText returns the text of an error code. */
export function errText(code: string): string {
  const k = 'err.' + code;
  const s = t(k);
  return s === k ? t('err.ipc_error') : s;
}
