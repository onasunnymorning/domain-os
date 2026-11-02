/**
 * React Query hooks for the EPP admin API. Query keys all start with 'epp' so
 * a change to one registrar's access invalidates only ['epp','access',clid].
 */

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  addEPPAllowedNetwork,
  addEPPCertificatePin,
  addEPPPassword,
  assignRegistrarEPPPolicy,
  createEPPLogin,
  deleteEPPPolicy,
  deleteRegistrarFamily,
  endEPPSession,
  endEPPSessionsMatching,
  getEPPAccess,
  getEPPExpiringCertificatePins,
  getEPPLockouts,
  getEPPPolicies,
  getEPPPolicy,
  getEPPSessions,
  getEPPSuspensions,
  getRegistrarEPPCreateOutcomes,
  getRegistrarEPPPolicy,
  getRegistrarFamilies,
  getRegistrarFamily,
  liftEPPLockouts,
  liftEPPSuspension,
  retireEPPCredential,
  saveEPPPolicy,
  saveRegistrarFamily,
  updateEPPLogin,
  type RetireEPPCredentialArgs,
} from "@/lib/api/epp";
import type {
  AddEPPAllowedNetworkRequest,
  AddEPPCertificatePinRequest,
  AddEPPPasswordRequest,
  CreateEPPLoginRequest,
  EPPSessionListParams,
  SaveEPPPolicyRequest,
  SaveRegistrarFamilyRequest,
  UpdateEPPLoginRequest,
} from "@/lib/types/epp";

export const eppKeys = {
  all: ["epp"] as const,
  access: (clid: string) => ["epp", "access", clid] as const,
  registrarPolicy: (clid: string) => ["epp", "policy", clid] as const,
  lockouts: (clid?: string) => ["epp", "lockouts", clid ?? "all"] as const,
  suspensions: (clid?: string) => ["epp", "suspensions", clid ?? "all"] as const,
  sessions: (params?: EPPSessionListParams) => ["epp", "sessions", params ?? {}] as const,
  policies: () => ["epp", "policies"] as const,
  policy: (name: string) => ["epp", "policies", name] as const,
  families: () => ["epp", "families"] as const,
  family: (name: string) => ["epp", "families", name] as const,
  expiringPins: (within: string) => ["epp", "expiring-pins", within] as const,
  createOutcomes: (clid: string) => ["epp", "create-outcomes", clid] as const,
};

/** The text the API put in its error body, or a fallback. */
export function eppErrorMessage(err: unknown, fallback: string): string {
  const e = err as { response?: { status?: number; data?: { error?: string } } } | undefined;
  const msg = e?.response?.data?.error;
  if (e?.response?.status === 403) {
    return msg ? `Not allowed: ${msg}` : "You do not have the permission this action needs.";
  }
  return msg || fallback;
}

// =============================================================================
// Credentials
// =============================================================================

export function useEPPAccess(clid: string, enabled = true) {
  return useQuery({
    queryKey: eppKeys.access(clid),
    queryFn: () => getEPPAccess(clid),
    enabled: enabled && !!clid,
  });
}

function useInvalidateAccess(clid: string) {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: eppKeys.access(clid) });
    qc.invalidateQueries({ queryKey: ["epp", "expiring-pins"] });
  };
}

export function useCreateEPPLogin(clid: string) {
  const invalidate = useInvalidateAccess(clid);
  return useMutation({
    mutationFn: (body: CreateEPPLoginRequest) => createEPPLogin(clid, body),
    onSuccess: invalidate,
  });
}

export function useUpdateEPPLogin(clid: string) {
  const invalidate = useInvalidateAccess(clid);
  return useMutation({
    mutationFn: ({ login, body }: { login: string; body: UpdateEPPLoginRequest }) => updateEPPLogin(clid, login, body),
    onSuccess: invalidate,
  });
}

export function useAddEPPPassword(clid: string) {
  const invalidate = useInvalidateAccess(clid);
  return useMutation({
    mutationFn: ({ login, body }: { login: string; body?: AddEPPPasswordRequest }) => addEPPPassword(clid, login, body),
    onSuccess: invalidate,
  });
}

export function useAddEPPCertificatePin(clid: string) {
  const invalidate = useInvalidateAccess(clid);
  return useMutation({
    mutationFn: (body: AddEPPCertificatePinRequest) => addEPPCertificatePin(clid, body),
    onSuccess: invalidate,
  });
}

export function useAddEPPAllowedNetwork(clid: string) {
  const invalidate = useInvalidateAccess(clid);
  return useMutation({
    mutationFn: (body: AddEPPAllowedNetworkRequest) => addEPPAllowedNetwork(clid, body),
    onSuccess: invalidate,
  });
}

export function useRetireEPPCredential(clid: string) {
  const invalidate = useInvalidateAccess(clid);
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: Omit<RetireEPPCredentialArgs, "clid">) => retireEPPCredential({ clid, ...args }),
    onSuccess: () => {
      invalidate();
      // A compromise retire ends sessions.
      qc.invalidateQueries({ queryKey: ["epp", "sessions"] });
    },
  });
}

