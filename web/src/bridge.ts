// SPDX-License-Identifier: AGPL-3.0-or-later
// The bridge to the Go side of the VLT GUI: the Wails bindings and the
// Wails events. The front end has no other access to vltd (TASK
// 03-00-00-002 step 4.2). The Wails runtime is served by the VLT GUI at
// /wails/runtime.js; it is not an npm dependency.

interface WailsRuntime {
  Call: { ByName(name: string, ...args: unknown[]): Promise<unknown> };
  Events: { On(name: string, cb: (ev: { name: string; data: unknown }) => void): () => void };
}

const RUNTIME_URL = '/wails/runtime.js';
const SERVICE = 'github.com/abuzer1495/voltiovolt/internal/gui.API.';

let rt: Promise<WailsRuntime> | null = null;

function runtime(): Promise<WailsRuntime> {
  if (!rt) {
    rt = import(/* @vite-ignore */ RUNTIME_URL) as Promise<WailsRuntime>;
  }
  return rt;
}

export async function call<T>(method: string, ...args: unknown[]): Promise<T> {
  const r = await runtime();
  return (await r.Call.ByName(SERVICE + method, ...args)) as T;
}

export function on(name: string, cb: (data: unknown) => void): () => void {
  let off: (() => void) | null = null;
  let stopped = false;
  void runtime().then((r) => {
    if (!stopped) off = r.Events.On(name, (ev) => cb(ev.data));
  });
  return () => {
    stopped = true;
    if (off) off();
  };
}
