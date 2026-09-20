import { describe, it, expect } from 'vitest';
import type { EscrowFindingTally, EscrowSummaryCount } from '@/lib/api/escrow-runs';
import {
  depositHeading,
  depositKindLabel,
  formatUtc,
  objectCounts,
  pipelineSteps,
  reachedObjects,
  resendLabel,
} from '../runOverview';

type Run = Parameters<typeof pipelineSteps>[0];

const run = (over: Partial<Run> = {}): Run => ({
  profile: 'ryde+sig',
  outcome: 'PASS',
  stageReached: 'rde',
  findingTally: [],
  findings: [],
  ...over,
});

const tally = (stage: string, severity: string, count: number, code = 'X'): EscrowFindingTally => ({
  code,
  stage,
  severity,
  count,
});

const states = (r: Run) => Object.fromEntries(pipelineSteps(r).map((s) => [s.key, s.state]));

describe('pipelineSteps', () => {
  it('passes every step of a clean signed deposit', () => {
    expect(states(run())).toEqual({
      signature: 'passed',
      decrypt: 'passed',
      structure: 'passed',
      validation: 'passed',
    });
  });

  // The whole reason the chips are not four unconditional ticks: an unsigned
  // deposit has no signature and no encryption, so nothing was verified.
  it('marks signature and decrypt not applicable for the unsigned profile', () => {
    expect(states(run({ profile: 'xml' }))).toEqual({
      signature: 'not-applicable',
      decrypt: 'not-applicable',
      structure: 'passed',
      validation: 'passed',
    });
  });

  it('stops at the failing step and leaves the rest not reached', () => {
    const r = run({
      outcome: 'FAIL',
      stageReached: 'decrypt',
      findingTally: [tally('decrypt', 'ERROR', 1, 'DECRYPT_FAILED')],
    });
    expect(states(r)).toEqual({
      signature: 'passed',
      decrypt: 'failed',
      structure: 'not-reached',
      validation: 'not-reached',
    });
  });

  it('does not call a step failed when the service could not decide', () => {
    const r = run({
      outcome: 'ERROR',
      stageReached: 'decrypt',
      findingTally: [tally('decrypt', 'ERROR', 1, 'DECRYPT_KEY_UNAVAILABLE')],
    });
    expect(states(r).decrypt).toBe('undecided');
    expect(states(r).signature).toBe('passed');
  });

  it('folds unpack and xml into one structure step', () => {
    const r = run({
      outcome: 'FAIL',
      stageReached: 'xml',
      findingTally: [tally('xml', 'ERROR', 2, 'XML_MALFORMED')],
    });
    expect(states(r).structure).toBe('failed');
    expect(states(r).validation).toBe('not-reached');
    expect(pipelineSteps(r).find((s) => s.key === 'structure')?.errors).toBe(2);
  });

  it('counts warnings on a passed step so a tick cannot hide them', () => {
    const r = run({ findingTally: [tally('rde', 'WARNING', 324)] });
    const validation = pipelineSteps(r).find((s) => s.key === 'validation');
    expect(validation).toMatchObject({ state: 'passed', warnings: 324, errors: 0 });
  });

  it('reads the exact tally in preference to the retained sample', () => {
    const r = run({
      outcome: 'FAIL',
      findingTally: [tally('rde', 'ERROR', 900)],
      findings: [{ code: 'X', severity: 'ERROR', stage: 'rde', message: 'm' }],
    });
    expect(pipelineSteps(r).find((s) => s.key === 'validation')?.errors).toBe(900);
  });

  it('falls back to retained findings on a run recorded before the tally', () => {
    const r = run({
      outcome: 'FAIL',
      findingTally: [],
      findings: [{ code: 'X', severity: 'ERROR', stage: 'rde', message: 'm' }],
    });
    expect(pipelineSteps(r).find((s) => s.key === 'validation')).toMatchObject({
      state: 'failed',
      errors: 1,
    });
  });

  it('shows a step in progress as running', () => {
    expect(states(run({ outcome: 'RUNNING', stageReached: 'xml' })).structure).toBe('running');
  });

  it('treats a passed run with no recorded stage as having cleared everything', () => {
    expect(states(run({ stageReached: undefined })).validation).toBe('passed');
  });

  it('shows an intake step only when intake went wrong', () => {
    expect(pipelineSteps(run()).some((s) => s.key === 'intake')).toBe(false);
    const r = run({
      outcome: 'FAIL',
      stageReached: 'intake',
      findingTally: [tally('intake', 'ERROR', 1, 'INTAKE_LIMIT_COMPRESSED_SIZE')],
    });
    expect(states(r)).toMatchObject({ intake: 'failed', signature: 'not-reached' });
  });
});

