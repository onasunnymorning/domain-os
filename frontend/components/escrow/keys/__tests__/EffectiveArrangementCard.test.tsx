import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { EffectiveArrangementCard } from '../EffectiveArrangementCard';
import * as api from '@/lib/api/escrow-keys';

vi.mock('@/lib/api/escrow-keys', async (orig) => {
  const actual = await (orig() as Promise<typeof api>);
  return { ...actual, getEffectiveEscrowArrangement: vi.fn(), listEscrowParties: vi.fn() };
});

describe('EffectiveArrangementCard', () => {
  it('names each side and says where it is inherited from', async () => {
    vi.mocked(api.getEffectiveEscrowArrangement).mockResolvedValue({
      depositor: { partyId: 'acme', from: 'operator' },
      receiver: { partyId: 'eve', from: 'platform' },
      tldRevision: 0,
      operatorRevision: 1,
      platformRevision: 3,
    });
    vi.mocked(api.listEscrowParties).mockResolvedValue({
      items: [
        { id: 'acme', name: 'Acme Registry', owner: { kind: 'operator', operator: 'ryop1' }, kind: 'RSP', side: 'external', purposes: [], manageable: true, createdAt: '' },
        { id: 'eve', name: 'EVE', owner: { kind: 'platform' }, kind: 'DEA', side: 'self', purposes: [], manageable: false, createdAt: '' },
      ],
      count: 2,
    });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <EffectiveArrangementCard tenantId="ryop1" tld="example" />
      </QueryClientProvider>
    );
    expect(await screen.findByText('Acme Registry')).toBeTruthy();
    expect(screen.getByText('EVE')).toBeTruthy();
    expect(screen.getByText('inherited from operator default')).toBeTruthy();
    expect(screen.getByText('inherited from platform default')).toBeTruthy();
    expect(api.getEffectiveEscrowArrangement).toHaveBeenCalledWith('ryop1', 'example');
  });
});
