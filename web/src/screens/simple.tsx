// SPDX-License-Identifier: AGPL-3.0-or-later
// Simple mode (VLT-IS-011 TASK 03-00-00-003, OD-53): one screen with a large
// status card, tiles, the automatic shutdown, a 1 h live chart, the last 3
// Events in plain language and a small settings panel. It has no elevated
// settings. The countdown banner with Cancel is the same as in Expert mode.
import { useState } from 'preact/hooks';
import type { Countdown, PolicyInfo, StatusPush, VltEvent } from '../api';
import { DASH, num, time, value } from '../format';
import { t } from '../i18n';
import { setMode, ui, type UIMode } from '../mode';
import { app } from '../state';
import { useStore } from '../store';
import { LanguageSettings } from './general';
import { History } from './history';
import { ScaleChoice } from './scalechoice';
import { startWizard } from './wizard';

export type Health = 'good' | 'attention' | 'problem';

export interface HealthReason {
  key: string;
  vars?: Record<string, string | number>;
}

export interface HealthInfo {
  level: Health;
  reasons: HealthReason[];
}

/** LOAD_ATTENTION and the other limits are the rules of TASK 03-00-00-003. */
export const LOAD_ATTENTION = 90;
export const LOAD_PROBLEM = 100;
export const TEMP_ATTENTION = 45;

/**
 * health is the state of the large status card.
 * - Good: mains power, no WARNING or CRITICAL condition active.
 * - Attention: on battery, AVR, bypass, load >= 90 %, temperature >= 45 °C,
 *   communication lost (and the other WARNING conditions).
 * - Problem: battery low, UPS fault, overload >= 100 %, over temperature,
 *   a shutdown countdown, the UPS shutdown.
 */
export function health(s: StatusPush | null, cd: Countdown | null): HealthInfo {
  const problem: HealthReason[] = [];
  const attention: HealthReason[] = [];
  if (cd?.active) problem.push({ key: 'health.countdown' });
  if (!s || s.device?.state !== 'connected' || s.ups?.comm_lost) {
    attention.push({ key: 'health.comm_lost' });
    return { level: problem.length ? 'problem' : 'attention', reasons: [...problem, ...attention] };
  }
  const active = new Set(s.ups?.active ?? []);
  const m = s.measurements ?? {};
  const load = m['load.percent']?.value;
  const temp = m['ups.temperature']?.value;
  if (s.ups?.low_battery || active.has('LOW_BATTERY')) problem.push({ key: 'health.low_battery' });
  if (active.has('UPS_FAULT')) problem.push({ key: 'health.fault' });
  if (active.has('OVERLOAD') || (load !== undefined && load >= LOAD_PROBLEM)) problem.push({ key: 'health.overload', vars: { n: num(load, 0) } });
  if (active.has('OVER_TEMPERATURE')) problem.push({ key: 'health.over_temp', vars: { n: num(temp, 0) } });
  if (active.has('UPS_SHUTDOWN_ACTIVE')) problem.push({ key: 'health.ups_shutdown' });
  if (s.ups?.on_battery || active.has('ON_BATTERY')) attention.push({ key: 'health.on_battery' });
  if (active.has('AVR_ACTIVE')) attention.push({ key: 'health.avr' });
  if (active.has('BYPASS_ACTIVE')) attention.push({ key: 'health.bypass' });
  if (load !== undefined && load >= LOAD_ATTENTION && load < LOAD_PROBLEM && !active.has('OVERLOAD')) attention.push({ key: 'health.load_high', vars: { n: num(load, 0) } });
  if (temp !== undefined && temp >= TEMP_ATTENTION && !active.has('OVER_TEMPERATURE')) attention.push({ key: 'health.temp_high', vars: { n: num(temp, 0) } });
  if (active.has('RUNTIME_LOW')) attention.push({ key: 'health.runtime_low' });
  if (problem.length) return { level: 'problem', reasons: [...problem, ...attention] };
  if (attention.length) return { level: 'attention', reasons: attention };
  return { level: 'good', reasons: [{ key: 'health.good.sentence' }] };
}

const HEALTH_ICON: Record<Health, string> = { good: '✓', attention: '⚠', problem: '⛔' };
const HEALTH_KIND: Record<Health, string> = { good: 'ok', attention: 'warn', problem: 'crit' };

/** plainEvent is one Event in plain language, for example "Power failure". */
export function plainEvent(e: VltEvent): string {
  const key = 'plain.' + e.type;
  const title = t(key) === key ? e.type : t(key);
  return e.data?.active === false ? t('plain.ended', { title }) : title;
}

