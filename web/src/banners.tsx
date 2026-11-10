// SPDX-License-Identifier: AGPL-3.0-or-later
// Countdown banner and Fallback banner (TASK 03-00-00-004 step 1.4, 1.5).
import { useState } from 'preact/hooks';
import { api, errorCode, type Countdown } from './api';
import { t } from './i18n';

/** CountdownBanner shows a running countdown with the Cancel button. */
export function CountdownBanner({ cd, elevated, lowBattery }: { cd: Countdown | null; elevated: boolean; lowBattery: boolean }) {
  const [err, setErr] = useState<string | null>(null);
  if (!cd || !cd.active) return null;
  const blocked = lowBattery || !cd.cancelable;
  const disabled = blocked || !elevated;
  async function cancel() {
    setErr(null);
    try {
      await api.cancelShutdown();
    } catch (e) {
      const c = errorCode(e);
      setErr(t('err.' + c) === 'err.' + c ? t('err.ipc_error') : t('err.' + c));
    }
  }
  return (
    <div class="banner banner-crit" role="alert">
      <span aria-hidden="true" class="banner-icon">⏻</span>
      <div>
        <strong>{t(cd.action === 'hibernate' ? 'cd.title.hibernate' : 'cd.title.shutdown', { seconds: cd.remaining_seconds })}</strong>
        <div>{t('cd.trigger', { trigger: t('trigger.' + (cd.trigger ?? 'unknown')) })}</div>
        {blocked ? <div class="banner-reason">{t('cd.lowbattery')}</div> : !elevated ? <div class="banner-reason">{t('cd.unlock')}</div> : null}
        {err ? <div class="banner-reason">{err}</div> : null}
      </div>
      <button type="button" class="danger" disabled={disabled} aria-disabled={disabled} onClick={() => void cancel()}>
        {t('cd.cancel')}
      </button>
    </div>
  );
}

/** FallbackBanner is the WARNING banner of the Fallback Policy. */
export function FallbackBanner({ fallback }: { fallback: boolean }) {
  if (!fallback) return null;
  return (
    <div class="banner banner-warn" role="alert">
      <span aria-hidden="true" class="banner-icon">⚠</span>
      <strong>{t('fallback.banner')}</strong>
    </div>
  );
}
