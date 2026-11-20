// SPDX-License-Identifier: AGPL-3.0-or-later
// First-run wizard (VLT-IS-007 TASK 03-00-00-008). Starts when no PIN is
// set. The user can also start it from the General screen.
import { useEffect, useRef, useState } from 'preact/hooks';
import { api, errorCode, type PolicyInfo, type SimResult } from '../api';
import { Confirm, errText } from '../components';
import { useModal } from '../dialog';
import { DASH } from '../format';
import { HELP_LINKS, HelpLine } from '../help';
import { t } from '../i18n';
import { canActivate, FIELDS } from '../policyform';
import { app, refreshAuth, refreshPolicy } from '../state';
import { createStore, useStore } from '../store';
import { ModeView, Values, modeOf } from './dashboard';
import { Ports } from './device';
import { Login, SetPin } from './pin';
import { ModeChoice } from './simple';
import { ScaleChoice } from './scalechoice';
import { ui } from '../mode';

export const wizardWanted = createStore({ open: false, start: '' as Step | '' });

/** startWizard opens the wizard. A start step of "pin" is for Forgot PIN (OD-40). */
export function startWizard(start?: Step): void {
  wizardWanted.set({ open: true, start: start ?? '' });
}

// VLT-IS-012 TASK 03-00-00-001 step 2 (Q-2): no step scrolls. "UPS port" and
// "E-mail" are each divided into two steps.
export const STEPS = ['pin', 'port', 'scan', 'values', 'policy', 'telegram', 'email', 'email2', 'modem', 'interface', 'summary'] as const;
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

function StepIndicator({ step }: { step: Step }) {
  const i = STEPS.indexOf(step);
  // VLT-IS-012 Q-2: numbered steps on one row, so the body has the space.
  // The name of each step is the tooltip and the text for a screen reader.
  return (
    <ol class="wizard-steps" aria-label={t('wizard.progress', { n: i + 1, total: STEPS.length, title: t('wizard.step.' + step) })}>
      {STEPS.map((s, n) => (
        <li aria-current={s === step ? 'step' : undefined} title={t('wizard.step.' + s)}>
          <span aria-hidden="true">{n + 1}</span>
          <span class="sr-only">{t('wizard.step.' + s)}</span>
        </li>
      ))}
    </ol>
  );
}

function Nav({
  step,
  first,
  pinOptional,
  pinSet,
  modemReady,
  onBack,
  onSkip,
  onNext,
}: {
  step: Step;
  first: boolean;
  pinOptional: boolean;
  pinSet: boolean;
  modemReady: boolean;
  onBack: () => void;
  onSkip: () => void;
  onNext: () => void;
}) {
  const isFirst = step === 'pin' && first;
  const isLast = step === 'summary';
  const canSkip = (!isFirst && !isLast) || (step === 'pin' && pinOptional);
  const nextDisabled = (step === 'modem' && !modemReady) || (step === 'pin' && !pinOptional && !pinSet);
  return (
    <div class="wizard-footer">
      <button type="button" onClick={onBack} disabled={isFirst}>{t('wizard.back')}</button>
      <button type="button" onClick={onSkip} disabled={!canSkip}>{t('wizard.skip')}</button>
      <button type="button" class="primary" onClick={onNext} disabled={nextDisabled}>
        {isLast ? t('wizard.finish') : t('wizard.next')}
      </button>
    </div>
  );
}

