import { apiClient } from './client';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface DomainTombstone {
  roid: string;
  name: string;
  uname?: string;
  tld_name: string;
  registrar_clid: string;
  registered_at: string;
  expired_at?: string;
  purged_at: string;
  purge_reason: string;
  drop_catch: boolean;
  last_snapshot?: any;
  created_at: string;
}

export interface ListTombstonesParams {
  pagesize?: number;
  cursor?: string;
  name?: string;
  name_like?: string;
  tld?: string;
  registrar?: string;
  purge_reason?: string;
}

// ---------------------------------------------------------------------------
// API functions
// ---------------------------------------------------------------------------

/**
 * Get a single tombstone by its ROID (primary key).
 */
export async function getTombstoneByRoID(roid: string): Promise<DomainTombstone> {
  const { data } = await apiClient.get(`/tombstones/${encodeURIComponent(roid)}`);
  return data;
}

/**
 * Get all tombstones for a given domain name (multiple incarnations),
 * ordered by purged_at DESC (most recent first).
 */
export async function getTombstonesByName(name: string): Promise<DomainTombstone[]> {
  const { data } = await apiClient.get(`/tombstones/by-name/${encodeURIComponent(name)}`);
  return data;
}

/**
 * List tombstones with pagination and optional filters.
 */
export async function listTombstones(params: ListTombstonesParams = {}): Promise<{
  items: DomainTombstone[];
  cursor: string;
}> {
  const { data } = await apiClient.get('/tombstones', { params });
  return data;
}

/**
 * Count tombstones matching optional filters.
 */
export async function countTombstones(params: Omit<ListTombstonesParams, 'pagesize' | 'cursor'> = {}): Promise<number> {
  const { data } = await apiClient.get('/tombstones/count', { params });
  return data.count;
}
