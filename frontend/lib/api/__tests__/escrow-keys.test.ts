/**
 * Tests for the escrow key registry API client (issue #429).
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AxiosError, AxiosHeaders } from 'axios';
import { apiClient } from '../client';
import {
  getDefaultEscrowArrangement,
  groupParties,
  importEscrowPrivateKey,
  inheritedFromLabel,
  isPermissionError,
  listEscrowParties,
  partyRole,
  escrowKeyErrorMessage,
  activateEscrowKeyVersion,
  type EscrowParty,
} from '../escrow-keys';

vi.mock('../client', () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}));

function axiosError(status: number, error?: string) {
  const headers = new AxiosHeaders();
  return new AxiosError('failed', String(status), { headers }, undefined, {
    status,
    statusText: '',
    headers: {},
    config: { headers },
    data: error ? { error } : {},
  });
}

const party = (overrides: Partial<EscrowParty>): EscrowParty => ({
  id: 'p', owner: { kind: 'platform' }, name: 'p', kind: 'DEA', side: 'self',
  purposes: [], manageable: true, createdAt: '2026-09-12T00:00:00Z', ...overrides,
});

describe('escrow key registry client', () => {
  beforeEach(() => vi.clearAllMocks());

  it('sends the operator scope header, and none for the platform registry', async () => {
    vi.mocked(apiClient.get).mockResolvedValue({ data: { items: [], count: 0 } });
    await listEscrowParties({ tenantId: 'ryop1' });
    expect(apiClient.get).toHaveBeenLastCalledWith('/escrow/parties', { headers: { 'X-Tenant-ID': 'ryop1' }, params: undefined });
    await listEscrowParties({});
    expect(apiClient.get).toHaveBeenLastCalledWith('/escrow/parties', { params: undefined });
  });

  it('posts an import to the private-key endpoint of the party', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: { id: 'v1' } });
    await importEscrowPrivateKey({}, 'party-1', { purpose: 'decrypt-inbound', armoredPrivateKey: 'KEY', passphrase: 'pw' });
    expect(apiClient.post).toHaveBeenCalledWith(
      '/escrow/parties/party-1/versions/private',
      { purpose: 'decrypt-inbound', armoredPrivateKey: 'KEY', passphrase: 'pw' },
      {}
    );
  });

  it('asks to confirm a replacement only when told to', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: {} });
    await activateEscrowKeyVersion({ tenantId: 'ryop1' }, 'v1');
    expect(apiClient.post).toHaveBeenLastCalledWith('/escrow/key-versions/v1/activate', { confirmReplace: false }, { headers: { 'X-Tenant-ID': 'ryop1' } });
    await activateEscrowKeyVersion({}, 'v2', true);
    expect(apiClient.post).toHaveBeenLastCalledWith('/escrow/key-versions/v2/activate', { confirmReplace: true }, {});
  });

  it('treats a missing default arrangement as none, and other errors as errors', async () => {
    vi.mocked(apiClient.get).mockRejectedValueOnce(axiosError(404));
    await expect(getDefaultEscrowArrangement({ tenantId: 'ryop1' })).resolves.toBeNull();
    vi.mocked(apiClient.get).mockRejectedValueOnce(axiosError(500));
    await expect(getDefaultEscrowArrangement({ tenantId: 'ryop1' })).rejects.toBeTruthy();
  });

  it('recognises a missing permission and surfaces the server message', () => {
    expect(isPermissionError(axiosError(403))).toBe(true);
    expect(isPermissionError(axiosError(400))).toBe(false);
    expect(escrowKeyErrorMessage(axiosError(409, 'no successful probe'))).toBe('no successful probe');
    expect(escrowKeyErrorMessage(new Error('x'), 'fallback')).toBe('fallback');
  });

  it('groups parties by the direction deposits travel, and explains inheritance', () => {
    const groups = groupParties([
      party({ id: 'eve' }),
      party({ id: 'acme', kind: 'RSP', side: 'external' }),
      party({ id: 'agent', kind: 'DEA', side: 'external' }),
    ]);
    expect(groups.ourIdentities.map((p: EscrowParty) => p.id)).toEqual(['eve']);
    expect(groups.sources.map((p: EscrowParty) => p.id)).toEqual(['acme']);
    expect(groups.destinations.map((p: EscrowParty) => p.id)).toEqual(['agent']);
    expect(inheritedFromLabel('operator')).toBe('inherited from operator default');
    expect(inheritedFromLabel('tld')).toBe('set for this TLD');
  });

  it('takes the role the server derived, and still derives one when it is absent', () => {
    // The server sends `role` now, so there is one mapping rather than one per
    // client. Kind and side are still sent and still correct, which is what
    // lets an older response, or a role this build has no words for, fall
    // back instead of showing nothing.
    expect(partyRole({ ...party({ id: 'x', kind: 'RSP', side: 'external' }), role: 'source' })).toBe('source');
    expect(partyRole(party({ id: 'x', kind: 'RSP', side: 'external' }))).toBe('source');
    expect(partyRole(party({ id: 'eve' }))).toBe('receiving-identity');
    expect(
      partyRole({ ...party({ id: 'eve' }), role: 'custodian' } as unknown as EscrowParty),
    ).toBe('receiving-identity');
  });
});
