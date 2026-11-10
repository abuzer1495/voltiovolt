// SPDX-License-Identifier: AGPL-3.0-or-later
// The GUI state: connection, auth, the last push messages of vltd.
import { api, events, on, type ConnInfo } from './api';
import { createStore } from './store';

export interface AppState {
  conn: ConnInfo;
  status: unknown;
  countdown: unknown;
}

export const app = createStore<AppState>({
  conn: { state: 'connecting', since: '' },
  status: null,
  countdown: null,
});

let started = false;

/** start subscribes to the Wails events and reads the first state. */
export function start(): void {
  if (started) return;
  started = true;
  on(events.conn, (d) => app.set({ conn: d as ConnInfo }));
  on(events.status, (d) => app.set({ status: d }));
  on(events.countdown, (d) => app.set({ countdown: d }));
  void api.connection().then((c) => app.set({ conn: c }), () => undefined);
  void api.last().then((l) => app.set({ status: l.status ?? null, countdown: l.countdown ?? null }), () => undefined);
}
