import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock IntersectionObserver
global.IntersectionObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
  takeRecords: vi.fn(),
}));

// Mock React's experimental `use` to synchronously resolve the Next.js params Promise
vi.mock('react', async (orig) => {
  const actual = await (orig() as any);
  return {
    ...actual,
    use: (v: any) => {
      // In our page, `use` is only used to unwrap route params Promise
      if (v && typeof v.then === 'function') {
        return { name: 'example' };
      }
      return v;
    },
  };
});
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import TLDDetailPage from '../[name]/page';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import * as tldHooks from '@/lib/hooks/useTLDs';
import * as accHooks from '@/lib/hooks/useAccreditations';
import * as regHooks from '@/lib/hooks/useRegistrars';
import * as domainHooks from '@/lib/hooks/useDomains';

vi.mock('@/lib/hooks/useTLDs');
vi.mock('@/lib/hooks/useAccreditations');
vi.mock('@/lib/hooks/useRegistrars');
vi.mock('@/lib/hooks/useDomains');

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

function baseTLD() {
  return {
    Name: 'example',
    Type: 'generic',
    RyID: 'operator-1',
    AllowEscrowImport: false,
    EnableDNS: true,
    CreatedAt: '2025-01-01T00:00:00Z',
    UpdatedAt: '2025-01-02T00:00:00Z',
  } as any;
}

