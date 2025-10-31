import { apiClient } from './client';

export interface PresignResponse {
  objectKey: string;
  url: string;
  method: string; // "PUT"
  expiresIn: number; // seconds
}

export interface StartEscrowImportResponse {
  workflowId: string;
  runId: string;
  status: string;
  url?: string;
}

export interface EscrowRunItem {
  tld: string;
  runPrefix: string;
  date: string;
  workflowId: string;
  summaryKey?: string;
  hasSummary: boolean;
  url: string;
  summaryUrl?: string;
  runReportUrl?: string;
  analysisUrl?: string;
  registrarMappingUrl?: string;
  registrarMappingJsonUrl?: string;
  artifacts?: Record<string, string>;
}

export interface EscrowImportListResponse {
  items: EscrowRunItem[];
  count: number;
}

export async function presignUpload(filename: string): Promise<PresignResponse> {
  const { data } = await apiClient.post(`/escrow/uploads/presign?filename=${encodeURIComponent(filename)}`);
  return {
    objectKey: data.objectKey,
    url: data.url,
    method: data.method,
    expiresIn: data.expiresIn,
  } as PresignResponse;
}

export async function startEscrowImport(params: { tld: string; objectKey: string; options?: Record<string, any> }): Promise<StartEscrowImportResponse> {
  const payload = { tld: params.tld, objectKey: params.objectKey, options: params.options || {} };
  const { data } = await apiClient.post('/escrow/imports', payload);
  return data as StartEscrowImportResponse;
}

export async function listEscrowImports(tld: string, limit = 20): Promise<EscrowImportListResponse> {
  const { data } = await apiClient.get('/escrow/imports', { params: { tld, limit } });
  return data as EscrowImportListResponse;
}
