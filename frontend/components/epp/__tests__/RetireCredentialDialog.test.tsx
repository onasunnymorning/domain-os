import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { RetireCredentialDialog } from '../RetireCredentialDialog';
import * as hooks from '@/lib/hooks/useEPP';

vi.mock('@/lib/hooks/useEPP', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/hooks/useEPP')>();
  return { ...actual, useRetireEPPCredential: vi.fn() };
});

describe('RetireCredentialDialog', () => {
  const mutateAsync = vi.fn().mockResolvedValue({ retired: true });
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(hooks.useRetireEPPCredential).mockReturnValue({ mutateAsync, isPending: false } as any);
  });

  it('retires normally by default, with the 7-day explanation', async () => {
    const onOpenChange = vi.fn();
    render(
      <RetireCredentialDialog
        clid="rar-001"
        target={{ kind: 'password', id: 'p1', login: 'rar-001', label: 'the password' }}
        onOpenChange={onOpenChange}
      />,
    );
    expect(screen.getByText(/keeps working for 7 more days/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^retire$/i }));
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(mutateAsync).toHaveBeenCalledWith({ kind: 'password', id: 'p1', login: 'rar-001', body: { kind: 'normal' } });
  });

  it('revokes as compromised when chosen', async () => {
    render(
      <RetireCredentialDialog
        clid="rar-001"
        target={{ kind: 'certificate_pin', id: 'c1', label: 'certificate CN=x' }}
        onOpenChange={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByLabelText(/revoke, it is compromised/i));
    const btn = screen.getByRole('button', { name: /revoke now/i });
    fireEvent.click(btn);
    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());
    expect(mutateAsync).toHaveBeenCalledWith({ kind: 'certificate_pin', id: 'c1', login: undefined, body: { kind: 'compromise' } });
  });

  it('offers only revoke for a credential already retiring', () => {
    render(
      <RetireCredentialDialog
        clid="rar-001"
        target={{ kind: 'allowed_network', id: 'n1', label: 'network 192.0.2.0/24', alreadyRetired: true }}
        onOpenChange={vi.fn()}
      />,
    );
    expect(screen.queryByText(/keeps working for 7 more days/i)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /revoke now/i })).toBeInTheDocument();
  });
});
