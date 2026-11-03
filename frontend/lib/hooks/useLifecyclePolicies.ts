import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  deleteLifecyclePolicy,
  getLifecyclePolicies,
  getLifecyclePolicy,
  saveLifecyclePolicy,
  setPhaseLifecyclePolicy,
  type SaveLifecycleOptions,
} from '@/lib/api/lifecycle';
import type { SaveLifecyclePolicyRequest } from '@/lib/types/lifecycle';

export const lifecycleKeys = {
  all: ['lifecycle-policies'] as const,
  policy: (name: string) => ['lifecycle-policies', name] as const,
};

export function useLifecyclePolicies(enabled = true) {
  return useQuery({ queryKey: lifecycleKeys.all, queryFn: getLifecyclePolicies, enabled });
}

export function useLifecyclePolicy(name: string, enabled = true, opts?: { fresh?: boolean }) {
  return useQuery({
    queryKey: lifecycleKeys.policy(name),
    queryFn: () => getLifecyclePolicy(name),
    enabled: enabled && !!name,
    ...(opts?.fresh ? { staleTime: 0, refetchOnMount: 'always' as const } : {}),
  });
}

export function useSaveLifecyclePolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, body, ...opts }: { name: string; body: SaveLifecyclePolicyRequest } & SaveLifecycleOptions) => saveLifecyclePolicy(name, body, opts),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: lifecycleKeys.all });
      // Every phase that names the policy answers with the new resolved values.
      qc.invalidateQueries({ queryKey: ['phases'] });
      qc.invalidateQueries({ queryKey: ['phase'] });
    },
  });
}

export function useDeleteLifecyclePolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deleteLifecyclePolicy(name),
    onSuccess: () => qc.invalidateQueries({ queryKey: lifecycleKeys.all }),
  });
}

export function useSetPhaseLifecyclePolicy(tldName: string, phaseName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (lifecyclePolicy: string) => setPhaseLifecyclePolicy(tldName, phaseName, lifecyclePolicy),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['phases', tldName] });
      qc.invalidateQueries({ queryKey: ['phase', tldName, phaseName] });
      qc.invalidateQueries({ queryKey: lifecycleKeys.all });
    },
  });
}
