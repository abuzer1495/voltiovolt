// SPDX-License-Identifier: AGPL-3.0-or-later
// First-run wizard (VLT-IS-007 TASK 03-00-00-008). Starts when no PIN is
// set. The user can also start it from the General screen.
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type PolicyInfo, type SimResult } from '../api';
import { Confirm, errText } from '../components';
import { DASH } from '../format';
import { t } from '../i18n';
import { canActivate, FIELDS } from '../policyform';
import { app, refreshAuth, refreshPolicy } from '../state';
import { createStore, useStore } from '../store';
import { ModeView, Values, modeOf } from './dashboard';
import { Ports } from './device';
import { SetPin } from './pin';

export const wizardWanted = createStore({ open: false });

/** startWizard opens the wizard from the General screen. */
export function startWizard(): void {
  wizardWanted.set({ open: true });
}

export const STEPS = ['pin', 'port', 'values', 'policy', 'telegram', 'email', 'modem', 'summary'] as const;
export type Step = (typeof STEPS)[number];

export interface WizardResult {
  pin: boolean;
  port: string;
  values: boolean;
  policy: boolean;
  telegram: boolean;
  email: boolean;
  modem: 'yes' | 'no' | 'skip' | '';
}

const EMPTY: WizardResult = { pin: false, port: '', values: false, policy: false, telegram: false, email: false, modem: '' };

function Nav({
  step,
  first,
  onBack,
  onSkip,
}: {
  step: Step;
  first: boolean;
  onBack: () => void;
  onSkip: () => void;
}) {
  const i = STEPS.indexOf(step);
  const isFirst = step === 'pin' && first;
  const isLast = step === 'summary';
  return (
    <div class="row">
      {!isFirst ? <button type="button" onClick={onBack}>{t('wizard.back')}</button> : null}
      {!isFirst && !isLast ? <button type="button" onClick={onSkip}>{t('wizard.skip')}</button> : null}
      <p class="note">{t('wizard.progress', { n: i + 1, total: STEPS.length, title: t('wizard.step.' + step) })}</p>
    </div>
  );
}

