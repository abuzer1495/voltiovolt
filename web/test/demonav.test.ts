// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest';
import { parseDemoHash } from '../src/demonav';

describe('parseDemoHash', () => {
  it('parses screens, modes, wizard steps and help', () => {
    expect(parseDemoHash('#device')).toEqual({ route: 'device' });
    expect(parseDemoHash('#simple')).toEqual({ mode: 'simple', route: 'dashboard' });
    expect(parseDemoHash('#expert')).toEqual({ mode: 'expert', route: 'dashboard' });
    expect(parseDemoHash('#wizard')).toEqual({ wizard: true });
    expect(parseDemoHash('#wizard/pin')).toEqual({ wizard: 'pin' });
    expect(parseDemoHash('#wizard/email2')).toEqual({ wizard: 'email2' });
    expect(parseDemoHash('#help')).toEqual({ route: 'device', help: true });
    expect(parseDemoHash('')).toEqual({});
  });
});
