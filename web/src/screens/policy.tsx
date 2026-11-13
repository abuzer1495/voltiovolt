// SPDX-License-Identifier: AGPL-3.0-or-later
// Shutdown Policy screen (TASK 03-00-00-006).
// WARNING: This screen controls the automatic shutdown of the computer.
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type Countdown, type PolicyInfo, type SimResult } from '../api';
import { Badge, Confirm, errText } from '../components';
import { HelpLine } from '../help';
import { DASH, time } from '../format';
import { t } from '../i18n';
import { canActivate, FIELDS, toValues, type Field } from '../policyform';
import { app, refreshPolicy } from '../state';
import { useStore } from '../store';

function show(f: Field, v: unknown): string {
  if (f.kind === 'bool') return t(v ? 'common.on' : 'common.off');
  if (f.kind === 'action') return t('policy.action.' + String(v));
  return `${v} ${f.unit ? t('unit.' + f.unit) : ''}`.trim();
}

function formOf(p: PolicyInfo): Record<string, string | boolean> {
  const out: Record<string, string | boolean> = {};
  for (const f of FIELDS) {
    const v = p.settings[f.key];
    out[f.key] = f.kind === 'int' ? String(v) : (v as string | boolean);
  }
  return out;
}

/**
 * UPSOffNote is the WARNING and the explanation of the UPS output off step
 * (VLT-IS-006 TASK 03-00-00-008 step 2).
 */
export function UPSOffNote({ form }: { form: Record<string, string | boolean> }) {
  return (
    <div class="upsoff">
      <h4>{t('policy.upsoff.title')}</h4>
      <div class="banner banner-warn" role="note">
        <span aria-hidden="true" class="banner-icon">⚠</span>
        <span>{t('policy.upsoff.warning')}</span>
      </div>
      <p class="note">
        {t('policy.upsoff.text', { delay: String(form['ups_off.delay_minutes']), restore: String(form['ups_off.restore_minutes']) })}
      </p>
      <HelpLine text={t('help.policy.ups_off.example')} />
    </div>
  );
}

export function PolicyForm({ p, onSaved }: { p: PolicyInfo; onSaved: (p: PolicyInfo) => void }) {
  const [form, setForm] = useState(formOf(p));
  const [problems, setProblems] = useState<Record<string, string>>({});
  const [err, setErr] = useState<string | null>(null);
  async function submit(e: Event) {
    e.preventDefault();
    setErr(null);
    const r = toValues(form);
    setProblems(r.problems);
    if (!r.values) return; // out of range: no policy.set
    try {
      onSaved(await api.setPolicy(r.values));
    } catch (x) {
      setErr(errText(errorCode(x)));
    }
  }
  return (
    <form class="policy-form" onSubmit={submit} noValidate>
      {FIELDS.map((f) => {
        const id = 'pf-' + f.key.replace(/\./g, '-');
        const hint = id + '-hint';
        const prob = problems[f.key];
        return (
          <>
          {f.key === 'ups_off.enabled' ? <UPSOffNote form={form} /> : null}
          <div class="field">
            {f.kind === 'bool' ? (
              <label class="check" for={id}>
                <input id={id} type="checkbox" checked={!!form[f.key]} aria-describedby={hint}
                  onChange={(e) => setForm({ ...form, [f.key]: (e.target as HTMLInputElement).checked })} />
                {t('policy.f.' + f.key)}
              </label>
            ) : (
              <>
                <label for={id}>{t('policy.f.' + f.key)}</label>
                {f.kind === 'action' ? (
                  <select id={id} value={String(form[f.key])} aria-describedby={hint}
                    onChange={(e) => setForm({ ...form, [f.key]: (e.target as HTMLSelectElement).value })}>
                    <option value="shutdown">{t('policy.action.shutdown')}</option>
                    <option value="hibernate">{t('policy.action.hibernate')}</option>
                  </select>
                ) : (
                  <input id={id} inputMode="numeric" value={String(form[f.key])} aria-invalid={prob ? true : undefined} aria-describedby={hint}
                    onInput={(e) => setForm({ ...form, [f.key]: (e.target as HTMLInputElement).value })} />
                )}
              </>
            )}
            <span id={hint} class="hint">
              {t('policy.default', { value: show(f, f.def) })}
              {f.kind === 'int' ? ' · ' + t('policy.range', { min: f.min ?? 0, max: f.max ?? 0, unit: f.unit ? t('unit.' + f.unit) : '' }) : ''}
              {f.kind === 'int' && f.key.startsWith('trigger.') ? ' · ' + t('policy.zero_off') : ''}
            </span>
            {t('help.policy.' + f.key) !== 'help.policy.' + f.key ? (
              <HelpLine
                text={t('help.policy.' + f.key)}
                more={t('help.policy.' + f.key + '.more') !== 'help.policy.' + f.key + '.more' ? t('help.policy.' + f.key + '.more') : undefined}
              />
            ) : null}
            {prob ? <span class="msg msg-error" role="alert">{t(prob, { min: f.min ?? 0, max: f.max ?? 0 })}</span> : null}
          </div>
          </>
        );
      })}
      <button type="submit" class="primary">{t('common.save')}</button>
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
    </form>
  );
}