/** FirstStartPrompt is the small first-start dialog (VLT-IS-009 OD-41). */
export function FirstStartPrompt({ onStart, onLater }: { onStart: () => void; onLater: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useModal(ref);
  return (
    <div class="prompt-overlay" ref={ref}>
      <div class="prompt-dialog" role="dialog" aria-labelledby="wiz-prompt-title" aria-modal="true">
        <h2 id="wiz-prompt-title">{t('wizard.prompt.title')}</h2>
        <div class="row">
          <button type="button" class="primary" onClick={onStart}>{t('wizard.prompt.start')}</button>
          <button type="button" onClick={onLater}>{t('wizard.prompt.later')}</button>
        </div>
      </div>
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
      {(sim?.os_shutdown && !sim.os_shutdown.pass) || (info?.os_shutdown && !info.os_shutdown.pass) ? (
        <p role="alert" class="msg msg-error">{t('sim.os_shutdown.error')}</p>
      ) : null}
      {ask ? (
        <Confirm text={t('policy.activate.confirm')} confirmLabel={t('policy.activate')}
          onConfirm={() => void activate()} onCancel={() => setAsk(false)} />
      ) : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

function TelegramStep({ elevated }: { elevated: boolean; onDone?: () => void }) {
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
          placeholder={t('help.telegram.token.example')}
          onInput={(e) => setToken((e.target as HTMLInputElement).value)} />
        <HelpLine text={t('help.telegram.token')} more={t('help.telegram.token.more')} href={HELP_LINKS.telegram} />
        <button type="submit" disabled={!elevated || !token}>{t('common.save')}</button>
      </form>
      <p>{t('notify.telegram.steps')}</p>
      <HelpLine text={t('help.telegram.pair')} more={t('help.telegram.pair.more')} />
      <button type="button" disabled={!elevated} onClick={() => void pair()}>{t('notify.telegram.pair.button')}</button>
      {active && code ? <p role="status" class="banner banner-info">{t('notify.telegram.code', { code, until: expires })}</p> : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

/** The e-mail values of the two e-mail steps (kept between the steps). */
const emailDraft = createStore({
  server: '', port: '465', security: 'tls', user: '', from: '', to: '', gmail: false,
});

function EmailServerStep({ elevated }: { elevated: boolean }) {
  const d = useStore(emailDraft);
  const set = (v: Partial<typeof d>) => emailDraft.set({ ...emailDraft.get(), ...v });
  return (
    <section class="card" aria-labelledby="wiz-em">
      <h3 id="wiz-em">{t('wizard.step.email')}</h3>
      <p>{t('wizard.notify.intro.email')}</p>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <button type="button" disabled={!elevated} onClick={() => set({ server: 'smtp.gmail.com', port: '465', security: 'tls', gmail: true })}>{t('notify.email.preset.gmail')}</button>
      {d.gmail ? <p class="note" role="note">{t('notify.email.gmail.help')}</p> : null}
      {d.gmail ? (
        <HelpLine text={t('help.email.gmail')} more={t('help.email.gmail.more')} href={HELP_LINKS.gmail} />
      ) : (
        <HelpLine text={t('help.email.server')} more={t('help.email.server.more')} />
      )}
      <div class="form">
        <label for="wiz-em-server">{t('notify.email.server')}</label>
        <input id="wiz-em-server" disabled={!elevated} value={d.server} autoComplete="off" placeholder={t('help.email.server.example')} onInput={(e) => set({ server: (e.target as HTMLInputElement).value })} />
        <label for="wiz-em-port">{t('notify.email.port')}</label>
        <input id="wiz-em-port" type="number" disabled={!elevated} value={d.port} placeholder="465" onInput={(e) => set({ port: (e.target as HTMLInputElement).value })} />
        <label for="wiz-em-sec">{t('notify.email.security')}</label>
        <select id="wiz-em-sec" disabled={!elevated} value={d.security} onChange={(e) => set({ security: (e.target as HTMLSelectElement).value })}>
          <option value="tls">{t('notify.email.security.tls')}</option>
          <option value="starttls">{t('notify.email.security.starttls')}</option>
        </select>
      </div>
      <p class="note">{t('wizard.email.next')}</p>
    </section>
  );
}

function EmailAccountStep({ elevated, onSaved }: { elevated: boolean; onSaved: () => void }) {
  const d = useStore(emailDraft);
  const set = (v: Partial<typeof d>) => emailDraft.set({ ...emailDraft.get(), ...v });
  const [password, setPassword] = useState('');
  const [saved, setSaved] = useState(false);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  async function save(e: Event) {
    e.preventDefault();
    const receivers = d.to.split(/\n/).map((x) => x.trim()).filter((x) => x !== '');
    try {
      await api.setChannel('email', true, 'WARNING', {
        server: d.server, port: Number(d.port), security: d.security, user: d.user, from: d.from, to: receivers,
      });
      if (password) {
        await api.setSecret('smtp.password', password);
        setPassword('');
      }
      setSaved(true);
      onSaved();
      setMsg({ kind: 'ok', text: t('notify.saved') });
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  async function test() {
    setMsg(null);
    try {
      await api.channelTest('email');
      setMsg({ kind: 'ok', text: t('notify.test.sent') });
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  return (
    <section class="card" aria-labelledby="wiz-em2">
      <h3 id="wiz-em2">{t('wizard.step.email2')}</h3>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <form class="form" onSubmit={save}>
        <label for="wiz-em-user">{t('notify.email.user')}</label>
        <input id="wiz-em-user" disabled={!elevated} value={d.user} autoComplete="username" placeholder={t('help.email.user.example')} onInput={(e) => set({ user: (e.target as HTMLInputElement).value })} />
        <label for="wiz-em-pw">{t('notify.email.password')}</label>
        <input id="wiz-em-pw" type="password" autoComplete="new-password" disabled={!elevated} value={password}
          placeholder={t('help.email.password.example')}
          onInput={(e) => setPassword((e.target as HTMLInputElement).value)} />
        <label for="wiz-em-from">{t('notify.email.from')}</label>
        <input id="wiz-em-from" disabled={!elevated} value={d.from} autoComplete="off" onInput={(e) => set({ from: (e.target as HTMLInputElement).value })} />
        <label for="wiz-em-to">{t('notify.email.to')}</label>
        <textarea id="wiz-em-to" rows={2} disabled={!elevated} value={d.to} onInput={(e) => set({ to: (e.target as HTMLTextAreaElement).value })} />
        <div class="row">
          <button type="submit" disabled={!elevated}>{t('common.save')}</button>
          <button type="button" disabled={!elevated || !saved} onClick={() => void test()}>{t('notify.test')}</button>
        </div>
      </form>
      {d.gmail ? <p class="hint">{t('help.email.gmail')}</p> : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

export function Wizard({ first = false, portable = false, startAt, onDone }: { first?: boolean; portable?: boolean; startAt?: Step; onDone?: () => void }) {
  const s = useStore(app);
  const uiMode = useStore(ui).mode;
  const elevated = !!s.auth?.elevated;
  const start: Step = startAt || (first && !s.auth?.pin_set ? 'pin' : 'port');
  const [step, setStep] = useState<Step>(start);
  const [res, setRes] = useState<WizardResult>({ ...EMPTY, pin: !!s.auth?.pin_set });
  const [modem, setModem] = useState<'yes' | 'no' | ''>('');
  const [portName, setPortName] = useState('');
  const pinOptional = portable;
  // Steps that change Settings need the elevated state (OD-51).
  const needPIN = !!s.auth?.pin_set && !elevated && (step === 'port' || step === 'scan' || step === 'policy' || step === 'telegram' || step === 'email' || step === 'email2');
  const overlay = useRef<HTMLDivElement>(null);
  useModal(overlay);

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
    if (step === 'pin' && !pinOptional) return;
    if (step === 'modem') setRes((r) => ({ ...r, modem: 'skip' }));
    go(STEPS[Math.min(idx() + 1, STEPS.length - 1)]);
  }
  function next() {
    go(STEPS[Math.min(idx() + 1, STEPS.length - 1)]);
  }

  function footerNext() {
    if (step === 'summary') {
      onDone?.();
      return;
    }
    if (step === 'values') {
      setRes((r) => ({ ...r, values: true }));
    }
    if (step === 'modem') {
      if (!modem) return;
      setRes((r) => ({ ...r, modem: modem as 'yes' | 'no' }));
    }
    if (step === 'pin' && !pinOptional && !s.auth?.pin_set) return;
    if (step === 'pin' && pinOptional && !s.auth?.pin_set) {
      skip();
      return;
    }
    next();
  }

  return (
    <div class="wizard-overlay" ref={overlay}>
      <div class="wizard-dialog" role="dialog" aria-labelledby="wiz-title" aria-modal="true">
      <header>
        <div class="wizard-head">
          <h2 id="wiz-title">{t('wizard.title')}</h2>
          <StepIndicator step={step} />
        </div>
        <p class="intro">{t('wizard.progress', { n: STEPS.indexOf(step) + 1, total: STEPS.length, title: t('wizard.step.' + step) })}</p>
      </header>
      <div class="wizard-body">

      {needPIN ? (
        // OD-51 (VLT-IS-011): the lock screen never covers the wizard. A
        // step that needs the elevated state shows the PIN field here.
        <section class="card wizard-unlock" aria-labelledby="wiz-unlock">
          <h3 id="wiz-unlock">{t('login.title')}</h3>
          <p>{t('wizard.unlock.intro')}</p>
          <Login blockUntil={s.auth?.block_until} onDone={() => refreshAuth()} />
        </section>
      ) : null}

      {step === 'pin' ? (
        <>
          {pinOptional ? <p>{t('wizard.pin.help')}</p> : null}
          <SetPin nested onDone={() => {
            setRes((r) => ({ ...r, pin: true }));
            refreshAuth();
            next();
          }} />
        </>
      ) : null}

      {step === 'port' ? (
        <>
          <p class="intro">{t('wizard.port.intro')}</p>
          <Ports part="select" elevated={elevated} onUsed={() => {
            void api.deviceStatus().then((d) => {
              const n = d?.profile?.port || d?.config?.serial_port || '';
              setPortName(n);
              setRes((r) => ({ ...r, port: n }));
            }, () => undefined);
          }} />
          <div class="row">
            <button type="button" disabled={!elevated} onClick={() => {
              void api.setDeviceSettings({ 'device.transport': 'demo' }).then(() => {
                setPortName(t('device.transport.demo'));
                setRes((r) => ({ ...r, port: t('device.transport.demo') }));
              }, () => undefined);
            }}>{t('device.transport.demo')}</button>
            <button type="button" disabled={!elevated} onClick={() => {
              void api.setDeviceSettings({ 'device.transport': 'test' }).then(() => {
                setPortName(t('device.transport.test'));
                setRes((r) => ({ ...r, port: t('device.transport.test') }));
              }, () => undefined);
            }}>{t('device.transport.test')}</button>
          </div>
          {res.port || portName ? <p role="status">{t('wizard.port.selected', { port: res.port || portName })}</p> : null}
        </>
      ) : null}

      {step === 'scan' ? (
        <>
          <p class="intro">{t('wizard.scan.intro')}</p>
          <Ports part="scan" elevated={elevated} onUsed={() => undefined} />
        </>
      ) : null}

      {step === 'interface' ? (
        // VLT-IS-011 TASK 03-00-00-003: the wizard asks for the mode (OD-53).
        <section class="card" aria-labelledby="wiz-ui">
          <h3 id="wiz-ui">{t('wizard.step.interface')}</h3>
          <p>{t('wizard.interface.intro')}</p>
          <ModeChoice name="wiz-ui-mode" />
          <ScaleChoice id="wiz-ui-scale" />
        </section>
      ) : null}

      {step === 'values' ? (
        <section class="card" aria-labelledby="wiz-val">
          <h3 id="wiz-val">{t('wizard.step.values')}</h3>
          <p>{t('wizard.values.intro')}</p>
          <ModeView mode={modeOf(s.status)} />
          <Values s={s.status} />
          {s.status?.device?.state !== 'connected' ? <p class="note">{t('wizard.values.wait')}</p> : null}
        </section>
      ) : null}

      {step === 'policy' ? (
        <>
          <PolicyStep elevated={elevated} onActivated={() => setRes((r) => ({ ...r, policy: true }))} />
        </>
      ) : null}

      {step === 'telegram' ? (
        <TelegramStep elevated={elevated} onDone={() => { setRes((r) => ({ ...r, telegram: true })); next(); }} />
      ) : null}

      {step === 'email' ? <EmailServerStep elevated={elevated} /> : null}

      {step === 'email2' ? (
        <EmailAccountStep elevated={elevated} onSaved={() => setRes((r) => ({ ...r, email: true }))} />
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
        </section>
      ) : null}

      {step === 'summary' ? (
        <section class="card" aria-labelledby="wiz-sum">
          <h3 id="wiz-sum">{t('wizard.step.summary')}</h3>
          <dl class="kv">
            <div><dt>{t('wizard.step.pin')}</dt><dd>{res.pin ? t('wizard.summary.yes') : t('wizard.summary.no')}</dd></div>
            <div><dt>{t('wizard.summary.port')}</dt><dd>{res.port || t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.values')}</dt><dd>{res.values ? t('wizard.summary.yes') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.policy')}</dt><dd>{res.policy ? t('wizard.summary.active') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.telegram')}</dt><dd>{res.telegram ? t('wizard.summary.yes') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('notify.channel.email')}</dt><dd>{res.email ? t('wizard.summary.yes') : t('wizard.summary.skipped')}</dd></div>
            <div><dt>{t('wizard.step.modem')}</dt><dd>
              {res.modem === 'yes' ? t('wizard.modem.yes') : res.modem === 'no' ? t('wizard.modem.no') : t('wizard.summary.skipped')}
            </dd></div>
            <div><dt>{t('wizard.step.interface')}</dt><dd>{t('mode.ui.' + uiMode)}</dd></div>
          </dl>
          {res.modem === 'no' ? (
            <p class="banner banner-warn" role="note">
              <span aria-hidden="true">⚠</span> {t('wizard.modem.warn')}
            </p>
          ) : null}
        </section>
      ) : null}
      </div>
      <Nav
        step={step}
        first={first}
        pinOptional={pinOptional}
        pinSet={!!s.auth?.pin_set || res.pin}
        modemReady={!!modem}
        onBack={back}
        onSkip={skip}
        onNext={footerNext}
      />
      </div>
    </div>
  );
}
