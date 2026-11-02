/**
 * EPP admin API client: credentials, policies, families, sessions, lockouts
 * and suspensions. One function per route; see docs/epp/POLICY_REFERENCE.md
 * § Credentials, § Policies, § Sessions and § Authentication for what each does.
 */

import { apiClient } from './client';
import type {
  AddEPPAllowedNetworkRequest,
  AddEPPCertificatePinRequest,
  AddEPPPasswordRequest,
  AddEPPPasswordResponse,
  AssignEPPPolicyRequest,
  CreateEPPLoginRequest,
  EPPAllowedNetwork,
  EPPCertificatePin,
  EPPCreateOutcomes,
  EPPCredentialKind,
  EPPExpiringPinsResponse,
  EPPLogin,
  EPPLoginLockout,
  EPPLoginSuspension,
  EPPPolicy,
  EPPRegistrarAccess,
  EPPRegistrarPolicy,
  EPPSession,
  EPPSessionListParams,
  EPPSessionListResponse,
  RegistrarFamily,
  RetireEPPCredentialRequest,
  SaveEPPPolicyRequest,
  SaveRegistrarFamilyRequest,
  UpdateEPPLoginRequest,
} from '@/lib/types/epp';

const enc = encodeURIComponent;

// =============================================================================
// Credentials: /registrars/{clid}/epp
// =============================================================================

export async function getEPPAccess(clid: string): Promise<EPPRegistrarAccess> {
  const { data } = await apiClient.get(`/registrars/${enc(clid)}/epp`);
  return data;
}

export async function createEPPLogin(clid: string, body: CreateEPPLoginRequest): Promise<EPPLogin> {
  const { data } = await apiClient.post(`/registrars/${enc(clid)}/epp/logins`, body);
  return data;
}

export async function updateEPPLogin(clid: string, login: string, body: UpdateEPPLoginRequest): Promise<EPPLogin> {
  const { data } = await apiClient.patch(`/registrars/${enc(clid)}/epp/logins/${enc(login)}`, body);
  return data;
}

/** With no password in the body the server generates one and returns it once. */
export async function addEPPPassword(clid: string, login: string, body: AddEPPPasswordRequest = {}): Promise<AddEPPPasswordResponse> {
  const { data } = await apiClient.post(`/registrars/${enc(clid)}/epp/logins/${enc(login)}/passwords`, body);
  return data;
}

export async function addEPPCertificatePin(clid: string, body: AddEPPCertificatePinRequest): Promise<EPPCertificatePin> {
  const { data } = await apiClient.post(`/registrars/${enc(clid)}/epp/certificate-pins`, body);
  return data;
}

export async function addEPPAllowedNetwork(clid: string, body: AddEPPAllowedNetworkRequest): Promise<EPPAllowedNetwork> {
  const { data } = await apiClient.post(`/registrars/${enc(clid)}/epp/allowed-networks`, body);
  return data;
}

export interface RetireEPPCredentialArgs {
  clid: string;
  kind: EPPCredentialKind;
  id: string;
  /** Required when kind is 'password'. */
  login?: string;
  body?: RetireEPPCredentialRequest;
}

export async function retireEPPCredential({ clid, kind, id, login, body = {} }: RetireEPPCredentialArgs): Promise<{ id: string; kind: EPPCredentialKind; retired: boolean }> {
  const base = `/registrars/${enc(clid)}/epp`;
  const path =
    kind === 'password'
      ? `${base}/logins/${enc(login ?? '')}/passwords/${enc(id)}/retire`
      : kind === 'certificate_pin'
        ? `${base}/certificate-pins/${enc(id)}/retire`
        : `${base}/allowed-networks/${enc(id)}/retire`;
  const { data } = await apiClient.post(path, body);
  return data;
}

/** A page of usable pins across every registrar whose certificate expires within the window (Go duration, e.g. "720h"). */
export async function getEPPExpiringCertificatePins(
  expiringWithin = '720h',
  page?: { pagesize?: number; cursor?: string },
): Promise<EPPExpiringPinsResponse> {
  const { data } = await apiClient.get('/epp/certificate-pins', { params: { expiring_within: expiringWithin, ...page } });
  return data;
}

// =============================================================================
// Lockouts and suspensions
// =============================================================================

export async function getEPPLockouts(clid?: string): Promise<EPPLoginLockout[]> {
  const { data } = await apiClient.get('/epp/lockouts', { params: clid ? { clid } : undefined });
  return data ?? [];
}

export async function liftEPPLockouts(clid: string, login: string): Promise<EPPLoginLockout[]> {
  const { data } = await apiClient.delete(`/registrars/${enc(clid)}/epp/logins/${enc(login)}/lockouts`);
  return data ?? [];
}

