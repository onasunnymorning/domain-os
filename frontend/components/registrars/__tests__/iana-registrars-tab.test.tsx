/**
 * Tests for IANA Registrars Tab Component
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { IANARegistrarsTab } from '../iana-registrars-tab';
import * as registrarHooks from '@/lib/hooks/useRegistrars';
import { IANARegistrarStatus } from '@/lib/types/registrar';

// Mock the hooks
vi.mock('@/lib/hooks/useRegistrars');
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('IANARegistrarsTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading State', () => {
    it('should display loading spinner when fetching data', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: undefined,
        isLoading: true,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText('Loading registrars...')).toBeInTheDocument();
    });
  });

  describe('Data Display', () => {
    it('should display IANA registrars when data is loaded', () => {
      const mockData = {
        Data: [
          {
            GurID: 1,
            Name: 'Example Registrar',
            Status: IANARegistrarStatus.Accredited,
            RdapURL: 'https://rdap.example.com',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
          {
            GurID: 2,
            Name: 'Test Registrar',
            Status: IANARegistrarStatus.Terminated,
            RdapURL: 'https://rdap.test.com',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
        ],
        Meta: {
          Cursor: '2',
          Count: 2,
          PageSize: 50,
        },
      };

      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: mockData,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 2, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText('Example Registrar')).toBeInTheDocument();
      expect(screen.getByText('Test Registrar')).toBeInTheDocument();
      
      // Check for IANA IDs in table (use getAllByText since numbers may appear in count too)
      const ianaIds = screen.getAllByText('1');
      expect(ianaIds.length).toBeGreaterThan(0);
      
      expect(screen.getByText('Showing 2 registrars')).toBeInTheDocument();
    });

    it('should display empty state when no registrars found', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: { Data: [], Meta: undefined },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 0, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText(/No.*registrars.*found/i)).toBeInTheDocument();
    });
  });

  describe('Status Badges', () => {
    it('should display correct status badges', () => {
      const mockData = {
        Data: [
          {
            GurID: 1,
            Name: 'Accredited Registrar',
            Status: IANARegistrarStatus.Accredited,
            RdapURL: '',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
          {
            GurID: 2,
            Name: 'Terminated Registrar',
            Status: IANARegistrarStatus.Terminated,
            RdapURL: '',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
          {
            GurID: 3,
            Name: 'Reserved Registrar',
            Status: IANARegistrarStatus.Reserved,
            RdapURL: '',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
        ],
      };

      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: mockData,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 3, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText('Accredited')).toBeInTheDocument();
      expect(screen.getByText('Terminated')).toBeInTheDocument();
      expect(screen.getByText('Reserved')).toBeInTheDocument();
    });
  });

  describe('Search Functionality', () => {
    it('should update search query when typing in search input', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: { Data: [] },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 0, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      const searchInput = screen.getByPlaceholderText('Search by name or IANA ID...');
      fireEvent.change(searchInput, { target: { value: 'example' } });

      expect(searchInput).toHaveValue('example');
    });
  });

  describe('Status Filter', () => {
    it('should have a status filter dropdown', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: { Data: [] },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 0, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  describe('Sync Functionality', () => {
    it('should display sync button', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: { Data: [] },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 0, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText('Sync from IANA')).toBeInTheDocument();
    });

    it('should show loading state when syncing', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: { Data: [] },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 0, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: true,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText('Syncing...')).toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    it('should display error message when data fetch fails', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Failed to fetch registrars'),
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: undefined,
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText(/Error loading registrars/)).toBeInTheDocument();
    });
  });

  describe('Count Display', () => {
    it('should display total count when available', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: { Data: [] },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: {
          ObjectType: 'IANARegistrar',
          Count: 3500,
          Timestamp: '2024-01-01T00:00:00Z',
        },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText(/Total.*IANA.*Registrars:/i)).toBeInTheDocument();
      expect(screen.getByText('3500')).toBeInTheDocument();
    });

    it('should show "Last updated: Never" when count is 0', () => {
      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: { Data: [] },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      // Backend may return a Timestamp even when Count is 0; UI should show Never instead
      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: {
          ObjectType: 'IANARegistrar',
          Count: 0,
          Timestamp: '2025-10-29T11:24:38Z',
        },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      expect(screen.getByText(/Total.*IANA.*Registrars:/i)).toBeInTheDocument();
      expect(screen.getByText('0')).toBeInTheDocument();
      expect(screen.getByText(/Last updated: Never/i)).toBeInTheDocument();
    });
  });

  describe('RDAP URLs', () => {
    it('should render RDAP URLs as clickable links', () => {
      const mockData = {
        Data: [
          {
            GurID: 1,
            Name: 'Example Registrar',
            Status: IANARegistrarStatus.Accredited,
            RdapURL: 'https://rdap.example.com',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
        ],
      };

      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: mockData,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 1, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      const link = screen.getByRole('link', { name: /rdap.example.com/ });
      expect(link).toHaveAttribute('href', 'https://rdap.example.com');
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('should show "-" when RDAP URL is not available', () => {
      const mockData = {
        Data: [
          {
            GurID: 1,
            Name: 'Example Registrar',
            Status: IANARegistrarStatus.Accredited,
            RdapURL: '',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
        ],
      };

      vi.mocked(registrarHooks.useIANARegistrars).mockReturnValue({
        data: mockData,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      } as any);

      vi.mocked(registrarHooks.useIANARegistrarCount).mockReturnValue({
        data: { ObjectType: 'IANARegistrar', Count: 1, Timestamp: '2024-01-01T00:00:00Z' },
        isLoading: false,
      } as any);

      vi.mocked(registrarHooks.useSyncIANARegistrars).mockReturnValue({
        mutateAsync: vi.fn(),
        isPending: false,
      } as any);

      render(<IANARegistrarsTab />, { wrapper: createWrapper() });

      // Look for the "-" in the table cells
      const cells = screen.getAllByRole('cell');
      const rdapCell = cells.find(cell => cell.textContent === '-');
      expect(rdapCell).toBeInTheDocument();
    });
  });
});