describe('reachedObjects', () => {
  it('is false before the xml stage and true from it on', () => {
    expect(reachedObjects({ stageReached: 'decrypt', outcome: 'FAIL' })).toBe(false);
    expect(reachedObjects({ stageReached: 'xml', outcome: 'FAIL' })).toBe(true);
    expect(reachedObjects({ stageReached: 'rde', outcome: 'PASS' })).toBe(true);
  });
});

describe('objectCounts', () => {
  const row = (uri: string, observed: number, status: EscrowSummaryCount['status'], declared?: number) => ({
    uri,
    observed,
    status,
    declared,
  });
  const byKey = (counts: EscrowSummaryCount[]) =>
    Object.fromEntries(objectCounts(counts).map((c) => [c.key, c]));

  it('always returns the five tiles, in order, with zero for what is absent', () => {
    expect(objectCounts([]).map((c) => [c.key, c.observed])).toEqual([
      ['domains', 0],
      ['contacts', 0],
      ['hosts', 0],
      ['registrars', 0],
      ['other', 0],
    ]);
  });

  it('groups namespaces and folds IDN and NNDN into other', () => {
    const c = byKey([
      row('urn:ietf:params:xml:ns:rdeDomain-1.0', 6241, 'match', 6241),
      row('urn:ietf:params:xml:ns:rdeContact-1.0', 8913, 'match', 8913),
      row('urn:ietf:params:xml:ns:rdeHost-1.0', 327, 'match', 327),
      row('urn:ietf:params:xml:ns:rdeRegistrar-1.0', 42, 'match', 42),
      row('urn:ietf:params:xml:ns:rdeIDN-1.0', 3, 'match', 3),
      row('urn:ietf:params:xml:ns:rdeNNDN-1.0', 15, 'match', 15),
    ]);
    expect(c.domains.observed).toBe(6241);
    expect(c.contacts.observed).toBe(8913);
    expect(c.hosts.observed).toBe(327);
    expect(c.registrars.observed).toBe(42);
    expect(c.other.observed).toBe(18);
    expect(Object.values(c).some((t) => t.mismatch)).toBe(false);
  });

  it('flags a header that disagrees with the deposit', () => {
    const c = byKey([row('urn:ietf:params:xml:ns:rdeDomain-1.0', 6241, 'mismatch', 6300)]);
    expect(c.domains).toMatchObject({ observed: 6241, declared: 6300, mismatch: true });
  });

  // rdeEppParams / rdePolicy declare one element each and are never walked;
  // rendering them as "1 declared, 0 found" would invent a discrepancy.
  it('drops namespaces the validator does not count', () => {
    const c = byKey([
      row('urn:ietf:params:xml:ns:rdeEppParams-1.0', 0, 'not-checked', 1),
      row('urn:ietf:params:xml:ns:rdePolicy-1.0', 0, 'not-checked', 1),
    ]);
    expect(c.other).toMatchObject({ observed: 0, mismatch: false });
    expect(c.other.declared).toBeUndefined();
  });

  it('still shows objects found in a namespace the header never declared', () => {
    const c = byKey([row('urn:ietf:params:xml:ns:rdeHost-1.0', 5, 'undeclared')]);
    expect(c.hosts).toMatchObject({ observed: 5, mismatch: false });
  });
});

describe('formatUtc', () => {
  it('renders in UTC whatever the viewer zone', () => {
    expect(formatUtc('2026-07-16T00:00:00Z')).toBe('2026-07-16 00:00:00 UTC');
    expect(formatUtc('2026-09-19T16:11:47Z', false)).toBe('2026-09-19 16:11 UTC');
  });

  it('shows a dash for a missing time and passes through what it cannot parse', () => {
    expect(formatUtc(undefined)).toBe('—');
    expect(formatUtc('not a date')).toBe('not a date');
  });
});

describe('depositHeading', () => {
  it('names the deposit by its watermark date and its own id', () => {
    expect(
      depositHeading({
        rdeWatermark: '2026-07-16T00:00:00Z',
        rdeDepositId: 'TI8QO0',
        depositId: '93446ed7-70ba-46e0-9702-bc98348c9a5d',
      })
    ).toBe('Deposit 2026-07-16 / TI8QO0');
  });

  it('falls back to the intake id when the header was never read', () => {
    expect(depositHeading({ depositId: '93446ed7-70ba-46e0-9702-bc98348c9a5d' })).toBe(
      'Deposit 93446ed7'
    );
  });

  it('does not invent a separator for one missing half', () => {
    expect(depositHeading({ rdeDepositId: 'TI8QO0', depositId: 'abcdef123' })).toBe('Deposit TI8QO0');
  });
});

describe('labels', () => {
  it('names deposit kinds and resends', () => {
    expect(depositKindLabel('FULL')).toBe('Full deposit');
    expect(depositKindLabel('DIFF')).toBe('Differential deposit');
    expect(depositKindLabel(undefined)).toBe('Deposit');
    expect(resendLabel(0)).toBe('No');
    expect(resendLabel(2)).toBe('#2');
  });
});
