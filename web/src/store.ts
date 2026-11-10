// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'preact/hooks';

export interface Store<T> {
  get(): T;
  set(p: Partial<T>): void;
  sub(f: () => void): () => void;
}

export function createStore<T extends object>(init: T): Store<T> {
  let v = init;
  const subs = new Set<() => void>();
  return {
    get: () => v,
    set: (p) => {
      v = { ...v, ...p };
      subs.forEach((f) => f());
    },
    sub: (f) => {
      subs.add(f);
      return () => {
        subs.delete(f);
      };
    },
  };
}

export function useStore<T extends object>(s: Store<T>): T {
  const [, setN] = useState(0);
  useEffect(() => s.sub(() => setN((n) => n + 1)), [s]);
  return s.get();
}
