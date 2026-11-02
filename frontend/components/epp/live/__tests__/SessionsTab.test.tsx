import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { SessionsTab, describeFilter } from '../SessionsTab';
import * as hooks from '@/lib/hooks/useEPP';
import { useSearchParams } from 'next/navigation';

vi.mock('@/lib/hooks/useEPP', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/hooks/useEPP')>();
  return { ...actual, useEPPSessions: vi.fn(), useEndEPPSession: vi.fn(), useEndEPPSessionsMatching: vi.fn() };
});

const session = {
  id: 's1', node: 'epp-1', client_ip: '203.0.113.9', clid: 'acme-rar', login_id: 'acme-rar', role: 'transform',
  connected_at: '2026-11-02T12:00:00Z', commands: 7,
};

describe('SessionsTab', () => {
  const endOne = vi.fn().mockResolvedValue(session);
  const endMatching = vi.fn().mockResolvedValue({ killed: 1, sessions: [session] });
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(hooks.useEndEPPSession).mockReturnValue({ mutateAsync: endOne, isPending: false } as any);
    vi.mocked(hooks.useEndEPPSessionsMatching).mockReturnValue({ mutateAsync: endMatching, isPending: false } as any);
    vi.mocked(hooks.useEPPSessions).mockReturnValue({ data: { Meta: { PageSize: 50, PageCursor: '' }, Data: [session] }, isLoading: false } as any);
  });

  it('describes a filter in words', () => {
    expect(describeFilter({ clid: 'acme-rar', client_ip: '203.0.113.9' })).toBe('of registrar acme-rar, from 203.0.113.9');
  });

  it('ends one session after confirmation', async () => {
    render(<SessionsTab active />);
    fireEvent.click(screen.getByRole('button', { name: /end session/i }));
    const dlg = await screen.findByRole('alertdialog');
    expect(within(dlg).getByText(/can log in again at once/i)).toBeInTheDocument();
    fireEvent.click(within(dlg).getByRole('button', { name: /^end session$/i }));
    await waitFor(() => expect(endOne).toHaveBeenCalledWith('s1'));
  });

  it('offers End all only with a filter, with the count, and sends the filter', async () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams('tab=sessions&clid=acme-rar') as any);
    render(<SessionsTab active />);
    const btn = await screen.findByRole('button', { name: /end all 1 matching/i });
    fireEvent.click(btn);
    fireEvent.click(screen.getByRole('button', { name: /^end sessions$/i }));
    await waitFor(() => expect(endMatching).toHaveBeenCalledWith({ clid: 'acme-rar' }));
  });

  it('shows no End all without a filter', () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams('tab=sessions') as any);
    render(<SessionsTab active />);
    expect(screen.queryByRole('button', { name: /end all/i })).not.toBeInTheDocument();
  });
});
