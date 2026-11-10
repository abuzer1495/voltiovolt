// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ComponentType } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { CountdownBanner, FallbackBanner } from './banners';
import { ConnProblem, ConnState } from './components';
import { t, useLang } from './i18n';
import { Dashboard } from './screens/dashboard';
import { Device } from './screens/device';
import { Events } from './screens/events';
import { History } from './screens/history';
import { Login, SetPin } from './screens/pin';
import { Policy } from './screens/policy';
import { Security } from './screens/security';
import { app, lock, refreshAuth, start } from './state';
import { useStore } from './store';

export interface Route {
  id: string;
  label: string;
  comp: ComponentType;
}

export const routes: Route[] = [
  { id: 'dashboard', label: 'nav.dashboard', comp: Dashboard },
  { id: 'device', label: 'nav.device', comp: Device },
  { id: 'history', label: 'nav.history', comp: History },
  { id: 'events', label: 'nav.events', comp: Events },
  { id: 'policy', label: 'nav.policy', comp: Policy },
  { id: 'security', label: 'nav.security', comp: Security },
];

function currentRoute(): string {
  const h = (typeof location !== 'undefined' ? location.hash : '').replace(/^#\/?/, '');
  return routes.some((r) => r.id === h) ? h : 'dashboard';
}

export function App() {
  useLang();
  const s = useStore(app);
  const [route, setRoute] = useState(currentRoute());
  const [unlock, setUnlock] = useState(false);
  useEffect(() => start(), []);
  useEffect(() => {
    const f = () => setRoute(currentRoute());
    window.addEventListener('hashchange', f);
    return () => window.removeEventListener('hashchange', f);
  }, []);
  const connected = s.conn.state === 'connected';
  const auth = s.auth;
  const firstStart = connected && auth !== null && !auth.pin_set;
  const elevated = !!auth?.elevated;
  const Screen = routes.find((r) => r.id === route)?.comp ?? Dashboard;

  return (
    <div class="layout">
      <a class="skip" href="#main">{t('app.skip')}</a>
      <header class="top">
        <h1 class="brand">{t('app.name')}</h1>
        <ConnState conn={s.conn} />
        {connected && auth?.pin_set ? (
          <div class="lockstate">
            <span role="status">
              <span aria-hidden="true">{elevated ? '🔓' : '🔒'}</span> {t(elevated ? 'lock.unlocked' : 'lock.locked')}
            </span>
            {elevated ? (
              <button type="button" onClick={() => void lock()}>{t('lock.button')}</button>
            ) : (
              <button type="button" aria-expanded={unlock} onClick={() => setUnlock(!unlock)}>{t('lock.unlock')}</button>
            )}
          </div>
        ) : null}
      </header>
      {connected && !firstStart ? (
        <nav class="nav" aria-label={t('nav.label')}>
          {routes.map((r) => (
            <a href={'#/' + r.id} aria-current={r.id === route ? 'page' : undefined}>{t(r.label)}</a>
          ))}
        </nav>
      ) : (
        <div />
      )}
      <main id="main" tabIndex={-1}>
        {!connected ? (
          s.conn.state === 'connecting' ? <p role="status">{t('conn.connecting')}</p> : <ConnProblem conn={s.conn} />
        ) : firstStart ? (
          <SetPin onDone={refreshAuth} />
        ) : (
          <>
            <FallbackBanner fallback={!!s.policy?.fallback} />
            <CountdownBanner cd={s.countdown} elevated={elevated} lowBattery={!!s.status?.ups?.low_battery} />
            {unlock && !elevated ? <Login blockUntil={auth?.block_until} onDone={() => { setUnlock(false); refreshAuth(); }} /> : null}
            <Screen />
          </>
        )}
      </main>
    </div>
  );
}
