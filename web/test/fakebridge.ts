// SPDX-License-Identifier: AGPL-3.0-or-later
// A fake of src/bridge.ts for the component tests: no Wails runtime, no
// Go side. The tests set handlers and read the calls.
export const calls: { method: string; args: unknown[] }[] = [];
export const handlers: Record<string, (...args: unknown[]) => unknown> = {};
const subs: Record<string, ((d: unknown) => void)[]> = {};

export async function call<T>(method: string, ...args: unknown[]): Promise<T> {
  calls.push({ method, args });
  const h = handlers[method];
  if (!h) return undefined as T;
  return (await h(...args)) as T;
}

export function on(name: string, cb: (data: unknown) => void): () => void {
  (subs[name] ??= []).push(cb);
  return () => {
    subs[name] = (subs[name] ?? []).filter((f) => f !== cb);
  };
}

export function emit(name: string, data: unknown): void {
  for (const f of subs[name] ?? []) f(data);
}

export function reset(): void {
  calls.length = 0;
  for (const k of Object.keys(handlers)) delete handlers[k];
}

/** goError is an error like a Go error of a binding: the message is the code. */
export function goError(code: string): Error {
  return new Error(code);
}
