import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { LockoutsCard, OpenSessionsCard, SuspensionsCard } from '../NowCards';
import * as hooks from '@/lib/hooks/useEPP';

vi.mock('@/lib/hooks/useEPP', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/hooks/useEPP')>();
  return {
    ...actual,
    useEPPSessions: vi.fn(),
    useEPPLockouts: vi.fn(),
    useEPPSuspensions: vi.fn(),
    useLiftEPPLockouts: vi.fn(),
    useLiftEPPSuspension: vi.fn(),
  };
});

describe('Now cards', () => {
  const liftLockouts = vi.fn().mockResolvedValue([]);
  const liftSuspension = vi.fn().mockResolvedValue({});
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(hooks.useLiftEPPLockouts).mockReturnValue({ mutateAsync: liftLockouts, isPending: false } as any);
    vi.mocked(hooks.useLiftEPPSuspension).mockReturnValue({ mutateAsync: liftSuspension, isPending: false } as any);
  });

  it('counts open sessions and marks a further page as a lower bound', () => {
    vi.mocked(hooks.useEPPSessions).mockReturnValue({
      data: { Meta: { PageSize: 100, PageCursor: 'more' }, Data: [{ id: 'a', clid: 'r1' }, { id: 'b', clid: 'r2' }] },
      isLoading: false,
    } as any);
    render(<OpenSessionsCard active />);
    expect(screen.getByText('2+')).toBeInTheDocument();
    expect(screen.getByText(/from 2 registrars/)).toBeInTheDocument();
  });

  it('lists lockouts and lifts one', async () => {
    vi.mocked(hooks.useEPPLockouts).mockReturnValue({
      data: [{ clid: 'r1', login: 'r1', client_ip: '203.0.113.9', ends_at: '2026-11-02T14:32:00Z' }],
      isLoading: false,
    } as any);
    render(<LockoutsCard active />);
    expect(screen.getByText('1')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^lift$/i }));
    await waitFor(() => expect(liftLockouts).toHaveBeenCalledWith({ clid: 'r1', login: 'r1' }));
  });

  it('shows the scope of a suspension and lifts it', async () => {
    vi.mocked(hooks.useEPPSuspensions).mockReturnValue({
      data: [{ clid: 'r1', login: 'r1', scope: 'family', family: 'acme', ends_at: '2026-11-02T14:32:00Z' }],
      isLoading: false,
    } as any);
    render(<SuspensionsCard active />);
    expect(screen.getByText(/counted for family acme/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^lift$/i }));
    await waitFor(() => expect(liftSuspension).toHaveBeenCalledWith({ clid: 'r1', login: 'r1' }));
  });

  it('shows a query error instead of a count', () => {
    vi.mocked(hooks.useEPPLockouts).mockReturnValue({ error: { response: { status: 503, data: { error: 'the lockouts cannot be read' } } }, isLoading: false } as any);
    render(<LockoutsCard active />);
    expect(screen.getByText(/could not read lockouts: the lockouts cannot be read/i)).toBeInTheDocument();
  });
});
