// SPDX-License-Identifier: AGPL-3.0-or-later
// Device screen (VLT-IS-005 TASK 03-00-00-005 step 1; VLT-IS-006 TASK
// 03-00-00-004 step 3.4 and TASK 03-00-00-008): status, port list, port
// scan, "Use this port", Device settings.
import { useEffect, useState } from 'preact/hooks';
import { api, errorCode, type DeviceStatus, type Line, type ScanResult, type SerialPort } from '../api';
import { Confirm, errText } from '../components';
import { DASH, time } from '../format';
import { t } from '../i18n';
import { app } from '../state';
import { useStore } from '../store';

export const SCAN_TEXT = 'SCAN PORTS';
const LINES: Line[] = ['unchanged', 'on', 'off'];

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

const DEFAULT_CFG: Cfg = { transport: 'not_configured', serial_port: '', serial_dtr: 'unchanged', serial_rts: 'unchanged', allowed_ports: [] };

function portsText(l: string[] | null | undefined): string {
  return (l ?? []).join(', ');
}

function parsePorts(s: string): string[] {
  return s.split(',').map((x) => x.trim()).filter((x) => x !== '');
}

export function DeviceSettings({ cfg, elevated, onSaved }: { cfg: Cfg; elevated: boolean; onSaved: () => void }) {
  const [v, setV] = useState<Cfg>(cfg);
  const [allowed, setAllowed] = useState(portsText(cfg.allowed_ports));
  const [ask, setAsk] = useState(false);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  useEffect(() => {
    setV(cfg);
    setAllowed(portsText(cfg.allowed_ports));
  }, [cfg.transport, cfg.serial_port, cfg.serial_dtr, cfg.serial_rts, portsText(cfg.allowed_ports)]);
  async function save() {
    setAsk(false);
    try {
      await api.setDeviceSettings({
        'device.transport': v.transport,
        'device.serial_port': v.serial_port,
        'device.serial_dtr': v.serial_dtr,
        'device.serial_rts': v.serial_rts,
        'device.serial.allowed_ports': parsePorts(allowed),
      });
      setMsg({ kind: 'ok', text: t('device.saved') });
      onSaved();
    } catch (e) {
      setMsg({ kind: 'error', text: errText(errorCode(e)) });
    }
  }
  const line = (id: string, label: string, val: Line, set: (l: Line) => void) => (
    <>
      <label for={id}>{t(label)}</label>
      <select id={id} disabled={!elevated} value={val} onChange={(e) => set((e.target as HTMLSelectElement).value as Line)}>
        {LINES.map((l) => <option value={l}>{t('device.line.' + l)}</option>)}
      </select>
    </>
  );
  return (
    <section class="card" aria-labelledby="devset-title">
      <h3 id="devset-title">{t('device.settings')}</h3>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      <form class="form" onSubmit={(e) => { e.preventDefault(); setAsk(true); }}>
        <label for="dev-transport">{t('device.transport')}</label>
        <select id="dev-transport" disabled={!elevated} value={v.transport} onChange={(e) => setV({ ...v, transport: (e.target as HTMLSelectElement).value })}>
          <option value="not_configured">{t('device.transport.not_configured')}</option>
          <option value="auto">{t('device.transport.auto')}</option>
          <option value="hid">{t('device.transport.hid')}</option>
          <option value="serial">{t('device.transport.serial')}</option>
        </select>
        <label for="dev-port">{t('device.serial_port')}</label>
        <input id="dev-port" disabled={!elevated} value={v.serial_port} onInput={(e) => setV({ ...v, serial_port: (e.target as HTMLInputElement).value })} />
        <label for="dev-allowed">{t('device.allowed_ports')}</label>
        <input id="dev-allowed" disabled={!elevated} value={allowed} onInput={(e) => setAllowed((e.target as HTMLInputElement).value)} />
        {line('dev-dtr', 'device.dtr', v.serial_dtr ?? 'unchanged', (l) => setV({ ...v, serial_dtr: l }))}
        {line('dev-rts', 'device.rts', v.serial_rts ?? 'unchanged', (l) => setV({ ...v, serial_rts: l }))}
        <button type="submit" class="primary" disabled={!elevated}>{t('common.save')}</button>
      </form>
      {ask ? <Confirm text={t('device.confirm')} confirmLabel={t('device.confirm.button')} onConfirm={() => void save()} onCancel={() => setAsk(false)} /> : null}
      {msg ? <p role={msg.kind === 'error' ? 'alert' : 'status'} class={'msg msg-' + msg.kind}>{msg.text}</p> : null}
    </section>
  );
}

function desc(p: SerialPort): string {
  if (!p.is_usb) return DASH;
  const id = p.vid || p.pid ? `${p.vid ?? '?'}:${p.pid ?? '?'}` : '';
  return [p.product, id].filter((x) => x).join(' ') || DASH;
}

function resultText(r: ScanResult | undefined): string {
  if (!r) return DASH;
  if (r.result === 'response' && r.valid_q1) return t('device.scan.result.response.q1');
  return t('device.scan.result.' + r.result);
}

