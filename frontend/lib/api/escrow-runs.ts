/**
 * Escrow Validation & Sanitization API Client
 *
 * These endpoints are tenant-scoped: the operator scope is never a body
 * parameter, it is the `X-Tenant-ID` header, and the server derives everything
 * else from it (ADR-0006). Every call here therefore takes `tenantId`.
 */

import { apiClient } from './client';
import type { WorkflowStartResponse } from './workflows';

// =============================================================================
// Types
// =============================================================================

/** Artifact shapes accepted by the validation workflow. */
export type EscrowProfile = 'ryde+sig' | 'xml';

export interface EscrowFinding {
  code: string;
  severity: string;
  stage: string;
  message: string;
  locator?: string;
}

export interface EscrowValidationRun {
  id: string;
  depositId: string;
  tld: string;
  workflowId: string;
  runId?: string;
  profile: string;
  outcome: string;
  verified: boolean;
  stageReached?: string;
  findings: EscrowFinding[];
  plaintextSha256?: string;
  rdeDepositId?: string;
  rdeKind?: string;
  rdeResend: number;
  rdeWatermark?: string;
  reportObjectKey?: string;
  notificationObjectKey?: string;
  notificationStatus?: string;
  startedAt: string;
  completedAt?: string;
}

export interface EscrowSanitizationCounts {
  objectsByType?: Record<string, number>;
  kept?: number;
  dropped?: number;
  synthesized?: number;
  tokenized?: number;
  rewritten?: number;
}

export interface EscrowSanitizationRun {
  id: string;
  tld: string;
  tenantId: string;
  /** Always "sanitized-pseudonymized" — the derivative is not anonymous. */
  label: string;
  sourceValidationRunId: string;
  sourceDepositId: string;
  sourceArtifactSha256: string;
  policyVersion: string;
  workflowVersion: string;
  syntheticSuffix: string;
  workflowId: string;
  runId?: string;
  outcome: string;
  stageReached?: string;
  findings: EscrowFinding[];
  derivativeObjectKey?: string;
  derivativeSha256?: string;
  derivativeBytes?: number;
  manifestObjectKey?: string;
  counts: EscrowSanitizationCounts;
  startedAt: string;
  completedAt?: string;
}

export interface EscrowListResponse<T> {
  items: T[];
  count: number;
  nextCursor?: string;
}

export interface StartEscrowValidationRequest {
  tld: string;
  /** Defaults to "ryde+sig" server-side. */
  profile?: EscrowProfile;
  artifactObjectKey: string;
  /** Required for "ryde+sig", must be omitted otherwise. */
  signatureObjectKey?: string;
  intakeRef?: string;
}

export interface StartEscrowSanitizationRequest {
  sourceValidationRunId: string;
  /** Overrides the configured default synthetic suffix for this run. */
  syntheticSuffix?: string;
}

// =============================================================================
// Endpoints
// =============================================================================

const scoped = (tenantId: string) => ({ headers: { 'X-Tenant-ID': tenantId } });

/**
 * Launch an escrow validation (EVE) run.
 * POST /escrow/validations
 */
export async function startEscrowValidation(
  tenantId: string,
  body: StartEscrowValidationRequest
): Promise<WorkflowStartResponse> {
  const { data } = await apiClient.post('/escrow/validations', body, scoped(tenantId));
  return data as WorkflowStartResponse;
}

/**
 * List validation runs in the caller's scope, newest first.
 * GET /escrow/validations
 */
export async function listEscrowValidations(
  tenantId: string,
  params?: { tld?: string; outcome?: string; pagesize?: number; cursor?: string }
): Promise<EscrowListResponse<EscrowValidationRun>> {
  const { data } = await apiClient.get('/escrow/validations', { ...scoped(tenantId), params });
  return data;
}

/**
 * Launch a sanitized-pseudonymized derivative run against an accepted validation.
 * POST /escrow/sanitizations
 */
export async function startEscrowSanitization(
  tenantId: string,
  body: StartEscrowSanitizationRequest
): Promise<WorkflowStartResponse> {
  const { data } = await apiClient.post('/escrow/sanitizations', body, scoped(tenantId));
  return data as WorkflowStartResponse;
}

/**
 * List derivative runs in the caller's scope, newest first.
 * GET /escrow/sanitizations
 */
export async function listEscrowSanitizations(
  tenantId: string,
  params?: {
    tld?: string;
    outcome?: string;
    sourceValidationRunId?: string;
    pagesize?: number;
    cursor?: string;
  }
): Promise<EscrowListResponse<EscrowSanitizationRun>> {
  const { data } = await apiClient.get('/escrow/sanitizations', { ...scoped(tenantId), params });
  return data;
}
