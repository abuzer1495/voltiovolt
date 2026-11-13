// SPDX-License-Identifier: AGPL-3.0-or-later
// Notifications screen (VLT-IS-007 TASK 03-00-00-007): one section for
// each Channel, Secret fields write-only, Telegram pairing, e-mail
// Settings and the Gmail preset.
import { useEffect, useState } from 'preact/hooks';
import {
  api,
  errorCode,
  type ChannelInfo,
  type ChannelsList,
  type EmailPreset,
  type TelegramChat,
} from '../api';
import { errText } from '../components';
import { DASH, time } from '../format';
import { HELP_LINKS, HelpLine } from '../help';
import { t } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';

const SEVERITIES = ['INFO', 'WARNING', 'CRITICAL'] as const;

type Msg = { kind: 'ok' | 'error'; text: string } | null;

function message(e: unknown): string {
  return errText(errorCode(e));
}

function SecretField({
  id,
  label,
  hint,
  set,
  elevated,
  onSaved,
  placeholder,
  help,
  more,
  href,
}: {
  id: string;
  label: string;
  hint?: string;
  set: boolean;
  elevated: boolean;
  onSaved: () => void;
  placeholder?: string;
  help?: string;
  more?: string;
  href?: string;
}) {
  const [val, setVal] = useState('');
  const [msg, setMsg] = useState<Msg>(null);
  const inputId = 'secret-' + id.replace(/\./g, '-');
  async function save(e: Event) {
    e.preventDefault();
    if (!val) return;
    try {
      await api.setSecret(id, val);
      setVal('');
      setMsg({ kind: 'ok', text: t('notify.secret.saved') });
      onSaved();
    } catch (x) {
      setMsg({ kind: 'error', text: message(x) });
    }
  }
  return (
    <form class="form" onSubmit={save}>
      <label for={inputId}>{label}</label>
      <input
        id={inputId}
        type="password"
        autoComplete="new-password"
        disabled={!elevated}
        value={val}
        placeholder={placeholder}
        onInput={(e) => setVal((e.target as HTMLInputElement).value)}
      />
      {help ? <HelpLine text={help} more={more} href={href} /> : null}
      {set ? (
        <p class="note">{t('notify.secret.hint', { hint: hint || DASH })}</p>
      ) : (
        <p class="note">{t('notify.secret.unset')}</p>
      )}
      <button type="submit" disabled={!elevated || !val}>{t('common.save')}</button>
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </form>
  );
}

