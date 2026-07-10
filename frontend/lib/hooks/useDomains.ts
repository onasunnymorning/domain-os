"use client";

import { useMutation, useQuery, useQueries, useQueryClient } from "@tanstack/react-query";
import { createDomain, getDomains, getDomainCount, getDomainByName, getDomainDNS, updateDomain, setDropCatch, unsetDropCatch, getQuote, getDomainEvents } from "@/lib/api/domains";
import { DomainListParams, DomainCreateRequest, QuoteRequest } from "@/lib/types/domain";

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

export function useUpdateDomain() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, payload }: { name: string; payload: any }) => updateDomain(name, payload),
    onSuccess: async (_, { name }) => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["domain", name] }),
        qc.invalidateQueries({ queryKey: ["domains"] }),
      ]);
    },
  });
}

export function useSetDropCatch() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => setDropCatch(name),
    onSuccess: async (_, name) => {
      await qc.invalidateQueries({ queryKey: ["domain", name] });
    },
  });
}

export function useUnsetDropCatch() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => unsetDropCatch(name),
    onSuccess: async (_, name) => {
      await qc.invalidateQueries({ queryKey: ["domain", name] });
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

export function useDomainCountsForRegistrars(tldName: string, clids: string[]) {
  return useQueries({
    queries: clids.map((clid) => ({
      queryKey: ["domains", "count", { tld_equals: tldName, clid_equals: clid }],
      queryFn: () => getDomainCount({ tld_equals: tldName, clid_equals: clid }),
      staleTime: 60000,
    })),
  });
}

export function useDomainQuote(payload: QuoteRequest | null) {
  return useQuery({
    queryKey: ["domain", "quote", payload],
    queryFn: () => getQuote(payload!),
    enabled: !!payload,
    staleTime: 1000 * 60 * 5, // 5 minutes
  });
}

export function useDomainQuotes(payloads: QuoteRequest[]) {
  return useQueries({
    queries: payloads.map((payload) => ({
      queryKey: ["domain", "quote", payload],
      queryFn: () => getQuote(payload),
      staleTime: 1000 * 60 * 5,
    })),
  });
}

export function useDomainEvents(name: string, enabled = true) {
  return useQuery({
    queryKey: ["domain", name, "events"],
    queryFn: () => getDomainEvents(name),
    enabled: enabled && !!name,
  });
}

