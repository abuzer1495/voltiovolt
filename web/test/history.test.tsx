// SPDX-License-Identifier: AGPL-3.0-or-later
// History series visibility (VLT-IS-010 TASK 03-00-00-001, RQ-03, OD-49).
import { render } from 'preact';
import { act } from 'preact/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLang, t } from '../src/i18n';
import { History, SERIES_VISIBILITY_KEY, parseSeriesVisibility, seriesIsVisible } from '../src/screens/history';
import * as fake from './fakebridge';

vi.mock('../src/bridge', () => import('./fakebridge'));

// uPlot needs a 2D canvas, which happy-dom does not give. The mock also
// counts constructors: a data refresh must not make a new chart (RQ-03).
let plotsCreated = 0;
vi.mock('uplot', () => {
  class Fake {
    series: { show?: boolean }[];
    constructor(opts: { series: { show?: boolean }[] }) {
      plotsCreated++;
      this.series = opts.series.map((s) => ({ ...s }));
    }
    setData(): void { /* keep this instance */ }
    setSeries(i: number, opts: { show?: boolean }): void {
      this.series[i] = { ...this.series[i], ...opts };
    }
    destroy(): void { /* no canvas */ }
  }
  return { default: Fake };
});

function series(name: string, unit: string, mean: number, from: number, to: number) {
  return {
    name,
    unit,
    step: 'raw',
    rows: [
      { ts: new Date(from).toISOString(), min: mean - 1, max: mean + 1, mean, count: 1 },
      { ts: new Date(to).toISOString(), min: mean - 1, max: mean + 1, mean: mean + 1, count: 1 },
    ],
  };
}

let root: HTMLElement;
beforeEach(() => {
  fake.reset();
  setLang('en');
  plotsCreated = 0;
  root = document.createElement('div');
  document.body.appendChild(root);
  fake.handlers.Measurements = (name: unknown, from: unknown, to: unknown) =>
    series(String(name), 'V', 230, Number(from), Number(to));
  fake.handlers.Settings = () => ({ settings: [] });
  fake.handlers.SetSetting = () => undefined;
});
afterEach(() => {
  render(null, root);
  root.remove();
});

async function show(ticks = 8) {
  await act(async () => render(<History />, root));
  for (let i = 0; i < ticks; i++) {
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
  }
}

describe('series visibility helpers', () => {
  it('parses a stored map and treats missing as visible', () => {
    expect(parseSeriesVisibility({ 'input.voltage': false, 'output.voltage': true })).toEqual({
      'input.voltage': false,
      'output.voltage': true,
    });
    expect(seriesIsVisible({}, 'input.voltage')).toBe(true);
    expect(seriesIsVisible({ 'input.voltage': false }, 'input.voltage')).toBe(false);
    expect(parseSeriesVisibility('no')).toEqual({});
  });
});

describe('History series visibility (RQ-03)', () => {
  it('keeps a hidden series hidden after 10 data refreshes and a range change', async () => {
    await show();
    const box = root.querySelector('input[data-series="input.voltage"]') as HTMLInputElement;
    expect(box).toBeTruthy();
    expect(box.checked).toBe(true);
    const created = plotsCreated;
    expect(created).toBe(5); // one object for each chart, not one per refresh
    await act(async () => {
      box.checked = false;
      box.dispatchEvent(new Event('change', { bubbles: true }));
    });
    expect(box.checked).toBe(false);
    const saved = fake.calls.filter((c) => c.method === 'SetSetting' && c.args[0] === SERIES_VISIBILITY_KEY);
    expect(saved.length).toBeGreaterThan(0);
    expect((saved[saved.length - 1].args[1] as Record<string, boolean>)['input.voltage']).toBe(false);

    const before = fake.calls.filter((c) => c.method === 'Measurements').length;
    const refresh = [...root.querySelectorAll('button')].find((b) => b.textContent === t('hist.refresh'));
    expect(refresh).toBeTruthy();
    for (let i = 0; i < 10; i++) {
      await act(async () => { refresh!.click(); });
      await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
    }
    expect(fake.calls.filter((c) => c.method === 'Measurements').length).toBeGreaterThanOrEqual(before + 10);
    expect(plotsCreated).toBe(created);
    const box2 = root.querySelector('input[data-series="input.voltage"]') as HTMLInputElement;
    expect(box2.checked).toBe(false);

    const range24 = [...root.querySelectorAll('button')].find((b) => b.textContent === t('hist.range.24h'));
    await act(async () => { range24!.click(); });
    await act(async () => { await new Promise((r) => setTimeout(r, 0)); });
    expect(plotsCreated).toBe(created);
    const box3 = root.querySelector('input[data-series="input.voltage"]') as HTMLInputElement;
    expect(box3.checked).toBe(false);
  });

  it('restores the stored visibility at start', async () => {
    fake.handlers.Settings = () => ({
      settings: [{ key: SERIES_VISIBILITY_KEY, value: { 'input.voltage': false }, version: 1, default: false }],
    });
    await show();
    const box = root.querySelector('input[data-series="input.voltage"]') as HTMLInputElement;
    expect(box).toBeTruthy();
    expect(box.checked).toBe(false);
    const out = root.querySelector('input[data-series="output.voltage"]') as HTMLInputElement;
    expect(out.checked).toBe(true);
  });
});
