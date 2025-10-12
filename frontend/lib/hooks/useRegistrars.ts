/**
 * React Query Hooks for Registrar Management
 * Custom hooks for fetching and mutating IANA and System Registrars
 */

"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  getIANARegistrars,
  getIANARegistrarByGurID,
  getIANARegistrarCount,
  syncIANARegistrars,
  getRegistrars,
  getRegistrarByClID,
  getRegistrarByGurID,
  getRegistrarCount,
  createRegistrar,
  updateRegistrar,
  updateRegistrarStatus,
  deleteRegistrar,
  bulkCreateRegistrars,
} from "@/lib/api/registrars";
import {
  IANARegistrarListParams,
  RegistrarListParams,
  Registrar,
} from "@/lib/types/registrar";

// =============================================================================
// IANA Registrar Hooks
// =============================================================================

/**
 * Hook to fetch IANA Registrars with filtering and pagination
 */
export function useIANARegistrars(params?: IANARegistrarListParams) {
  return useQuery({
    queryKey: ["ianaRegistrars", params],
    queryFn: () => getIANARegistrars(params),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook to fetch a single IANA Registrar by GurID
 */
export function useIANARegistrar(gurID: number, enabled = true) {
  return useQuery({
    queryKey: ["ianaRegistrar", gurID],
    queryFn: () => getIANARegistrarByGurID(gurID),
    enabled: enabled && gurID > 0,
    staleTime: 10 * 60 * 1000, // 10 minutes
  });
}

/**
 * Hook to fetch IANA Registrar count
 */
export function useIANARegistrarCount() {
  return useQuery({
    queryKey: ["ianaRegistrars", "count"],
    queryFn: () => getIANARegistrarCount(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook to sync IANA Registrars from IANA XML repository
 * This is a mutation that triggers a background sync operation
 */
export function useSyncIANARegistrars() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: syncIANARegistrars,
    onSuccess: () => {
      // Invalidate all IANA registrar queries to refetch fresh data
      queryClient.invalidateQueries({ queryKey: ["ianaRegistrars"] });
    },
  });
}

// =============================================================================
// System Registrar Hooks
// =============================================================================

/**
 * Hook to fetch System Registrars with pagination
 */
export function useRegistrars(params?: RegistrarListParams) {
  return useQuery({
    queryKey: ["registrars", params],
    queryFn: () => getRegistrars(params),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook to fetch a single System Registrar by ClID
 */
export function useRegistrar(clid: string, enabled = true) {
  return useQuery({
    queryKey: ["registrar", clid],
    queryFn: () => getRegistrarByClID(clid),
    enabled: enabled && !!clid,
    staleTime: 10 * 60 * 1000, // 10 minutes
  });
}

/**
 * Hook to fetch a single System Registrar by GurID
 */
export function useRegistrarByGurID(gurid: number, enabled = true) {
  return useQuery({
    queryKey: ["registrar", "gurid", gurid],
    queryFn: () => getRegistrarByGurID(gurid),
    enabled: enabled && gurid > 0,
    staleTime: 10 * 60 * 1000, // 10 minutes
  });
}

/**
 * Hook to fetch System Registrar count
 */
export function useRegistrarCount() {
  return useQuery({
    queryKey: ["registrars", "count"],
    queryFn: () => getRegistrarCount(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook to create a new System Registrar
 */
export function useCreateRegistrar() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: Partial<Registrar>) => createRegistrar(data),
    onSuccess: () => {
      // Invalidate registrar queries to refetch fresh data
      queryClient.invalidateQueries({ queryKey: ["registrars"] });
    },
  });
}

/**
 * Hook to update an existing System Registrar
 */
export function useUpdateRegistrar() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ clid, data }: { clid: string; data: Partial<Registrar> }) =>
      updateRegistrar(clid, data),
    onSuccess: (_, variables) => {
      // Invalidate specific registrar and list queries
      queryClient.invalidateQueries({ queryKey: ["registrar", variables.clid] });
      queryClient.invalidateQueries({ queryKey: ["registrars"] });
    },
  });
}

/**
 * Hook to update System Registrar status
 */
export function useUpdateRegistrarStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ clid, status }: { clid: string; status: string }) =>
      updateRegistrarStatus(clid, status),
    onSuccess: (_, variables) => {
      // Invalidate specific registrar and list queries
      queryClient.invalidateQueries({ queryKey: ["registrar", variables.clid] });
      queryClient.invalidateQueries({ queryKey: ["registrars"] });
    },
  });
}

/**
 * Hook to delete a System Registrar
 */
export function useDeleteRegistrar() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (clid: string) => deleteRegistrar(clid),
    onSuccess: (_, clid) => {
      // Invalidate specific registrar and list queries
      queryClient.invalidateQueries({ queryKey: ["registrar", clid] });
      queryClient.invalidateQueries({ queryKey: ["registrars"] });
    },
  });
}

/**
 * Hook to bulk create System Registrars
 */
export function useBulkCreateRegistrars() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: Partial<Registrar>[]) => bulkCreateRegistrars(data),
    onSuccess: () => {
      // Invalidate registrar queries to refetch fresh data
      queryClient.invalidateQueries({ queryKey: ["registrars"] });
    },
  });
}
