// SPDX-License-Identifier: AGPL-3.0-or-later
// Live History charts (VLT-IS-011 TASK 03-00-00-002, R-09).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Series } from '../src/api';
import { setLang, t } from '../src/i18n';
import { History, RANGES, mergeSeries, nextView } from '../src/screens/history';
import { view } from '../src/state';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

// uPlot needs a canvas. The fake records setData and setScale.
interface FakePlot {
  data: number[][];
  scales: { x: { min?: number; max?: number } };
  series: { show?: boolean }[];
  setDataCalls: { reset: boolean }[];
  setScaleCalls: { min: number; max: number }[];
}
const plots: FakePlot[] = [];
vi.mock('uplot', () => {
  class Fake {
    data: number[][];
    scales: { x: { min?: number; max?: number } } = { x: {} };
    series: { show?: boolean }[];
    setDataCalls: { reset: boolean }[] = [];
    setScaleCalls: { min: number; max: number }[] = [];
    constructor(opts: { series: { show?: boolean }[] }, data: number[][]) {
      this.series = opts.series.map((s) => ({ ...s }));
      this.data = data;
      this.fit();
      plots.push(this as unknown as FakePlot);
    }
    fit() {
      const xs = this.data[0];
      this.scales.x = { min: xs[0], max: xs[xs.length - 1] };
    }
    setData(d: number[][], reset = true): void {
      this.data = d;
      this.setDataCalls.push({ reset });
      if (reset) this.fit();
    }
    setScale(_k: string, v: { min: number; max: number }): void {
      this.scales.x = { ...v };
      this.setScaleCalls.push(v);
    }
    setSeries(i: number, opts: { show?: boolean }): void {
      this.series[i] = { ...this.series[i], ...opts };
    }
    destroy(): void { /* no canvas */ }
  }
  return { default: Fake };
});

const T0 = Date.parse('2026-11-14T10:00:00Z');
const row = (ms: number, mean: number) => ({ ts: new Date(ms).toISOString(), min: mean, max: mean, mean, count: 1 });

let root: HTMLElement;
beforeEach(() => {
  vi.useFakeTimers({ now: T0 });
  fake.reset();
  setLang('en');
  plots.length = 0;
  view.set({ visible: true });
  Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
  // One point each second between from and to.
  fake.handlers.Measurements = (name: unknown, from: unknown, to: unknown, step: unknown) => {
    const f = Math.ceil(Number(from) / 1000) * 1000;
    const rows = [];
    for (let x = f; x <= Number(to); x += 1000) rows.push(row(x, 230));
    return { name: String(name), unit: 'V', step: String(step), rows: rows.slice(-200) };
  };
  fake.handlers.Settings = () => ({ settings: [] });
  fake.handlers.SetSetting = () => undefined;
  root = document.createElement('div');
  document.body.appendChild(root);
});
afterEach(() => {
  render(null, root);
  root.remove();
  vi.useRealTimers();
});

async function advance(ms: number) {
  await act(async () => { await vi.advanceTimersByTimeAsync(ms); });
}
async function show() {
  await act(async () => render(<History />, root));
  await advance(0);
  await advance(0);
}
const reads = () => fake.calls.filter((c) => c.method === 'Measurements');
function btn(text: string): HTMLButtonElement {
  const b = [...root.querySelectorAll('button')].find((x) => x.textContent?.trim() === text);
  if (!b) throw new Error('no button ' + text);
  return b as HTMLButtonElement;
}

describe('live update helpers', () => {
  it('uses the update times of the instruction', () => {
    expect(RANGES.map((r) => [r.id, r.step, r.live])).toEqual([
      ['1h', 'raw', 2000], ['24h', '1m', 60000], ['7d', '1m', 60000], ['30d', '1h', 600000], ['1y', '1h', 600000],
    ]);
  });

  it('adds newer points, replaces the open bucket and drops old points', () => {
    const old: Series = { name: 'load.percent', unit: '%', step: '1m', rows: [row(0, 1), row(60e3, 2), row(120e3, 3)] };
    const add: Series = { name: 'load.percent', unit: '%', step: '1m', rows: [row(120e3, 4), row(180e3, 5)] };
    const m = mergeSeries(old, add, 60e3);
    expect(m.rows.map((r) => [Date.parse(r.ts), r.mean])).toEqual([[60e3, 2], [120e3, 4], [180e3, 5]]);
  });

  it('keeps a zoom and follows now only when the view includes now', () => {
    expect(nextView(0, 100, 0, 100, 102)).toBeNull(); // no zoom: show all
    expect(nextView(50, 100, 0, 100, 102)).toEqual({ min: 52, max: 102 }); // zoom at now: follow
    expect(nextView(10, 50, 0, 100, 102)).toBe('keep'); // zoom in the past: keep
  });
});

