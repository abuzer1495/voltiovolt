// SPDX-License-Identifier: AGPL-3.0-or-later
// The "Interface size" choice (VLT-IS-011 TASK 03-00-00-004, OD-54). It
// applies at once, with no restart. No PIN is necessary ("ui." Setting).
import { lang, t } from '../i18n';
import { SCALES, closeNotice, parseScale, scale, setScale } from '../scale';
import { useStore } from '../store';

/** pct writes a percent: "%125" in Turkish, "125 %" in English. */
export function pct(n: number): string {
  return lang() === 'tr' ? `%${n}` : `${n} %`;
}

export function ScaleChoice({ id = 'ui-scale' }: { id?: string }) {
  const s = useStore(scale);
  const autoPct = typeof window !== 'undefined' ? Math.round((window.devicePixelRatio || 1) * 100) : 100;
  return (
    <div class="scale-choice">
      <label for={id}>{t('scale.title')}</label>
      <select id={id} value={String(s.setting)} onChange={(e) => {
        const v = parseScale((e.target as HTMLSelectElement).value);
        if (v) void setScale(v);
      }}>
        <option value="auto">{t('scale.auto', { n: autoPct })}</option>
        {SCALES.map((p) => <option value={String(p)}>{pct(p)}</option>)}
      </select>
      <p class="small">{t('scale.help')}</p>
      {s.reduced ? <p class="small" role="status">{t('scale.used', { n: s.percent })}</p> : null}
    </div>
  );
}

/** ScaleNotice is shown one time when the screen is too small for the size. */
export function ScaleNotice() {
  const s = useStore(scale);
  if (!s.notice) return null;
  const want = s.setting === 'auto' ? t('scale.auto.short') : pct(s.setting);
  return (
    <div class="banner banner-warn" role="status">
      <span aria-hidden="true" class="banner-icon">⚠</span>
      <div>{t('scale.reduced', { want, used: s.percent })}</div>
      <button type="button" onClick={() => closeNotice()}>{t('lock.screen.close')}</button>
    </div>
  );
}
