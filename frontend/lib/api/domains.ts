import { apiClient } from './client';
import { DomainListParams, DomainListResponse, DomainCountResponse, DomainListItem, DomainCreateRequest, DomainDetail } from '@/lib/types/domain';

export async function getDomains(params?: DomainListParams): Promise<DomainListResponse> {
  const { data } = await apiClient.get('/domains', { params });
  return data;
}

export async function getDomainByName(name: string): Promise<DomainDetail> {
  const { data } = await apiClient.get(`/domains/${encodeURIComponent(name)}`);
  return data;
}

export async function getDomainCount(params?: DomainListParams): Promise<DomainCountResponse> {
  const { data } = await apiClient.get('/domains/count', { params });
  return data;
}

export async function createDomain(payload: DomainCreateRequest): Promise<DomainListItem> {
  const { data } = await apiClient.post('/domains', payload);
  return data;
}
export async function getDomainDNS(name: string): Promise<string[]> {
  const { data } = await apiClient.get(`/domains/${encodeURIComponent(name)}/dns`);
  return data;
}
