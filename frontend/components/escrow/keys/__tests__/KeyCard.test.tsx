import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { KeyCard } from '../KeyCard';
import type { EscrowKeyVersion } from '@/lib/api/escrow-keys';

vi.mock('@/lib/api/escrow-keys', async (orig) => ({
  ...(await (orig() as Promise<Record<string, unknown>>)),
  getEscrowPublicKey: vi.fn(),
}));

function version(over: Partial<EscrowKeyVersion> = {}): EscrowKeyVersion {
  return {
    id: 'v1', partyId: 'p', owner: 'platform', purpose: 'verify-inbound', material: 'openpgp-public',
    version: 1, fingerprint: 'ABCD'.repeat(10), hasPublicKey: true, state: 'ACTIVE',
    lastProbeOk: false, compromised: false, createdAt: '2026-09-01T00:00:00Z',
    activatedAt: '2026-09-02T00:00:00Z', ...over,
  };
}

const show = (versions: EscrowKeyVersion[]) =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <KeyCard scope={{ tenantId: 'ryop1' }} partyId="p" purpose="verify-inbound" versions={versions} manageable />
    </QueryClientProvider>
  );

describe('KeyCard', () => {
  // Routine setup should answer "is this working" without reading a table of
  // states and probe timestamps; the table is for incident and audit work.
  it('leads with the status and keeps the version table behind technical details', async () => {
    show([version()]);
    expect(screen.getByText('Ready')).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /technical details/i }));
    expect(screen.getByRole('table')).toBeInTheDocument();
    // The registry's own state name stays reachable for support work.
    expect(screen.getByText('ACTIVE')).toBeInTheDocument();
  });

  it('shows the fingerprint of the key actually in use, and when it started', () => {
    show([version()]);
    expect(screen.getByText('ABCD'.repeat(10))).toBeInTheDocument();
    expect(screen.getByText(/In use since/)).toBeInTheDocument();
  });

  it('tells the reader to check a source fingerprint out of band', () => {
    show([version()]);
    expect(screen.getByText(/over a channel you trust/)).toBeInTheDocument();
  });

  it('says what is missing, and offers to add a key, when there is none', () => {
    show([]);
    expect(screen.getByText('Setup incomplete')).toBeInTheDocument();
    expect(screen.getByText(/No verification key yet/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add key/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /technical details/i })).not.toBeInTheDocument();
  });
});
