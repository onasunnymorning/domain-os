import { apiClient } from './client';
import type { DomainListItem } from '@/lib/types/domain';
import type { TLD } from './tlds';
import type { RegistrarListItem } from '@/lib/types/registrar';
import type { NNDN } from './nndns';
import type { RegistryOperator } from './types';

export interface SearchResults {
  domains: DomainListItem[];
  tlds: TLD[];
  registrars: RegistrarListItem[];
  nndns: NNDN[];
  registryOperators: RegistryOperator[];
}

/**
 * Search across all entity types in parallel.
 * Uses the existing `_like` filter params on each list endpoint,
 * capped at 5 results per category for speed.
 *
 * Uses Promise.allSettled so a failure in one category
 * doesn't block results from others.
 */
export async function searchAll(query: string): Promise<SearchResults> {
  const pagesize = 5;

  const [
    domainsResult,
    tldsResult,
    registrarsResult,
    nndnsResult,
    registryOperatorsResult,
  ] = await Promise.allSettled([
    apiClient.get('/domains', { params: { name_like: query, pagesize } }),
    apiClient.get('/tlds', { params: { name_like: query, pagesize } }),
    apiClient.get('/registrars', { params: { name_like: query, pagesize } }),
    apiClient.get('/nndns', { params: { name_like: query, pagesize } }),
    apiClient.get('/registry-operators', { params: { name_like: query, pagesize } }),
  ]);

  return {
    domains:
      domainsResult.status === 'fulfilled'
        ? (domainsResult.value.data?.Data ?? [])
        : [],
    tlds:
      tldsResult.status === 'fulfilled'
        ? (tldsResult.value.data?.Data ?? [])
        : [],
    registrars:
      registrarsResult.status === 'fulfilled'
        ? (registrarsResult.value.data?.Data ?? [])
        : [],
    nndns:
      nndnsResult.status === 'fulfilled'
        ? (nndnsResult.value.data?.Data ?? [])
        : [],
    registryOperators:
      registryOperatorsResult.status === 'fulfilled'
        ? (registryOperatorsResult.value.data?.Data ?? [])
        : [],
  };
}
