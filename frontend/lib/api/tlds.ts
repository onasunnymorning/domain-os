import { apiClient } from './client';

export interface TLD {
  Name: string;
  Type: 'generic' | 'country-code' | 'second-level';
  UName: string;
  RyID: string;
  AllowEscrowImport: boolean;
  EnableDNS: boolean;
  Phases: any[]; // Will type properly in Phase 2
  CreatedAt: string;
  UpdatedAt: string;
}

export interface CreateTLDRequest {
  Name: string;
  RyID: string;
}

export interface ListQueryParams {
  pagesize?: number;
  cursor?: string;
  name_like?: string;
  type_equals?: 'generic' | 'country-code' | 'second-level';
  ryid_equals?: string;
}

export interface TLDListResponse {
  Data: TLD[];
  Meta?: {
    Cursor?: string;
    Count?: number;
    PageSize?: number;
    Filter?: any;
  };
}

export interface TLDCountResponse {
  ObjectType: string;
  Count: number;
  Timestamp: string;
  Filter?: any;
}

export const tldsApi = {
  list: async (params?: ListQueryParams): Promise<TLDListResponse> => {
    const { data } = await apiClient.get('/tlds', { params });
    return data;
  },
  
  count: async (params?: ListQueryParams): Promise<TLDCountResponse> => {
    const { data } = await apiClient.get('/tlds/count', { params });
    return data;
  },
  
  get: async (name: string): Promise<TLD> => {
    const { data } = await apiClient.get(`/tlds/${name}`);
    return data;
  },
  
  create: async (tld: CreateTLDRequest) => {
    const { data } = await apiClient.post('/tlds', tld);
    return data;
  },

  // Note: Backend doesn't expose a generic PUT /tlds/{name} endpoint.
  // For now, we support toggling AllowEscrowImport via status endpoints
  // and then refetch the TLD to return the latest state.
  update: async (name: string, body: Partial<Pick<TLD, 'AllowEscrowImport' | 'EnableDNS' | 'RyID' | 'UName'>>) => {
    const encoded = encodeURIComponent(name);

    // Handle AllowEscrowImport toggle via status endpoints
    if (typeof body.AllowEscrowImport === 'boolean') {
      const statusPath = `/tlds/${encoded}/status/AllowEscrowImport`;
      if (body.AllowEscrowImport) {
        await apiClient.post(statusPath);
      } else {
        await apiClient.delete(statusPath);
      }
    }

    // Currently there is no REST endpoint to update EnableDNS/RyID/UName.
    // We ignore these fields for now to avoid sending unsupported requests.
    // When backend adds endpoints, wire them here.

    // Always return the latest TLD after any operation above
    const { data } = await apiClient.get(`/tlds/${encoded}`);
    return data as TLD;
  },
  
  delete: async (name: string) => {
    const { data } = await apiClient.delete(`/tlds/${name}`);
    return data;
  },
};
