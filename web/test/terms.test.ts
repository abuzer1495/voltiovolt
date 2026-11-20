// SPDX-License-Identifier: AGPL-3.0-or-later
// VLT-IS-012 TASK 03-00-00-002 step 3 (R-18): one Turkish term for each
// interface term of docs/terms.md. tr.json has no form of the deny list.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import en from '../i18n/en.json';
import tr from '../i18n/tr.json';

const TR = tr as Record<string, string>;
const EN = en as Record<string, string>;

// The word "kip" in each form ("kip", "kipi", "kipte", "kipini" ...) and
// the other forms of the terms.
const DENY: [string, RegExp][] = [
  ['kip', /(?<!\p{L})kip\p{L}*/iu],
  ['Uzman kipi', /Uzman kipi/iu],
  ['Basit kipi', /Basit kipi/iu],
  ['Kapatma Politikası', /Kapatma Politika/u],
  ['Şebeke elektriği', /şebeke elektri/iu],
  ['pil gücü', /pil güc/iu],
  ['pil şarjı', /pil şarj/iu],
  ['çalışma süresi', /çalışma süre/iu],
];

describe('interface terms (docs/terms.md)', () => {
  it('tr.json uses no form of the deny list', () => {
    const bad: string[] = [];
    for (const [k, v] of Object.entries(TR)) for (const [name, re] of DENY) if (re.test(v)) bad.push(`${k}: "${name}" in "${v}"`);
    expect(bad).toEqual([]);
  });

  it('the mode names and the menu items are "Basit mod" and "Uzman mod"', () => {
    expect(TR['mode.ui.to_simple']).toBe('Basit mod');
    expect(TR['mode.ui.to_expert']).toBe('Uzman mod');
    expect(TR['menu.view.simple']).toBe('&Basit mod');
    expect(TR['menu.view.expert']).toBe('&Uzman mod');
    expect(EN['menu.view.simple']).toBe('&Simple mode');
    expect(EN['menu.view.expert']).toBe('&Expert mode');
  });

  it('uses the Turkish terms of the table', () => {
    expect(TR['wizard.title']).toBe('Kurulum sihirbazı');
    expect(TR['nav.policy']).toBe('Kapatma politikası');
    expect(TR['policy.f.countdown_seconds']).toBe('Geri sayım');
    expect(TR['mode.mains']).toBe('Şebeke');
    expect(TR['mode.battery']).toBe('Aküde');
    expect(TR['m.battery.charge']).toBe('Akü doluluğu');
    expect(TR['dash.runtime']).toBe('Kalan süre');
    expect(TR['dash.load']).toBe('Yük');
    expect(TR['dash.input_voltage']).toBe('Giriş gerilimi');
    expect(TR['dash.output_voltage']).toBe('Çıkış gerilimi');
    expect(TR['conn.ups.test']).toBe('Test bağlantısı');
    expect(TR['scale.title']).toBe('Arayüz boyutu');
    expect(TR['lock.button']).toBe('Kilitle');
    expect(TR['lock.unlock']).toBe('Kilidi aç');
    expect(TR['nav.events']).toBe('Olaylar');
    expect(TR['nav.notifications']).toBe('Bildirimler');
  });

  it('docs/terms.md has the table of the terms', () => {
    const md = readFileSync(join(__dirname, '../../docs/terms.md'), 'utf8');
    for (const row of ['| Simple mode | Basit mod |', '| Expert mode | Uzman mod |', '| Shutdown Policy | Kapatma politikası |', '| Battery replace | Akü değişimi |']) {
      expect(md).toContain(row);
    }
  });
});
