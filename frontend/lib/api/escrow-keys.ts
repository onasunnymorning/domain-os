/**
 * Escrow Key Registry API Client (issue #429, ADR-0009)
 *
 * Keys belong to parties, not TLDs: our own identities (e.g. EVE, the Data
 * Escrow Agent) and counterparties (the registry service providers that send
 * us deposits). A TLD only says which parties are on each side of its escrow,
 * through an arrangement that is inherited platform → operator → TLD.
 *
 * Scope. With an operator (`tenantId`) the calls send `X-Tenant-ID` and see
 * that operator's parties plus the platform's; without one they address the
 * platform registry, which the server only serves to principals holding the
 * `escrow:platform-keys:admin` permission. Every change also needs
 * `escrow:keys:admin` (operator) or `escrow:platform-keys:admin` (platform);
 * the server answers 403 otherwise, and `isPermissionError` recognises it.
 *
 * Private key material and passphrases are sent once, to the import endpoint,
 * and never returned by any call.
 */

import axios from 'axios';
import { apiClient } from './client';

// =============================================================================
// Types
// =============================================================================

export type EscrowPartyKind = 'RSP' | 'DEA';
export type EscrowPartySide = 'self' | 'external';
export type EscrowKeyPurpose = 'decrypt-inbound' | 'verify-inbound' | 'pseudonymise';
export type EscrowKeyMaterial = 'openpgp-private' | 'openpgp-public' | 'symmetric';
export type EscrowKeyState = 'STAGED' | 'ACTIVE' | 'HISTORICAL' | 'REVOKED' | 'DESTROYED';
export type EscrowArrangementLevel = 'platform' | 'operator' | 'tld';

/** Empty `tenantId` addresses the platform registry. */
export interface EscrowKeyScope {
  tenantId?: string;
}

export interface EscrowParty {
  id: string;
  owner: { kind: 'platform' | 'operator'; operator?: string };
  name: string;
  kind: EscrowPartyKind;
  side: EscrowPartySide;
  purposes: EscrowKeyPurpose[];
  /** Whether this scope may change the party (platform parties are read-only to operators). */
  manageable: boolean;
  createdAt: string;
  createdBy?: string;
}

export interface EscrowKeyVersion {
  id: string;
  partyId: string;
  owner: string;
  purpose: EscrowKeyPurpose;
  material: EscrowKeyMaterial;
  version: number;
  fingerprint: string;
  hasPublicKey: boolean;
  state: EscrowKeyState;
  notBefore?: string;
  notAfter?: string;
  keyExpiresAt?: string;
  lastProbeAt?: string;
  lastProbeOk: boolean;
  activatedAt?: string;
  deactivatedAt?: string;
  revokedAt?: string;
  revocationReason?: string;
  compromised: boolean;
  destroyedAt?: string;
  createdAt: string;
  createdBy?: string;
  probeWorkflowId?: string;
  /** Set when the key was accepted with a change, e.g. unreadable subkeys left out. */
  notice?: string;
}

export interface EscrowArrangement {
  id: string;
  level: EscrowArrangementLevel;
  operator?: string;
  tld?: string;
  depositorPartyId?: string;
  receiverPartyId?: string;
  revision: number;
  createdAt: string;
  createdBy?: string;
}

export interface EscrowArrangementSide {
  partyId: string;
  from: EscrowArrangementLevel;
}

export interface EffectiveEscrowArrangement {
  depositor?: EscrowArrangementSide;
  receiver?: EscrowArrangementSide;
  tldRevision: number;
  operatorRevision: number;
  platformRevision: number;
}

export interface EscrowKeyAuditEvent {
  id: string;
  action: string;
  subject: string;
  subjectId: string;
  purpose?: string;
  version?: number;
  fingerprint?: string;
  stateBefore?: string;
  stateAfter?: string;
  reason?: string;
  compromised?: boolean;
  actor?: string;
  correlationId?: string;
  at: string;
}

export interface EscrowPartyDetail {
  party: EscrowParty;
  versions: EscrowKeyVersion[];
  usedBy: EscrowArrangement[];
}

export interface EscrowKeyList<T> {
  items: T[];
  count: number;
  nextCursor?: string;
}

