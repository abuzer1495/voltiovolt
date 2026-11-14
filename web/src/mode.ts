// SPDX-License-Identifier: AGPL-3.0-or-later
// Simple and Expert mode (VLT-IS-011 TASK 03-00-00-003, OD-53). The mode is
// the "ui." Setting ui.mode; the default is Simple. A "ui." Setting needs
// no elevated state. Without a store (for example no PIN in service mode)
// the choice is kept in localStorage.
import { api } from './api';
import { createStore } from './store';

export type UIMode = 'simple' | 'expert';
export const UI_MODE_KEY = 'ui.mode';
const LOCAL_KEY = 'vlt.ui-mode';

export const ui = createStore<{ mode: UIMode; loaded: boolean }>({ mode: 'simple', loaded: false });

function parse(v: unknown): UIMode | null {
  return v === 'simple' || v === 'expert' ? v : null;
}

function local(): UIMode | null {
  try {
    return parse(typeof localStorage !== 'undefined' ? localStorage.getItem(LOCAL_KEY) : null);
  } catch {
    return null;
  }
}

/** loadMode reads ui.mode from the Settings of vltd. */
export async function loadMode(): Promise<UIMode> {
  let m: UIMode | null = null;
  let stored = false;
  try {
    const r = await api.settings();
    const s = (r?.settings ?? []).find((x) => x.key === UI_MODE_KEY);
    if (s && !s.default) stored = true;
    m = parse(s?.value);
  } catch {
    /* no Settings: use the local choice */
  }
  if (!stored) m = local() ?? m;
  const mode = m ?? 'simple';
  ui.set({ mode, loaded: true });
  return mode;
}

/** setMode changes the mode at once and stores it. */
export async function setMode(mode: UIMode): Promise<void> {
  ui.set({ mode, loaded: true });
  try {
    localStorage.setItem(LOCAL_KEY, mode);
  } catch {
    /* no local store */
  }
  try {
    await api.setSetting(UI_MODE_KEY, mode);
  } catch {
    /* kept in localStorage */
  }
}
