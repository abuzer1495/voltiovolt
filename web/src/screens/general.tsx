// SPDX-License-Identifier: AGPL-3.0-or-later
// General screen (TASK 03-00-00-007 step 2): language, theme, About.
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type AboutInfo } from '../api';
import { errText } from '../components';
import { DASH } from '../format';
import { lang, setLang, t, type Lang } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';

export function General() {
  const s = useStore(app);
  const elevated = !!s.auth?.elevated;
  const [about, setAbout] = useState<AboutInfo | null>(null);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  useEffect(() => {
    void api.about().then(setAbout, () => undefined);
  }, []);
  async function choose(l: Lang) {
    setLang(l);
    setMsg(null);
    try {
      await api.setLanguage(l);
      setMsg({ kind: 'ok', text: t('general.lang.saved') });
    } catch (e) {
      const c = errorCode(e);
      setMsg({ kind: 'error', text: c === 'not_elevated' ? t('general.lang.notsaved') : errText(c) });
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
        {!elevated ? <p class="small">{t('general.lang.unlock')}</p> : null}
        {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
      </section>
      <section class="card">
        <h3>{t('general.theme')}</h3>
        <p>{t('general.theme.os')}</p>
      </section>
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
