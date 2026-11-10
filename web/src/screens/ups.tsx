// SPDX-License-Identifier: AGPL-3.0-or-later
// UPS Control screen (VLT-IS-006 TASK 03-00-00-008 step 1): battery test,
// cancel test, beeper. Each button needs the elevated state and the
// function in ups.control.capabilities. There is no button for the
// shutdown commands (SEC-098).
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type UPSCaps, type UPSControlResult } from '../api';
import { Badge, Confirm, errText } from '../components';
import { DASH, time } from '../format';
import { t } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';

export const START_TEXT = 'START TEST';
export const UNTIL_LOW_TEXT = 'TEST UNTIL LOW';

type Msg = { kind: 'ok' | 'error'; text: string } | null;

function resultText(r: UPSControlResult): string {
  if (r.no_send) return t('ups.r.nosend');
  return t(r.verified ? 'ups.r.verified' : 'ups.r.notverified', { bytes: r.bytes, sends: r.sends });
}

export function UPSControl() {
  const s = useStore(app);
  const elevated = !!s.auth?.elevated;
  const [caps, setCaps] = useState<UPSCaps | null>(null);
  const [minutes, setMinutes] = useState('1');
  const [ask, setAsk] = useState<null | '10s' | 'minutes'>(null);
  const [low, setLow] = useState(false);
  const [lowText, setLowText] = useState('');
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);
  const load = () => api.upsCaps().then(setCaps, (e) => setMsg({ kind: 'error', text: errText(errorCode(e)) }));
  useEffect(() => {
    void load();
  }, []);
  const has = (f: string) => !!caps?.functions?.includes(f);
  const off = (f: string) => !elevated || !has(f) || busy;
  async function run(p: () => Promise<UPSControlResult>) {
    setBusy(true);
    setMsg(null);
    try {
      const r = await p();
      setMsg({ kind: r.verified ? 'ok' : 'error', text: resultText(r) });
    } catch (e) {
      setMsg({ kind: 'error', text: errText(errorCode(e)) });
    } finally {
      setBusy(false);
      void load();
    }
  }
  const n = Number(minutes.trim());
  const minutesOK = /^\d+$/.test(minutes.trim()) && n >= 1 && n <= 99;
  const lt = caps?.last_test;
  return (
    <div class="screen">
      <h2>{t('nav.ups')}</h2>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      {caps && !caps.connected ? <p class="note">{t('ups.noups')}</p> : null}
      <section class="card" aria-labelledby="ups-test-title">
        <h3 id="ups-test-title">{t('ups.test')}</h3>
        <div class="buttons">
          <button type="button" disabled={off('test.10s')} onClick={() => setAsk('10s')}>{t('ups.test.10s')}</button>
        </div>
        <div class="form">
          <label for="ups-minutes">{t('ups.test.minutes.label')}</label>
          <input id="ups-minutes" inputMode="numeric" value={minutes} disabled={off('test.minutes')} aria-invalid={minutesOK ? undefined : true}
            aria-describedby="ups-minutes-hint" onInput={(e) => setMinutes((e.target as HTMLInputElement).value)} />
          <span id="ups-minutes-hint" class="hint">{t('policy.range', { min: 1, max: 99, unit: t('unit.min') })}</span>
          <button type="button" disabled={off('test.minutes') || !minutesOK} onClick={() => setAsk('minutes')}>{t('ups.test.minutes')}</button>
        </div>
        <div class="buttons">
          <button type="button" class="danger" disabled={off('test.until_low')} onClick={() => { setLow(true); setLowText(''); }}>{t('ups.test.low')}</button>
          <button type="button" disabled={off('test.cancel')} onClick={() => void run(() => api.upsTestCancel())}>{t('ups.test.cancel')}</button>
        </div>
        {low ? (
          <form class="form" onSubmit={(e) => { e.preventDefault(); if (lowText !== UNTIL_LOW_TEXT) return; setLow(false); void run(() => api.upsTestStart('until_low', 0, UNTIL_LOW_TEXT)); }}>
            <div class="banner banner-warn" role="note">
              <span aria-hidden="true" class="banner-icon">⚠</span>
              <span>{t('ups.test.low.warning')}</span>
            </div>
            <label for="ups-low-text">{t('ups.test.low.label', { text: UNTIL_LOW_TEXT })}</label>
            <input id="ups-low-text" autoComplete="off" value={lowText} onInput={(e) => setLowText((e.target as HTMLInputElement).value)} />
            <button type="submit" class="danger" disabled={lowText !== UNTIL_LOW_TEXT || off('test.until_low')}>{t('ups.test.low.start')}</button>
            <button type="button" onClick={() => setLow(false)}>{t('common.cancel')}</button>
          </form>
        ) : null}
        {ask ? (
          <Confirm text={t('ups.test.confirm')} confirmLabel={t('ups.test.start')}
            onConfirm={() => { const k = ask; setAsk(null); void run(() => api.upsTestStart(k, k === 'minutes' ? n : 0, START_TEXT)); }}
            onCancel={() => setAsk(null)} />
        ) : null}
        <h4>{t('ups.last')}</h4>
        {lt ? (
          <p>
            {lt.result === 'TEST_PASSED' ? <Badge kind="ok" icon="✓" text={t('ups.passed')} /> : <Badge kind="crit" icon="✕" text={t('ups.failed')} />}{' '}
            {time(lt.end)}
            {lt.cancelled ? ' · ' + t('ups.cancelled') : ''}
            {lt.battery_low ? ' · ' + t('ups.battery_low') : ''}
            {lt.ups_failed ? ' · ' + t('ups.ups_failed') : ''}
          </p>
        ) : <p>{DASH}</p>}
      </section>
      <section class="card" aria-labelledby="ups-beeper-title">
        <h3 id="ups-beeper-title">{t('ups.beeper')}</h3>
        <div class="buttons">
          <button type="button" disabled={off('beeper')} onClick={() => void run(() => api.upsBeeperSet(true))}>{t('ups.beeper.on')}</button>
          <button type="button" disabled={off('beeper')} onClick={() => void run(() => api.upsBeeperSet(false))}>{t('ups.beeper.off')}</button>
        </div>
      </section>
      {busy ? <p role="status">{t('ups.busy')}</p> : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </div>
  );
}
