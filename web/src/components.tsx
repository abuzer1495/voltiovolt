// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'preact/hooks';
import type { ConnInfo, StatusPush } from './api';
import { useClearSelection } from './dialog';
import { t } from './i18n';
import { confirmNative, native } from './native';
import { useStore } from './store';

/**
 * ConnState is the connection text of the header. Service mode: the IPC
 * connection to vltd. Portable mode has no vltd (VLT-IS-012 R-16): the
 * text is the state of the UPS link. "service" must be true for the vltd
 * text; while the mode is not known, the portable text is used.
 */
export function ConnState({ conn, service = false, status }: { conn: ConnInfo; service?: boolean; status?: StatusPush | null }) {
  let state = conn.state;
  let text = t('conn.' + conn.state);
  if (!service) {
    if (conn.state !== 'connected') {
      text = t('conn.portable.' + (conn.state === 'connecting' ? 'connecting' : 'disconnected'));
    } else {
      const dev = status?.device;
      const transport = dev?.config?.transport;
      const port = dev?.profile?.port || dev?.config?.serial_port || '';
      if (transport === 'demo') {
        text = t('conn.ups.demo');
      } else if (transport === 'test') {
        text = t('conn.ups.test');
      } else if (dev?.state === 'connected') {
        text = port ? t('conn.ups.connected', { port }) : t('conn.ups.connected.noport');
      } else {
        state = 'disconnected';
        text = t('conn.ups.disconnected');
      }
    }
  }
  const icon = state === 'connected' ? '●' : state === 'connecting' ? '◌' : '✕';
  return (
    <div class={`conn conn-${state}`} role="status" aria-live="polite">
      <span aria-hidden="true" class="conn-icon">{icon}</span>
      <span>{text}</span>
    </div>
  );
}

export function isWindows(): boolean {
  return typeof navigator !== 'undefined' && /windows/i.test(navigator.userAgent);
}

/** ConnProblem shows the reason of a failed connection and the help text. */
export function ConnProblem({ conn, service = true }: { conn: ConnInfo; service?: boolean }) {
  return (
    <section class="card" aria-labelledby="conn-title">
      <h2 id="conn-title">{t(service ? 'conn.disconnected' : 'conn.portable.disconnected')}</h2>
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
  useClearSelection();
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
