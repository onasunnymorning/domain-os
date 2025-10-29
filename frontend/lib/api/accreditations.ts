import { apiClient } from './client';
import type { RegistrarListResponse, RegistrarListParams } from '@/lib/types/registrar';
import type { TLDListResponse } from './tlds';

export interface RegistrarAccreditationsParams {
  pagesize?: number;
  cursor?: string;
}

export const accreditationsApi = {
  // List TLDs a registrar is accredited for
  listByRegistrar: async (
    rarClID: string,
    params?: RegistrarAccreditationsParams
  ): Promise<TLDListResponse> => {
    const { data } = await apiClient.get(`/accreditations/registrar/${encodeURIComponent(rarClID)}`, { params });
    return data;
  },
  // List registrars accredited for a TLD
  listRegistrarsByTLD: async (
    tldName: string,
    params?: RegistrarListParams
  ): Promise<RegistrarListResponse> => {
    const { data } = await apiClient.get(`/accreditations/tld/${encodeURIComponent(tldName)}`, { params });
    return data;
  },
  // Accredit registrar for a TLD
  accredit: async (tldName: string, rarClID: string): Promise<void> => {
    await apiClient.post(`/accreditations/${encodeURIComponent(tldName)}/${encodeURIComponent(rarClID)}`);
  },
  // Deaccredit registrar for a TLD
  deaccredit: async (tldName: string, rarClID: string): Promise<void> => {
    await apiClient.delete(`/accreditations/${encodeURIComponent(tldName)}/${encodeURIComponent(rarClID)}`);
  },
};
