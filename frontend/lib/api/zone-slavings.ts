import { apiClient } from './client';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CreateSlavingRequest {
  zone: string;
  masterNS: string[];
  slaveNS: string[];
  checkIntervalSeconds?: number;
  stalledAfterN?: number;
  confidenceN?: number;
  graceMultiplier?: number;
}

export interface ZoneSlavingRecord {
  id: string;
  tenantId: string;
  zone: string;
  masterNS: string[];
  slaveNS: string[];
  status: string;
  checkIntervalS: number;
  stalledAfterN: number;
  confidenceN: number;
  graceMultiplier: number;
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// API functions
// ---------------------------------------------------------------------------

/**
 * Create a zone slaving monitor and start its Temporal schedule.
 * POST /zone-slavings
 */
export async function createSlaving(
  tenantId: string,
  req: CreateSlavingRequest
): Promise<ZoneSlavingRecord> {
  const { data } = await apiClient.post('/zone-slavings', req, {
    headers: { 'X-Tenant-ID': tenantId },
  });
  return data;
}

/**
 * List active slaving monitors for a tenant.
 * GET /zone-slavings
 */
export async function listActiveSlavings(
  tenantId: string
): Promise<ZoneSlavingRecord[]> {
  const { data } = await apiClient.get('/zone-slavings', {
    headers: { 'X-Tenant-ID': tenantId },
  });
  return data;
}

/**
 * Abandon (stop) a slaving monitor and delete its schedule.
 * PATCH /zone-slavings/:id
 */
export async function abandonSlaving(
  tenantId: string,
  id: string
): Promise<void> {
  await apiClient.patch(`/zone-slavings/${id}`, { action: 'abandon' }, {
    headers: { 'X-Tenant-ID': tenantId },
  });
}
