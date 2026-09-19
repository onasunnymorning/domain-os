import { describe, it, expect } from 'vitest';
import { keyReadiness, partyReadiness, partyReadinessFromList, requiredPurposes } from '../readiness';
import type { EscrowKeyPurpose, EscrowKeyState, EscrowKeyVersion, EscrowParty } from '@/lib/api/escrow-keys';

function version(over: Partial<EscrowKeyVersion> = {}): EscrowKeyVersion {
  return {
    id: 'v', partyId: 'p', owner: 'platform', purpose: 'verify-inbound', material: 'openpgp-public',
    version: 1, fingerprint: 'F'.repeat(40), hasPublicKey: true, state: 'ACTIVE' as EscrowKeyState,
    lastProbeOk: false, compromised: false, createdAt: '2026-09-01T00:00:00Z', ...over,
  };
}

const privateKey = (over: Partial<EscrowKeyVersion> = {}) =>
  version({ purpose: 'decrypt-inbound', material: 'openpgp-private', ...over });

function party(purposes: EscrowKeyPurpose[], activePurposes?: EscrowKeyPurpose[]): EscrowParty {
  return {
    id: 'p', owner: { kind: 'platform' }, name: 'Party', kind: 'DEA', side: 'self',
    purposes, activePurposes, manageable: true, createdAt: '',
  };
}

describe('keyReadiness', () => {
  it('names what is missing when there is no key at all', () => {
    const r = keyReadiness('verify-inbound', []);
    expect(r.status).toBe('setup-incomplete');
    expect(r.detail).toMatch(/No verification key yet/);
  });

  it('is ready with one active version', () => {
    expect(keyReadiness('verify-inbound', [version()]).status).toBe('ready');
  });

  // Overlap is how a correct rotation looks. Reporting it as a problem would
  // teach people to ignore the status exactly when it matters.
  it('calls two active versions a rotation, not a fault, and says how it ends', () => {
    const r = keyReadiness('verify-inbound', [version({ id: 'a', version: 2 }), version({ id: 'b' })]);
    expect(r.status).toBe('rotating');
    expect(r.detail).toMatch(/retire the old one/);
  });

  it('distinguishes a key being tested from one waiting to be activated', () => {
    expect(keyReadiness('decrypt-inbound', [privateKey({ state: 'STAGED' })]).status).toBe('testing');
    expect(
      keyReadiness('decrypt-inbound', [privateKey({ state: 'STAGED', lastProbeAt: 'now', lastProbeOk: true })]).status
    ).toBe('ready-to-activate');
  });

  // A public key has nothing to unlock, so there is nothing to test: waiting
  // for a test that will never run would strand the source forever.
  it('treats a pasted public key as ready to activate without a test', () => {
    expect(keyReadiness('verify-inbound', [version({ state: 'STAGED' })]).status).toBe('ready-to-activate');
  });

  it('reports a failed test as action required, and says whether deposits are affected', () => {
    const failed = privateKey({ id: 'f', version: 2, state: 'STAGED', lastProbeAt: 'now', lastProbeOk: false });
    const alone = keyReadiness('decrypt-inbound', [failed]);
    expect(alone.status).toBe('action-required');
    expect(alone.detail).not.toMatch(/unaffected/);

    const withActive = keyReadiness('decrypt-inbound', [failed, privateKey()]);
    expect(withActive.status).toBe('action-required');
    expect(withActive.detail).toMatch(/Deposits are unaffected/);
  });

  it('mentions a pending version while the active one keeps working', () => {
    const r = keyReadiness('verify-inbound', [version({ id: 'n', version: 2, state: 'STAGED' }), version()]);
    expect(r.status).toBe('ready');
    expect(r.detail).toMatch(/Version 2 is ready to activate/);
  });

  it('separates a revoked key from a retired one', () => {
    expect(keyReadiness('verify-inbound', [version({ state: 'REVOKED' })]).status).toBe('revoked');
    const retired = keyReadiness('verify-inbound', [version({ state: 'HISTORICAL' })]);
    expect(retired.status).toBe('setup-incomplete');
    expect(retired.detail).toMatch(/Older deposits still open/);
  });

  it('ignores destroyed versions, which are only a record', () => {
    expect(keyReadiness('verify-inbound', [version({ state: 'DESTROYED' })]).detail).toMatch(/No verification key yet/);
  });
});

describe('partyReadiness', () => {
  // Sanitisation is a capability, not a precondition for receiving deposits.
  // Counting it would report every installation that never sanitizes as
  // half-configured, which is the fastest way to make the badge meaningless.
  it('does not hold a missing pseudonymisation key against a receiving identity', () => {
    expect(requiredPurposes(party(['decrypt-inbound', 'pseudonymise']))).toEqual(['decrypt-inbound']);
    const r = partyReadiness(party(['decrypt-inbound', 'pseudonymise']), [privateKey()]);
    expect(r.status).toBe('ready');
  });

  it('reports the most alarming of the keys a party holds', () => {
    const r = partyReadiness(party(['decrypt-inbound']), [
      privateKey({ id: 'f', version: 2, state: 'STAGED', lastProbeAt: 'now', lastProbeOk: false }),
      privateKey(),
    ]);
    expect(r.status).toBe('action-required');
  });
});

describe('partyReadinessFromList', () => {
  it('says nothing when the server did not report active purposes', () => {
    expect(partyReadinessFromList(party(['verify-inbound'])).status).toBe('unknown');
  });

  it('names the missing key by what it is for', () => {
    const r = partyReadinessFromList(party(['verify-inbound'], []));
    expect(r.status).toBe('setup-incomplete');
    expect(r.detail).toBe('No active verification key.');
  });

  it('is ready once the purposes that matter hold an active key', () => {
    expect(partyReadinessFromList(party(['decrypt-inbound', 'pseudonymise'], ['decrypt-inbound'])).status).toBe('ready');
  });
});
