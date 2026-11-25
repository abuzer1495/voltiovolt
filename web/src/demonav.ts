// SPDX-License-Identifier: AGPL-3.0-or-later
// Demo / test-build navigation for CI screenshots. The Go side sets
// window.__VLT_DEMO_NAV__ only in non-production builds when --demo or
// --demo-screen is used. Production builds never set the flag, and this
// module does nothing without it.

import { startWizard, type Step, STEPS } from './screens/wizard';
import { setMode, type UIMode } from './mode';
import { api } from './api';

export type DemoNav = {
  route?: string;
  mode?: UIMode;
  wizard?: Step | true;
  help?: boolean;
};

declare global {
  interface Window {
    __VLT_DEMO_NAV__?: boolean;
    __VLT_DEMO_HELP__?: boolean;
    applyVltDemoNav?: () => void;
  }
}

/** parseDemoHash reads a test-only location hash such as #device or #wizard/pin. */
export function parseDemoHash(hash: string): DemoNav {
  const h = hash.replace(/^#\/?/, '').trim();
  if (!h) return {};
  if (h === 'simple' || h === 'expert') return { mode: h, route: 'dashboard' };
  if (h === 'help') return { route: 'device', help: true };
  if (h === 'wizard') return { wizard: true };
  if (h.startsWith('wizard/')) {
    const step = h.slice('wizard/'.length) as Step;
    if ((STEPS as readonly string[]).includes(step)) return { wizard: step };
    return { wizard: true };
  }
  return { route: h };
}

/** applyDemoNav applies the hash when the Go side has enabled demo navigation. */
export function applyDemoNav(): void {
  if (typeof window === 'undefined' || !window.__VLT_DEMO_NAV__) return;
  const nav = parseDemoHash(location.hash);
  if (nav.help) window.__VLT_DEMO_HELP__ = true;
  if (nav.mode) setMode(nav.mode);
  if (nav.wizard) startWizard(nav.wizard === true ? undefined : nav.wizard);
  if (nav.route && nav.route !== 'simple' && nav.route !== 'expert') {
    const want = '#/' + nav.route.replace(/^\//, '');
    // currentRoute() accepts "#device" or "#/device".
    if (location.hash.replace(/^#\/?/, '') !== nav.route) {
      location.hash = '#' + nav.route;
    } else if (want && location.hash === '') {
      location.hash = '#' + nav.route;
    }
  }
}

if (typeof window !== 'undefined') {
  window.applyVltDemoNav = applyDemoNav;
}

/** bootDemoNav asks the Go side whether this is a screenshot run, applies
 * the hash, and signals that the window is ready to capture. */
export async function bootDemoNav(): Promise<void> {
  try {
    const d = await api.demoNav();
    if (!d?.active) return;
    window.__VLT_DEMO_NAV__ = true;
    const screen = (d.screen || 'dashboard').replace(/^#/, '');
    if (screen === 'help') window.__VLT_DEMO_HELP__ = true;
    if (screen) location.hash = '#' + screen;
    applyDemoNav();
    await api.demoShotReady();
  } catch {
    /* production, or the method is not present */
  }
}