describe('History live updates', () => {
  it('reads only new points every 2 s in 1 h and keeps the chart objects', async () => {
    await show();
    expect(plots).toHaveLength(5);
    const first = reads().length;
    expect(first).toBe(9);
    expect(root.querySelector('.live-badge')?.textContent).toBe(t('hist.live'));
    await advance(2000);
    const live = reads().slice(first);
    expect(live).toHaveLength(9);
    for (const c of live) {
      // from = the last timestamp of the series, not the start of the range
      expect(c.args[1]).toBe(T0);
      expect(c.args[2]).toBe(T0 + 2000);
      expect(c.args[3]).toBe('raw');
    }
    expect(plots).toHaveLength(5);
    expect(plots[0].setDataCalls.length).toBeGreaterThan(0);
    expect(plots[0].data[0][plots[0].data[0].length - 1]).toBe((T0 + 2000) / 1000);
    // points older than the range are dropped
    vi.setSystemTime(Date.now() + 3600e3);
    await advance(2000);
    expect(plots[0].data[0][0]).toBeGreaterThanOrEqual((Date.now() - 3600e3) / 1000);
  });

  it('keeps a hidden series hidden during live updates', async () => {
    await show();
    const box = root.querySelector('input[data-series="input.voltage"]') as HTMLInputElement;
    await act(async () => {
      box.checked = false;
      box.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await advance(6000);
    expect((root.querySelector('input[data-series="input.voltage"]') as HTMLInputElement).checked).toBe(false);
    expect(plots[0].series[1].show).toBe(false);
  });

  it('keeps the zoom, and follows new data when the zoom includes now', async () => {
    await show();
    const p = plots[0];
    const xs = p.data[0];
    const last = xs[xs.length - 1];
    p.scales.x = { min: last - 100, max: last };
    await advance(2000);
    expect(p.setDataCalls[p.setDataCalls.length - 1].reset).toBe(false);
    expect(p.scales.x).toEqual({ min: last - 98, max: last + 2 });
    p.scales.x = { min: last - 500, max: last - 300 };
    await advance(2000);
    expect(p.scales.x).toEqual({ min: last - 500, max: last - 300 });
  });

  it('Pause stops the updates; Resume catches up', async () => {
    await show();
    await act(async () => btn(t('hist.pause')).click());
    expect(root.querySelector('.live-badge')?.textContent).toBe(t('hist.live.off'));
    const n = reads().length;
    await advance(10000);
    expect(reads().length).toBe(n);
    await act(async () => btn(t('hist.resume')).click());
    await advance(0);
    expect(reads().length).toBe(n + 9);
    expect(btn(t('hist.refresh'))).toBeTruthy();
  });

  it('stops while the window is minimised or the page is hidden', async () => {
    await show();
    const n = reads().length;
    await act(async () => view.set({ visible: false }));
    await advance(10000);
    expect(reads().length).toBe(n);
    await act(async () => view.set({ visible: true }));
    await advance(0);
    expect(reads().length).toBe(n + 9);
    const m = reads().length;
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await advance(10000);
    expect(reads().length).toBe(m);
  });

  it('updates 24 h every 60 s with 1m data', async () => {
    await show();
    await act(async () => btn(t('hist.range.24h')).click());
    await advance(0);
    await advance(0);
    const n = reads().length;
    await advance(30000);
    expect(reads().length).toBe(n);
    await advance(30000);
    const live = reads().slice(n);
    expect(live).toHaveLength(9);
    expect(live.every((c) => c.args[3] === '1m')).toBe(true);
  });
});
