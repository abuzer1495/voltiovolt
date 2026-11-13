// SPDX-License-Identifier: AGPL-3.0-or-later
// History screen (TASK 03-00-00-005 step 2): uPlot charts with a text table
// for screen readers.
import { useEffect, useRef, useState } from 'preact/hooks';
import uPlot from 'uplot';
import { api, errorCode, type Series } from '../api';
import { errText } from '../components';
import { DASH, num, time } from '../format';
import { t } from '../i18n';

export interface Range {
  id: string;
  ms: number;
  step: 'raw' | '1m' | '1h';
}

export const RANGES: Range[] = [
  { id: '1h', ms: 3600e3, step: 'raw' },
  { id: '24h', ms: 24 * 3600e3, step: '1m' },
  { id: '7d', ms: 7 * 24 * 3600e3, step: '1m' },
  { id: '30d', ms: 30 * 24 * 3600e3, step: '1h' },
  { id: '1y', ms: 365 * 24 * 3600e3, step: '1h' },
];

interface ChartDef {
  id: string;
  title: string;
  names: string[];
}

export const CHARTS: ChartDef[] = [
  { id: 'voltage', title: 'hist.voltages', names: ['input.voltage', 'output.voltage'] },
  { id: 'frequency', title: 'hist.frequency', names: ['input.frequency', 'output.frequency'] },
  { id: 'load', title: 'hist.load', names: ['load.percent'] },
  { id: 'battery', title: 'hist.battery', names: ['battery.voltage', 'battery.charge', 'battery.charge_estimate'] },
  { id: 'temperature', title: 'hist.temperature', names: ['ups.temperature'] },
];

const STROKES = ['#0b57d0', '#b3261e'];
const DASHES = [[], [6, 4]]; // the second series is dashed: not color only

/** align joins series on the time axis (seconds). */
export function align(series: Series[]): (number | null)[][] {
  const ts = new Set<number>();
  for (const s of series) for (const r of s.rows) ts.add(Math.floor(Date.parse(r.ts) / 1000));
  const xs = [...ts].sort((a, b) => a - b);
  const idx = new Map(xs.map((x, i) => [x, i]));
  const out: (number | null)[][] = [xs];
  for (const s of series) {
    const col: (number | null)[] = new Array(xs.length).fill(null);
    for (const r of s.rows) col[idx.get(Math.floor(Date.parse(r.ts) / 1000))!] = r.mean;
    out.push(col);
  }
  return out;
}

/** sample returns at most n evenly spaced indexes of a length. */
export function sample(len: number, n: number): number[] {
  if (len <= n) return [...Array(len).keys()];
  const out: number[] = [];
  for (let i = 0; i < n; i++) out.push(Math.round((i * (len - 1)) / (n - 1)));
  return out;
}

const TABLE_ROWS = 48;

function Chart({ def, series }: { def: ChartDef; series: Series[] }) {
  const ref = useRef<HTMLDivElement>(null);
  const data = align(series);
  const label = (i: number) => `${t('m.' + def.names[i])} (${series[i]?.unit || DASH})`;
  useEffect(() => {
    if (!ref.current || data[0].length === 0) return;
    const css = getComputedStyle(document.documentElement);
    const fg = css.getPropertyValue('--fg').trim() || '#000';
    const grid = css.getPropertyValue('--border').trim() || '#888';
    const scales: uPlot.Scales = { x: { time: true } };
    const axes: uPlot.Axis[] = [{ stroke: fg, grid: { stroke: grid, width: 0.5 } }, { stroke: fg, grid: { stroke: grid, width: 0.5 }, scale: 'a' }];
    const two = def.id === 'battery';
    if (two) axes.push({ stroke: fg, side: 1, scale: 'b', grid: { show: false } });
    const opts: uPlot.Options = {
      width: Math.max(320, ref.current.clientWidth || 640),
      height: 220,
      scales,
      axes,
      legend: { show: true },
      series: [
        {},
        ...def.names.map((_, i) => ({
          label: label(i),
          stroke: STROKES[i % 2],
          dash: DASHES[i % 2],
          width: 2,
          scale: two && i === 1 ? 'b' : 'a',
        })),
      ],
    };
    const u = new uPlot(opts, data as uPlot.AlignedData, ref.current);
    return () => u.destroy();
  }, [series]);
  const idx = sample(data[0].length, TABLE_ROWS);
  return (
    <section class="card chart" aria-labelledby={'ch-' + def.id}>
      <h3 id={'ch-' + def.id}>{t(def.title)}</h3>
      {data[0].length === 0 ? <p>{t('hist.nodata')}</p> : <div ref={ref} class="plot" aria-hidden="true" />}
      <details>
        <summary>{t('hist.table')}</summary>
        <table class="table">
          <caption>{t('hist.table.caption', { title: t(def.title), n: idx.length, total: data[0].length })}</caption>
          <thead>
            <tr>
              <th scope="col">{t('event.time')}</th>
              {def.names.map((_, i) => <th scope="col">{label(i)}</th>)}
            </tr>
          </thead>
          <tbody>
            {idx.map((i) => (
              <tr>
                <td>{time(new Date((data[0][i] as number) * 1000).toISOString())}</td>
                {def.names.map((_, j) => <td>{num(data[j + 1][i] as number | null, 1)}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </details>
    </section>
  );
}

export function History() {
  const [range, setRange] = useState<Range>(RANGES[0]);
  const [data, setData] = useState<Record<string, Series>>({});
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showDemo, setShowDemo] = useState(false);
  useEffect(() => {
    let stop = false;
    const to = Date.now();
    const from = to - range.ms;
    setBusy(true);
    setErr(null);
    const names = CHARTS.flatMap((c) => c.names);
    Promise.all(names.map((n) => api.measurements(n, from, to, range.step, showDemo)))
      .then((list) => {
        if (stop) return;
        const m: Record<string, Series> = {};
        list.forEach((s, i) => (m[names[i]] = s));
        setData(m);
      })
      .catch((e) => !stop && setErr(errText(errorCode(e))))
      .finally(() => !stop && setBusy(false));
    return () => {
      stop = true;
    };
  }, [range, showDemo]);
  return (
    <div class="screen">
      <h2>{t('nav.history')}</h2>
      <div class="row" role="group" aria-label={t('hist.range')}>
        {RANGES.map((r) => (
          <button type="button" aria-pressed={r.id === range.id} class={r.id === range.id ? 'primary' : ''} onClick={() => setRange(r)}>
            {t('hist.range.' + r.id)}
          </button>
        ))}
        <label class="check" for="hist-demo">
          <input id="hist-demo" type="checkbox" checked={showDemo} onChange={(e) => setShowDemo((e.target as HTMLInputElement).checked)} />
          {t('hist.show_demo')}
        </label>
      </div>
      {busy ? <p role="status">{t('common.loading')}</p> : null}
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
      {CHARTS.map((c) => (
        <Chart def={c} series={c.names.map((n) => data[n] ?? { name: n, unit: '', step: range.step, rows: [] })} />
      ))}
    </div>
  );
}
