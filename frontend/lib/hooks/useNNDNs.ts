"use client";

import { useQuery } from "@tanstack/react-query";
import { getNNDNCount, NNDNListParams } from "@/lib/api/nndns";

export function useNNDNCount(params?: NNDNListParams) {
  return useQuery({
    queryKey: ["nndns", "count", params],
    queryFn: () => getNNDNCount(params),
  });
}