/** What each purpose means, for people. The server is the authority on rules. */
export const PURPOSE_LABELS: Record<EscrowKeyPurpose, { title: string; description: string }> = {
  'decrypt-inbound': {
    title: 'Decryption key',
    description: 'Registries encrypt their deposits to this key. Its private half stays in the key store.',
  },
  'verify-inbound': {
    title: 'Signing keys',
    description: 'The registry signs every deposit with one of these; we verify with the public keys.',
  },
  pseudonymise: {
    title: 'Pseudonymisation key',
    description:
      'Tokenises sanitized copies of deposits. Only one version is active: rotating it means new copies no longer join with older ones.',
  },
};

// =============================================================================
// Endpoints
// =============================================================================

const headers = (scope: EscrowKeyScope) =>
  scope.tenantId ? { headers: { 'X-Tenant-ID': scope.tenantId } } : {};

/** True when the server refused the call for want of a permission. */
export function isPermissionError(err: unknown): boolean {
  return axios.isAxiosError(err) && err.response?.status === 403;
}

/** The server's error text, or a fallback. Never contains submitted material. */
export function escrowKeyErrorMessage(err: unknown, fallback = 'The request failed'): string {
  if (axios.isAxiosError(err)) {
    const msg = (err.response?.data as { error?: string } | undefined)?.error;
    if (msg) return msg;
  }
  return fallback;
}

export async function listEscrowParties(scope: EscrowKeyScope, params?: { pagesize?: number; cursor?: string }): Promise<EscrowKeyList<EscrowParty>> {
  const { data } = await apiClient.get('/escrow/parties', { ...headers(scope), params });
  return data;
}

export async function createEscrowParty(
  scope: EscrowKeyScope,
  body: { name: string; kind: EscrowPartyKind; side: EscrowPartySide }
): Promise<EscrowParty> {
  const { data } = await apiClient.post('/escrow/parties', body, headers(scope));
  return data;
}

export async function getEscrowParty(scope: EscrowKeyScope, id: string): Promise<EscrowPartyDetail> {
  const { data } = await apiClient.get(`/escrow/parties/${id}`, headers(scope));
  return data;
}

export async function getEscrowPartyAudit(scope: EscrowKeyScope, id: string, params?: { pagesize?: number; cursor?: string }): Promise<EscrowKeyList<EscrowKeyAuditEvent>> {
  const { data } = await apiClient.get(`/escrow/parties/${id}/audit`, { ...headers(scope), params });
  return data;
}

/** Imports a private key. The key and passphrase are sent once and never returned. */
export async function importEscrowPrivateKey(
  scope: EscrowKeyScope,
  partyId: string,
  body: { purpose: EscrowKeyPurpose; armoredPrivateKey: string; passphrase?: string; notBefore?: string; notAfter?: string }
): Promise<EscrowKeyVersion> {
  const { data } = await apiClient.post(`/escrow/parties/${partyId}/versions/private`, body, headers(scope));
  return data;
}

export async function addEscrowPublicKey(
  scope: EscrowKeyScope,
  partyId: string,
  body: { purpose: EscrowKeyPurpose; armoredPublicKey: string; notBefore?: string; notAfter?: string }
): Promise<EscrowKeyVersion> {
  const { data } = await apiClient.post(`/escrow/parties/${partyId}/versions/public`, body, headers(scope));
  return data;
}

export async function generateEscrowKey(scope: EscrowKeyScope, partyId: string, purpose: EscrowKeyPurpose): Promise<EscrowKeyVersion> {
  const { data } = await apiClient.post(`/escrow/parties/${partyId}/versions/generate`, { purpose }, headers(scope));
  return data;
}

export async function probeEscrowKeyVersion(scope: EscrowKeyScope, id: string): Promise<{ workflowId: string }> {
  const { data } = await apiClient.post(`/escrow/key-versions/${id}/probe`, undefined, headers(scope));
  return data;
}

export async function activateEscrowKeyVersion(scope: EscrowKeyScope, id: string, confirmReplace = false): Promise<EscrowKeyVersion> {
  const { data } = await apiClient.post(`/escrow/key-versions/${id}/activate`, { confirmReplace }, headers(scope));
  return data;
}

