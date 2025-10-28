/**
 * Workflows API Client
 * Endpoints for triggering backend workflows
 */

import { apiClient } from './client';

export interface WorkflowStartResponse {
  workflowId: string;
  runId: string;
  status: string; // e.g., "started"
}

/**
 * Start the registrar sync workflow
 * POST /workflows/registrars/sync
 */
export async function startRegistrarSyncWorkflow(batchSize?: number): Promise<WorkflowStartResponse> {
  const payload = typeof batchSize === 'number' ? { batchSize } : undefined;
  const { data } = await apiClient.post('/workflows/registrars/sync', payload);
  return data as WorkflowStartResponse;
}
