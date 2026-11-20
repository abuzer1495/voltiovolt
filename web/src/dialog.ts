// SPDX-License-Identifier: AGPL-3.0-or-later
// Dialog behaviour (VLT-IS-012 TASK 03-00-00-001, R-13 and R-15).
//
// 1. While a modal dialog is open, the page behind it does not scroll:
//    the class "modal-open" on <html> locks the page scroll (styles.css).
// 2. The mouse wheel over a dialog scrolls only an element of the dialog
//    that can scroll. The wheel over the dimmed area does nothing.
// 3. A text selection is cleared when a dialog opens or closes.
import type { RefObject } from 'preact';
import { useEffect } from 'preact/hooks';

let open = 0;

/** clearSelection removes any text selection of the page. */
export function clearSelection(): void {
  try {
    window.getSelection()?.removeAllRanges();
  } catch {
    /* no selection API */
  }
}

/** lockPage locks the page scroll. Each call needs one unlockPage call. */
export function lockPage(): void {
  open++;
  document.documentElement.classList.add('modal-open');
}

/** unlockPage releases one lock of lockPage. */
export function unlockPage(): void {
  open = Math.max(0, open - 1);
  if (open === 0) document.documentElement.classList.remove('modal-open');
}

/** pageLocked tells if a modal dialog locks the page (for tests). */
export function pageLocked(): boolean {
  return document.documentElement.classList.contains('modal-open');
}

function canScroll(el: Element, dy: number): boolean {
  const s = getComputedStyle(el);
  if (!/(auto|scroll)/.test(s.overflowY)) return false;
  const e = el as HTMLElement;
  if (e.scrollHeight <= e.clientHeight) return false;
  if (dy < 0) return e.scrollTop > 0;
  if (dy > 0) return e.scrollTop + e.clientHeight < e.scrollHeight;
  return false;
}

/**
 * wheelGuard is the wheel handler of an overlay. It stops the wheel unless
 * an element between the target and the overlay can scroll in that
 * direction.
 */
export function wheelGuard(overlay: Element, e: WheelEvent): void {
  let el = e.target instanceof Element ? e.target : null;
  while (el && el !== overlay) {
    if (canScroll(el, e.deltaY)) return;
    el = el.parentElement;
  }
  e.preventDefault();
}

/** useModal applies the rules 1 to 3 to the overlay of a modal dialog. */
export function useModal(ref: RefObject<HTMLElement | null>): void {
  useEffect(() => {
    const el = ref.current;
    lockPage();
    clearSelection();
    const h = (e: WheelEvent) => {
      if (el) wheelGuard(el, e);
    };
    el?.addEventListener('wheel', h, { passive: false });
    return () => {
      el?.removeEventListener('wheel', h);
      unlockPage();
      clearSelection();
    };
  }, []);
}

/** useClearSelection clears the selection when an inline dialog opens and closes. */
export function useClearSelection(): void {
  useEffect(() => {
    clearSelection();
    return () => clearSelection();
  }, []);
}