/** Ports is the port list with the scan and "Use this port". */
export function Ports({ elevated, onUsed }: { elevated: boolean; onUsed: () => void }) {
  const [ports, setPorts] = useState<SerialPort[] | null>(null);
  const [marked, setMarked] = useState<Record<string, boolean>>({});
  const [results, setResults] = useState<Record<string, ScanResult>>({});
  const [text, setText] = useState('');
  const [use, setUse] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);
  const load = () => api.devicePorts().then((r) => setPorts(r?.ports ?? []), (e) => setMsg({ kind: 'error', text: errText(errorCode(e)) }));
  useEffect(() => {
    void load();
  }, []);
  const list = Object.keys(marked).filter((k) => marked[k]);
  async function scan(e: Event) {
    e.preventDefault();
    setMsg(null);
    if (list.length === 0) return setMsg({ kind: 'error', text: t('device.scan.none') });
    if (text !== SCAN_TEXT) return setMsg({ kind: 'error', text: t('device.scan.mismatch') });
    try {
      const r = await api.deviceScan(list, text);
      const m: Record<string, ScanResult> = {};
      for (const x of r?.results ?? []) m[x.name] = x;
      setResults(m);
      setText('');
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  async function useNow(name: string) {
    setUse(null);
    try {
      await api.setDeviceSettings({ 'device.transport': 'serial', 'device.serial_port': name });
      setMsg({ kind: 'ok', text: t('device.saved') });
      onUsed();
    } catch (x) {
      setMsg({ kind: 'error', text: errText(errorCode(x)) });
    }
  }
  return (
    <section class="card" aria-labelledby="ports-title">
      <h3 id="ports-title">{t('device.ports')}</h3>
      {!elevated ? <p class="note">{t('locked.need')}</p> : null}
      {ports && ports.length === 0 ? <p>{t('device.ports.none')}</p> : null}
      {ports && ports.length > 0 ? (
        <table class="table">
          <caption>{t('device.ports')}</caption>
          <thead>
            <tr>
              <th scope="col">{t('device.ports.mark')}</th>
              <th scope="col">{t('device.ports.name')}</th>
              <th scope="col">{t('device.ports.desc')}</th>
              <th scope="col">{t('device.ports.result')}</th>
              <th scope="col">{t('device.ports.use')}</th>
            </tr>
          </thead>
          <tbody>
            {ports.map((p) => (
              <tr>
                <td>
                  <input type="checkbox" aria-label={t('device.ports.mark.label', { name: p.name })} disabled={!elevated} checked={!!marked[p.name]}
                    onChange={(e) => setMarked({ ...marked, [p.name]: (e.target as HTMLInputElement).checked })} />
                </td>
                <td><code>{p.name}</code></td>
                <td>{desc(p)}</td>
                <td>{resultText(results[p.name])}</td>
                <td>
                  <button type="button" aria-label={t('device.ports.use.label', { name: p.name })} disabled={!elevated} onClick={() => setUse(p.name)}>{t('device.ports.use')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      <button type="button" onClick={() => void load()}>{t('common.refresh')}</button>
      <div class="banner banner-warn" role="note">
        <span aria-hidden="true" class="banner-icon">⚠</span>
        <span>{t('device.scan.warning')}</span>
      </div>
      <form class="form" onSubmit={(e) => void scan(e)}>
        <label for="scan-text">{t('device.scan.label', { text: SCAN_TEXT })}</label>
        <input id="scan-text" disabled={!elevated} value={text} autoComplete="off" onInput={(e) => setText((e.target as HTMLInputElement).value)} />
        <button type="submit" class="danger" disabled={!elevated || text !== SCAN_TEXT || list.length === 0}>{t('device.scan.button')}</button>
      </form>
      {use ? (
        <Confirm text={t('device.ports.use.confirm', { name: use })} confirmLabel={t('device.ports.use.button')} onConfirm={() => void useNow(use)} onCancel={() => setUse(null)} />
      ) : null}
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
  const elevated = !!s.auth?.elevated;
  return (
    <div class="screen">
      <h2>{t('nav.device')}</h2>
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
      <section class="card" aria-labelledby="dev-title">
        <h3 id="dev-title">{t('device.status')}</h3>
        <dl class="kv">
          <div><dt>{t('device.state')}</dt><dd>{d?.state ? t('device.state.' + d.state) : DASH}</dd></div>
          <div><dt>{t('device.since')}</dt><dd>{time(d?.since)}</dd></div>
          <div><dt>{t('device.transport')}</dt><dd>{p?.transport ?? DASH}</dd></div>
          <div><dt>{t('device.port')}</dt><dd>{p?.port || DASH}</dd></div>
          <div><dt>{t('device.dialect')}</dt><dd>{p?.dialect ?? DASH}</dd></div>
          <div><dt>{t('device.supported')}</dt><dd>{p?.supported?.join(', ') || DASH}</dd></div>
          <div><dt>{t('device.serial')}</dt><dd>{p?.serial_number || DASH}</dd></div>
        </dl>
        {d?.state === 'port busy' ? <p role="alert" class="msg msg-error">{t('device.busy.help')}</p> : null}
        {d?.state === 'not configured' ? <p class="note">{t('device.notconfigured.help')}</p> : null}
        <button type="button" onClick={() => void load()}>{t('common.refresh')}</button>
      </section>
      <Ports elevated={elevated} onUsed={() => void load()} />
      <section class="card"><h3>{t('device.nominal')}</h3><KV obj={p?.nominal} /></section>
      <section class="card"><h3>{t('device.model')}</h3><KV obj={p?.info} /><KV obj={p?.model_data} /><KV obj={p?.firmware} /></section>
      {d ? <DeviceSettings cfg={{ ...DEFAULT_CFG, ...(d.config ?? {}) }} elevated={elevated} onSaved={() => void load()} /> : null}
    </div>
  );
}
