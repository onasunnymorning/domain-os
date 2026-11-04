import { apiClient } from './client';
import type { LaunchApplication, LaunchApplicationStatus } from '@/lib/types/phase';

// The operator's side of launch applications (ADR 0024 §3). Every request is
// within the operator's tenant (X-Tenant-ID, ADR-0006); deciding needs the
// registry:launch:admin permission as well, which the token carries.

export interface LaunchApplicationPage {
  Data: LaunchApplication[];
  Meta: { PageSize: number; PageCursor: string; NextLink: string };
}

const tenant = (operatorId: string) => ({ headers: { 'X-Tenant-ID': operatorId } });

export const launchApplicationsApi = {
  // A page of the applications filed in a phase, oldest first.
  list: async (tldName: string, phaseName: string, operatorId: string, opts: { status?: LaunchApplicationStatus | ''; cursor?: string; pageSize?: number } = {}): Promise<LaunchApplicationPage> => {
    const params = new URLSearchParams();
    if (opts.status) params.set('status', opts.status);
    if (opts.cursor) params.set('cursor', opts.cursor);
    if (opts.pageSize) params.set('pagesize', String(opts.pageSize));
    const query = params.toString();
    const response = await apiClient.get(`/tlds/${encodeURIComponent(tldName)}/phases/${encodeURIComponent(phaseName)}/applications${query ? `?${query}` : ''}`, tenant(operatorId));
    return response.data;
  },

  get: async (applicationId: string, operatorId: string): Promise<LaunchApplication> => {
    const response = await apiClient.get(`/launch-applications/${encodeURIComponent(applicationId)}`, tenant(operatorId));
    return response.data;
  },

  // Records the outcome of validating a pendingValidation application: valid,
  // or invalid with the reason.
  validate: async (applicationId: string, operatorId: string, outcome: { valid: boolean; reason?: string }): Promise<LaunchApplication> => {
    const response = await apiClient.post(`/launch-applications/${encodeURIComponent(applicationId)}/validate`, outcome, tenant(operatorId));
    return response.data;
  },

  // Registers the name to the application and rejects the other applications
  // for it.
  allocate: async (applicationId: string, operatorId: string): Promise<LaunchApplication> => {
    const response = await apiClient.post(`/launch-applications/${encodeURIComponent(applicationId)}/allocate`, {}, tenant(operatorId));
    return response.data;
  },

  reject: async (applicationId: string, operatorId: string, reason: string): Promise<LaunchApplication> => {
    const response = await apiClient.post(`/launch-applications/${encodeURIComponent(applicationId)}/reject`, { reason }, tenant(operatorId));
    return response.data;
  },
};
