// SPDX-License-Identifier: AGPL-3.0-or-later
// In-page part of the layout test (VLT-IS-012 TASK 03-00-00-001 step 2.4).
// It opens the first-run wizard, goes through all steps with "Skip" and
// "Next", and writes the result as JSON into <pre id="layout-result">.
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const TOL = 1;

function label(el) {
  return (el.getAttribute('aria-label') || el.textContent || el.id || el.tagName).trim().replace(/\s+/g, ' ').slice(0, 40);
}

/** problems of one wizard step: scroll, cut controls, wrapped buttons. */
function stepProblems(dlg) {
  const out = [];
  const body = dlg.querySelector('.wizard-body');
  if (body.scrollHeight > body.clientHeight + TOL) out.push(`scroll ${body.scrollHeight}>${body.clientHeight}`);
  if (body.scrollWidth > body.clientWidth + TOL) out.push(`hscroll ${body.scrollWidth}>${body.clientWidth}`);
  const d = dlg.getBoundingClientRect();
  if (d.left < -TOL || d.top < -TOL || d.right > innerWidth + TOL || d.bottom > innerHeight + TOL) out.push('dialog outside the window');
  const b = body.getBoundingClientRect();
  for (const el of dlg.querySelectorAll('button, label, input, select, textarea, h2, h3, p, th, td, li, legend')) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 && r.height === 0) continue;
    if (el.closest('.sr-only')) continue;
    const box = el.closest('.wizard-body') ? b : d;
    if (r.bottom > box.bottom + TOL || r.right > box.right + TOL || r.left < box.left - TOL) out.push('cut: ' + label(el));
    if (el.tagName === 'BUTTON') {
      if (el.scrollWidth > el.clientWidth + TOL) out.push('button text cut: ' + label(el));
      const lh = parseFloat(getComputedStyle(el).lineHeight) || parseFloat(getComputedStyle(el).fontSize) * 1.4;
      const rects = el.getClientRects();
      if (rects.length && el.textContent.trim() && el.firstChild && el.firstChild.nodeType === 3) {
        const range = document.createRange();
        range.selectNodeContents(el);
        const lines = new Set([...range.getClientRects()].map((x) => Math.round(x.top)));
        if (lines.size > 1 && r.height > lh * 1.6) out.push('button on two lines: ' + label(el));
      }
    }
  }
  return out;
}

async function run() {
  await sleep(1200);
  const res = { viewport: [innerWidth, innerHeight], dpr: devicePixelRatio, steps: [], errors: [] };
  // The first-start dialog of a new data folder: "Start".
  const start = document.querySelector('.prompt-dialog .primary');
  if (!start) res.errors.push('no first-start dialog');
  else start.click();
  await sleep(400);
  for (let i = 0; i < 20; i++) {
    const dlg = document.querySelector('.wizard-dialog');
    if (!dlg) break;
    const cur = dlg.querySelector('[aria-current="step"]');
    const name = cur ? cur.getAttribute('title') : '?';
    const r = dlg.getBoundingClientRect();
    res.title = dlg.querySelector('#wiz-title')?.textContent;
    res.steps.push({ step: name, size: [Math.round(r.width), Math.round(r.height)], problems: stepProblems(dlg), pageLocked: document.documentElement.classList.contains('modal-open') });
    const btns = [...dlg.querySelectorAll('.wizard-footer button')];
    const skip = btns[1];
    const next = btns[2];
    if (skip && !skip.disabled) skip.click();
    else if (next && !next.disabled && dlg.querySelector('.wizard-steps li:last-child[aria-current="step"]') === null) next.click();
    else break;
    await sleep(250);
  }
  const pre = document.createElement('pre');
  pre.id = 'layout-result';
  pre.textContent = JSON.stringify(res);
  document.body.appendChild(pre);
}
run();
