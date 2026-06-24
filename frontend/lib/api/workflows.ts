/**
 * Workflows API Client
 * Endpoints for triggering backend workflows
 */

import { apiClient } from './client';

// =============================================================================
// Existing Types
// =============================================================================

export interface WorkflowStartResponse {
  workflowId: string;
  runId: string;
  status: string; // e.g., "started"
  url?: string;   // deep link to Temporal UI for this run (if provided by backend)
}

// =============================================================================
// Workflow Registry Types
// =============================================================================

export interface WorkflowStepMeta {
  key: string;
  label: string;
}

export interface WorkflowMeta {
  key: string;
  name: string;
  description: string;
  queue: string;
  category: string;
  tags: string[];
  hasSignal: boolean;
  signalName?: string;
  scheduled: boolean;
  scheduleInfo?: string;
  steps: WorkflowStepMeta[];
  docMarkdown?: string;
}

export interface WorkflowRegistryResponse {
  items: WorkflowMeta[];
  count: number;
}

export interface LaunchWorkflowResponse {
  workflowId: string;
  runId: string;
  status: string;
  url: string;
  steps: WorkflowStepMeta[];
}

export interface ActiveWorkflowItem {
  workflowId: string;
  runId: string;
  workflowType: string;
  status: string;
  startTime: string;
  url: string;
}

export interface ActiveWorkflowsResponse {
  items: ActiveWorkflowItem[];
  count: number;
}

export interface WorkflowStatusResponse {
  workflowId: string;
  runId: string;
  status: string;
  startTime?: string;
  closeTime?: string;
  url: string;
}

export interface WorkflowResultResponse {
  status: string;
  result?: Record<string, any> | null;
  error?: string;
  state?: Record<string, any> | null;
}

export interface DownloadURLResponse {
  url: string;
}

// =============================================================================
// Existing Endpoints
// =============================================================================

/**
 * Start the registrar sync workflow
 * POST /workflows/registrars/sync
 */
export async function startRegistrarSyncWorkflow(batchSize?: number): Promise<WorkflowStartResponse> {
  const payload = typeof batchSize === 'number' ? { batchSize } : undefined;
  const { data } = await apiClient.post('/workflows/registrars/sync', payload);
  return data as WorkflowStartResponse;
}

// =============================================================================
// Workflow Launchpad Endpoints
// =============================================================================

/**
 * Fetch the workflow registry (all available workflow types)
 * GET /workflows/registry
 */
export async function getWorkflowRegistry(): Promise<WorkflowRegistryResponse> {
  const { data } = await apiClient.get('/workflows/registry');
  return data;
}

/**
 * Launch a workflow by type with optional parameters
 * POST /workflows/launch
 */
export async function launchWorkflow(
  workflowType: string,
  params?: Record<string, any>
): Promise<LaunchWorkflowResponse> {
  const { data } = await apiClient.post('/workflows/launch', { workflowType, params });
  return data;
}

/**
 * Get all currently active (running) workflows
 * GET /workflows/active
 */
export async function getActiveWorkflows(): Promise<ActiveWorkflowsResponse> {
  const { data } = await apiClient.get('/workflows/active');
  return data;
}

/**
 * Get the current status of a specific workflow
 * GET /workflows/:workflowId/status
 */
export async function getWorkflowStatus(workflowId: string): Promise<WorkflowStatusResponse> {
  const { data } = await apiClient.get(`/workflows/${workflowId}/status`);
  return data;
}

/**
 * Send a signal to a running workflow (e.g., HITL approve/reject)
 * POST /workflows/:workflowId/signal
 */
export async function signalWorkflow(
  workflowId: string,
  signalName: string,
  payload?: any
): Promise<void> {
  await apiClient.post(`/workflows/${workflowId}/signal`, { signalName, payload });
}

/**
 * Get the result of a completed workflow
 * GET /workflows/:workflowId/result
 */
export async function getWorkflowResult(workflowId: string): Promise<WorkflowResultResponse> {
  const { data } = await apiClient.get(`/workflows/${workflowId}/result`);
  return data;
}

/**
 * Get a presigned download URL for an S3 storage key
 * GET /workflows/storage/download?key=...
 */
export async function getStorageDownloadURL(key: string): Promise<DownloadURLResponse> {
  const { data } = await apiClient.get('/workflows/storage/download', { params: { key } });
  return data;
}
