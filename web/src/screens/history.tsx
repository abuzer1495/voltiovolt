// SPDX-License-Identifier: AGPL-3.0-or-later
// History screen (TASK 03-00-00-005 step 2): uPlot charts with a text table
// for screen readers. VLT-IS-010 TASK 03-00-00-001 (RQ-03, OD-49): one
// chart object per chart; setData on a refresh; series visibility is a
// user choice and is stored in ui.history.series_visibility.
// VLT-IS-011 TASK 03-00-00-002 (R-09): live updates. Only the points
// newer than the last timestamp are read; setData on the same chart;
// points older than the range are dropped; a zoom is kept.
import { useEffect, useRef, useState } from 'preact/hooks';
import uPlot from 'uplot';
import { api, errorCode, type Series } from '../api';
import { errText } from '../components';
import { DASH, num, time } from '../format';
import { t } from '../i18n';
import { view } from '../state';
import { useStore } from '../store';

export interface Range {
  id: string;
  ms: number;
  step: 'raw' | '1m' | '1h';
  /** live is the time between two live updates (VLT-IS-011 002). */
  live: number;
}

export const RANGES: Range[] = [
  { id: '1h', ms: 3600e3, step: 'raw', live: 2e3 },
  { id: '24h', ms: 24 * 3600e3, step: '1m', live: 60e3 },
  { id: '7d', ms: 7 * 24 * 3600e3, step: '1m', live: 60e3 },
  { id: '30d', ms: 30 * 24 * 3600e3, step: '1h', live: 600e3 },
  { id: '1y', ms: 365 * 24 * 3600e3, step: '1h', live: 600e3 },
];

/** lastTs is the time (ms) of the last row of a series, or null. */
export function lastTs(s: Series | undefined): number | null {
  const r = s?.rows;
  if (!r || r.length === 0) return null;
  return Date.parse(r[r.length - 1].ts);
}

/**
 * mergeSeries adds the rows of a live read to a series. A new row with the
 * time of an old row replaces it (the open 1m or 1h bucket). Rows older
 * than fromMs (the start of the range) are dropped.
 */
export function mergeSeries(old: Series | undefined, add: Series, fromMs: number): Series {
  const rows = add.rows ?? [];
  const first = rows.length ? Date.parse(rows[0].ts) : Infinity;
  const kept = (old?.rows ?? []).filter((r) => {
    const x = Date.parse(r.ts);
    return x >= fromMs && x < first;
  });
  return { ...(old ?? add), unit: add.unit || old?.unit || '', rows: [...kept, ...rows.filter((r) => Date.parse(r.ts) >= fromMs)] };
}

/**
 * nextView is the x scale after new data. Without a zoom, null: the chart
 * shows the full data again. With a zoom that includes the last point
 * ("now"), the view moves with the new data. Other zooms stay.
 */
export function nextView(min: number | null | undefined, max: number | null | undefined, oldFirst: number | undefined, oldLast: number | undefined, newLast: number | undefined): { min: number; max: number } | null | 'keep' {
  if (min == null || max == null || oldFirst == null || oldLast == null) return null;
  const zoomed = min > oldFirst + 0.5 || max < oldLast - 0.5;
  if (!zoomed) return null;
  if (newLast != null && max >= oldLast - 0.5 && newLast > oldLast) {
    const d = newLast - oldLast;
    return { min: min + d, max: max + d };
  }
  return 'keep';
}

export interface ChartDef {
  id: string;
  title: string;
  names: string[];
  /** two: the second series has its own scale on the right. */
  two?: boolean;
}

export const CHARTS: ChartDef[] = [
  { id: 'voltage', title: 'hist.voltages', names: ['input.voltage', 'output.voltage'] },
  { id: 'frequency', title: 'hist.frequency', names: ['input.frequency', 'output.frequency'] },
  { id: 'load', title: 'hist.load', names: ['load.percent'] },
  { id: 'battery', title: 'hist.battery', names: ['battery.voltage', 'battery.charge', 'battery.charge_estimate'], two: true },
  { id: 'temperature', title: 'hist.temperature', names: ['ups.temperature'] },
];

/** SIMPLE_CHART is the 1 h live chart of the Simple mode (VLT-IS-011 003). */
export const SIMPLE_CHART: ChartDef = { id: 'simple', title: 'simple.chart', names: ['input.voltage', 'load.percent'], two: true };

/** SERIES_VISIBILITY_KEY is the "ui." Setting of OD-49. */
export const SERIES_VISIBILITY_KEY = 'ui.history.series_visibility';

const STROKES = ['#0b57d0', '#b3261e'];
const DASHES = [[], [6, 4]]; // the second series is dashed: not color only

/** seriesIsVisible is true unless the stored map has an explicit false. */
export function seriesIsVisible(vis: Record<string, boolean>, name: string): boolean {
  return vis[name] !== false;
}

