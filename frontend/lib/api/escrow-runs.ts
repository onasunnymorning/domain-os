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
  /** The specific check behind the code, where the code alone is too coarse. */
  rule?: string;
  message: string;
  /** The kind of object the finding is about: domain, contact, host, ... */
  objectType?: string;
  /**
   * The identifier of the object itself, as the deposit wrote it — a domain
   * or host name, a contact or registrar id. `locator` is exact but only
   * usable with the deposit open; this is what an operator can act on.
   * Absent on runs recorded before #423, and on findings about no one object.
   */
  object?: string;
  locator?: string;
}

/**
 * One kind of finding with its exact count. `findings` on a run keeps worked
 * examples only; this does not, so it is the only place a run says how many
 * times a check actually fired.
 *
 * Rows are keyed by rule and object type as well as by code: "56,926 objects
 * were rejected" is not something an operator can act on, and "43,313 hosts
 * were rejected because their roid is not in this registry's format" is.
 */
export interface EscrowFindingTally {
  code: string;
  severity: string;
  stage: string;
  objectType?: string;
  rule?: string;
  count: number;
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
  /** Exact where `findings` is truncated. Empty on runs recorded before #420. */
  findingTally: EscrowFindingTally[];
  plaintextSha256?: string;
  rdeDepositId?: string;
  rdeKind?: string;
  rdeResend: number;
  rdeWatermark?: string;
  signingKeyFingerprint?: string;
  decryptionKeyFingerprint?: string;
  /** Written for every run that produced a result, an ERROR one included. */
  summaryObjectKey?: string;
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

export interface EscrowDeposit {
  id: string;
  tld: string;
  profile: string;
  receivedAt: string;
  submittedBy?: string;
  intakeRef?: string;
  artifactObjectKey: string;
  artifactSha256: string;
  artifactBytes: number;
  signatureObjectKey?: string;
  signatureSha256?: string;
  signatureBytes?: number;
  createdAt: string;
}

/** GET /escrow/validations/:id returns the run with the deposit it bound. */
export interface EscrowValidationRunDetail {
  run: EscrowValidationRun;
  deposit?: EscrowDeposit;
}

/**
 * The findings summary written to the reports bucket as `summary.json`.
 *
 * It is the only artifact carrying the findings, and its `byCode` tally is the
 * only exact account of a deposit that is wrong in more places than the run
 * record can hold (`retained` stops at 1000).
 */
export interface EscrowValidationSummary {
  schemaVersion: string;
  tenantId: string;
  tld: string;
  profile: string;
  depositId: string;
  validationRunId: string;
  correlationId: string;
  traceId?: string;
  outcome: string;
  stageReached: string;
  verified: boolean;
  notificationStatus?: string;
  receivedAt: string;
  startedAt: string;
  validatedAt: string;
  completedAt: string;
  deposit: {
    id?: string;
    prevId?: string;
    kind?: string;
    resend: number;
    watermark?: string;
    headerFound: boolean;
    layout?: string;
    counts: Array<{ uri: string; declared?: number; observed: number; matches: boolean }>;
  };
  digests: { artifactSha256?: string; signatureSha256?: string; plaintextSha256?: string };
  keys: { signingFingerprint?: string; decryptionFingerprint?: string; innerSigned?: boolean };
  findings: {
    total: number;
    retained: number;
    suppressed: number;
    bySeverity: Record<string, number>;
    byStage: Record<string, number>;
    byCode: Array<{
      code: string;
      severity: string;
      stage: string;
      count: number;
      errorClass?: boolean;
    }>;
    sample?: EscrowFinding[];
  };
  artifacts?: Record<string, string>;
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
 * Fetch one validation run with the deposit it bound.
 * GET /escrow/validations/:id
 */
export async function getEscrowValidation(
  tenantId: string,
  id: string
): Promise<EscrowValidationRunDetail> {
  const { data } = await apiClient.get(`/escrow/validations/${id}`, scoped(tenantId));
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
