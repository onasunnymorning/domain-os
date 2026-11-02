import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AddPasswordDialog } from '../AddPasswordDialog';
import * as hooks from '@/lib/hooks/useEPP';

vi.mock('@/lib/hooks/useEPP', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/hooks/useEPP')>();
  return { ...actual, useAddEPPPassword: vi.fn() };
});

describe('AddPasswordDialog', () => {
  const mutateAsync = vi.fn();
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(hooks.useAddEPPPassword).mockReturnValue({ mutateAsync, isPending: false } as any);
  });

  it('shows a generated password once and only closes through the button', async () => {
    mutateAsync.mockResolvedValue({ id: 'p1', state: 'active', created_at: '2026-11-02T00:00:00Z', password: 'Gen-Pass-16chars' });
    const onOpenChange = vi.fn();
    render(<AddPasswordDialog clid="rar-001" login="rar-001" onOpenChange={onOpenChange} />);

    fireEvent.click(screen.getByRole('button', { name: /generate password/i }));
    await waitFor(() => expect(screen.getByTestId('generated-password')).toHaveTextContent('Gen-Pass-16chars'));
    expect(mutateAsync).toHaveBeenCalledWith({ login: 'rar-001', body: {} });
    expect(screen.getByText(/only time it will be shown/i)).toBeInTheDocument();

    await userEvent.keyboard('{Escape}');
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(screen.getByTestId('generated-password')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /i have copied it/i }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('cannot be dismissed while the request is in flight', async () => {
    let resolve!: (v: unknown) => void;
    mutateAsync.mockReturnValue(new Promise((r) => { resolve = r; }));
    vi.mocked(hooks.useAddEPPPassword).mockReturnValue({ mutateAsync, isPending: true } as any);
    const onOpenChange = vi.fn();
    render(<AddPasswordDialog clid="rar-001" login="rar-001" onOpenChange={onOpenChange} />);
    await userEvent.keyboard('{Escape}');
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /close/i })).not.toBeInTheDocument();
    resolve({ id: 'p1', state: 'active', created_at: '2026-11-02T00:00:00Z' });
  });

  it('sends a chosen password and closes without showing it', async () => {
    mutateAsync.mockResolvedValue({ id: 'p1', state: 'active', created_at: '2026-11-02T00:00:00Z' });
    const onOpenChange = vi.fn();
    render(<AddPasswordDialog clid="rar-001" login="rar-001" onOpenChange={onOpenChange} />);

    fireEvent.click(screen.getByLabelText(/set my own/i));
    const btn = screen.getByRole('button', { name: /add password/i });
    expect(btn).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'short' } });
    expect(btn).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'chosen-pw-1' } });
    expect(btn).not.toBeDisabled();
    fireEvent.click(btn);
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(mutateAsync).toHaveBeenCalledWith({ login: 'rar-001', body: { password: 'chosen-pw-1' } });
    expect(screen.queryByTestId('generated-password')).not.toBeInTheDocument();
  });
});
