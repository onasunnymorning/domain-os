import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { EPPLoginsSection } from '../EPPLoginsSection';
import * as hooks from '@/lib/hooks/useEPP';
import type { EPPLogin } from '@/lib/types/epp';

vi.mock('@/lib/hooks/useEPP', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/hooks/useEPP')>();
  return {
    ...actual,
    useUpdateEPPLogin: vi.fn(),
    useLiftEPPLockouts: vi.fn(),
    useLiftEPPSuspension: vi.fn(),
    useCreateEPPLogin: vi.fn(),
    useAddEPPPassword: vi.fn(),
  };
});

const login: EPPLogin = {
  login_id: 'rar-001', role: 'transform', enabled: true, session_cap: 0,
  created_at: '2026-10-01T00:00:00Z', updated_at: '2026-10-01T00:00:00Z',
  passwords: [{ id: 'p1', created_at: '2026-10-01T00:00:00Z', state: 'active' }],
};

describe('EPPLoginsSection', () => {
  const update = vi.fn().mockResolvedValue(login);
  const liftLockouts = vi.fn().mockResolvedValue([]);
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(hooks.useUpdateEPPLogin).mockReturnValue({ mutateAsync: update, isPending: false } as any);
    vi.mocked(hooks.useLiftEPPLockouts).mockReturnValue({ mutateAsync: liftLockouts, isPending: false } as any);
    vi.mocked(hooks.useLiftEPPSuspension).mockReturnValue({ mutateAsync: vi.fn(), isPending: false } as any);
    vi.mocked(hooks.useCreateEPPLogin).mockReturnValue({ mutateAsync: vi.fn(), isPending: false } as any);
    vi.mocked(hooks.useAddEPPPassword).mockReturnValue({ mutateAsync: vi.fn(), isPending: false } as any);
  });

  it('toggling Enabled patches the login', async () => {
    render(<EPPLoginsSection clid="rar-001" logins={[login]} lockouts={[]} suspensions={[]} onRetire={vi.fn()} />);
    fireEvent.click(screen.getByRole('switch', { name: 'rar-001 enabled' }));
    await waitFor(() => expect(update).toHaveBeenCalledWith({ login: 'rar-001', body: { enabled: false } }));
  });

  it('shows passwords on expand and offers Retire for an active one', () => {
    const onRetire = vi.fn();
    render(<EPPLoginsSection clid="rar-001" logins={[login]} lockouts={[]} suspensions={[]} onRetire={onRetire} />);
    fireEvent.click(screen.getByRole('button', { name: /show passwords of rar-001/i }));
    fireEvent.click(screen.getByRole('button', { name: /^retire$/i }));
    expect(onRetire).toHaveBeenCalledWith(expect.objectContaining({ kind: 'password', id: 'p1', login: 'rar-001' }));
  });

  it('shows a lockout with a Lift button', async () => {
    render(
      <EPPLoginsSection
        clid="rar-001"
        logins={[login]}
        lockouts={[{ clid: 'rar-001', login: 'rar-001', client_ip: '203.0.113.9', ends_at: '2026-11-02T14:32:00Z' }]}
        suspensions={[]}
        onRetire={vi.fn()}
      />,
    );
    expect(screen.getByText(/locked out from/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /lift lockout/i }));
    await waitFor(() => expect(liftLockouts).toHaveBeenCalledWith({ clid: 'rar-001', login: 'rar-001' }));
  });

  it('disables Add password when two are usable', () => {
    const full = { ...login, passwords: [...login.passwords, { id: 'p2', created_at: '2026-10-02T00:00:00Z', state: 'retiring' as const }] };
    render(<EPPLoginsSection clid="rar-001" logins={[full]} lockouts={[]} suspensions={[]} onRetire={vi.fn()} />);
    expect(screen.getByRole('button', { name: /add password/i })).toBeDisabled();
  });
});
