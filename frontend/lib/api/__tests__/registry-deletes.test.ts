/**
 * Registry deletes name their operator (#415, ADR-0006).
 *
 * The API confines deleting a phase or TLD to the registry operator that runs
 * the TLD, and refuses a delete that names no operator. These clients must
 * therefore always send X-Tenant-ID.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { apiClient } from '../client';
import { phasesApi } from '../phases';
import { tldsApi } from '../tlds';

vi.mock('../client', () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}));

describe('registry delete clients', () => {
  beforeEach(() => vi.clearAllMocks());

  it('sends the operator with a phase delete', async () => {
    vi.mocked(apiClient.delete).mockResolvedValue({ data: undefined });
    await phasesApi.delete('paco', 'GA-2026', 'PacoRy');
    expect(apiClient.delete).toHaveBeenCalledWith('/tlds/paco/phases/GA-2026', {
      headers: { 'X-Tenant-ID': 'PacoRy' },
    });
  });

  it('sends the operator with a TLD delete', async () => {
    vi.mocked(apiClient.delete).mockResolvedValue({ data: undefined });
    await tldsApi.delete('paco', 'PacoRy');
    expect(apiClient.delete).toHaveBeenCalledWith('/tlds/paco', {
      headers: { 'X-Tenant-ID': 'PacoRy' },
    });
  });
});
