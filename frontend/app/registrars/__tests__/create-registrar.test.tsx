import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import CreateRegistrarPage from '../create/page';
import * as registrarHooks from '@/lib/hooks/useRegistrars';

vi.mock('@/lib/hooks/useRegistrars');
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('CreateRegistrarPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('submits minimal required fields and navigates to registrar detail page', async () => {
    const mutateAsync = vi.fn().mockResolvedValue({});
    vi.mocked(registrarHooks.useCreateRegistrar).mockReturnValue({
      mutateAsync,
      isPending: false,
    } as any);
    const { useRouter } = await import('next/navigation');
    const push = vi.fn();
    // Force the component to receive our router mock instance
    vi.mocked(useRouter as any).mockReturnValue({
      push,
      replace: vi.fn(),
      prefetch: vi.fn(),
      back: vi.fn(),
    } as any);

    // Render
    render(<CreateRegistrarPage />, { wrapper: createWrapper() });

    // Fill required fields
    fireEvent.change(screen.getByLabelText('ClID *'), { target: { value: 'abc-123' } });
    fireEvent.change(screen.getByLabelText('Name *'), { target: { value: 'Example Registrar, Inc.' } });
    fireEvent.change(screen.getByLabelText('Email *'), { target: { value: 'contact@example.com' } });
    fireEvent.change(screen.getByLabelText('Country Code (CC) *'), { target: { value: 'US' } });
    fireEvent.change(screen.getByLabelText('City *'), { target: { value: 'Austin' } });

    // Submit
    fireEvent.click(screen.getByRole('button', { name: /Create Registrar/i }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalled());

    // Verify payload has core fields
    const payload = mutateAsync.mock.calls[0][0];
    expect(payload.ClID).toBe('abc-123');
    expect(payload.Name).toBe('Example Registrar, Inc.');
    expect(payload.Email).toBe('contact@example.com');
    expect(payload.PostalInfo?.[0]?.Address?.CC).toBe('US');
    expect(payload.PostalInfo?.[0]?.Address?.City).toBe('Austin');
  // Default autorenew should be enabled
  expect(payload.Autorenew).toBe(true);

    // Should navigate to the registrar detail page for the submitted ClID
    await waitFor(() => {
      expect(push).toHaveBeenCalledWith('/registrars/abc-123');
    });
  });
});
