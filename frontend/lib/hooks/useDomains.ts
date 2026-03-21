"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createDomain, getDomains, getDomainCount, getDomainByName, getDomainDNS } from "@/lib/api/domains";
import { DomainListParams, DomainCreateRequest } from "@/lib/types/domain";

export function useDomains(params?: DomainListParams) {
  return useQuery({
    queryKey: ["domains", params],
    queryFn: () => getDomains(params),
  });
}

export function useDomain(name: string, enabled = true) {
  return useQuery({
    queryKey: ["domain", name],
    queryFn: () => getDomainByName(name),
    enabled: enabled && !!name,
  });
}

export function useDomainCount(params?: DomainListParams) {
  return useQuery({
    queryKey: ["domains", "count", params],
    queryFn: () => getDomainCount(params),
  });
}

export function useCreateDomain() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: DomainCreateRequest) => createDomain(payload),
    onSuccess: async () => {
      // Refresh lists and counts on create
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["domains"] }),
        qc.invalidateQueries({ queryKey: ["domains", "count"] }),
      ]);
    },
  });
}

export function useDomainDNS(name: string, enabled = true) {
  return useQuery({
    queryKey: ["domain", "dns", name],
    queryFn: () => getDomainDNS(name),
    enabled: enabled && !!name,
    retry: false, // DNS lookups shouldn't keep retrying if they fail (e.g. non-existent domain)
  });
}