export function SimView({ r }: { r: SimResult }) {
  return (
    <div class="sim">
      <p role="status">
        {r.passed ? <Badge kind="ok" icon="✓" text={t('sim.passed')} /> : <Badge kind="crit" icon="✕" text={t('sim.failed')} />}{' '}
        {t('sim.version', { version: r.version.slice(0, 12) })} · {time(r.time)}
      </p>
      {r.os_shutdown ? (
        <section class="scenario" aria-labelledby="sc-os-shutdown">
          <h4 id="sc-os-shutdown">
            {t('sim.os_shutdown')}{' '}
            {r.os_shutdown.pass ? <Badge kind="ok" icon="✓" text={t('sim.passed')} /> : <Badge kind="crit" icon="✕" text={t('sim.failed')} />}
          </h4>
          {!r.os_shutdown.pass ? <p role="alert" class="msg msg-error">{t('sim.os_shutdown.error')}</p> : null}
        </section>
      ) : null}
      {r.scenarios.map((s) => (
        <section class="scenario" aria-labelledby={'sc-' + s.name}>
          <h4 id={'sc-' + s.name}>
            {t('sim.s.' + s.name)}{' '}
            {s.pass ? <Badge kind="ok" icon="✓" text={t('sim.passed')} /> : <Badge kind="crit" icon="✕" text={t('sim.failed')} />}
          </h4>
          <p class="small">
            {t('sim.outcome', { outcome: t('sim.o.' + s.outcome) })}
            {s.expected ? ' · ' + t('sim.expected', { outcome: t('sim.o.' + s.expected) }) : ' · ' + t('sim.norule')}
            {s.deadline ? ' · ' + t('sim.deadline', { s: s.deadline }) : ''}
          </p>
          <ol class="steps">
            {s.steps.map((st) => (
              <li>
                <span class="vt">{t('sim.at', { s: st.t })}</span> {t('sim.k.' + st.kind)}
                {st.detail ? ': ' + st.detail : ''}
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  );
}

export function ShutdownStatus({ cd }: { cd: Countdown | null }) {
  return (
    <section class="card" aria-labelledby="sd-title">
      <h3 id="sd-title">{t('sd.title')}</h3>
      <p role="status" aria-live="polite">
        {!cd ? DASH : cd.active ? (
          <Badge kind="crit" icon="⏻" text={t('sd.running', { seconds: cd.remaining_seconds, trigger: t('trigger.' + (cd.trigger ?? 'unknown')) })} />
        ) : cd.requested ? (
          <Badge kind="crit" icon="⏻" text={t('sd.requested')} />
        ) : (
          <Badge kind="ok" icon="○" text={t('sd.none')} />
        )}
      </p>
    </section>
  );
}

export function Policy() {
  const s = useStore(app);
  const elevated = !!s.auth?.elevated;
  const p = s.policy;
  const [edit, setEdit] = useState(false);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error' | 'warn'; text: string } | null>(null);
  const [sim, setSim] = useState<SimResult | null>(null);
  const [ask, setAsk] = useState<'activate' | 'deactivate' | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    refreshPolicy();
    void api.shutdownStatus().then((cd) => app.set({ countdown: cd }), () => undefined);
  }, []);
  const lastSim = sim ?? p?.last_simulation?.result ?? null;
  async function run(f: () => Promise<unknown>, ok?: string) {
    setBusy(true);
    setMsg(null);
    try {
      await f();
      if (ok) setMsg({ kind: 'ok', text: t(ok) });
    } catch (e) {
      setMsg({ kind: 'error', text: errText(errorCode(e)) });
    } finally {
      setBusy(false);
      refreshPolicy();
    }
  }
  return (
    <div class="screen">
      <h2>{t('nav.policy')}</h2>
      <div class="banner banner-warn" role="note">
        <span aria-hidden="true" class="banner-icon">⚠</span>
        <span>{t('policy.warning')}</span>
      </div>
      <ShutdownStatus cd={s.countdown} />
      {!p ? (
        <p role="status">{t('common.loading')}</p>
      ) : (
        <>
          <section class="card" aria-labelledby="pol-title">
            <h3 id="pol-title">{t('policy.current')}</h3>
            <p>
              {p.active ? <Badge kind="ok" icon="✓" text={t('policy.active')} /> : <Badge kind="warn" icon="○" text={t('policy.inactive')} />}{' '}
              {p.fallback ? <Badge kind="warn" icon="⚠" text={t('policy.fallback')} /> : null} {t('sim.version', { version: p.version.slice(0, 12) })}
            </p>
            {p.fallback ? <p class="note">{t('fallback.banner')}</p> : null}
            <dl class="kv">
              {FIELDS.map((f) => (
                <div>
                  <dt>{t('policy.f.' + f.key)}</dt>
                  <dd>{show(f, p.settings[f.key])}</dd>
                </div>
              ))}
            </dl>
            <p class="small">
              {t('policy.lastsim')}{' '}
              {p.last_simulation
                ? `${t(p.last_simulation.passed ? 'sim.passed' : 'sim.failed')} · ${t('sim.version', { version: p.last_simulation.version.slice(0, 12) })} · ${time(p.last_simulation.time)}`
                : DASH}
            </p>
            {!elevated ? <p class="note">{t('locked.need')}</p> : null}
            <div class="row">
              <button type="button" disabled={!elevated || p.fallback || busy} onClick={() => setEdit(!edit)} aria-expanded={edit}>{t('policy.edit')}</button>
              <button type="button" disabled={!elevated || busy} onClick={() => void run(async () => setSim(await api.simulate()))}>{t('policy.simulate')}</button>
              <button type="button" class="primary" disabled={!canActivate(p, elevated) || busy} onClick={() => setAsk('activate')}>{t('policy.activate')}</button>
              <button type="button" disabled={!elevated || p.fallback || !p.active || busy} onClick={() => setAsk('deactivate')}>{t('policy.deactivate')}</button>
            </div>
            {!canActivate(p, elevated) && elevated && !p.active && !p.fallback ? <p class="small">{t('policy.activate.rule')}</p> : null}
            {p.os_shutdown && !p.os_shutdown.pass ? <p role="alert" class="msg msg-error">{t('sim.os_shutdown.error')}</p> : null}
            {ask === 'activate' ? (
              <Confirm text={t('policy.activate.confirm')} confirmLabel={t('policy.activate')}
                onConfirm={() => { setAsk(null); void run(api.activatePolicy, 'policy.activated'); }} onCancel={() => setAsk(null)} />
            ) : null}
            {ask === 'deactivate' ? (
              <Confirm text={t('policy.deactivate.confirm')} confirmLabel={t('policy.deactivate')}
                onConfirm={() => { setAsk(null); void run(api.deactivatePolicy, 'policy.deactivated'); }} onCancel={() => setAsk(null)} />
            ) : null}
            {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
          </section>
          {edit && elevated && !p.fallback ? (
            <section class="card" aria-labelledby="edit-title">
              <h3 id="edit-title">{t('policy.edit')}</h3>
              <PolicyForm p={p} onSaved={(np) => { app.set({ policy: np }); setEdit(false); setSim(null); setMsg({ kind: 'warn', text: t('policy.saved') }); refreshPolicy(); }} />
            </section>
          ) : null}
          {lastSim ? (
            <section class="card" aria-labelledby="sim-title">
              <h3 id="sim-title">{t('sim.title')}</h3>
              <SimView r={lastSim} />
            </section>
          ) : null}
        </>
      )}
    </div>
  );
}