function TelegramSection({
  ch,
  elevated,
  onReload,
}: {
  ch: ChannelInfo;
  elevated: boolean;
  onReload: () => void;
}) {
  const [code, setCode] = useState('');
  const [expires, setExpires] = useState(ch.pairing?.expires ?? '');
  const [chats, setChats] = useState<TelegramChat[]>([]);
  const [msg, setMsg] = useState<Msg>(null);
  async function loadChats() {
    try {
      const r = await api.telegramChats();
      setChats(r.chats ?? []);
    } catch {
      /* list is also in the Channel section after reload */
    }
  }
  useEffect(() => {
    void loadChats();
  }, []);
  useEffect(() => {
    if (!expires) return;
    const end = Date.parse(expires);
    if (!end || end <= Date.now()) return;
    const id = setInterval(() => {
      void loadChats();
      if (Date.now() >= end) {
        setCode('');
        setExpires('');
        clearInterval(id);
      }
    }, 2000);
    return () => clearInterval(id);
  }, [expires]);
  async function pair() {
    setMsg(null);
    try {
      const r = await api.telegramPairStart();
      setCode(r.code ?? '');
      setExpires(r.expires);
      onReload();
    } catch (e) {
      setMsg({ kind: 'error', text: message(e) });
    }
  }
  async function remove(id: number) {
    setMsg(null);
    try {
      await api.telegramChatRemove(id);
      setMsg({ kind: 'ok', text: t('notify.telegram.removed') });
      void loadChats();
      onReload();
    } catch (e) {
      setMsg({ kind: 'error', text: message(e) });
    }
  }
  const active = !!expires && Date.parse(expires) > Date.now();
  return (
    <>
      <SecretField
        id="telegram.token"
        label={t('notify.telegram.token')}
        hint={ch.secret?.hint}
        set={!!ch.secret?.set}
        elevated={elevated}
        onSaved={onReload}
        placeholder={t('help.telegram.token.example')}
        help={t('help.telegram.token')}
        more={t('help.telegram.token.more')}
        href={HELP_LINKS.telegram}
      />
      <h4>{t('notify.telegram.pair')}</h4>
      <p>{t('notify.telegram.steps')}</p>
      <HelpLine text={t('help.telegram.pair')} more={t('help.telegram.pair.more')} />
      <button type="button" disabled={!elevated} onClick={() => void pair()}>{t('notify.telegram.pair.button')}</button>
      {active && code ? (
        <p role="status" class="banner banner-info">
          {t('notify.telegram.code', { code, until: time(expires) })}
        </p>
      ) : null}
      <h4>{t('notify.telegram.chats')}</h4>
      {chats.length === 0 ? <p class="note">{t('notify.telegram.nochats')}</p> : (
        <table class="table">
          <caption>{t('notify.telegram.chats')}</caption>
          <thead>
            <tr>
              <th scope="col">{t('notify.telegram.chatname')}</th>
              <th scope="col">{t('notify.telegram.chatid')}</th>
              <th scope="col">{t('notify.telegram.pairedat')}</th>
              <th scope="col">{t('notify.telegram.remove')}</th>
            </tr>
          </thead>
          <tbody>
            {chats.map((c) => (
              <tr>
                <td>{c.name || DASH}</td>
                <td>{c.chat_id}</td>
                <td>{c.paired_at ? time(c.paired_at) : DASH}</td>
                <td>
                  <button type="button" disabled={!elevated} onClick={() => void remove(c.chat_id)}>
                    {t('notify.telegram.remove')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </>
  );
}

function EmailSection({
  ch,
  presets,
  elevated,
  onReload,
}: {
  ch: ChannelInfo;
  presets: EmailPreset[];
  elevated: boolean;
  onReload: () => void;
}) {
  const s = ch.settings ?? {};
  const [server, setServer] = useState(String(s.server ?? ''));
  const [port, setPort] = useState(String(s.port ?? 465));
  const [security, setSecurity] = useState(String(s.security ?? 'tls'));
  const [user, setUser] = useState(String(s.user ?? ''));
  const [from, setFrom] = useState(String(s.from ?? ''));
  const [to, setTo] = useState(Array.isArray(s.to) ? (s.to as string[]).join('\n') : '');
  const [gmail, setGmail] = useState(String(s.server ?? '') === 'smtp.gmail.com');
  const [msg, setMsg] = useState<Msg>(null);
  useEffect(() => {
    const ns = ch.settings ?? {};
    setServer(String(ns.server ?? ''));
    setPort(String(ns.port ?? 465));
    setSecurity(String(ns.security ?? 'tls'));
    setUser(String(ns.user ?? ''));
    setFrom(String(ns.from ?? ''));
    setTo(Array.isArray(ns.to) ? (ns.to as string[]).join('\n') : '');
    setGmail(String(ns.server ?? '') === 'smtp.gmail.com');
  }, [ch.settings]);
  function applyPreset(p: EmailPreset) {
    setServer(p.server);
    setPort(String(p.port));
    setSecurity(p.security);
    setGmail(p.name === 'gmail' || p.server === 'smtp.gmail.com');
  }
  async function save(e: Event) {
    e.preventDefault();
    setMsg(null);
    const receivers = to.split(/\n/).map((x) => x.trim()).filter((x) => x !== '');
    const n = Number(port);
    try {
      await api.setChannel(ch.name, ch.enabled, ch.min_severity, {
        server, port: n, security, user, from, to: receivers,
      });
      setMsg({ kind: 'ok', text: t('notify.saved') });
      onReload();
    } catch (x) {
      setMsg({ kind: 'error', text: message(x) });
    }
  }
  return (
    <>
      {presets.map((p) => (
        <button type="button" disabled={!elevated} onClick={() => applyPreset(p)}>
          {t('notify.email.preset.' + p.name)}
        </button>
      ))}
      {gmail ? <p class="note" role="note">{t('notify.email.gmail.help')}</p> : null}
      {gmail ? (
        <HelpLine text={t('help.email.gmail')} more={t('help.email.gmail.more')} href={HELP_LINKS.gmail} />
      ) : (
        <HelpLine text={t('help.email.server')} more={t('help.email.server.more')} />
      )}
      <form class="form" onSubmit={save}>
        <label for="email-server">{t('notify.email.server')}</label>
        <input id="email-server" disabled={!elevated} value={server} autoComplete="off" placeholder={t('help.email.server.example')} onInput={(e) => setServer((e.target as HTMLInputElement).value)} />
        <label for="email-port">{t('notify.email.port')}</label>
        <input id="email-port" type="number" min="1" max="65535" disabled={!elevated} value={port} placeholder="465" onInput={(e) => setPort((e.target as HTMLInputElement).value)} />
        <label for="email-security">{t('notify.email.security')}</label>
        <select id="email-security" disabled={!elevated} value={security} onChange={(e) => setSecurity((e.target as HTMLSelectElement).value)}>
          <option value="tls">{t('notify.email.security.tls')}</option>
          <option value="starttls">{t('notify.email.security.starttls')}</option>
        </select>
        <label for="email-user">{t('notify.email.user')}</label>
        <input id="email-user" disabled={!elevated} value={user} autoComplete="username" placeholder={t('help.email.user.example')} onInput={(e) => setUser((e.target as HTMLInputElement).value)} />
        <label for="email-from">{t('notify.email.from')}</label>
        <input id="email-from" disabled={!elevated} value={from} autoComplete="off" onInput={(e) => setFrom((e.target as HTMLInputElement).value)} />
        <label for="email-to">{t('notify.email.to')}</label>
        <textarea id="email-to" rows={3} disabled={!elevated} value={to} onInput={(e) => setTo((e.target as HTMLTextAreaElement).value)} />
        <button type="submit" disabled={!elevated}>{t('common.save')}</button>
      </form>
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
      <SecretField
        id="smtp.password"
        label={t('notify.email.password')}
        hint={ch.secret?.hint}
        set={!!ch.secret?.set}
        elevated={elevated}
        onSaved={onReload}
        placeholder={t('help.email.password.example')}
        help={gmail ? t('help.email.gmail') : t('help.email.server')}
        more={gmail ? t('help.email.gmail.more') : undefined}
        href={gmail ? HELP_LINKS.gmail : undefined}
      />
    </>
  );
}

function ChannelCard({
  ch,
  presets,
  elevated,
  onReload,
}: {
  ch: ChannelInfo;
  presets: EmailPreset[];
  elevated: boolean;
  onReload: () => void;
}) {
  const [enabled, setEnabled] = useState(ch.enabled);
  const [sev, setSev] = useState(ch.min_severity);
  const [msg, setMsg] = useState<Msg>(null);
  useEffect(() => {
    setEnabled(ch.enabled);
    setSev(ch.min_severity);
  }, [ch.enabled, ch.min_severity]);
  async function save(e: Event) {
    e.preventDefault();
    setMsg(null);
    try {
      await api.setChannel(ch.name, enabled, sev, {});
      setMsg({ kind: 'ok', text: t('notify.saved') });
      onReload();
    } catch (x) {
      setMsg({ kind: 'error', text: message(x) });
    }
  }
  async function test() {
    setMsg(null);
    try {
      await api.channelTest(ch.name);
      setMsg({ kind: 'ok', text: t('notify.test.sent') });
    } catch (x) {
      setMsg({ kind: 'error', text: message(x) });
    }
  }
  const titleId = 'ch-' + ch.name;
  return (
    <section class="card" aria-labelledby={titleId}>
      <h3 id={titleId}>{t('notify.channel.' + ch.name)}</h3>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <form class="form" onSubmit={save}>
        <label class="check">
          <input type="checkbox" disabled={!elevated} checked={enabled} onChange={(e) => setEnabled((e.target as HTMLInputElement).checked)} />
          {t('notify.enabled')}
        </label>
        <label for={'sev-' + ch.name}>{t('notify.severity')}</label>
        <select id={'sev-' + ch.name} disabled={!elevated} value={sev} onChange={(e) => setSev((e.target as HTMLSelectElement).value)}>
          {SEVERITIES.map((s) => <option value={s}>{t('notify.severity.' + s)}</option>)}
        </select>
        <HelpLine text={t('help.severity')} more={t('help.severity.more')} />
        <button type="submit" disabled={!elevated}>{t('common.save')}</button>
      </form>
      <button type="button" disabled={!elevated} onClick={() => void test()}>{t('notify.test')}</button>
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
      {ch.name === 'telegram' ? <TelegramSection ch={ch} elevated={elevated} onReload={onReload} /> : null}
      {ch.name === 'email' ? <EmailSection ch={ch} presets={presets} elevated={elevated} onReload={onReload} /> : null}
    </section>
  );
}

export function Notifications() {
  const s = useStore(app);
  const elevated = !!s.auth?.elevated;
  const [list, setList] = useState<ChannelsList | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const load = () => {
    api.channels().then(setList, (e) => setErr(message(e)));
  };
  useEffect(() => {
    void load();
  }, []);
  return (
    <div class="screen">
      <h2>{t('nav.notifications')}</h2>
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
      {!list ? <p>{t('common.loading')}</p> : list.channels.map((ch) => (
        <ChannelCard ch={ch} presets={list.email_presets ?? []} elevated={elevated} onReload={load} />
      ))}
    </div>
  );
}
