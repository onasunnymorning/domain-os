import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import CreateDomainPage from '../create/page';
import * as domainHooks from '@/lib/hooks/useDomains';
import * as registrarHooks from '@/lib/hooks/useRegistrars';
import * as tldHooks from '@/lib/hooks/useTLDs';
import { isLikelyCompliantAuthInfo } from '@/lib/utils/authinfo';

vi.mock('@/lib/hooks/useDomains');
vi.mock('@/lib/hooks/useRegistrars');
vi.mock('@/lib/hooks/useTLDs');

// Mock layout wrapper
vi.mock('@/components/layout/DashboardLayout', () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => <div data-testid="layout">{children}</div>,
}));

const wrap = (children: React.ReactNode) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
};

describe('CreateDomainPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(registrarHooks.useRegistrars).mockReturnValue({
      data: { Data: [{ ClID: 'REG1', Name: 'Example Registrar' }] },
    } as any);
    vi.mocked(tldHooks.useTLDs).mockReturnValue({
      data: { Data: [{ Name: 'test' }] },
    } as any);
  });

  it('submits required fields and converts datetime-local to ISO', async () => {
    const mutateAsync = vi.fn().mockResolvedValue({ Name: 'example.test' });
    vi.mocked(domainHooks.useCreateDomain).mockReturnValue({ mutateAsync, isPending: false } as any);

    render(wrap(<CreateDomainPage />));

    fireEvent.change(screen.getByLabelText(/Domain Label/i), { target: { value: 'example' } });
  // Open TLD combobox (first combobox) and choose
  fireEvent.click(screen.getAllByRole('combobox')[0]);
  fireEvent.click(screen.getByRole('button', { name: /^test$/i }));

  // Open registrar combobox (second combobox) and choose
  fireEvent.click(screen.getAllByRole('combobox')[1]);
    fireEvent.click(screen.getByRole('button', { name: /Example Registrar \(REG1\)/i }));

  fireEvent.change(screen.getByRole('textbox', { name: /AuthInfo/i }), { target: { value: 'secret' } });

    const dt = '2026-01-01T00:00';
    fireEvent.change(screen.getByLabelText(/Expiry Date/i), { target: { value: dt } });

    fireEvent.click(screen.getByRole('button', { name: /Create Domain/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());
    const arg = mutateAsync.mock.calls[0][0];

  expect(arg.Name).toBe('example.test');
    expect(arg.ClID).toBe('REG1');
    expect(arg.AuthInfo).toBe('secret');
    // Ensure ISO conversion
    expect(arg.ExpiryDate).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:00\.000Z$/);
  });

  it('can generate a compliant AuthInfo and submit it', async () => {
    const mutateAsync = vi.fn().mockResolvedValue({ Name: 'example.test' });
    vi.mocked(domainHooks.useCreateDomain).mockReturnValue({ mutateAsync, isPending: false } as any);

    render(wrap(<CreateDomainPage />));

  // Fill minimal required fields (label, tld, registrar, expiry)
  fireEvent.change(screen.getByLabelText(/Domain Label/i), { target: { value: 'example' } });
  fireEvent.click(screen.getAllByRole('combobox')[0]);
  fireEvent.click(screen.getByRole('button', { name: /^test$/i }));
  fireEvent.click(screen.getAllByRole('combobox')[1]);
    fireEvent.click(screen.getByRole('button', { name: /Example Registrar \(REG1\)/i }));
    const dt = '2026-01-01T00:00';
    fireEvent.change(screen.getByLabelText(/Expiry Date/i), { target: { value: dt } });

    // Click Generate
    fireEvent.click(screen.getByRole('button', { name: /Generate AuthInfo/i }));

  const authInput = screen.getByRole('textbox', { name: /AuthInfo/i }) as HTMLInputElement;
    expect(authInput.value).toBeTruthy();
    expect(isLikelyCompliantAuthInfo(authInput.value)).toBe(true);

    fireEvent.click(screen.getByRole('button', { name: /Create Domain/i }));
    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());
    const arg = mutateAsync.mock.calls[0][0];
    expect(arg.AuthInfo).toBe(authInput.value);
  });
});
