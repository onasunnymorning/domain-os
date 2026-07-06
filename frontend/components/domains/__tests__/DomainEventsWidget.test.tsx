import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { DomainEventsWidget } from '../DomainEventsWidget';
import * as eventSearchHooks from '@/lib/hooks/useEventSearch';

vi.mock('@/lib/hooks/useEventSearch');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

/** Helper to build a mock return value matching useInfiniteQuery's shape */
function mockInfiniteResult(overrides: Record<string, any>) {
  return {
    data: undefined,
    isLoading: false,
    error: null,
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isFetchingNextPage: false,
    ...overrides,
  } as any;
}

describe('DomainEventsWidget', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state', () => {
    vi.mocked(eventSearchHooks.useEventSearch).mockReturnValue(
      mockInfiniteResult({ isLoading: true })
    );

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });
    expect(screen.getByText('Loading lifecycle events...')).toBeInTheDocument();
  });

  it('renders error state', () => {
    vi.mocked(eventSearchHooks.useEventSearch).mockReturnValue(
      mockInfiniteResult({ error: new Error('Failed to load') })
    );

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });
    expect(screen.getByText('Failed to load activity history')).toBeInTheDocument();
  });

  it('renders empty state', () => {
    vi.mocked(eventSearchHooks.useEventSearch).mockReturnValue(
      mockInfiniteResult({
        data: { pages: [{ data: [], totalCount: 0, tier: 'hot' }], pageParams: [undefined] },
      })
    );

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });
    expect(screen.getByText('No events recorded for this domain.')).toBeInTheDocument();
  });

  it('renders list of events and allows toggling raw payload', () => {
    const mockEvents = [
      {
        id: 'evt-1',
        type: 'domain.registered',
        source: 'domain-os/api',
        subject: 'example.com',
        time: '2026-06-24T12:00:00Z',
        data: {
          ClientID: 'REG-1',
          SKU: 'COM-REGISTRATION-1Y',
          DomainYears: 1,
          TransactionType: 'REGISTRATION',
        },
      },
    ];

    vi.mocked(eventSearchHooks.useEventSearch).mockReturnValue(
      mockInfiniteResult({
        data: {
          pages: [{ data: mockEvents, totalCount: 1, tier: 'hot' }],
          pageParams: [undefined],
        },
      })
    );

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });

    // Verify event details
    expect(screen.getByText('Domain Created/Registered')).toBeInTheDocument();
    expect(screen.getByText('Registrar: REG-1')).toBeInTheDocument();
    expect(screen.getByText('SKU: COM-REGISTRATION-1Y')).toBeInTheDocument();

    // Verify Show Payload toggle
    const toggleButton = screen.getByText('Show Payload');
    expect(screen.queryByText(/\"evt-1\"/)).not.toBeInTheDocument();

    fireEvent.click(toggleButton);
    expect(screen.getByText('Hide Payload')).toBeInTheDocument();
    // Raw JSON is inside a <details> element — verify it's present in the DOM
    expect(screen.getByText('Raw Event JSON')).toBeInTheDocument();
  });

  it('renders diff button when states are present and opens the diff modal', async () => {
    const mockEvents = [
      {
        id: 'evt-2',
        type: 'domain.status_set',
        source: 'domain-os/api',
        subject: 'example.com',
        time: '2026-06-25T12:00:00Z',
        before_state: {
          Status: ['ok'],
        },
        after_state: {
          Status: ['clientHold'],
        },
      },
    ];

    vi.mocked(eventSearchHooks.useEventSearch).mockReturnValue(
      mockInfiniteResult({
        data: {
          pages: [{ data: mockEvents, totalCount: 1, tier: 'hot' }],
          pageParams: [undefined],
        },
      })
    );

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });

    // Diff button should be present because before/after state exist
    const diffButton = screen.getByText('Diff State');
    expect(diffButton).toBeInTheDocument();

    // Dialog/modal should not be open yet
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    // Click to open diff modal
    fireEvent.click(diffButton);

    // The dialog should appear in the DOM
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    
    // Check for title or elements in diff modal
    expect(await screen.findByText('State Difference')).toBeInTheDocument();
    expect(screen.getByText('Key Changes')).toBeInTheDocument();
    
    // Check that changed property 'Status.0' is rendered
    expect(await screen.findByText('Status.0')).toBeInTheDocument();
  });
});