export function useEPPExpiringCertificatePins(within = "720h", page?: { pagesize?: number; cursor?: string }, enabled = true) {
  return useQuery({
    queryKey: [...eppKeys.expiringPins(within), page ?? {}],
    queryFn: () => getEPPExpiringCertificatePins(within, page),
    enabled,
  });
}

// =============================================================================
// Lockouts and suspensions
// =============================================================================

export function useEPPLockouts(clid?: string, options?: { refetchInterval?: number; enabled?: boolean }) {
  return useQuery({
    queryKey: eppKeys.lockouts(clid),
    queryFn: () => getEPPLockouts(clid),
    refetchInterval: options?.refetchInterval,
    enabled: options?.enabled ?? true,
  });
}

export function useLiftEPPLockouts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ clid, login }: { clid: string; login: string }) => liftEPPLockouts(clid, login),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["epp", "lockouts"] }),
  });
}

export function useEPPSuspensions(clid?: string, options?: { refetchInterval?: number; enabled?: boolean }) {
  return useQuery({
    queryKey: eppKeys.suspensions(clid),
    queryFn: () => getEPPSuspensions(clid),
    refetchInterval: options?.refetchInterval,
    enabled: options?.enabled ?? true,
  });
}

export function useLiftEPPSuspension() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ clid, login }: { clid: string; login: string }) => liftEPPSuspension(clid, login),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["epp", "suspensions"] }),
  });
}

// =============================================================================
// Policies and families
// =============================================================================

export function useEPPPolicies(enabled = true) {
  return useQuery({ queryKey: eppKeys.policies(), queryFn: getEPPPolicies, enabled });
}

/** An editor reads fresh (never a cached copy) so a save cannot overwrite a newer change it never saw. */
const freshRead = { staleTime: 0, refetchOnMount: "always" as const };

export function useEPPPolicy(name: string, enabled = true, opts?: { fresh?: boolean }) {
  return useQuery({
    queryKey: eppKeys.policy(name),
    queryFn: () => getEPPPolicy(name),
    enabled: enabled && !!name,
    ...(opts?.fresh ? freshRead : {}),
  });
}

export function useSaveEPPPolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body, createOnly }: { name: string; body: SaveEPPPolicyRequest; createOnly?: boolean }) => saveEPPPolicy(name, body, { createOnly }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: eppKeys.policies() });
      qc.invalidateQueries({ queryKey: ["epp", "policy"] });
    },
  });
}

export function useDeleteEPPPolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deleteEPPPolicy(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: eppKeys.policies() }),
  });
}

export function useRegistrarEPPPolicy(clid: string, enabled = true) {
  return useQuery({
    queryKey: eppKeys.registrarPolicy(clid),
    queryFn: () => getRegistrarEPPPolicy(clid),
    enabled: enabled && !!clid,
  });
}

export function useAssignRegistrarEPPPolicy(clid: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (policy: string) => assignRegistrarEPPPolicy(clid, { policy }),
    onSuccess: () => qc.invalidateQueries({ queryKey: eppKeys.registrarPolicy(clid) }),
  });
}

export function useRegistrarEPPCreateOutcomes(clid: string, params?: { from?: string; to?: string }, enabled = true) {
  return useQuery({
    queryKey: [...eppKeys.createOutcomes(clid), params ?? {}],
    queryFn: () => getRegistrarEPPCreateOutcomes(clid, params),
    enabled: enabled && !!clid,
  });
}

export function useRegistrarFamilies(enabled = true) {
  return useQuery({ queryKey: eppKeys.families(), queryFn: getRegistrarFamilies, enabled });
}

export function useRegistrarFamily(name: string, enabled = true, opts?: { fresh?: boolean }) {
  return useQuery({
    queryKey: eppKeys.family(name),
    queryFn: () => getRegistrarFamily(name),
    enabled: enabled && !!name,
    ...(opts?.fresh ? freshRead : {}),
  });
}

export function useSaveRegistrarFamily() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body, createOnly }: { name: string; body: SaveRegistrarFamilyRequest; createOnly?: boolean }) => saveRegistrarFamily(name, body, { createOnly }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: eppKeys.families() });
      qc.invalidateQueries({ queryKey: ["epp", "policy"] });
    },
  });
}

export function useDeleteRegistrarFamily() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deleteRegistrarFamily(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: eppKeys.families() });
      qc.invalidateQueries({ queryKey: ["epp", "policy"] });
    },
  });
}

// =============================================================================
// Sessions
// =============================================================================

export function useEPPSessions(params?: EPPSessionListParams, options?: { refetchInterval?: number; enabled?: boolean }) {
  return useQuery({
    queryKey: eppKeys.sessions(params),
    queryFn: () => getEPPSessions(params),
    refetchInterval: options?.refetchInterval,
    enabled: options?.enabled ?? true,
  });
}

export function useEndEPPSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => endEPPSession(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["epp", "sessions"] }),
  });
}

export function useEndEPPSessionsMatching() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (params: Parameters<typeof endEPPSessionsMatching>[0]) => endEPPSessionsMatching(params),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["epp", "sessions"] }),
  });
}