export async function getEPPSuspensions(clid?: string): Promise<EPPLoginSuspension[]> {
  const { data } = await apiClient.get('/epp/suspensions', { params: clid ? { clid } : undefined });
  return data ?? [];
}

export async function liftEPPSuspension(clid: string, login: string): Promise<EPPLoginSuspension> {
  const { data } = await apiClient.delete(`/registrars/${enc(clid)}/epp/logins/${enc(login)}/suspension`);
  return data;
}

// =============================================================================
// Policies and families: /epp/policies, /epp/families
// =============================================================================

export async function getEPPPolicies(): Promise<EPPPolicy[]> {
  const { data } = await apiClient.get('/epp/policies');
  return data ?? [];
}

export async function getEPPPolicy(name: string): Promise<EPPPolicy> {
  const { data } = await apiClient.get(`/epp/policies/${enc(name)}`);
  return data;
}

/** Conditional headers: createOnly refuses an existing name (409); ifUnchangedSince (the updated_at read) refuses a save over a newer version (412). */
function conditional(opts?: { createOnly?: boolean; ifUnchangedSince?: string }) {
  const headers: Record<string, string> = {};
  if (opts?.createOnly) headers["If-None-Match"] = "*";
  if (opts?.ifUnchangedSince) headers["If-Match"] = `"${opts.ifUnchangedSince}"`;
  return Object.keys(headers).length ? { headers } : undefined;
}

export interface SaveOptions {
  createOnly?: boolean;
  /** The updated_at that was read; the save is refused (412) if it has changed. */
  ifUnchangedSince?: string;
}

export async function saveEPPPolicy(name: string, body: SaveEPPPolicyRequest, opts?: SaveOptions): Promise<EPPPolicy> {
  const { data } = await apiClient.put(`/epp/policies/${enc(name)}`, body, conditional(opts));
  return data;
}

export async function deleteEPPPolicy(name: string): Promise<void> {
  await apiClient.delete(`/epp/policies/${enc(name)}`);
}

export async function getRegistrarEPPPolicy(clid: string): Promise<EPPRegistrarPolicy> {
  const { data } = await apiClient.get(`/registrars/${enc(clid)}/epp/policy`);
  return data;
}

/** "default" or "" removes the assignment. */
export async function assignRegistrarEPPPolicy(clid: string, body: AssignEPPPolicyRequest): Promise<EPPRegistrarPolicy> {
  const { data } = await apiClient.put(`/registrars/${enc(clid)}/epp/policy`, body);
  return data;
}

export async function getRegistrarEPPCreateOutcomes(clid: string, params?: { from?: string; to?: string }): Promise<EPPCreateOutcomes[]> {
  const { data } = await apiClient.get(`/registrars/${enc(clid)}/epp/create-outcomes`, { params });
  return data ?? [];
}

export async function getRegistrarFamilies(): Promise<RegistrarFamily[]> {
  const { data } = await apiClient.get('/epp/families');
  return data ?? [];
}

export async function getRegistrarFamily(name: string): Promise<RegistrarFamily> {
  const { data } = await apiClient.get(`/epp/families/${enc(name)}`);
  return data;
}

export async function saveRegistrarFamily(name: string, body: SaveRegistrarFamilyRequest, opts?: SaveOptions): Promise<RegistrarFamily> {
  const { data } = await apiClient.put(`/epp/families/${enc(name)}`, body, conditional(opts));
  return data;
}

export async function deleteRegistrarFamily(name: string): Promise<void> {
  await apiClient.delete(`/epp/families/${enc(name)}`);
}

// =============================================================================
// Sessions: /epp/sessions
// =============================================================================

export async function getEPPSessions(params?: EPPSessionListParams): Promise<EPPSessionListResponse> {
  const { data } = await apiClient.get('/epp/sessions', { params });
  return data;
}

export async function getEPPSession(id: string): Promise<EPPSession> {
  const { data } = await apiClient.get(`/epp/sessions/${enc(id)}`);
  return data;
}

export async function endEPPSession(id: string): Promise<EPPSession> {
  const { data } = await apiClient.delete(`/epp/sessions/${enc(id)}`);
  return data;
}

/** At least one filter is required by the API. */
export async function endEPPSessionsMatching(params: Pick<EPPSessionListParams, 'clid' | 'login' | 'client_ip' | 'credential_id'>): Promise<{ killed: number; sessions: EPPSession[] }> {
  const { data } = await apiClient.delete('/epp/sessions', { params });
  return data;
}
