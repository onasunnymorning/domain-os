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
});
