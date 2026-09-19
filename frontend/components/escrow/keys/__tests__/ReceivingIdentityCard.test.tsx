import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReceivingIdentityCard } from '../ReceivingIdentityCard';
import * as api from '@/lib/api/escrow-keys';

vi.mock('@/lib/api/escrow-keys', async (orig) => {
  const actual = await (orig() as Promise<typeof api>);
  return { ...actual, getDefaultEscrowArrangement: vi.fn(), getEscrowParty: vi.fn(), getEscrowPublicKey: vi.fn() };
});

const arrangement = (receiverPartyId?: string): api.EscrowArrangement => ({
  id: 'a', level: 'operator', operator: 'ryop1', receiverPartyId, revision: 1, createdAt: '',
});

const identity = (state: api.EscrowKeyState): api.EscrowPartyDetail => ({
  party: {
    id: 'eve', owner: { kind: 'platform' }, name: 'Our escrow identity', kind: 'DEA', side: 'self',
    purposes: ['decrypt-inbound'], manageable: false, createdAt: '',
  },
  versions: [
    {
      id: 'v1', partyId: 'eve', owner: 'platform', purpose: 'decrypt-inbound', material: 'openpgp-private',
      version: 1, fingerprint: 'BEEF'.repeat(10), hasPublicKey: true, state, lastProbeOk: true,
      compromised: false, createdAt: '',
    },
  ],
  usedBy: [],
});

const show = () =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <ReceivingIdentityCard scope={{ tenantId: 'ryop1' }} />
    </QueryClientProvider>
  );

describe('ReceivingIdentityCard', () => {
  beforeEach(() => vi.clearAllMocks());

  it('hands over the fingerprint of the key the source must encrypt to', async () => {
    vi.mocked(api.getDefaultEscrowArrangement).mockResolvedValue(arrangement('eve'));
    vi.mocked(api.getEscrowParty).mockResolvedValue(identity('ACTIVE'));
    show();
    expect(await screen.findByText('BEEF'.repeat(10))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /copy public key/i })).toBeInTheDocument();
    expect(screen.getByText(/not secret/)).toBeInTheDocument();
  });

  // Setting up a source against an identity that cannot open anything is the
  // failure that only shows up when the first deposit arrives.
  it('warns when no identity is set to receive at all', async () => {
    vi.mocked(api.getDefaultEscrowArrangement).mockResolvedValue(arrangement(undefined));
    show();
    expect(await screen.findByText(/No identity is set to receive deposits/)).toBeInTheDocument();
    expect(api.getEscrowParty).not.toHaveBeenCalled();
  });

  it('warns when the receiving identity has no active decryption key', async () => {
    vi.mocked(api.getDefaultEscrowArrangement).mockResolvedValue(arrangement('eve'));
    vi.mocked(api.getEscrowParty).mockResolvedValue(identity('STAGED'));
    show();
    expect(await screen.findByText(/has no active decryption key/)).toBeInTheDocument();
  });
});
