/**
 * Lifecycle policy admin API client (ADR 0023): /lifecycle-policies and
 * the phase edit that names one.
 */

import { apiClient } from './client';
import type { Phase } from '@/lib/types/phase';
import type { LifecyclePolicy, SaveLifecyclePolicyRequest } from '@/lib/types/lifecycle';

const enc = encodeURIComponent;

export interface SaveLifecycleOptions {
  /** PUT with If-None-Match: * refuses an existing name (412). */
  createOnly?: boolean;
  /** The updatedAt that was read; the save is refused (412) if it has changed. */
  ifUnchangedSince?: string;
}

function conditional(opts?: SaveLifecycleOptions) {
  const headers: Record<string, string> = {};
  if (opts?.createOnly) headers['If-None-Match'] = '*';
  if (opts?.ifUnchangedSince) headers['If-Match'] = `"${opts.ifUnchangedSince}"`;
  return Object.keys(headers).length ? { headers } : undefined;
}

export async function getLifecyclePolicies(): Promise<LifecyclePolicy[]> {
  const { data } = await apiClient.get('/lifecycle-policies');
  return data ?? [];
}

export async function getLifecyclePolicy(name: string): Promise<LifecyclePolicy> {
  const { data } = await apiClient.get(`/lifecycle-policies/${enc(name)}`);
  return data;
}

export async function saveLifecyclePolicy(name: string, body: SaveLifecyclePolicyRequest, opts?: SaveLifecycleOptions): Promise<LifecyclePolicy> {
  const { data } = await apiClient.put(`/lifecycle-policies/${enc(name)}`, body, conditional(opts));
  return data;
}

export async function deleteLifecyclePolicy(name: string): Promise<void> {
  await apiClient.delete(`/lifecycle-policies/${enc(name)}`);
}

/** Names the lifecycle policy a phase follows: a phase edit, under the operator's tenant. */
export async function setPhaseLifecyclePolicy(tldName: string, phaseName: string, lifecyclePolicy: string): Promise<Phase> {
  const { data } = await apiClient.put(`/tlds/${enc(tldName)}/phases/${enc(phaseName)}/lifecycle-policy`, { lifecyclePolicy });
  return data;
}