/** triggerSentence is the trigger of the automatic shutdown in one sentence. */
export function triggerSentence(p: PolicyInfo | null): string {
  const st = p?.settings;
  if (!st) return DASH;
  const action = st.action === 'hibernate' ? t('simple.action.hibernate') : t('simple.action.shutdown');
  return t('simple.trigger', {
    action,
    battery: Math.round(Number(st['trigger.on_battery_seconds'] ?? 0) / 60),
    runtime: Number(st['trigger.runtime_below_minutes'] ?? 0),
    charge: Number(st['trigger.charge_below_percent'] ?? 0),
  });
}

/** ModeChoice switches between Simple and Expert. No PIN is necessary. */
export function ModeChoice({ name = 'ui-mode' }: { name?: string }) {
  const m = useStore(ui).mode;
  const pick = (v: UIMode) => void setMode(v);
  return (
    <fieldset class="radio">
      <legend>{t('mode.ui.title')}</legend>
      <label class="check"><input type="radio" name={name} checked={m === 'simple'} onChange={() => pick('simple')} /> {t('mode.ui.simple')}</label>
      <label class="check"><input type="radio" name={name} checked={m === 'expert'} onChange={() => pick('expert')} /> {t('mode.ui.expert')}</label>
      <p class="small">{t('mode.ui.help')}</p>
    </fieldset>
  );
}

function Tile({ label, text }: { label: string; text: string }) {
  return (
    <div class="tile">
      <div class="tile-label">{label}</div>
      <div class="tile-value">{text}</div>
    </div>
  );
}

export function Simple() {
  const s = useStore(app);
  const [panel, setPanel] = useState(false);
  const h = health(s.status, s.countdown);
  const m = s.status?.measurements ?? {};
  const charge = m['battery.charge'] ?? m['battery.charge_estimate'];
  const auto = s.status ? !!s.status.policy_active : !!s.policy?.active;
  return (
    <div class="screen simple">
      <h2 class="sr-only">{t('mode.ui.simple')}</h2>
      <section class={'card status-card status-' + HEALTH_KIND[h.level]} aria-labelledby="health-title" data-health={h.level}>
        <span class="status-icon" aria-hidden="true">{HEALTH_ICON[h.level]}</span>
        <div role="status" aria-live="polite">
          <h3 id="health-title">{t('health.' + h.level)}</h3>
          <p class="status-sentence">{t(h.reasons[0].key, h.reasons[0].vars)}</p>
          {h.reasons.length > 1 ? (
            <ul class="status-more">
              {h.reasons.slice(1).map((r) => <li>{t(r.key, r.vars)}</li>)}
            </ul>
          ) : null}
        </div>
      </section>
      <section class="tiles" aria-label={t('dash.values')}>
        <Tile label={t('dash.battery_charge')} text={value(charge, 0)} />
        <Tile label={t('dash.load')} text={value(m['load.percent'], 0)} />
        <Tile label={t('dash.runtime')} text={value(m['battery.runtime'], 0, t('unit.min'))} />
        <Tile label={t('dash.input_voltage')} text={value(m['input.voltage'], 0)} />
        <Tile label={t('dash.output_voltage')} text={value(m['output.voltage'], 0)} />
        <Tile label={t('simple.health')} text={t('health.' + h.level)} />
      </section>
      <section class="card" aria-labelledby="auto-title">
        <h3 id="auto-title">{t('simple.auto')}</h3>
        <p>
          <strong>{auto ? t('simple.auto.on') : t('simple.auto.off')}</strong>
          {auto ? <> {triggerSentence(s.policy)}</> : null}
        </p>
      </section>
      <section class="card" aria-labelledby="simple-chart-title">
        <h3 id="simple-chart-title">{t('simple.chart')}</h3>
        <History simple />
      </section>
      <section class="card" aria-labelledby="simple-events-title">
        <h3 id="simple-events-title">{t('simple.events')}</h3>
        {s.events.length === 0 ? <p>{t('event.none')}</p> : (
          <ul class="simple-events">
            {s.events.slice(0, 3).map((e) => <li><span class="small">{time(e.ts)}</span> {plainEvent(e)}</li>)}
          </ul>
        )}
      </section>
      <div class="row">
        <button type="button" onClick={() => void setMode('expert')}>{t('mode.ui.to_expert')}</button>
        <button type="button" aria-expanded={panel} onClick={() => setPanel(!panel)}>{t('simple.settings')}</button>
      </div>
      {panel ? (
        <section class="card simple-settings" aria-labelledby="simple-settings-title">
          <h3 id="simple-settings-title">{t('simple.settings')}</h3>
          <LanguageSettings />
          <ScaleChoice id="ui-scale-simple" />
          <ModeChoice name="ui-mode-simple" />
          <button type="button" onClick={() => startWizard()}>{t('wizard.general.start')}</button>
        </section>
      ) : null}
    </div>
  );
}
