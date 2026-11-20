// SPDX-License-Identifier: AGPL-3.0-or-later
// CI layout test of the wizard (VLT-IS-012 TASK 03-00-00-001 step 2.4).
// For each size of the table in VLT-RPT-011 section 4 and for each
// language, each wizard step must have no scroll (scrollHeight <=
// clientHeight), no cut control and no button text on two lines.
//
// Needs the built front end (npm run build) and Google Chrome or
// Chromium. It uses only Node.js: an HTTP server on 127.0.0.1 and the
// Chrome DevTools Protocol over the built-in WebSocket. No npm package.
// Usage: node scripts/layout-check.mjs [chrome binary]
import { execFileSync, spawn } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { dirname, extname, join, normalize } from 'node:path';
import { fileURLToPath } from 'node:url';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const dist = join(web, '..', 'cmd', 'vlt-gui', 'frontend', 'dist');
const layout = join(web, 'layout');
if (!existsSync(join(dist, 'index.html'))) {
  console.error('layout-check: build the front end first (npm run build)');
  process.exit(2);
}

// Screen, Windows display scale and interface size (VLT-RPT-011 section 4).
export const SIZES = [
  { w: 1024, h: 768, s: 1, scale: 'auto' },
  { w: 1366, h: 768, s: 1, scale: 'auto' },
  { w: 1920, h: 1080, s: 1.25, scale: 'auto' },
  { w: 2560, h: 1440, s: 1.5, scale: 'auto' },
  { w: 3840, h: 2160, s: 2, scale: 'auto' },
  { w: 1366, h: 768, s: 1, scale: '80' },
  { w: 1920, h: 1080, s: 1, scale: '200' },
];
const LANGS = ['en', 'tr'];

const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.png': 'image/png', '.woff2': 'font/woff2' };
const index = readFileSync(join(dist, 'index.html'), 'utf8').replace('</body>', '<script type="module" src="/layout/check.js"></script></body>');
const server = createServer((req, res) => {
  const path = decodeURIComponent(new URL(req.url, 'http://x').pathname);
  let file;
  if (path === '/' || path === '/index.html') {
    res.writeHead(200, { 'content-type': 'text/html' });
    return res.end(index);
  }
  if (path === '/wails/runtime.js') file = join(layout, 'runtime.js');
  else if (path.startsWith('/layout/')) file = join(layout, normalize(path.slice(8)));
  else file = join(dist, normalize(path));
  if (!file.startsWith(layout) && !file.startsWith(dist)) { res.writeHead(403); return res.end(); }
  if (!existsSync(file)) { res.writeHead(404); return res.end(); }
  res.writeHead(200, { 'content-type': types[extname(file)] || 'application/octet-stream' });
  res.end(readFileSync(file));
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const httpPort = server.address().port;

const chromeBin = process.argv[2] || ['google-chrome', 'chromium', 'chromium-browser'].find((b) => {
  try { execFileSync(b, ['--version'], { stdio: 'ignore' }); return true; } catch { return false; }
});
if (!chromeBin) {
  console.error('layout-check: no Google Chrome or Chromium');
  process.exit(2);
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function measure(size, lang) {
  const vw = Math.round(size.w / size.s);
  const vh = Math.round(size.h / size.s - 80); // taskbar and title bar
  const dir = mkdtempSync(join(tmpdir(), 'vlt-layout-'));
  const chrome = spawn(chromeBin, ['--headless=new', '--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--remote-debugging-port=0',
    `--user-data-dir=${dir}`, 'about:blank'], { stdio: ['ignore', 'ignore', 'pipe'] });
  let err = '';
  const wsUrl = await new Promise((resolve, reject) => {
    const t = setTimeout(() => reject(new Error('chrome did not start: ' + err)), 20000);
    chrome.stderr.on('data', (d) => {
      err += d;
      const m = /DevTools listening on (ws:\S+)/.exec(err);
      if (m) { clearTimeout(t); resolve(m[1]); }
    });
  });
  const browser = new WebSocket(wsUrl);
  await new Promise((r) => browser.addEventListener('open', r));
  let id = 0;
  const wait = new Map();
  browser.addEventListener('message', (e) => {
    const m = JSON.parse(e.data);
    if (m.id && wait.has(m.id)) { wait.get(m.id)(m); wait.delete(m.id); }
  });
  const send = (method, params = {}, sessionId) => new Promise((r) => {
    const i = ++id;
    wait.set(i, r);
    browser.send(JSON.stringify({ id: i, method, params, sessionId }));
  });
  const { result: { targetId } } = await send('Target.createTarget', { url: 'about:blank' });
  const { result: { sessionId } } = await send('Target.attachToTarget', { targetId, flatten: true });
  // The screen of the OS less the taskbar (48 px): scale.ts compares the
  // minimum window size with it.
  await send('Emulation.setDeviceMetricsOverride', { width: vw, height: vh, deviceScaleFactor: size.s, mobile: false,
    screenWidth: Math.round(size.w / size.s), screenHeight: Math.round((size.h - 48) / size.s) }, sessionId);
  // The first-run wizard (no PIN) uses the OS language (navigator.language).
  const ver = await send('Browser.getVersion');
  await send('Emulation.setUserAgentOverride', { userAgent: ver.result.userAgent.replace('HeadlessChrome', 'Chrome'), acceptLanguage: lang === 'tr' ? 'tr-TR,tr' : 'en-US,en' }, sessionId);
  await send('Page.navigate', { url: `http://127.0.0.1:${httpPort}/?mode=expert&first=1&scale=${size.scale}&lang=${lang}` }, sessionId);
  let out = null;
  for (let i = 0; i < 100 && !out; i++) {
    await sleep(200);
    const r = await send('Runtime.evaluate', { expression: "document.getElementById('layout-result')?.textContent || ''", returnByValue: true }, sessionId);
    const v = r.result?.result?.value;
    if (v) out = JSON.parse(v);
  }
  browser.close();
  chrome.kill('SIGKILL');
  await sleep(100);
  rmSync(dir, { recursive: true, force: true });
  return out ?? { errors: ['no result'], steps: [] };
}

let failed = 0;
for (const size of SIZES) {
  for (const lang of LANGS) {
    const r = await measure(size, lang);
    const name = `${size.w} x ${size.h}, ${size.s * 100} %, size ${size.scale}, ${lang}`;
    const bad = [...r.errors];
    if (r.steps.length !== 11) bad.push(`${r.steps.length} wizard steps, want 11`);
    const title = JSON.parse(readFileSync(join(web, 'i18n', lang + '.json'), 'utf8'))['wizard.title'];
    if (r.title !== title) bad.push(`wizard title "${r.title}", want "${title}" (language ${lang})`);
    for (const st of r.steps) {
      for (const p of st.problems) bad.push(`${st.step}: ${p}`);
      if (!st.pageLocked) bad.push(`${st.step}: the page behind the wizard is not locked`);
    }
    const sz = r.steps[0]?.size?.join(' x ') ?? '?';
    if (bad.length) {
      failed++;
      console.log(`FAIL ${name} (viewport ${r.viewport?.join(' x ')}, wizard ${sz})`);
      for (const b of bad) console.log('  ' + b);
    } else {
      console.log(`ok   ${name} (viewport ${r.viewport?.join(' x ')}, wizard ${sz}, ${r.steps.length} steps, no scroll)`);
    }
  }
}
server.close();
process.exit(failed ? 1 : 0);
