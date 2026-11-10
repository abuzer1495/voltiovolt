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
      {conn.reason && conn.state === 'disconnected' ? <span class="conn-reason">{t('conn.reason.' + conn.reason)}</span> : null}
    </div>
  );
}