/** parseSeriesVisibility reads the Setting value. Unknown shapes give {}. */
export function parseSeriesVisibility(v: unknown): Record<string, boolean> {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return {};
  const out: Record<string, boolean> = {};
  for (const [k, x] of Object.entries(v as Record<string, unknown>)) {
    if (typeof x === 'boolean') out[k] = x;
  }
  return out;
}

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

function Chart({ def, series, vis, onVisible }: {
  def: ChartDef;
  series: Series[];
  vis: Record<string, boolean>;
  onVisible: (name: string, show: boolean) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const plot = useRef<uPlot | null>(null);
  const visRef = useRef(vis);
  visRef.current = vis;
  const onVisibleRef = useRef(onVisible);
  onVisibleRef.current = onVisible;
  const data = align(series);
  const label = (i: number) => `${t('m.' + def.names[i])} (${series[i]?.unit || DASH})`;

  // Destroy only when this chart unmounts, not on a data refresh (RQ-03).
  useEffect(() => () => {
    plot.current?.destroy();
    plot.current = null;
  }, [def.id]);

  useEffect(() => {
    if (!ref.current || data[0].length === 0) return;
    if (plot.current) {
      const u = plot.current;
      const xs = (u.data?.[0] ?? []) as number[];
      const v = nextView(u.scales?.x?.min, u.scales?.x?.max, xs[0], xs[xs.length - 1], data[0][data[0].length - 1] as number | undefined);
      if (v === null) {
        u.setData(data as uPlot.AlignedData);
      } else {
        // Keep the zoom of the user; follow "now" if the view includes it.
        u.setData(data as uPlot.AlignedData, false);
        if (v !== 'keep') u.setScale('x', v);
      }
      return;
    }
    const css = getComputedStyle(document.documentElement);
    const fg = css.getPropertyValue('--fg').trim() || '#000';
    const grid = css.getPropertyValue('--border').trim() || '#888';
    const scales: uPlot.Scales = { x: { time: true } };
    const axes: uPlot.Axis[] = [{ stroke: fg, grid: { stroke: grid, width: 0.5 } }, { stroke: fg, grid: { stroke: grid, width: 0.5 }, scale: 'a' }];
    const two = !!def.two;
    if (two) axes.push({ stroke: fg, side: 1, scale: 'b', grid: { show: false } });
    const u = new uPlot({
      width: Math.max(320, ref.current.clientWidth || 640),
      height: 220,
      scales,
      axes,
      legend: { show: true },
      series: [
        {},
        ...def.names.map((n, i) => ({
          label: label(i),
          stroke: STROKES[i % 2],
          dash: DASHES[i % 2],
          width: 2,
          scale: two && i === 1 ? 'b' : 'a',
          show: seriesIsVisible(visRef.current, n),
        })),
      ],
      hooks: {
        setSeries: [
          (p, idx) => {
            if (idx == null || idx < 1 || idx > def.names.length) return;
            const name = def.names[idx - 1];
            const show = p.series[idx].show !== false;
            if (seriesIsVisible(visRef.current, name) === show) return;
            onVisibleRef.current(name, show);
          },
        ],
      },
    }, data as uPlot.AlignedData, ref.current);
    plot.current = u;
  }, [data, def]);

  useEffect(() => {
    const u = plot.current;
    if (!u) return;
    def.names.forEach((n, i) => {
      const show = seriesIsVisible(vis, n);
      if (u.series[i + 1] && u.series[i + 1].show !== show) {
        u.setSeries(i + 1, { show }, false);
      }
    });
  }, [vis, def.names]);

  const idx = sample(data[0].length, TABLE_ROWS);
  return (
    <section class="card chart" aria-labelledby={'ch-' + def.id} data-chart={def.id}>
      <h3 id={'ch-' + def.id}>{t(def.title)}</h3>
      <div class="row" role="group" aria-label={t('hist.series')}>
        {def.names.map((n, i) => (
          <label class="check" for={'ch-vis-' + def.id + '-' + i}>
            <input
              id={'ch-vis-' + def.id + '-' + i}
              type="checkbox"
              checked={seriesIsVisible(vis, n)}
              data-series={n}
              onChange={(e) => onVisible(n, (e.target as HTMLInputElement).checked)}
            />
            {label(i)}
          </label>
        ))}
      </div>
      {data[0].length === 0 ? <p>{t('hist.nodata')}</p> : null}
      <div ref={ref} class="plot" aria-hidden="true" hidden={data[0].length === 0} />
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

export function History({ simple = false }: { simple?: boolean } = {}) {
  const charts = simple ? [SIMPLE_CHART] : CHARTS;
  const [range, setRange] = useState<Range>(RANGES[0]);
  const [data, setData] = useState<Record<string, Series>>({});
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showDemo, setShowDemo] = useState(false);
  const [vis, setVis] = useState<Record<string, boolean>>({});
  const [visReady, setVisReady] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [paused, setPaused] = useState(false);
  const [docVisible, setDocVisible] = useState(typeof document === 'undefined' || document.visibilityState !== 'hidden');
  const win = useStore(view);
  const dataRef = useRef<Record<string, Series>>({});
  dataRef.current = data;
  const loaded = useRef(0); // the time of the last full read or live update
  const live = !paused && docVisible && win.visible;

  useEffect(() => {
    const f = () => setDocVisible(document.visibilityState !== 'hidden');
    document.addEventListener('visibilitychange', f);
    return () => document.removeEventListener('visibilitychange', f);
  }, []);

  useEffect(() => {
    let stop = false;
    void api.settings().then((r) => {
      if (stop) return;
      const s = (r.settings ?? []).find((x) => x.key === SERIES_VISIBILITY_KEY);
      if (s) setVis(parseSeriesVisibility(s.value));
    }, () => undefined).finally(() => { if (!stop) setVisReady(true); });
    return () => {
      stop = true;
    };
  }, []);

  useEffect(() => {
    let stop = false;
    const to = Date.now();
    const from = to - range.ms;
    setBusy(true);
    setErr(null);
    const names = charts.flatMap((c) => c.names);
    Promise.all(names.map((n) => api.measurements(n, from, to, range.step, showDemo)))
      .then((list) => {
        if (stop) return;
        const m: Record<string, Series> = {};
        list.forEach((s, i) => (m[names[i]] = s));
        loaded.current = Date.now();
        setData(m);
      })
      .catch((e) => !stop && setErr(errText(errorCode(e))))
      .finally(() => !stop && setBusy(false));
    return () => {
      stop = true;
    };
  }, [range, showDemo, refresh]);

  // Live updates (VLT-IS-011 002): read only the points after the last
  // timestamp of each series. No updates while paused, while the screen is
  // not shown (unmounted) or while the window is minimised or hidden.
  useEffect(() => {
    if (!live) return;
    let stop = false;
    let inflight = false;
    const tick = () => {
      if (stop || inflight || loaded.current === 0) return;
      inflight = true;
      const to = Date.now();
      const from = to - range.ms;
      const names = charts.flatMap((c) => c.names);
      const cur = dataRef.current;
      Promise.all(names.map((n) => api.measurements(n, lastTs(cur[n]) ?? from, to, range.step, showDemo)))
        .then((list) => {
          if (stop) return;
          const m: Record<string, Series> = { ...dataRef.current };
          list.forEach((s, i) => {
            if (s) m[names[i]] = mergeSeries(m[names[i]], s, from);
          });
          loaded.current = Date.now();
          setErr(null);
          setData(m);
        })
        .catch((e) => !stop && setErr(errText(errorCode(e))))
        .finally(() => { inflight = false; });
    };
    // After a pause or a hidden window, catch up at once.
    if (loaded.current !== 0 && Date.now() - loaded.current >= range.live) tick();
    const id = setInterval(tick, range.live);
    return () => {
      stop = true;
      clearInterval(id);
    };
  }, [live, range, showDemo]);

  const onVisible = (name: string, show: boolean) => {
    setVis((cur) => {
      const next = { ...cur, [name]: show };
      void api.setSetting(SERIES_VISIBILITY_KEY, next);
      return next;
    });
  };

  const liveControls = (
    <>
      <button type="button" aria-pressed={paused} onClick={() => setPaused((p) => !p)}>{t(paused ? 'hist.resume' : 'hist.pause')}</button>
      <span class={'live-badge' + (live ? ' live-on' : '')} role="status" data-live={live ? 'on' : 'off'}>
        {live ? t('hist.live') : t('hist.live.off')}
      </span>
    </>
  );
  if (simple) {
    return (
      <div class="simple-chart">
        <div class="row">{liveControls}</div>
        {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
        {visReady ? (
          <Chart
            def={SIMPLE_CHART}
            series={SIMPLE_CHART.names.map((n) => data[n] ?? { name: n, unit: '', step: range.step, rows: [] })}
            vis={vis}
            onVisible={onVisible}
          />
        ) : null}
      </div>
    );
  }
  return (
    <div class="screen">
      <h2>{t('nav.history')}</h2>
      <div class="row" role="group" aria-label={t('hist.range')}>
        {RANGES.map((r) => (
          <button type="button" aria-pressed={r.id === range.id} class={r.id === range.id ? 'primary' : ''} onClick={() => setRange(r)}>
            {t('hist.range.' + r.id)}
          </button>
        ))}
        <button type="button" onClick={() => setRefresh((n) => n + 1)}>{t('hist.refresh')}</button>
        {liveControls}
        <label class="check" for="hist-demo">
          <input id="hist-demo" type="checkbox" checked={showDemo} onChange={(e) => setShowDemo((e.target as HTMLInputElement).checked)} />
          {t('hist.show_demo')}
        </label>
      </div>
      {busy ? <p role="status">{t('common.loading')}</p> : null}
      {err ? <p role="alert" class="msg msg-error">{err}</p> : null}
      {visReady ? charts.map((c) => (
        <Chart
          def={c}
          series={c.names.map((n) => data[n] ?? { name: n, unit: '', step: range.step, rows: [] })}
          vis={vis}
          onVisible={onVisible}
        />
      )) : null}
    </div>
  );
}
