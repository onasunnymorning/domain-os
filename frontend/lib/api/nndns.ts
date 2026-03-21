import { apiClient } from "./client";

export interface NNDNListParams {
  name_like?: string;
  reason_like?: string;
  reason_equals?: string;
  tld_equals?: string;
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
