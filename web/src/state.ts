// SPDX-License-Identifier: AGPL-3.0-or-later
// The GUI state: connection, auth, the last push messages of vltd.
import { api, events, on, type AuthInfo, type ConnInfo } from './api';
import { createStore } from './store';

export interface AppState {
  conn: ConnInfo;
  auth: AuthInfo | null;
  status: unknown;
  countdown: unknown;
}

export const app = createStore<AppState>({
  conn: { state: 'connecting', since: '' },
  auth: null,
  status: null,
  countdown: null,
});

let started = false;

/** start subscribes to the Wails events and reads the first state. */
export function start(): void {
  if (started) return;
  started = true;
  on(events.conn, (d) => {
    const c = d as ConnInfo;
    app.set({ conn: c });
    if (c.state === 'connected') refreshAuth();
    else app.set({ auth: null });
  });
  on(events.auth, (d) => app.set({ auth: d as AuthInfo }));
  on(events.status, (d) => app.set({ status: d }));
  on(events.countdown, (d) => app.set({ countdown: d }));
  void api.connection().then((c) => {
    app.set({ conn: c });
    if (c.state === 'connected') refreshAuth();
  }, () => undefined);
  void api.last().then((l) => app.set({ status: l.status ?? null, countdown: l.countdown ?? null }), () => undefined);
}

export function refreshAuth(): void {
  void api.authStatus().then((a) => app.set({ auth: a }), () => undefined);
}

/** lock calls auth.logout (the "Lock" button). */
export async function lock(): Promise<void> {
  try {
    await api.logout();
  } finally {
    refreshAuth();
  }
}
