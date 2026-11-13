// SPDX-License-Identifier: AGPL-3.0-or-later
// General screen (TASK 03-00-00-007 step 2): language, theme, About.
// VLT-IS-006 Q-IA-1: vltd saves ui.language without the elevated state
// (a set PIN is necessary).
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type AboutInfo } from '../api';
import { errText } from '../components';
import { DASH } from '../format';
import { lang, setLang, t, type Lang } from '../i18n';
import { startWizard } from './wizard';

export function General() {
  const [about, setAbout] = useState<AboutInfo | null>(null);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  const [logon, setLogon] = useState(false);
  useEffect(() => {
    void api.about().then(async (a) => {
      setAbout(a);
      if (a.mode === 'portable') {
        try { setLogon(await api.startAtLogon()); } catch { /* hide errors: switch stays off */ }
      }
    }, () => undefined);
  }, []);
  async function chooseLogon(on: boolean) {
    setMsg(null);
    try {
      await api.setStartAtLogon(on);
      setLogon(on);
      setMsg({ kind: 'ok', text: t('general.startAtLogon.saved') });
    } catch (e) {
      setMsg({ kind: 'error', text: errText(errorCode(e)) });
    }
  }
  async function choose(l: Lang) {
    setLang(l);
    setMsg(null);
    try {
      await api.setLanguage(l);
      setMsg({ kind: 'ok', text: t('general.lang.saved') });
    } catch (e) {
      const c = errorCode(e);
      setMsg({ kind: 'error', text: c === 'not_elevated' || c === 'pin_not_set' ? t('general.lang.notsaved') : errText(c) });
    }
  }
  return (
    <div class="screen">
      <h2>{t('nav.general')}</h2>
      <section class="card" aria-labelledby="lang-title">
        <h3 id="lang-title">{t('general.lang')}</h3>
        <fieldset class="radio">
          <legend>{t('general.lang')}</legend>
          <label class="check"><input type="radio" name="lang" checked={lang() === 'tr'} onChange={() => void choose('tr')} /> {t('lang.tr')}</label>
          <label class="check"><input type="radio" name="lang" checked={lang() === 'en'} onChange={() => void choose('en')} /> {t('lang.en')}</label>
        </fieldset>
        {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
      </section>
      <section class="card">
        <h3>{t('general.theme')}</h3>
        <p>{t('general.theme.os')}</p>
      </section>
      {about?.mode === 'portable' ? (
        <section class="card" aria-labelledby="logon-title">
          <h3 id="logon-title">{t('general.startAtLogon')}</h3>
          <label class="check">
            <input type="checkbox" checked={logon} onChange={(e) => void chooseLogon((e.target as HTMLInputElement).checked)} />
            {t('general.startAtLogon')}
          </label>
          <p id="logon-help">{t('general.startAtLogon.help')}</p>
        </section>
      ) : null}
      <section class="card" aria-labelledby="wiz-title">
        <h3 id="wiz-title">{t('wizard.title')}</h3>
        <p>{t('wizard.general.intro')}</p>
        <button type="button" onClick={() => startWizard()}>{t('wizard.general.start')}</button>
      </section>
      {about?.data_dir ? (
        <section class="card" aria-labelledby="data-dir-title">
          <h3 id="data-dir-title">{t('general.dataDir')}</h3>
          <p><code>{about.data_dir}</code></p>
          <p id="data-dir-help">{t('general.dataDir.help')}</p>
        </section>
      ) : null}
      <section class="card" aria-labelledby="about-title">
        <h3 id="about-title">{t('general.about')}</h3>
        <dl class="kv">
          <div><dt>{t('about.version')}</dt><dd>{about?.version ?? DASH}</dd></div>
          <div><dt>{t('about.vltd')}</dt><dd>{about?.vltd_version || DASH}</dd></div>
          <div><dt>{t('about.protocol')}</dt><dd>{about?.protocol || DASH}</dd></div>
          <div><dt>{t('about.license')}</dt><dd>{about?.license ?? DASH}</dd></div>
          <div><dt>{t('about.source')}</dt><dd>
            {about ? (
              <>
                <code>{about.source}</code>{' '}
                <button type="button" onClick={() => void api.openSource()}>{t('about.open')}</button>
              </>
            ) : DASH}
          </dd></div>
        </dl>
      </section>
    </div>
  );
}
