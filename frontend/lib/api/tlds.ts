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
  
  delete: async (name: string) => {
    const { data } = await apiClient.delete(`/tlds/${name}`);
    return data;
  },
};
