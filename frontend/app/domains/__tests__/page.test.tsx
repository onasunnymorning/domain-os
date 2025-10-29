import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import DomainsPage from '../page';
import * as domainHooks from '@/lib/hooks/useDomains';
import * as registrarHooks from '@/lib/hooks/useRegistrars';
import * as tldHooks from '@/lib/hooks/useTLDs';

vi.mock('@/lib/hooks/useDomains');
vi.mock('@/lib/hooks/useRegistrars');
vi.mock('@/lib/hooks/useTLDs');

// Mock the DashboardLayout to avoid layout complexity in tests
vi.mock('@/components/layout/DashboardLayout', () => ({
  DashboardLayout: ({ children }: { children: React.ReactNode }) => <div data-testid="layout">{children}</div>,
}));

// Router and navigation mocks
const mockReplace = vi.fn();
const mockPush = vi.fn();
const mockBack = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ replace: mockReplace, push: mockPush, back: mockBack }),
  usePathname: () => '/domains',
  useSearchParams: () => new URLSearchParams(''),
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('DomainsPage', () => {
  let lastParams: any;

  beforeEach(() => {
    vi.clearAllMocks();
    mockReplace.mockReset();
    mockPush.mockReset();
    mockBack.mockReset();
    lastParams = undefined;

    // Default registrars and TLDs for dropdowns
    vi.mocked(registrarHooks.useRegistrars).mockReturnValue({
      data: { Data: [{ ClID: 'REG1', Name: 'Example Registrar', GurID: 1, Status: 'ok', Autorenew: true }] },
      isLoading: false,
    } as any);

    vi.mocked(tldHooks.useTLDs).mockReturnValue({
      data: { Data: [{ Name: 'example', UName: 'EXAMPLE' }] },
      isLoading: false,
    } as any);

    // Default domains list/count
    vi.mocked(domainHooks.useDomainCount).mockReturnValue({
      data: { ObjectType: 'Domain', Count: 1, Timestamp: '2024-01-01T00:00:00Z' },
    } as any);

    vi.mocked(domainHooks.useDomains).mockImplementation((params?: any) => {
      lastParams = params;
      return {
        data: {
          Data: [
            { Name: 'example.test', TLDName: 'test', ClID: 'REG1', ExpiryDate: '2025-01-01T00:00:00Z' },
          ],
          Meta: { PageCursor: undefined },
        },
        isLoading: false,
        error: null,
      } as any;
    });
  });

  it('renders domain rows', () => {
    render(<DomainsPage />, { wrapper: createWrapper() });
    expect(screen.getByText('example.test')).toBeInTheDocument();
    expect(screen.getByText('REG1')).toBeInTheDocument();
  });

  it('uses name_like for fuzzy search by default', async () => {
    render(<DomainsPage />, { wrapper: createWrapper() });

    const input = screen.getByPlaceholderText('Search domains...');
    fireEvent.change(input, { target: { value: 'foo' } });

    await waitFor(() => {
      expect(lastParams?.name_like).toBe('foo');
      expect(lastParams?.name_equals).toBeUndefined();
    });
  });

  it('switches to name_equals when Exact match is checked', async () => {
    render(<DomainsPage />, { wrapper: createWrapper() });

    const input = screen.getByPlaceholderText('Search domains...');
    fireEvent.change(input, { target: { value: 'bar.example' } });
    const checkbox = screen.getByText('Exact match').previousSibling as HTMLInputElement;
    fireEvent.click(checkbox);

    await waitFor(() => {
      expect(lastParams?.name_equals).toBe('bar.example');
      expect(lastParams?.name_like).toBeUndefined();
    });
  });

  it('applies clid_equals filter when selecting a registrar', async () => {
    render(<DomainsPage />, { wrapper: createWrapper() });

    // Open Registrar combobox
    const combos = screen.getAllByRole('combobox');
    fireEvent.click(combos[0]);
    // Select the option
    fireEvent.click(screen.getByRole('button', { name: /Example Registrar \(REG1\)/i }));

    await waitFor(() => {
      expect(lastParams?.clid_equals).toBe('REG1');
    });
  });

  it('applies tld_equals filter when selecting a TLD', async () => {
    render(<DomainsPage />, { wrapper: createWrapper() });

    // Open TLD combobox
    const combos = screen.getAllByRole('combobox');
    fireEvent.click(combos[1]);
    // Select the option
    fireEvent.click(screen.getByRole('button', { name: /^example$/i }));

    await waitFor(() => {
      expect(lastParams?.tld_equals).toBe('example');
    });
  });

  it('syncs filters to the URL via router.replace', async () => {
    render(<DomainsPage />, { wrapper: createWrapper() });

    const input = screen.getByPlaceholderText('Search domains...');
    fireEvent.change(input, { target: { value: 'syncme' } });

    // Toggling exact match triggers an immediate URL update even if debounced name hasn't flushed yet
    const checkbox = screen.getByText('Exact match').previousSibling as HTMLInputElement;
    fireEvent.click(checkbox);

    await waitFor(() => {
      const lastUrl = mockReplace.mock.calls.at(-1)?.[0] as string;
      expect(lastUrl).toContain('?');
      expect(lastUrl).toContain('exact=1');
    });

    // Eventually, one of the replace calls should include the debounced query string
    await waitFor(() => {
      const anyWithQuery = mockReplace.mock.calls.some(([url]) => String(url).includes('q=syncme'));
      expect(anyWithQuery).toBe(true);
    });
  });

  it('Reset filters clears all filters and URL query', async () => {
    render(<DomainsPage />, { wrapper: createWrapper() });

    // set something first
    const input = screen.getByPlaceholderText('Search domains...');
    fireEvent.change(input, { target: { value: 'clearme' } });

    // click Reset
    const resetBtn = screen.getByRole('button', { name: /Reset filters/i });
    fireEvent.click(resetBtn);

    await waitFor(() => {
      const lastUrl = mockReplace.mock.calls.at(-1)?.[0] as string;
      expect(lastUrl).toBe('/domains');
    });
  });

  it('row click navigates to domain details, registrar badge opens registrar', async () => {
    const { container } = render(<DomainsPage />, { wrapper: createWrapper() });

    // Click the table row (not the link) to trigger row navigation
    const regCell = screen.getByText('REG1');
    const row = regCell.closest('tr')!;
    fireEvent.click(row);

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalled();
      const target = mockPush.mock.calls[0]?.[0];
      expect(String(target)).toContain('/domains/');
    });

    // registrar badge link href should point to registrar detail
    const regLink = container.querySelector('a[href^="/registrars/"]') as HTMLAnchorElement;
    expect(regLink).toBeTruthy();
    expect(regLink.getAttribute('href')).toBe('/registrars/REG1');
  });
});
