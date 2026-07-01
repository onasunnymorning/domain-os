import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ArchivedDomainView } from '../ArchivedDomainView';
import * as eventSearchHooks from '@/lib/hooks/useEventSearch';
import type { DomainTombstone } from '@/lib/api/tombstones';

// Mock the event search hook
vi.mock('@/lib/hooks/useEventSearch');

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

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

const baseTombstone: DomainTombstone = {
  roid: '12345_DOMAIN-DOM',
  name: 'example.com',
  tld_name: 'com',
  registrar_clid: 'REG-001',
  registered_at: '2023-01-15T00:00:00Z',
  expired_at: '2025-01-15T00:00:00Z',
  purged_at: '2025-03-15T00:00:00Z',
  purge_reason: 'expired',
  drop_catch: false,
  last_snapshot: { Name: 'example.com', ClID: 'REG-001' },
  created_at: '2025-03-15T00:00:00Z',
};

const secondTombstone: DomainTombstone = {
  roid: '67890_DOMAIN-DOM',
  name: 'example.com',
  tld_name: 'com',
  registrar_clid: 'REG-002',
  registered_at: '2020-06-01T00:00:00Z',
  expired_at: '2022-06-01T00:00:00Z',
  purged_at: '2022-08-15T00:00:00Z',
  purge_reason: 'expired',
  drop_catch: true,
  created_at: '2022-08-15T00:00:00Z',
};

describe('ArchivedDomainView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default mock: empty events
    vi.mocked(eventSearchHooks.useEventSearch).mockReturnValue(
      mockInfiniteResult({
        data: { pages: [{ data: [], totalCount: 0, tier: 'hot' }], pageParams: [undefined] },
      })
    );
  });

  it('renders archive banner with purge date', () => {
    render(
      <ArchivedDomainView tombstones={[baseTombstone]} domainName="example.com" />,
      { wrapper: createWrapper() }
    );
    // "purged" may appear in both banner text and purge reason badge
    const purgedElements = screen.getAllByText(/purged/i);
    expect(purgedElements.length).toBeGreaterThanOrEqual(1);
    // Should contain at least one "Archived" element (banner or badge)
    const archivedElements = screen.getAllByText(/archived/i);
    expect(archivedElements.length).toBeGreaterThanOrEqual(1);
  });

  it('renders tombstone metadata', () => {
    render(
      <ArchivedDomainView tombstones={[baseTombstone]} domainName="example.com" />,
      { wrapper: createWrapper() }
    );
    // Check that key metadata is displayed
    expect(screen.getByText('12345_DOMAIN-DOM')).toBeInTheDocument();
    expect(screen.getByText('REG-001')).toBeInTheDocument();
    expect(screen.getByText('expired')).toBeInTheDocument();
  });

  it('renders incarnation picker when multiple tombstones', () => {
    render(
      <ArchivedDomainView
        tombstones={[baseTombstone, secondTombstone]}
        domainName="example.com"
      />,
      { wrapper: createWrapper() }
    );
    // Should show incarnation buttons
    const buttons = screen.getAllByRole('button').filter(
      b => b.textContent?.includes('Incarnation')
    );
    expect(buttons.length).toBe(2);
  });

  it('switches incarnation when clicking picker', () => {
    render(
      <ArchivedDomainView
        tombstones={[baseTombstone, secondTombstone]}
        domainName="example.com"
      />,
      { wrapper: createWrapper() }
    );
    // Click the second incarnation
    const buttons = screen.getAllByRole('button').filter(
      b => b.textContent?.includes('Incarnation')
    );
    fireEvent.click(buttons[1]);
    // Should now show the second tombstone's ROID
    expect(screen.getByText('67890_DOMAIN-DOM')).toBeInTheDocument();
  });

  it('renders last snapshot expander when snapshot exists', () => {
    render(
      <ArchivedDomainView tombstones={[baseTombstone]} domainName="example.com" />,
      { wrapper: createWrapper() }
    );
    // Should have a snapshot toggle button
    const snapshotButton = screen.getAllByRole('button').find(
      b => b.textContent?.toLowerCase().includes('snapshot')
    );
    expect(snapshotButton).toBeDefined();
  });

  it('renders history link', () => {
    render(
      <ArchivedDomainView tombstones={[baseTombstone]} domainName="example.com" />,
      { wrapper: createWrapper() }
    );
    // Find link by href since text may be an icon or combined content
    const links = screen.getAllByRole('link');
    const historyLink = links.find(l => l.getAttribute('href')?.includes('/history'));
    expect(historyLink).toBeDefined();
    expect(historyLink?.getAttribute('href')).toBe('/domains/example.com/history');
  });
});
