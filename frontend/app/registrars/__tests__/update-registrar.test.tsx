import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import EditRegistrarPage from '../[clid]/edit/page';
import * as registrarHooks from '@/lib/hooks/useRegistrars';

vi.mock('@/lib/hooks/useRegistrars');
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('EditRegistrarPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('prefills values and submits update to registrar detail', async () => {
    // Mock router params and navigation
    const { useRouter, useParams } = await import('next/navigation');
    const push = vi.fn();
    vi.mocked(useRouter as any).mockReturnValue({ push } as any);
    vi.mocked(useParams as any).mockReturnValue({ clid: 'abc-123' });

    // Mock data fetch
    const registrar = {
      ClID: 'abc-123',
      Name: 'Example Registrar, Inc.',
      Email: 'contact@example.com',
      GurID: 468,
      Status: 'ok',
      IANAStatus: 'Accredited',
      PostalInfo: [
        {
          Type: 'int',
          Address: { CC: 'US', City: 'Austin', Street1: '123 Main' },
        },
      ],
      RdapBaseURL: 'https://rdap.example',
      URL: 'https://example.com',
      Voice: '+1.5555555555',
      Fax: '',
      WhoisInfo: { Name: 'whois.example', URL: 'https://whois.example' },
    } as any;

    vi.mocked(registrarHooks.useRegistrar).mockReturnValue({
      data: registrar,
      isLoading: false,
      error: null,
    } as any);

    const mutateAsync = vi.fn().mockResolvedValue({});
    vi.mocked(registrarHooks.useUpdateRegistrar).mockReturnValue({
      mutateAsync,
      isPending: false,
    } as any);

    // Render
    render(<EditRegistrarPage />, { wrapper: createWrapper() });

    // Prefill check: Name field should have existing value
    const nameInput = screen.getByLabelText('Name') as HTMLInputElement;
    expect(nameInput.value).toBe('Example Registrar, Inc.');

    // Change a field and submit
    fireEvent.change(nameInput, { target: { value: 'Updated Registrar Name' } });
    fireEvent.click(screen.getByRole('button', { name: /Save changes/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());
    const call = mutateAsync.mock.calls[0][0];
    expect(call.clid).toBe('abc-123');
    expect(call.data.Name).toBe('Updated Registrar Name');

    // Navigation back to detail page
    await waitFor(() => {
      expect(push).toHaveBeenCalledWith('/registrars/abc-123');
    });
  });
});
