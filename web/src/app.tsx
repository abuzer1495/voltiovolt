// SPDX-License-Identifier: AGPL-3.0-or-later
import type { ComponentType } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { CountdownBanner, DemoBanner, DemoShutdownBanner, FallbackBanner, TestBanner, TestShutdownBanner } from './banners';
import { ConnProblem, ConnState } from './components';
import { api } from './api';
import { t, useLang } from './i18n';
import { Dashboard } from './screens/dashboard';
import { Device } from './screens/device';
import { Events } from './screens/events';
import { General } from './screens/general';
import { FirstStartPrompt, Wizard, startWizard, wizardWanted } from './screens/wizard';
import { Notifications } from './screens/notifications';
import { History } from './screens/history';
import { Login } from './screens/pin';
import { Policy } from './screens/policy';
import { Security } from './screens/security';
import { UPSControl } from './screens/ups';
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
  { id: 'ups', label: 'nav.ups', comp: UPSControl },
  { id: 'history', label: 'nav.history', comp: History },
  { id: 'events', label: 'nav.events', comp: Events },
  { id: 'policy', label: 'nav.policy', comp: Policy },
  { id: 'notifications', label: 'nav.notifications', comp: Notifications },
  { id: 'security', label: 'nav.security', comp: Security },
  { id: 'general', label: 'nav.general', comp: General },
];

function currentRoute(): string {
  const h = (typeof location !== 'undefined' ? location.hash : '').replace(/^#\/?/, '');
  return routes.some((r) => r.id === h) ? h : 'dashboard';
}

export function App() {
  useLang();
  const s = useStore(app);
  const wiz = useStore(wizardWanted);
  const [route, setRoute] = useState(currentRoute());
  const [unlock, setUnlock] = useState(false);
  const [portable, setPortable] = useState(false);
  const [wizDone, setWizDone] = useState(false);
  const [later, setLater] = useState(0);
  const [laterSession, setLaterSession] = useState(false);
  useEffect(() => start(), []);
  useEffect(() => {
    void api.about().then(async (a) => {
      const p = a.mode === 'portable';
      setPortable(p);
      if (p) {
        try { setWizDone(await api.wizardCompleted()); } catch { /* leave false: first start */ }
      }
      try {
        setLater(await api.wizardLaterCount());
      } catch {
        const n = Number((typeof localStorage !== 'undefined' && localStorage.getItem('vlt.wizard-later')) || '0');
        if (n > 0) setLater(n);
      }
    }, () => undefined);
  }, []);
  useEffect(() => {
    const f = () => setRoute(currentRoute());
    window.addEventListener('hashchange', f);
    return () => window.removeEventListener('hashchange', f);
  }, []);
  const connected = s.conn.state === 'connected';
  const auth = s.auth;
  // Portable (OD-37, OD-41): a small first-start dialog; the wizard is
  // not shown again after it is completed or after "Later" two times.
  // Service: the same dialog, until a PIN is set.
  const needsPrompt = connected && auth !== null && later < 2 && (portable ? !wizDone : !auth.pin_set);
  const showPrompt = needsPrompt && !wiz.open && !laterSession;
  const firstStart = needsPrompt;
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
      {connected && !showPrompt ? (
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
        ) : (
          <>
            {showPrompt ? (
              <FirstStartPrompt
                onStart={() => startWizard((portable && !auth?.pin_set) || (!portable && !auth?.pin_set) ? 'pin' : undefined)}
                onLater={() => {
                  setLaterSession(true);
                  void (async () => {
                    try {
                      setLater(await api.addWizardLater());
                    } catch {
                      const n = later + 1;
                      try { localStorage.setItem('vlt.wizard-later', String(n)); } catch { /* no store */ }
                      setLater(n);
                    }
                  })();
                }}
              />
            ) : null}
            {wiz.open ? (
              <Wizard first={firstStart} portable={portable} startAt={wiz.start || undefined} onDone={() => {
                wizardWanted.set({ open: false, start: '' });
                if (portable) {
                  void api.setWizardCompleted().then(() => setWizDone(true), () => setWizDone(true));
                }
                refreshAuth();
              }} />
            ) : null}
            {!showPrompt ? (
              <>
                <DemoBanner demo={!!s.status?.demo || s.status?.device?.config?.transport === 'demo'} />
                <DemoShutdownBanner message={s.status?.demo_message} />
                <TestBanner test={!!s.status?.test || s.status?.device?.config?.transport === 'test'} />
                <TestShutdownBanner message={s.status?.test_message} />
                <FallbackBanner fallback={!!s.policy?.fallback} />
                <CountdownBanner cd={s.countdown} elevated={elevated} lowBattery={!!s.status?.ups?.low_battery} />
                {unlock && !elevated ? <Login portable={portable} blockUntil={auth?.block_until} onDone={() => { setUnlock(false); refreshAuth(); }} onForgot={() => { setUnlock(false); refreshAuth(); startWizard('pin'); }} /> : null}
                <Screen />
              </>
            ) : null}
          </>
        )}
      </main>
    </div>
  );
}
