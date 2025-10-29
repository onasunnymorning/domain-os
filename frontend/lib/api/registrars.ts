/**
 * Registrar API Client
 * Functions for interacting with IANA and System Registrar endpoints
 */

import { apiClient } from './client';
import {
  IANARegistrar,
  IANARegistrarListParams,
  IANARegistrarListResponse,
  IANARegistrarCountResponse,
  Registrar,
  RegistrarListItem,
  RegistrarListParams,
  RegistrarListResponse,
  RegistrarCountResponse,
  SyncResult,
} from "@/lib/types/registrar";

// =============================================================================
// IANA Registrar API Functions
// =============================================================================

/**
 * List IANA Registrars
 * GET /ianaregistrars
 */
export async function getIANARegistrars(
  params?: IANARegistrarListParams
): Promise<IANARegistrarListResponse> {
  const { data } = await apiClient.get('/ianaregistrars', { params });
  return data;
}

/**
 * Get IANA Registrar by GurID
 * GET /ianaregistrars/:gurID
 */
export async function getIANARegistrarByGurID(gurID: number): Promise<IANARegistrar> {
  const { data } = await apiClient.get(`/ianaregistrars/${gurID}`);
  return data;
}

/**
 * Count IANA Registrars
 * GET /ianaregistrars/count
 */
export async function getIANARegistrarCount(): Promise<IANARegistrarCountResponse> {
  const { data } = await apiClient.get('/ianaregistrars/count');
  return data;
}

/**
 * Sync IANA Registrars from IANA XML repository
 * PUT /sync/iana-registrars
 * 
 * Note: This is a slow operation that downloads and parses XML data
 */
export async function syncIANARegistrars(): Promise<SyncResult> {
  const { data } = await apiClient.put('/sync/iana-registrars');
  return data;
}

// =============================================================================
// System Registrar API Functions
// =============================================================================

/**
 * List System Registrars
 * GET /registrars
 */
export async function getRegistrars(
  params?: RegistrarListParams
): Promise<RegistrarListResponse> {
  const { data } = await apiClient.get('/registrars', { params });
  return data;
}

/**
 * Get System Registrar by ClID
 * GET /registrars/:clid
 */
export async function getRegistrarByClID(clid: string): Promise<Registrar> {
  const { data } = await apiClient.get(`/registrars/${clid}`);
  return data;
}

/**
 * Get System Registrar by GurID
 * GET /registrars/gurid/:gurid
 */
export async function getRegistrarByGurID(gurid: number): Promise<Registrar> {
  const { data } = await apiClient.get(`/registrars/gurid/${gurid}`);
  return data;
}

/**
 * Count System Registrars
 * GET /registrars/count
 */
export async function getRegistrarCount(): Promise<RegistrarCountResponse> {
  const { data } = await apiClient.get('/registrars/count');
  return data;
}

/**
 * Create System Registrar
 * POST /registrars
 */
export async function createRegistrar(registrar: Partial<Registrar>): Promise<Registrar> {
  const { data } = await apiClient.post('/registrars', registrar);
  return data;
}

/**
 * Update System Registrar
 * PUT /registrars/:clid
 */
export async function updateRegistrar(
  clid: string,
  registrar: Partial<Registrar>
): Promise<Registrar> {
  // Ensure ClID in body matches the path parameter for backends that require it
  const incoming = (registrar as any) || {};
  const body = { ...incoming, ClID: clid };
  const { data } = await apiClient.put(`/registrars/${clid}`, body);
  return data;
}

/**
 * Update System Registrar Status
 * PUT /registrars/:clid/status/:status
 */
export async function updateRegistrarStatus(
  clid: string,
  status: string
): Promise<Registrar> {
  const { data } = await apiClient.put(`/registrars/${clid}/status/${status}`);
  return data;
}

/**
 * Delete System Registrar
 * DELETE /registrars/:clid
 */
export async function deleteRegistrar(clid: string): Promise<void> {
  await apiClient.delete(`/registrars/${clid}`);
}

/**
 * Bulk Create System Registrars
 * POST /registrars/bulk
 */
export async function bulkCreateRegistrars(
  registrars: Partial<Registrar>[]
): Promise<Registrar[]> {
  const { data } = await apiClient.post('/registrars/bulk', registrars);
  return data;
}
