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
import { ui, setMode } from './mode';
import { Simple } from './screens/simple';
import { ScaleNotice } from './screens/scalechoice';
import { watchScale } from './scale';
import { onWizard, watchNative } from './native';
import { NavIcon } from './navicons';
import { app, lock, refreshAuth, start, watchInput } from './state';
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
  { id: 'history', label: 'nav.history', comp: () => <History /> },
  { id: 'events', label: 'nav.events', comp: Events },
  { id: 'policy', label: 'nav.policy', comp: Policy },
  { id: 'notifications', label: 'nav.notifications', comp: Notifications },
  { id: 'security', label: 'nav.security', comp: Security },
  { id: 'general', label: 'nav.general', comp: General },
];

const NAV_KEY = 'vlt.nav-collapsed';
/** NAV_NARROW: below this window width (CSS px) the pane starts collapsed. */
export const NAV_NARROW = 1100;

/** navCollapsed is the first state of the navigation pane (step 1.4). */
export function navCollapsed(): boolean {
  try {
    const v = localStorage.getItem(NAV_KEY);
    if (v === '1' || v === '0') return v === '1';
  } catch {
    /* no local store */
  }
  return typeof window !== 'undefined' && window.innerWidth < NAV_NARROW;
}

function currentRoute(): string {
  const h = (typeof location !== 'undefined' ? location.hash : '').replace(/^#\/?/, '');
  return routes.some((r) => r.id === h) ? h : 'dashboard';
}

export function App() {
  useLang();
  const s = useStore(app);
  const wiz = useStore(wizardWanted);
  const uiMode = useStore(ui).mode;
  const simple = uiMode === 'simple';
  const [route, setRoute] = useState(currentRoute());
  const [unlock, setUnlock] = useState(false);
  const [portable, setPortable] = useState(false);
  const [wizDone, setWizDone] = useState(false);
  const [later, setLater] = useState(0);
  const [laterSession, setLaterSession] = useState(false);
  const [lockClosed, setLockClosed] = useState(false);
  const [collapsed, setCollapsed] = useState(navCollapsed());
  useEffect(() => {
    start();
    watchInput();
    watchScale();
    // VLT-IS-011 TASK 03-00-00-005: menu bar, accent color, F1.
    watchNative();
    onWizard(() => startWizard(app.get().auth && !app.get().auth?.pin_set ? 'pin' : undefined));
  }, []);
  const toggleNav = () => {
    const c = !collapsed;
    setCollapsed(c);
    try { localStorage.setItem(NAV_KEY, c ? '1' : '0'); } catch { /* no local store */ }
  };
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
  // VLT-IS-011 TASK 03-00-00-001 step 5: the lock screen opens only when the
  // state becomes locked (the "Lock" button, the idle time, an elevated
  // request that fails) or at the start with a PIN set. Never over the
  // wizard (OD-51).
  useEffect(() => {
    if (elevated) setLockClosed(false);
  }, [elevated]);
  const lockScreen = connected && !!auth?.pin_set && !elevated && !wiz.open && !showPrompt && (unlock || !lockClosed);
  const showNav = connected && !showPrompt && !simple;
  const Screen = routes.find((r) => r.id === route)?.comp ?? Dashboard;

  return (
    <div class={'layout' + (simple ? ' layout-simple' : '') + (showNav ? '' : ' no-nav')} data-mode={uiMode}>
      <a class="skip" href="#main">{t('app.skip')}</a>
      <header class="top">
        <h1 class="brand">{t('app.name')}</h1>
        <ConnState conn={s.conn} />
        {connected && !simple ? (
          <button type="button" class="mode-switch" onClick={() => void setMode('simple')}>{t('mode.ui.to_simple')}</button>
        ) : null}
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
      {showNav ? (
        <nav class={'nav' + (collapsed ? ' nav-collapsed' : '')} aria-label={t('nav.label')}>
          <button type="button" class="nav-toggle" aria-expanded={!collapsed} aria-label={t(collapsed ? 'nav.expand' : 'nav.collapse')}
            title={t(collapsed ? 'nav.expand' : 'nav.collapse')} onClick={toggleNav}>
            <NavIcon name="menu" />
          </button>
          {routes.map((r) => (
            <a href={'#/' + r.id} aria-current={r.id === route ? 'page' : undefined} title={collapsed ? t(r.label) : undefined}>
              <NavIcon name={r.id} />
              <span class="nav-text">{t(r.label)}</span>
            </a>
          ))}
        </nav>
      ) : null}
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
                <ScaleNotice />
                <DemoBanner demo={!!s.status?.demo || s.status?.device?.config?.transport === 'demo'} />
                <DemoShutdownBanner message={s.status?.demo_message} />
                <TestBanner test={!!s.status?.test || s.status?.device?.config?.transport === 'test'} />
                <TestShutdownBanner message={s.status?.test_message} />
                <FallbackBanner fallback={!!s.policy?.fallback} />
                <CountdownBanner cd={s.countdown} elevated={elevated} lowBattery={!!s.status?.ups?.low_battery} />
                {lockScreen ? (
                  <section class="card lock-screen" aria-labelledby="lock-screen-title">
                    <h2 id="lock-screen-title">{t('lock.screen.title')}</h2>
                    <p>{t('lock.screen.intro')}</p>
                    <Login portable={portable} blockUntil={auth?.block_until} onDone={() => { setUnlock(false); refreshAuth(); }} onForgot={() => { setUnlock(false); refreshAuth(); startWizard('pin'); }} />
                    <button type="button" onClick={() => { setUnlock(false); setLockClosed(true); }}>{t('lock.screen.close')}</button>
                  </section>
                ) : null}
                {simple ? <Simple /> : <Screen />}
              </>
            ) : null}
          </>
        )}
      </main>
    </div>
  );
}
