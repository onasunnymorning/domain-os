import { apiClient } from './client';
import { DomainListParams, DomainListResponse, DomainCountResponse, DomainListItem, DomainCreateRequest, DomainDetail, DomainEvent } from '@/lib/types/domain';

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

export async function updateDomain(name: string, payload: any): Promise<DomainDetail> {
  const { data } = await apiClient.put(`/domains/${encodeURIComponent(name)}`, payload);
  return data;
}

export async function setDropCatch(name: string): Promise<void> {
  await apiClient.post(`/domains/${encodeURIComponent(name)}/dropcatch`);
}

export async function unsetDropCatch(name: string): Promise<void> {
  await apiClient.delete(`/domains/${encodeURIComponent(name)}/dropcatch`);
}

export async function getQuote(payload: import('@/lib/types/domain').QuoteRequest): Promise<import('@/lib/types/domain').Quote> {
  const { data } = await apiClient.post('/domains/quote', payload);
  return data;
}

export async function setDomainStatus(name: string, status: string): Promise<DomainDetail> {
  const { data } = await apiClient.post(`/domains/${encodeURIComponent(name)}/status/${encodeURIComponent(status)}`);
  return data;
}

export async function unsetDomainStatus(name: string, status: string): Promise<DomainDetail> {
  const { data } = await apiClient.delete(`/domains/${encodeURIComponent(name)}/status/${encodeURIComponent(status)}`);
  return data;
}

export async function getDomainEvents(name: string): Promise<DomainEvent[]> {
  const { data } = await apiClient.get(`/domains/${encodeURIComponent(name)}/events`);
  return data;
}


