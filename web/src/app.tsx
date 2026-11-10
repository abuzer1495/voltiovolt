// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect } from 'preact/hooks';
import { ConnState } from './components';
import { t, useLang } from './i18n';
import { app, start } from './state';
import { useStore } from './store';

export function App() {
  useLang();
  const s = useStore(app);
  useEffect(() => start(), []);
  return (
    <div class="layout">
      <header class="top">
        <h1 class="brand">{t('app.name')}</h1>
        <ConnState conn={s.conn} />
      </header>
      <main id="main" tabIndex={-1}>
        <p>{t('app.tagline')}</p>
      </main>
    </div>
  );
}