export async function deactivateEscrowKeyVersion(scope: EscrowKeyScope, id: string): Promise<EscrowKeyVersion> {
  const { data } = await apiClient.post(`/escrow/key-versions/${id}/deactivate`, undefined, headers(scope));
  return data;
}

export async function revokeEscrowKeyVersion(scope: EscrowKeyScope, id: string, reason: string, compromised: boolean): Promise<EscrowKeyVersion> {
  const { data } = await apiClient.post(`/escrow/key-versions/${id}/revoke`, { reason, compromised }, headers(scope));
  return data;
}

export async function destroyEscrowKeyVersion(scope: EscrowKeyScope, id: string, confirmFingerprint: string): Promise<EscrowKeyVersion> {
  const { data } = await apiClient.post(`/escrow/key-versions/${id}/destroy`, { confirmFingerprint }, headers(scope));
  return data;
}

/** The armored public key, e.g. to hand to a registry. */
export async function getEscrowPublicKey(scope: EscrowKeyScope, id: string): Promise<string> {
  const { data } = await apiClient.get(`/escrow/key-versions/${id}/public-key`, { ...headers(scope), responseType: 'text' });
  return data as string;
}

/** The scope's own default: the operator default, or the platform default without a tenant. */
export async function getDefaultEscrowArrangement(scope: EscrowKeyScope): Promise<EscrowArrangement | null> {
  try {
    const { data } = await apiClient.get('/escrow/arrangements/default', headers(scope));
    return data;
  } catch (err) {
    if (axios.isAxiosError(err) && err.response?.status === 404) return null;
    throw err;
  }
}

export async function setDefaultEscrowArrangement(
  scope: EscrowKeyScope,
  body: { depositorPartyId?: string; receiverPartyId?: string }
): Promise<EscrowArrangement> {
  const { data } = await apiClient.put('/escrow/arrangements/default', body, headers(scope));
  return data;
}

export async function removeDefaultEscrowArrangement(scope: EscrowKeyScope): Promise<void> {
  await apiClient.delete('/escrow/arrangements/default', headers(scope));
}

export async function listTLDEscrowArrangements(tenantId: string): Promise<EscrowKeyList<EscrowArrangement>> {
  const { data } = await apiClient.get('/escrow/arrangements/tlds', headers({ tenantId }));
  return data;
}

export async function setTLDEscrowArrangement(
  tenantId: string,
  tld: string,
  body: { depositorPartyId?: string; receiverPartyId?: string }
): Promise<EscrowArrangement> {
  const { data } = await apiClient.put(`/escrow/arrangements/tlds/${encodeURIComponent(tld)}`, body, headers({ tenantId }));
  return data;
}

export async function removeTLDEscrowArrangement(tenantId: string, tld: string): Promise<void> {
  await apiClient.delete(`/escrow/arrangements/tlds/${encodeURIComponent(tld)}`, headers({ tenantId }));
}

export async function getEffectiveEscrowArrangement(tenantId: string, tld: string): Promise<EffectiveEscrowArrangement> {
  const { data } = await apiClient.get(`/escrow/arrangements/tlds/${encodeURIComponent(tld)}/effective`, headers({ tenantId }));
  return data;
}

// =============================================================================
// Helpers
// =============================================================================

/** Groups parties the way people think about them. */
export function groupParties(parties: EscrowParty[]) {
  return {
    ourIdentities: parties.filter((p) => p.side === 'self'),
    registryProviders: parties.filter((p) => p.kind === 'RSP' && p.side === 'external'),
    escrowAgents: parties.filter((p) => p.kind === 'DEA' && p.side === 'external'),
  };
}

/** Versions of one purpose, newest first. */
export function versionsFor(versions: EscrowKeyVersion[], purpose: EscrowKeyPurpose): EscrowKeyVersion[] {
  return versions.filter((v) => v.purpose === purpose).sort((a, b) => b.version - a.version);
}

/** Human description of where a resolved side comes from. */
export function inheritedFromLabel(level: EscrowArrangementLevel): string {
  switch (level) {
    case 'tld':
      return 'set for this TLD';
    case 'operator':
      return 'inherited from operator default';
    case 'platform':
      return 'inherited from platform default';
  }
}
