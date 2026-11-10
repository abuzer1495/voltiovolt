// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ConnInfo } from './api';
import { t } from './i18n';

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
