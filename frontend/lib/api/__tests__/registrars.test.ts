/**
 * Tests for Registrar API Client Functions
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { apiClient } from '../client';
import {
  getIANARegistrars,
  getIANARegistrarByGurID,
  getIANARegistrarCount,
  syncIANARegistrars,
  getRegistrars,
  getRegistrarByClID,
  getRegistrarByGurID,
  getRegistrarCount,
  createRegistrar,
  updateRegistrar,
  updateRegistrarStatus,
  deleteRegistrar,
  bulkCreateRegistrars,
} from '../registrars';
import { IANARegistrarStatus, RegistrarStatus } from '@/lib/types/registrar';

// Mock the API client
vi.mock('../client', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('IANA Registrar API Functions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getIANARegistrars', () => {
    it('should fetch IANA registrars without parameters', async () => {
      const mockResponse = {
        Data: [
          {
            GurID: 1,
            Name: 'Example Registrar',
            Status: IANARegistrarStatus.Accredited,
            RdapURL: 'https://rdap.example.com',
            CreatedAt: '2024-01-01T00:00:00Z',
          },
        ],
        Meta: {
          Cursor: '1',
          Count: 1,
          PageSize: 50,
        },
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockResponse });

      const result = await getIANARegistrars();

      expect(apiClient.get).toHaveBeenCalledWith('/ianaregistrars', { params: undefined });
      expect(result).toEqual(mockResponse);
    });

    it('should fetch IANA registrars with filter parameters', async () => {
      const params = {
        pagesize: 10,
        cursor: 'abc123',
        name_like: 'example',
        status: IANARegistrarStatus.Accredited,
      };

      const mockResponse = {
        Data: [],
        Meta: { Cursor: '', Count: 0, PageSize: 10 },
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockResponse });

      const result = await getIANARegistrars(params);

      expect(apiClient.get).toHaveBeenCalledWith('/ianaregistrars', { params });
      expect(result).toEqual(mockResponse);
    });
  });

  describe('getIANARegistrarByGurID', () => {
    it('should fetch a single IANA registrar by GurID', async () => {
      const mockRegistrar = {
        GurID: 123,
        Name: 'Test Registrar',
        Status: IANARegistrarStatus.Accredited,
        RdapURL: 'https://rdap.test.com',
        CreatedAt: '2024-01-01T00:00:00Z',
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockRegistrar });

      const result = await getIANARegistrarByGurID(123);

      expect(apiClient.get).toHaveBeenCalledWith('/ianaregistrars/123');
      expect(result).toEqual(mockRegistrar);
    });
  });

  describe('getIANARegistrarCount', () => {
    it('should fetch the count of IANA registrars', async () => {
      const mockCount = {
        ObjectType: 'IANARegistrar',
        Count: 3500,
        Timestamp: '2024-01-01T00:00:00Z',
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockCount });

      const result = await getIANARegistrarCount();

      expect(apiClient.get).toHaveBeenCalledWith('/ianaregistrars/count');
      expect(result).toEqual(mockCount);
    });
  });

  describe('syncIANARegistrars', () => {
    it('should trigger IANA registrar sync', async () => {
      const mockResult = {
        message: 'Sync completed successfully',
        success: true,
      };

      vi.mocked(apiClient.put).mockResolvedValue({ data: mockResult });

      const result = await syncIANARegistrars();

      expect(apiClient.put).toHaveBeenCalledWith('/sync/iana-registrars');
      expect(result).toEqual(mockResult);
    });
  });
});

describe('System Registrar API Functions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getRegistrars', () => {
    it('should fetch system registrars', async () => {
      const mockResponse = {
        Data: [
          {
            ClID: 'test-registrar',
            Name: 'Test Registrar Inc',
            GurID: 123,
            Status: RegistrarStatus.OK,
            Autorenew: true,
          },
        ],
        Meta: {
          Cursor: '1',
          Count: 1,
          PageSize: 50,
        },
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockResponse });

      const result = await getRegistrars();

      expect(apiClient.get).toHaveBeenCalledWith('/registrars', { params: undefined });
      expect(result).toEqual(mockResponse);
    });

    it('should fetch system registrars with pagination', async () => {
      const params = {
        pagesize: 20,
        cursor: 'xyz789',
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: { Data: [], Meta: {} } });

      await getRegistrars(params);

      expect(apiClient.get).toHaveBeenCalledWith('/registrars', { params });
    });
  });

  describe('getRegistrarByClID', () => {
    it('should fetch a registrar by ClID', async () => {
      const mockRegistrar = {
        ClID: 'test-123',
        Name: 'Test Registrar',
        GurID: 456,
        Status: RegistrarStatus.OK,
        Autorenew: false,
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockRegistrar });

      const result = await getRegistrarByClID('test-123');

      expect(apiClient.get).toHaveBeenCalledWith('/registrars/test-123');
      expect(result).toEqual(mockRegistrar);
    });
  });

  describe('getRegistrarByGurID', () => {
    it('should fetch a registrar by GurID', async () => {
      const mockRegistrar = {
        ClID: 'test-456',
        Name: 'Another Registrar',
        GurID: 789,
        Status: RegistrarStatus.Readonly,
        Autorenew: true,
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockRegistrar });

      const result = await getRegistrarByGurID(789);

      expect(apiClient.get).toHaveBeenCalledWith('/registrars/gurid/789');
      expect(result).toEqual(mockRegistrar);
    });
  });

  describe('getRegistrarCount', () => {
    it('should fetch the count of system registrars', async () => {
      const mockCount = {
        ObjectType: 'Registrar',
        Count: 50,
        Timestamp: '2024-01-01T00:00:00Z',
      };

      vi.mocked(apiClient.get).mockResolvedValue({ data: mockCount });

      const result = await getRegistrarCount();

      expect(apiClient.get).toHaveBeenCalledWith('/registrars/count');
      expect(result).toEqual(mockCount);
    });
  });

  describe('createRegistrar', () => {
    it('should create a new registrar', async () => {
      const newRegistrar = {
        ClID: 'new-registrar',
        Name: 'New Registrar Inc',
        GurID: 999,
      };

      const mockResponse = {
        ...newRegistrar,
        Status: RegistrarStatus.Readonly,
        Autorenew: false,
      };

      vi.mocked(apiClient.post).mockResolvedValue({ data: mockResponse });

      const result = await createRegistrar(newRegistrar);

      expect(apiClient.post).toHaveBeenCalledWith('/registrars', newRegistrar);
      expect(result).toEqual(mockResponse);
    });
  });

  describe('updateRegistrar', () => {
    it('should update an existing registrar', async () => {
      const updates = {
        Name: 'Updated Name',
        Autorenew: true,
      };

      const mockResponse = {
        ClID: 'test-123',
        ...updates,
        GurID: 456,
        Status: RegistrarStatus.OK,
      };

      vi.mocked(apiClient.put).mockResolvedValue({ data: mockResponse });

      const result = await updateRegistrar('test-123', updates);

      expect(apiClient.put).toHaveBeenCalledWith('/registrars/test-123', updates);
      expect(result).toEqual(mockResponse);
    });
  });

  describe('updateRegistrarStatus', () => {
    it('should update registrar status', async () => {
      const mockResponse = {
        ClID: 'test-123',
        Name: 'Test Registrar',
        GurID: 456,
        Status: RegistrarStatus.Terminated,
        Autorenew: false,
      };

      vi.mocked(apiClient.put).mockResolvedValue({ data: mockResponse });

      const result = await updateRegistrarStatus('test-123', 'terminated');

      expect(apiClient.put).toHaveBeenCalledWith('/registrars/test-123/status/terminated');
      expect(result).toEqual(mockResponse);
    });
  });

  describe('deleteRegistrar', () => {
    it('should delete a registrar', async () => {
      vi.mocked(apiClient.delete).mockResolvedValue({ data: null });

      await deleteRegistrar('test-123');

      expect(apiClient.delete).toHaveBeenCalledWith('/registrars/test-123');
    });
  });

  describe('bulkCreateRegistrars', () => {
    it('should bulk create registrars', async () => {
      const registrars = [
        { ClID: 'bulk-1', Name: 'Bulk Registrar 1', GurID: 111 },
        { ClID: 'bulk-2', Name: 'Bulk Registrar 2', GurID: 222 },
      ];

      const mockResponse = registrars.map(r => ({
        ...r,
        Status: RegistrarStatus.Readonly,
        Autorenew: false,
      }));

      vi.mocked(apiClient.post).mockResolvedValue({ data: mockResponse });

      const result = await bulkCreateRegistrars(registrars);

      expect(apiClient.post).toHaveBeenCalledWith('/registrars/bulk', registrars);
      expect(result).toEqual(mockResponse);
      expect(result).toHaveLength(2);
    });
  });
});
