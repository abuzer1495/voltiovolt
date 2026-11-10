// SPDX-License-Identifier: AGPL-3.0-or-later
// Device screen (TASK 03-00-00-005 step 1).
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type DeviceStatus } from '../api';
import { Confirm, errText } from '../components';
import { DASH, time } from '../format';
import { t } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';

function KV({ obj }: { obj?: Record<string, unknown> }) {
  if (!obj) return <p>{DASH}</p>;
  return (
    <dl class="kv">
      {Object.entries(obj).map(([k, v]) => (
        <div>
          <dt>{k}</dt>
          <dd>{v === null || v === undefined || v === '' ? DASH : typeof v === 'object' ? JSON.stringify(v) : String(v)}</dd>
        </div>
      ))}
    </dl>
  );
}

type Cfg = DeviceStatus['config'];

export function DeviceSettings({ cfg, elevated, onSaved }: { cfg: Cfg; elevated: boolean; onSaved: () => void }) {
  const [v, setV] = useState<Cfg>(cfg);
  const [ask, setAsk] = useState(false);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  useEffect(() => setV(cfg), [cfg.transport, cfg.serial_port, cfg.serial_dtr, cfg.serial_rts]);
  async function save() {
    setAsk(false);
    try {
      await api.setDeviceSettings({
        'device.transport': v.transport,
        'device.serial_port': v.serial_port,
        'device.serial_dtr': v.serial_dtr,
        'device.serial_rts': v.serial_rts,
      });
      setMsg({ kind: 'ok', text: t('device.saved') });
      onSaved();
    } catch (e) {
      setMsg({ kind: 'error', text: errText(errorCode(e)) });
    }
  }
  return (
    <section class="card" aria-labelledby="devset-title">
      <h3 id="devset-title">{t('device.settings')}</h3>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <form class="form" onSubmit={(e) => { e.preventDefault(); setAsk(true); }}>
        <label for="dev-transport">{t('device.transport')}</label>
        <select id="dev-transport" disabled={!elevated} value={v.transport} onChange={(e) => setV({ ...v, transport: (e.target as HTMLSelectElement).value })}>
          <option value="auto">{t('device.transport.auto')}</option>
          <option value="hid">{t('device.transport.hid')}</option>
          <option value="serial">{t('device.transport.serial')}</option>
        </select>
        <label for="dev-port">{t('device.serial_port')}</label>
        <input id="dev-port" disabled={!elevated} value={v.serial_port} onInput={(e) => setV({ ...v, serial_port: (e.target as HTMLInputElement).value })} />
        <label class="check">
          <input type="checkbox" disabled={!elevated} checked={v.serial_dtr} onChange={(e) => setV({ ...v, serial_dtr: (e.target as HTMLInputElement).checked })} /> {t('device.dtr')}
        </label>
        <label class="check">
          <input type="checkbox" disabled={!elevated} checked={v.serial_rts} onChange={(e) => setV({ ...v, serial_rts: (e.target as HTMLInputElement).checked })} /> {t('device.rts')}
        </label>
        <button type="submit" class="primary" disabled={!elevated}>{t('common.save')}</button>
      </form>
      {ask ? <Confirm text={t('device.confirm')} confirmLabel={t('device.confirm.button')} onConfirm={() => void save()} onCancel={() => setAsk(false)} /> : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

export function Device() {
  const s = useStore(app);
  const [d, setD] = useState<DeviceStatus | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const load = () => api.deviceStatus().then(setD, (e) => setErr(errText(errorCode(e))));
  useEffect(() => {
    void load();
  }, []);
  const p = d?.profile;
  return (
    <div class="screen">
      <h2>{t('nav.device')}</h2>
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
      <section class="card" aria-labelledby="dev-title">
        <h3 id="dev-title">{t('device.status')}</h3>
        <dl class="kv">
          <div><dt>{t('device.state')}</dt><dd>{d ? t('device.state.' + d.state) : DASH}</dd></div>
          <div><dt>{t('device.since')}</dt><dd>{time(d?.since)}</dd></div>
          <div><dt>{t('device.transport')}</dt><dd>{p?.transport ?? DASH}</dd></div>
          <div><dt>{t('device.port')}</dt><dd>{p?.port || DASH}</dd></div>
          <div><dt>{t('device.dialect')}</dt><dd>{p?.dialect ?? DASH}</dd></div>
          <div><dt>{t('device.supported')}</dt><dd>{p?.supported?.join(', ') || DASH}</dd></div>
          <div><dt>{t('device.serial')}</dt><dd>{p?.serial_number || DASH}</dd></div>
        </dl>
        <button type="button" onClick={() => void load()}>{t('common.refresh')}</button>
      </section>
      <section class="card"><h3>{t('device.nominal')}</h3><KV obj={p?.nominal} /></section>
      <section class="card"><h3>{t('device.model')}</h3><KV obj={p?.info} /><KV obj={p?.model_data} /><KV obj={p?.firmware} /></section>
      {d ? <DeviceSettings cfg={d.config} elevated={!!s.auth?.elevated} onSaved={() => void load()} /> : null}
    </div>
  );
}