function PolicyStep({ elevated, onActivated }: { elevated: boolean; onActivated: () => void }) {
  const s = useStore(app);
  const p = s.policy;
  const [sim, setSim] = useState<SimResult | null>(null);
  const [ask, setAsk] = useState(false);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    refreshPolicy();
  }, []);
  const info: PolicyInfo | null = p;
  async function simulate() {
    setBusy(true);
    setMsg(null);
    try {
      setSim(await api.simulate());
      refreshPolicy();
    } catch (e) {
      setMsg({ kind: 'error', text: errText(errorCode(e)) });
    } finally {
      setBusy(false);
    }
  }
  async function activate() {
    setAsk(false);
    setBusy(true);
    setMsg(null);
    try {
      await api.activatePolicy();
      setMsg({ kind: 'ok', text: t('policy.activated') });
      refreshPolicy();
      onActivated();
    } catch (e) {
      setMsg({ kind: 'error', text: errText(errorCode(e)) });
    } finally {
      setBusy(false);
    }
  }
  const settings = info?.settings;
  return (
    <section class="card" aria-labelledby="wiz-pol">
      <h3 id="wiz-pol">{t('wizard.step.policy')}</h3>
      <p>{t('wizard.policy.intro')}</p>
      {!settings ? <p>{t('common.loading')}</p> : (
        <dl class="kv">
          {FIELDS.map((f) => (
            <div>
              <dt>{t('policy.f.' + f.key)}</dt>
              <dd>{settings[f.key] === true ? t('common.on') : settings[f.key] === false ? t('common.off') : String(settings[f.key] ?? DASH)}</dd>
            </div>
          ))}
        </dl>
      )}
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <div class="row">
        <button type="button" disabled={!elevated || busy} onClick={() => void simulate()}>{t('policy.simulate')}</button>
        <button type="button" class="primary" disabled={!canActivate(info, elevated) || busy} onClick={() => setAsk(true)}>{t('policy.activate')}</button>
      </div>
      {sim ? <p role="status">{t(sim.passed ? 'sim.passed' : 'sim.failed')}</p> : null}
      {ask ? (
        <Confirm text={t('policy.activate.confirm')} confirmLabel={t('policy.activate')}
          onConfirm={() => void activate()} onCancel={() => setAsk(false)} />
      ) : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

function TelegramStep({ elevated, onDone }: { elevated: boolean; onDone: () => void }) {
  const [token, setToken] = useState('');
  const [code, setCode] = useState('');
  const [expires, setExpires] = useState('');
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  async function saveToken(e: Event) {
    e.preventDefault();
    if (!token) return;
    try {
      await api.setSecret('telegram.token', token);
      setToken('');
      await api.setChannel('telegram', true, 'WARNING', {});
      setMsg({ kind: 'ok', text: t('notify.secret.saved') });
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  async function pair() {
    try {
      const r = await api.telegramPairStart();
      setCode(r.code ?? '');
      setExpires(r.expires);
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  const active = !!expires && Date.parse(expires) > Date.now();
  return (
    <section class="card" aria-labelledby="wiz-tg">
      <h3 id="wiz-tg">{t('wizard.step.telegram')}</h3>
      <p>{t('wizard.notify.intro.telegram')}</p>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <form class="form" onSubmit={saveToken}>
        <label for="wiz-tg-token">{t('notify.telegram.token')}</label>
        <input id="wiz-tg-token" type="password" autoComplete="new-password" disabled={!elevated} value={token}
          onInput={(e) => setToken((e.target as HTMLInputElement).value)} />
        <button type="submit" disabled={!elevated || !token}>{t('common.save')}</button>
      </form>
      <p>{t('notify.telegram.steps')}</p>
      <button type="button" disabled={!elevated} onClick={() => void pair()}>{t('notify.telegram.pair.button')}</button>
      {active && code ? <p role="status" class="banner banner-info">{t('notify.telegram.code', { code, until: expires })}</p> : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
      <button type="button" class="primary" onClick={onDone}>{t('wizard.notify.done')}</button>
    </section>
  );
}

function EmailStep({ elevated, onDone }: { elevated: boolean; onDone: () => void }) {
  const [server, setServer] = useState('');
  const [port, setPort] = useState('465');
  const [security, setSecurity] = useState('tls');
  const [user, setUser] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [password, setPassword] = useState('');
  const [gmail, setGmail] = useState(false);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  function gmailPreset() {
    setServer('smtp.gmail.com');
    setPort('465');
    setSecurity('tls');
    setGmail(true);
  }
  async function save(e: Event) {
    e.preventDefault();
    const receivers = to.split(/\n/).map((x) => x.trim()).filter((x) => x !== '');
    try {
      await api.setChannel('email', true, 'WARNING', {
        server, port: Number(port), security, user, from, to: receivers,
      });
      if (password) {
        await api.setSecret('smtp.password', password);
        setPassword('');
      }
      setMsg({ kind: 'ok', text: t('notify.saved') });
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  return (
    <section class="card" aria-labelledby="wiz-em">
      <h3 id="wiz-em">{t('wizard.step.email')}</h3>
      <p>{t('wizard.notify.intro.email')}</p>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <button type="button" disabled={!elevated} onClick={gmailPreset}>{t('notify.email.preset.gmail')}</button>
      {gmail ? <p class="note" role="note">{t('notify.email.gmail.help')}</p> : null}
      <form class="form" onSubmit={save}>
        <label for="wiz-em-server">{t('notify.email.server')}</label>
        <input id="wiz-em-server" disabled={!elevated} value={server} autoComplete="off" onInput={(e) => setServer((e.target as HTMLInputElement).value)} />
        <label for="wiz-em-port">{t('notify.email.port')}</label>
        <input id="wiz-em-port" type="number" disabled={!elevated} value={port} onInput={(e) => setPort((e.target as HTMLInputElement).value)} />
        <label for="wiz-em-sec">{t('notify.email.security')}</label>
        <select id="wiz-em-sec" disabled={!elevated} value={security} onChange={(e) => setSecurity((e.target as HTMLSelectElement).value)}>
          <option value="tls">{t('notify.email.security.tls')}</option>
          <option value="starttls">{t('notify.email.security.starttls')}</option>
        </select>
        <label for="wiz-em-user">{t('notify.email.user')}</label>
        <input id="wiz-em-user" disabled={!elevated} value={user} autoComplete="username" onInput={(e) => setUser((e.target as HTMLInputElement).value)} />
        <label for="wiz-em-from">{t('notify.email.from')}</label>
        <input id="wiz-em-from" disabled={!elevated} value={from} autoComplete="off" onInput={(e) => setFrom((e.target as HTMLInputElement).value)} />
        <label for="wiz-em-to">{t('notify.email.to')}</label>
        <textarea id="wiz-em-to" rows={3} disabled={!elevated} value={to} onInput={(e) => setTo((e.target as HTMLTextAreaElement).value)} />
        <label for="wiz-em-pw">{t('notify.email.password')}</label>
        <input id="wiz-em-pw" type="password" autoComplete="new-password" disabled={!elevated} value={password}
          onInput={(e) => setPassword((e.target as HTMLInputElement).value)} />
        <button type="submit" disabled={!elevated}>{t('common.save')}</button>
      </form>
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
      <button type="button" class="primary" onClick={onDone}>{t('wizard.notify.done')}</button>
    </section>
  );
}

export function Wizard({ first = false, onDone }: { first?: boolean; onDone?: () => void }) {
  const s = useStore(app);
  const elevated = !!s.auth?.elevated;
  const start: Step = first && !s.auth?.pin_set ? 'pin' : 'port';
  const [step, setStep] = useState<Step>(start);
  const [res, setRes] = useState<WizardResult>({ ...EMPTY, pin: !!s.auth?.pin_set });
  const [modem, setModem] = useState<'yes' | 'no' | ''>('');
  const [portName, setPortName] = useState('');

  function go(next: Step) {
    setStep(next);
  }
  function idx() {
    return STEPS.indexOf(step);
  }
  function back() {
    if (step === 'pin') return;
    if (step === 'port' && !first) {
      onDone?.();
      return;
    }
    go(STEPS[idx() - 1]);
  }
  function skip() {
    if (step === 'pin') return;
    if (step === 'modem') setRes((r) => ({ ...r, modem: 'skip' }));
    go(STEPS[Math.min(idx() + 1, STEPS.length - 1)]);
  }
  function next() {
    go(STEPS[Math.min(idx() + 1, STEPS.length - 1)]);
  }

  return (
    <div class="screen wizard">
      <h2>{t('wizard.title')}</h2>
      <p>{t('wizard.intro')}</p>
      <Nav step={step} first={first} onBack={back} onSkip={skip} />

      {step === 'pin' ? (
        <SetPin nested onDone={() => {
          setRes((r) => ({ ...r, pin: true }));
          refreshAuth();
          next();
        }} />
      ) : null}

      {step === 'port' ? (
        <section class="card" aria-labelledby="wiz-port">
          <h3 id="wiz-port">{t('wizard.step.port')}</h3>
          <p>{t('wizard.port.intro')}</p>
          <Ports elevated={elevated} onUsed={() => {
            void api.deviceStatus().then((d) => {
              const n = d?.profile?.port || d?.config?.serial_port || '';
              setPortName(n);
              setRes((r) => ({ ...r, port: n }));
            }, () => undefined);
          }} />
          {res.port || portName ? <p role="status">{t('wizard.port.selected', { port: res.port || portName })}</p> : null}
          <button type="button" onClick={next}>{t('wizard.next')}</button>
        </section>
      ) : null}

      {step === 'values' ? (
        <section class="card" aria-labelledby="wiz-val">
          <h3 id="wiz-val">{t('wizard.step.values')}</h3>
          <p>{t('wizard.values.intro')}</p>
          <ModeView mode={modeOf(s.status)} />
          <Values s={s.status} />
          {s.status?.device?.state !== 'connected' ? <p class="note">{t('wizard.values.wait')}</p> : null}
          <div class="row">
            <button type="button" class="primary" onClick={() => { setRes((r) => ({ ...r, values: true })); next(); }}>
              {t('wizard.values.confirm')}
            </button>
          </div>
        </section>
      ) : null}

      {step === 'policy' ? (
        <>
          <PolicyStep elevated={elevated} onActivated={() => setRes((r) => ({ ...r, policy: true }))} />
          <button type="button" onClick={next}>{t('wizard.next')}</button>
        </>
      ) : null}

      {step === 'telegram' ? (
        <TelegramStep elevated={elevated} onDone={() => { setRes((r) => ({ ...r, telegram: true })); next(); }} />
      ) : null}

      {step === 'email' ? (
        <EmailStep elevated={elevated} onDone={() => { setRes((r) => ({ ...r, email: true })); next(); }} />
      ) : null}

      {step === 'modem' ? (
        <section class="card" aria-labelledby="wiz-modem">
          <h3 id="wiz-modem">{t('wizard.step.modem')}</h3>
          <fieldset class="radio">
            <legend>{t('wizard.modem.question')}</legend>
            <label class="check">
              <input type="radio" name="modem" checked={modem === 'yes'} onChange={() => setModem('yes')} /> {t('wizard.modem.yes')}
            </label>
            <label class="check">
              <input type="radio" name="modem" checked={modem === 'no'} onChange={() => setModem('no')} /> {t('wizard.modem.no')}
            </label>
          </fieldset>
          {modem === 'no' ? (
            <p class="banner banner-warn" role="note">
              <span aria-hidden="true">⚠</span> {t('wizard.modem.warn')}
            </p>
          ) : null}
          <button type="button" class="primary" disabled={!modem} onClick={() => { setRes((r) => ({ ...r, modem: modem as 'yes' | 'no' })); next(); }}>
            {t('wizard.next')}
          </button>
        </section>
      ) : null}

      {step === 'summary' ? (
        <section class="card" aria-labelledby="wiz-sum">
          <h3 id="wiz-sum">{t('wizard.step.summary')}</h3>
          <dl class="kv">
            <div><dt>{t('wizard.step.pin')}</dt><dd>{res.pin ? t('wizard.summary.yes') : t('wizard.summary.no')}</dd></div>
            <div><dt>{t('wizard.step.port')}</dt><dd>{res.port || t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.values')}</dt><dd>{res.values ? t('wizard.summary.yes') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.policy')}</dt><dd>{res.policy ? t('wizard.summary.active') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.telegram')}</dt><dd>{res.telegram ? t('wizard.summary.yes') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.email')}</dt><dd>{res.email ? t('wizard.summary.yes') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.modem')}</dt><dd>
              {res.modem === 'yes' ? t('wizard.modem.yes') : res.modem === 'no' ? t('wizard.modem.no') : t('wizard.summary.skipped')}
            </dd></div>
          </dl>
          {res.modem === 'no' ? (
            <p class="banner banner-warn" role="note">
              <span aria-hidden="true">⚠</span> {t('wizard.modem.warn')}
            </p>
          ) : null}
          <button type="button" class="primary" onClick={() => onDone?.()}>{t('wizard.finish')}</button>
        </section>
      ) : null}
    </div>
  );
}
