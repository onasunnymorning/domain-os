import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import RegistrarDetailPage from '../[clid]/page';
import * as registrarHooks from '@/lib/hooks/useRegistrars';
import * as accHooks from '@/lib/hooks/useAccreditations';
import { useParams } from 'next/navigation';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

vi.mock('@/lib/hooks/useRegistrars');
vi.mock('@/lib/hooks/useAccreditations');

// Ensure route params are provided by the mocked next/navigation
// Default route param
(useParams as unknown as { mockReturnValue: (v: any) => any }).mockReturnValue({ clid: '1015-mobileco-do' } as any);

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

function mockRegistrar(status: string) {
  vi.mocked(registrarHooks.useRegistrar).mockReturnValue({
    data: {
      ClID: '1015-mobileco-do',
      Name: 'MOBILE.CO DOMAINS CORP',
      GurID: 1015,
      Status: status,
      IANAStatus: 'Terminated',
      Autorenew: false,
      Email: 'test@example.com',
      URL: '',
      RdapBaseURL: '',
      CreatedAt: '2025-01-01T00:00:00Z',
      UpdatedAt: '2025-01-01T00:00:00Z',
    },
    isLoading: false,
    error: null,
  } as any);
}

function mockAccreditations(count = 0) {
  vi.mocked(accHooks.useRegistrarAccreditations).mockReturnValue({
    data: { Data: Array.from({ length: count }, (_, i) => ({
      Name: `tld-${i}`,
      Type: 'generic',
      UName: '',
      RyID: 'ry',
      AllowEscrowImport: false,
      EnableDNS: true,
      Phases: [],
      CreatedAt: '2025-01-01T00:00:00Z',
      UpdatedAt: '2025-01-01T00:00:00Z',
    })) },
    isLoading: false,
  } as any);
  vi.mocked(accHooks.useAccreditRegistrar).mockReturnValue({ mutateAsync: vi.fn(), isPending: false } as any);
  vi.mocked(accHooks.useDeaccreditRegistrar).mockReturnValue({ mutateAsync: vi.fn(), isPending: false } as any);
}

describe('RegistrarDetailPage - accreditation button enablement', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('disables Add accreditation when registrar status is not ok', () => {
    mockRegistrar('terminated');
    mockAccreditations(0);

  render(<RegistrarDetailPage />, { wrapper: createWrapper() });

    const btn = screen.getByRole('button', { name: /add accreditation/i });
    expect(btn).toBeDisabled();
    expect(screen.getByText(/not in an eligible status/i)).toBeInTheDocument();
  });

  it('enables Add accreditation when registrar status is ok', () => {
    mockRegistrar('ok');
    mockAccreditations(0);

  render(<RegistrarDetailPage />, { wrapper: createWrapper() });

    const btn = screen.getByRole('button', { name: /add accreditation/i });
    expect(btn).not.toBeDisabled();
  });
});
