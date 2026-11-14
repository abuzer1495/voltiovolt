// SPDX-License-Identifier: AGPL-3.0-or-later
// Icons of the left navigation pane (VLT-IS-011 TASK 03-00-00-005 step
// 1.4). Simple original line icons in SVG, drawn with the text color
// (currentColor), so they follow the theme. Decorative: the link text is
// the accessible name.
import type { JSX } from 'preact';

const P: Record<string, JSX.Element> = {
  dashboard: <><rect x="3" y="3" width="6" height="6" rx="1" /><rect x="11" y="3" width="6" height="6" rx="1" /><rect x="3" y="11" width="6" height="6" rx="1" /><rect x="11" y="11" width="6" height="6" rx="1" /></>,
  device: <><path d="M7 2v4M13 2v4" /><path d="M5 6h10v3a5 5 0 0 1-10 0z" /><path d="M10 14v4" /></>,
  ups: <><rect x="4" y="5" width="12" height="13" rx="1.5" /><path d="M8 3h4" /><path d="M11 8l-3 4h3l-1 3" /></>,
  history: <><path d="M3 3v14h14" /><path d="M6 13l3-4 3 2 4-6" /></>,
  events: <><path d="M7 5h10M7 10h10M7 15h10" /><circle cx="3.5" cy="5" r="0.8" /><circle cx="3.5" cy="10" r="0.8" /><circle cx="3.5" cy="15" r="0.8" /></>,
  policy: <><path d="M10 2v7" /><path d="M6 4.5a7 7 0 1 0 8 0" /></>,
  notifications: <><path d="M5 14V9a5 5 0 0 1 10 0v5l1.5 2h-13z" /><path d="M8.5 18a1.5 1.5 0 0 0 3 0" /></>,
  security: <><rect x="4" y="9" width="12" height="9" rx="1.5" /><path d="M7 9V6a3 3 0 0 1 6 0v3" /></>,
  general: <><path d="M3 5h14M3 10h14M3 15h14" /><circle cx="7" cy="5" r="1.6" /><circle cx="13" cy="10" r="1.6" /><circle cx="9" cy="15" r="1.6" /></>,
  menu: <><path d="M3 5h14M3 10h14M3 15h14" /></>,
};

export const ICONS = Object.keys(P);

export function NavIcon({ name }: { name: string }) {
  return (
    <svg class="nav-icon" viewBox="0 0 20 20" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
      {P[name] ?? P.general}
    </svg>
  );
}
