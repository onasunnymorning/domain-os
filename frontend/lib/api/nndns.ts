import { apiClient } from "./client";

export interface NNDNListParams {
  name_like?: string;
  reason_like?: string;
  reason_equals?: string;
  tld_equals?: string;
  pagesize?: number;
  cursor?: string;
}

export interface NNDN {
  Name: string;
  UName: string;
  TLDName: string;
  NameState: string;
  Reason: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface NNDNCountResponse {
  Count: number;
  ObjectType: string;
  Timestamp: string;
  Filter?: Record<string, unknown>;
}

export async function getNNDNCount(params?: NNDNListParams): Promise<NNDNCountResponse> {
  const { data } = await apiClient.get<NNDNCountResponse>("/nndns/count", { params });
  return data;
}

export interface NNDNListResponse {
  Data: NNDN[];
  Meta?: {
    PageCursor?: string;
  };
}

export async function getNNDNs(params?: NNDNListParams): Promise<NNDNListResponse> {
  const { data } = await apiClient.get<NNDNListResponse>("/nndns", { params });
  return data;
}
