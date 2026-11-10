// SPDX-License-Identifier: AGPL-3.0-or-later
// Security screen (TASK 03-00-00-003 step 5, TASK 03-00-00-007 step 1).
import { t } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';
import { SetPin } from './pin';

export function Security() {
  const s = useStore(app);
  const elevated = !!s.auth?.elevated;
  return (
    <div class="screen">
      <h2>{t('nav.security')}</h2>
      {elevated ? <SetPin change /> : <p class="note">{t('locked.need')}</p>}
    </div>
  );
}
