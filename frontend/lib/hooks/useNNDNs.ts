"use client";

import { useQuery } from "@tanstack/react-query";
import { getNNDNCount, getNNDNs, NNDNListParams } from "@/lib/api/nndns";

export function useNNDNCount(params?: NNDNListParams) {
  return useQuery({
    queryKey: ["nndns", "count", params],
    queryFn: () => getNNDNCount(params),
  });
}

export function useNNDNs(params?: NNDNListParams) {
  return useQuery({
    queryKey: ["nndns", "list", params],
    queryFn: () => getNNDNs(params),
  });
}
