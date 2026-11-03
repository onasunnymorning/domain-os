import { describe, it, expect } from 'vitest';
import { utcDayLabel } from '../CreateOutcomesChart';

describe('utcDayLabel', () => {
  it('labels a UTC midnight bucket by its UTC date, whatever the local zone', () => {
    // In zones west of UTC, new Date("2026-11-02T00:00:00Z") is still Nov 1 locally.
    expect(utcDayLabel('2026-11-02T00:00:00Z')).toBe('Nov 2');
    expect(utcDayLabel('2026-01-01T00:00:00Z')).toBe('Jan 1');
  });
});
