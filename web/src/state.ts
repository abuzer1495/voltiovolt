// SPDX-License-Identifier: AGPL-3.0-or-later
// The GUI state: connection, auth, the last push messages of vltd.
import { api, events, on, type AuthInfo, type ConnInfo, type Countdown, type PolicyInfo, type StatusPush, type VltEvent } from './api';
import { lang, onLangChange, osLang, setLang, t, type Lang } from './i18n';
import { createStore } from './store';

export interface AppState {
  conn: ConnInfo;
  auth: AuthInfo | null;
  status: StatusPush | null;
  countdown: Countdown | null;
  events: VltEvent[];
  policy: PolicyInfo | null;
}

export const app = createStore<AppState>({
  conn: { state: 'connecting', since: '' },
  auth: null,
  status: null,
  countdown: null,
  events: [],
  policy: null,
});

/** LAST_EVENTS is the number of Events on the Dashboard. */
export const LAST_EVENTS = 10;

let started = false;

/** start subscribes to the Wails events and reads the first state. */
export function start(): void {
  if (started) return;
  started = true;
  on(events.conn, (d) => {
    const c = d as ConnInfo;
    app.set({ conn: c });
    if (c.state === 'connected') onConnected();
    else app.set({ auth: null });
  });
  on(events.auth, (d) => app.set({ auth: d as AuthInfo }));
  on(events.status, (d) => {
    const st = d as StatusPush;
    app.set({ status: st, countdown: st.countdown ?? app.get().countdown });
  });
  on(events.countdown, (d) => app.set({ countdown: d as Countdown }));
  // A new Event goes to the top of the list. No request to vltd: a
  // request while elevated starts the idle time of vltd again.
  on(events.event, (d) => app.set({ events: [d as VltEvent, ...app.get().events].slice(0, LAST_EVENTS) }));
  sendTrayTexts();
  onLangChange(sendTrayTexts);
  void api.connection().then((c) => {
    app.set({ conn: c });
    if (c.state === 'connected') onConnected();
  }, () => undefined);
  void api.last().then(
    (l) => app.set({ status: (l.status as StatusPush) ?? null, countdown: (l.countdown as Countdown) ?? (l.status as StatusPush)?.countdown ?? null }),
    () => undefined,
  );
}

export function refreshAuth(): void {
  void api.authStatus().then((a) => {
    const was = app.get().auth;
    app.set({ auth: a });
    if (a.pin_set && !was?.pin_set) {
      refreshEvents();
      refreshPolicy();
    }
  }, () => undefined);
}

/** TOUCH_MS is the shortest time between two keep-alive calls (VLT-IS-011 OD-52). */
export const TOUCH_MS = 30_000;
let lastTouch = 0;

/**
 * userInput is called after keyboard or mouse input in the VLT GUI. It
 * starts the idle time of the engine again with auth.touch, at most one
 * time in each 30 s and only when elevated (OD-52). Polling and push
 * messages do not call it.
 */
export function userInput(now: number = Date.now()): void {
  const a = app.get().auth;
  if (!a?.pin_set || !a.elevated) return;
  if (lastTouch !== 0 && now - lastTouch < TOUCH_MS) return;
  lastTouch = now;
  void api.touch().catch(() => undefined);
}

/** resetTouch is for tests. */
export function resetTouch(): void {
  lastTouch = 0;
}

let inputWatch = false;

/** watchInput adds the keyboard and mouse listeners of OD-52 (one time). */
export function watchInput(target: Pick<Window, 'addEventListener'> = window): void {
  if (inputWatch) return;
  inputWatch = true;
  const f = () => userInput();
  for (const ev of ['keydown', 'pointerdown', 'wheel']) target.addEventListener(ev, f, { passive: true, capture: true });
}

/** lock calls auth.logout (the "Lock" button). */
export async function lock(): Promise<void> {
  try {
    await api.logout();
  } finally {
    refreshAuth();
  }
}

/** onConnected reads the auth state, then (with a PIN) the last Events and the policy. */
function onConnected(): void {
  void api.authStatus().then((a) => {
    app.set({ auth: a });
    if (a.pin_set) {
      refreshEvents();
      refreshPolicy();
      loadLanguage();
    }
  }, () => undefined);
}

/** loadLanguage reads ui.language. Default: the OS language if it is Turkish, else English. */
export function loadLanguage(): void {
  void api.settings().then((r) => {
    const s = (r.settings ?? []).find((x) => x.key === 'ui.language');
    const v = s && !s.default && (s.value === 'tr' || s.value === 'en') ? (s.value as Lang) : osLang();
    setLang(v);
  }, () => undefined);
}

export function refreshEvents(): void {
  void api.events(0, LAST_EVENTS).then((p) => app.set({ events: p?.events ?? [] }), () => undefined);
}

export function refreshPolicy(): void {
  void api.policy().then((p) => app.set({ policy: p }), () => undefined);
}

function sendTrayTexts(): void {
  void lang;
  void api
    .setTrayTexts({
      mains: t('tray.mains'),
      battery: t('tray.battery'),
      critical: t('tray.critical'),
      offline: t('tray.offline'),
      runtime: t('tray.runtime'),
      open: t('tray.open'),
      lock: t('tray.lock'),
      quit: t('tray.quit'),
    })
    .catch(() => undefined);
}
