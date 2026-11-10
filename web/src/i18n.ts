// SPDX-License-Identifier: AGPL-3.0-or-later
// All user text is in i18n/en.json and i18n/tr.json (TASK 03-00-00-008).
import { useEffect, useState } from 'preact/hooks';
import en from '../i18n/en.json';
import tr from '../i18n/tr.json';

export type Lang = 'en' | 'tr';

export const dictionaries: Record<Lang, Record<string, string>> = { en, tr };

/** osLang is the OS language if it is Turkish, else English. */
export function osLang(): Lang {
  const l = (typeof navigator !== 'undefined' && navigator.language) || '';
  return l.toLowerCase().startsWith('tr') ? 'tr' : 'en';
}

let current: Lang = osLang();
const listeners = new Set<() => void>();

export function lang(): Lang {
  return current;
}

export function setLang(l: Lang): void {
  if (l === current) return;
  current = l;
  if (typeof document !== 'undefined') document.documentElement.lang = l;
  listeners.forEach((f) => f());
}

/** t returns the text of key with {name} values replaced. */
export function t(key: string, vars?: Record<string, string | number>): string {
  let s = dictionaries[current][key] ?? dictionaries.en[key] ?? key;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) s = s.split('{' + k + '}').join(String(v));
  }
  return s;
}

/** useLang renders the component again after a language change. */
export function useLang(): Lang {
  const [, setN] = useState(0);
  useEffect(() => {
    const f = () => setN((n) => n + 1);
    listeners.add(f);
    return () => {
      listeners.delete(f);
    };
  }, []);
  return current;
}
