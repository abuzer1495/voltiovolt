// SPDX-License-Identifier: AGPL-3.0-or-later
// Field help of VLT-IS-009 TASK 03-00-00-004 and OD-42.
import { useState } from 'preact/hooks';
import { api } from './api';
import { t } from './i18n';

/** Official help links (also in internal/gui OfficialHelpURLs and the CI check). */
export const HELP_LINKS = {
  telegram: 'https://core.telegram.org/bots/tutorial',
  gmail: 'https://support.google.com/accounts/answer/185833',
} as const;

export const HELP_HOSTS = ['core.telegram.org', 'support.google.com', 'support.microsoft.com'] as const;

/** isOfficialHelpURL is the same host rule as gui.AllowedHelpURL. */
export function isOfficialHelpURL(raw: string): boolean {
  try {
    const u = new URL(raw);
    if (u.protocol !== 'https:') return false;
    if (u.username || u.password) return false;
    if (u.port && u.port !== '443') return false;
    return (HELP_HOSTS as readonly string[]).includes(u.hostname.toLowerCase());
  } catch {
    return false;
  }
}

/** HelpLine is a one-sentence help text, with an optional "?" panel and official link. */
export function HelpLine({
  text,
  more,
  href,
}: {
  text: string;
  more?: string;
  href?: string;
}) {
  const [open, setOpen] = useState(false);
  const canMore = !!(more || href);
  return (
    <div class="field-help">
      <p class="hint">{text}</p>
      {canMore ? (
        <button
          type="button"
          class="help-q"
          aria-expanded={open}
          aria-label={t('help.more')}
          onClick={() => setOpen(!open)}
        >
          ?
        </button>
      ) : null}
      {open ? (
        <div class="help-panel" role="region" aria-label={t('help.more')}>
          {more ? <p>{more}</p> : null}
          {href ? (
            <p>
              <button type="button" class="link" onClick={() => void api.openHelp(href)}>
                {t('help.open')}
              </button>
              <span class="hint"> {href}</span>
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
