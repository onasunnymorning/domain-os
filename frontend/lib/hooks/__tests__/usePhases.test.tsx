/**
 * useDeletePhase sends the operator that runs the TLD (#415, ADR-0006).
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { phasesApi } from '@/lib/api/phases';
import { tldsApi, type TLD } from '@/lib/api/tlds';
import { useDeletePhase } from '../usePhases';

vi.mock('@/lib/api/phases', () => ({ phasesApi: { delete: vi.fn() } }));
vi.mock('@/lib/api/tlds', () => ({ tldsApi: { get: vi.fn() } }));

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

describe('useDeletePhase', () => {
  beforeEach(() => vi.clearAllMocks());

  it("deletes the phase as the TLD's operator, reusing the cached TLD", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
    client.setQueryData(['tld', 'paco'], { Name: 'paco', RyID: 'PacoRy' } as TLD);
    vi.mocked(phasesApi.delete).mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeletePhase('paco'), { wrapper: wrapper(client) });
    result.current.mutate('GA-2026');

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(phasesApi.delete).toHaveBeenCalledWith('paco', 'GA-2026', 'PacoRy');
    expect(tldsApi.get).not.toHaveBeenCalled();
  });

  it('fetches the TLD when it is not cached yet', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(tldsApi.get).mockResolvedValue({ Name: 'gza', RyID: 'GzaRy' } as TLD);
    vi.mocked(phasesApi.delete).mockResolvedValue(undefined);

    const { result } = renderHook(() => useDeletePhase('gza'), { wrapper: wrapper(client) });
    result.current.mutate('Sunrise');

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(tldsApi.get).toHaveBeenCalledWith('gza');
    expect(phasesApi.delete).toHaveBeenCalledWith('gza', 'Sunrise', 'GzaRy');
  });
});
