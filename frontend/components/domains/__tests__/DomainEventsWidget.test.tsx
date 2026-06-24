import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { DomainEventsWidget } from '../DomainEventsWidget';
import * as domainHooks from '@/lib/hooks/useDomains';

vi.mock('@/lib/hooks/useDomains');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('DomainEventsWidget', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state', () => {
    vi.mocked(domainHooks.useDomainEvents).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as any);

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });
    expect(screen.getByText('Loading lifecycle events...')).toBeInTheDocument();
  });

  it('renders error state', () => {
    vi.mocked(domainHooks.useDomainEvents).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to load'),
    } as any);

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });
    expect(screen.getByText('Failed to load activity history')).toBeInTheDocument();
  });

  it('renders empty state', () => {
    vi.mocked(domainHooks.useDomainEvents).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as any);

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

    vi.mocked(domainHooks.useDomainEvents).mockReturnValue({
      data: mockEvents,
      isLoading: false,
      error: null,
    } as any);

    render(<DomainEventsWidget domainName="example.com" />, { wrapper: createWrapper() });

    // Verify event details
    expect(screen.getByText('Domain Created/Registered')).toBeInTheDocument();
    expect(screen.getByText('Registrar: REG-1')).toBeInTheDocument();
    expect(screen.getByText('SKU: COM-REGISTRATION-1Y')).toBeInTheDocument();

    // Verify Show Payload toggle
    const toggleButton = screen.getByText('Show Payload');
    expect(screen.queryByText(/"evt-1"/)).not.toBeInTheDocument();

    fireEvent.click(toggleButton);
    expect(screen.getByText('Hide Payload')).toBeInTheDocument();
    expect(screen.getByText(/"id": "evt-1"/)).toBeInTheDocument();
  });
});