describe('TLDDetailPage accreditation and de-accreditation flows', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default TLD load
    vi.mocked(tldHooks.useTLD).mockReturnValue({
      data: baseTLD(),
      isLoading: false,
      error: null,
    } as any);

    // Mock delete TLD hook used by page header button
    vi.mocked(tldHooks.useDeleteTLD).mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    } as any);

    // Accredited registrars list for this TLD
    vi.mocked(accHooks.useTLDRegistrars).mockReturnValue({
      data: {
        Data: [
          { ClID: 'REG-OK', Name: 'Ok Registrar', Status: 'ok' },
          { ClID: 'REG-TERM', Name: 'Term Registrar', Status: 'terminated' },
        ],
      },
      isLoading: false,
    } as any);

    // Registrar search results when opening accredit modal
    vi.mocked(regHooks.useRegistrars).mockReturnValue({
      data: {
        Data: [
          { ClID: 'REG-SEARCH-OK', Name: 'Search Ok', Status: 'ok' },
          { ClID: 'REG-SEARCH-RO', Name: 'Search Readonly', Status: 'readonly' },
          { ClID: 'REG-SEARCH-TERM', Name: 'Search Terminated', Status: 'terminated' },
        ],
      },
      isLoading: false,
      error: null,
    } as any);

    // Default mutations: succeed
    vi.mocked(accHooks.useAccreditForTLD).mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
      isPending: false,
    } as any);
    vi.mocked(accHooks.useDeaccreditForTLD).mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
      isPending: false,
    } as any);
    
    // Default domains mock
    vi.mocked(domainHooks.useDomainCountsForRegistrars).mockReturnValue([] as any);
    vi.mocked(domainHooks.useDomainCount).mockReturnValue({
      data: { ObjectType: 'Domain', Count: 0, Timestamp: '2024-01-01T00:00:00Z' },
      isLoading: false,
      error: null,
    } as any);
  });

  // Helper: switch to the Registrars tab and wait for content to mount
  const switchToRegistrarsTab = async () => {
    // The tab accessible name includes the count badge text (e.g., "Registrars 2")
    const tabs = screen.getAllByRole('tab');
    const registrarsTab = tabs.find(t => t.textContent?.includes('Registrars'));
    expect(registrarsTab).toBeDefined();
    fireEvent.click(registrarsTab!);
    // Wait for tab content to become interactive
    await screen.findByRole('button', { name: /Accredit registrar/i });
  };

  it('opens accredit modal, enforces eligibility, and accredits successfully', async () => {
    const wrapper = createWrapper();
    render(<TLDDetailPage params={Promise.resolve({ name: 'example' })} />, { wrapper });
    await switchToRegistrarsTab();

    // Open the accredit modal
    fireEvent.click(screen.getByRole('button', { name: /Accredit registrar/i }));

    // Verify search results rows
    expect(await screen.findByText('Search Ok')).toBeInTheDocument();
    expect(screen.getByText('Search Readonly')).toBeInTheDocument();
    expect(screen.getByText('Search Terminated')).toBeInTheDocument();

    // Eligible registrar should have enabled Accredit button
    const okRowButton = screen.getByRole('button', { name: /^Accredit$/ });
    expect(okRowButton).toBeEnabled();

    // Ineligible registrar shows Not eligible and is disabled
    const notEligibleButtons = screen.getAllByRole('button', { name: /Not eligible/i });
    notEligibleButtons.forEach((b: HTMLElement) => expect(b).toBeDisabled());

    // Click Accredit and expect mutateAsync called and modal closed
    fireEvent.click(okRowButton);

    const accreditMock = vi.mocked(accHooks.useAccreditForTLD).mock.results[0]!.value as any;
    expect(accreditMock.mutateAsync).toHaveBeenCalledWith('REG-SEARCH-OK');

    // Modal should close after success (title disappears)
    await waitFor(() => {
      expect(screen.queryByText(/Accredit registrar to/i)).not.toBeInTheDocument();
    });
  });

  it('displays inline error when accreditation fails with backend error', async () => {
    // Override accredit hook to reject
    vi.mocked(accHooks.useAccreditForTLD).mockReturnValue({
      mutateAsync: vi.fn().mockRejectedValue({ response: { data: { error: 'Already accredited' } } }),
      isPending: false,
    } as any);

    const wrapper = createWrapper();
    render(<TLDDetailPage params={Promise.resolve({ name: 'example' })} />, { wrapper });
    await switchToRegistrarsTab();

    fireEvent.click(screen.getByRole('button', { name: /Accredit registrar/i }));

    // Click the first eligible Accredit button
    const okRowButton = await screen.findByRole('button', { name: /^Accredit$/ });
    fireEvent.click(okRowButton);

    // Inline error should be shown and modal remains open
    expect(await screen.findByText('Already accredited')).toBeInTheDocument();
    expect(screen.getByText(/Accredit registrar to/i)).toBeInTheDocument();
  });

  it('enforces confirmation and de-accredits successfully', async () => {
    const wrapper = createWrapper();
    render(<TLDDetailPage params={Promise.resolve({ name: 'example' })} />, { wrapper });
    await switchToRegistrarsTab();

    // Open de-accredit dialog for a registrar
    fireEvent.click(screen.getAllByRole('button', { name: /De-accredit/i })[0]);

    // Confirm button should be disabled until correct text is entered
    const confirmBtn = screen.getByRole('button', { name: /Confirm de-accredit/i });
    expect(confirmBtn).toBeDisabled();

    const input = screen.getByPlaceholderText(/delete REG-OK/i);
    // Type incorrect text first
    fireEvent.change(input, { target: { value: 'delete WRONG' } });
    expect(confirmBtn).toBeDisabled();

    // Type correct confirmation
    fireEvent.change(input, { target: { value: 'delete REG-OK' } });
    expect(confirmBtn).toBeEnabled();

    fireEvent.click(confirmBtn);

    const deaccMock = vi.mocked(accHooks.useDeaccreditForTLD).mock.results[0]!.value as any;
    expect(deaccMock.mutateAsync).toHaveBeenCalledWith('REG-OK');

    // Dialog should close after success
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  it('shows inline error when de-accredit fails', async () => {
    // Override de-accredit to reject with error
    vi.mocked(accHooks.useDeaccreditForTLD).mockReturnValue({
      mutateAsync: vi.fn().mockRejectedValue({ response: { data: { error: 'Cannot remove last registrar' } } }),
      isPending: false,
    } as any);

    const wrapper = createWrapper();
    render(<TLDDetailPage params={Promise.resolve({ name: 'example' })} />, { wrapper });
    await switchToRegistrarsTab();

    // Open de-accredit dialog
    fireEvent.click(screen.getAllByRole('button', { name: /De-accredit/i })[0]);

    const input = screen.getByPlaceholderText(/delete REG-OK/i);
    fireEvent.change(input, { target: { value: 'delete REG-OK' } });

    fireEvent.click(screen.getByRole('button', { name: /Confirm de-accredit/i }));

    // Error message should be shown and dialog remains open
    expect(await screen.findByText('Cannot remove last registrar')).toBeInTheDocument();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });
});
