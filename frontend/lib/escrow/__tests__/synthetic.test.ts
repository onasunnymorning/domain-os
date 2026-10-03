import { describe, it, expect } from 'vitest';
import {
  SYNTHETIC_LIMITS,
  estimateSyntheticBytes,
  fallbackSyntheticFilename,
  filenameFromContentDisposition,
  formatBytes,
  syntheticCounts,
  syntheticParamsProblem,
  type SyntheticDepositParams,
} from '../synthetic';

const ok: SyntheticDepositParams = { tld: 'test', domains: 1000, contactsPerDomain: 2, avgHostsPerDomain: 2.5, nndns: 10 };

describe('syntheticCounts', () => {
  it('multiplies contacts and floors hosts like the server', () => {
    expect(syntheticCounts({ domains: 3, contactsPerDomain: 4, avgHostsPerDomain: 1.5, nndns: 2 })).toEqual({
      domains: 3, contacts: 12, hosts: 4, registrars: 5, nndns: 2,
    });
  });
});

describe('estimateSyntheticBytes', () => {
  it('lands near the measured size of a typical 250k-domain deposit (~107 MB)', () => {
    const bytes = estimateSyntheticBytes(syntheticCounts({ domains: 250_000, contactsPerDomain: 2, avgHostsPerDomain: 2.5, nndns: 0 }));
    expect(bytes).toBeGreaterThan(80e6);
    expect(bytes).toBeLessThan(130e6);
  });
});

describe('formatBytes', () => {
  it.each([
    [512, '512 B'],
    [840_000, '840 KB'],
    [12_400_000, '12.4 MB'],
    [2_500_000_000, '2.50 GB'],
  ])('%d → %s', (n, s) => expect(formatBytes(n)).toBe(s));
});

describe('syntheticParamsProblem', () => {
  it('accepts valid params, including the upper bounds', () => {
    expect(syntheticParamsProblem(ok)).toBeNull();
    expect(syntheticParamsProblem({ tld: 'co.test', ...SYNTHETIC_LIMITS, seed: 7 })).toBeNull();
    expect(syntheticParamsProblem({ ...ok, tld: 'xn--p1ai' })).toBeNull();
  });

  it.each<[string, Partial<SyntheticDepositParams>, RegExp]>([
    ['empty tld', { tld: ' ' }, /TLD/],
    ['bad tld', { tld: '-bad-' }, /TLD/],
    ['too many domains', { domains: 250_001 }, /Domains/],
    ['fractional domains', { domains: 1.5 }, /Domains/],
    ['blank domains', { domains: NaN }, /Domains/],
    ['five contacts', { contactsPerDomain: 5 }, /Contacts/],
    ['too many hosts', { avgHostsPerDomain: 13.1 }, /hosts/],
    ['negative nndns', { nndns: -1 }, /NNDNs/],
    ['negative seed', { seed: -1 }, /seed/],
  ])('rejects %s', (_, patch, msg) => {
    expect(syntheticParamsProblem({ ...ok, ...patch })).toMatch(msg);
  });
});

describe('filenames', () => {
  it('reads quoted, bare and RFC 5987 Content-Disposition forms', () => {
    expect(filenameFromContentDisposition('attachment; filename="test_2026-10-02_full_S1_R0.xml.gz"')).toBe('test_2026-10-02_full_S1_R0.xml.gz');
    expect(filenameFromContentDisposition('attachment; filename=a.xml.gz')).toBe('a.xml.gz');
    expect(filenameFromContentDisposition("attachment; filename*=UTF-8''b%20c.xml.gz")).toBe('b c.xml.gz');
    expect(filenameFromContentDisposition(null)).toBeNull();
    expect(filenameFromContentDisposition('attachment')).toBeNull();
  });

  it('builds the ICANN-style fallback name', () => {
    expect(fallbackSyntheticFilename('.Test.', new Date('2026-10-02T12:00:00Z'))).toBe('test_2026-10-02_full_S1_R0.xml.gz');
  });
});
