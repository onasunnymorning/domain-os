import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { launchApplicationsApi } from '@/lib/api/launchApplications';
import { tldsApi } from '@/lib/api/tlds';
import type { LaunchApplicationStatus } from '@/lib/types/phase';

// The operator the TLD belongs to, from the ['tld', name] query the TLD page
// already holds: every application request is within that tenant.
function useOperator(tldName: string) {
  const queryClient = useQueryClient();
  // A TLD's operator does not change, so a cached TLD serves for a while.
  return () => queryClient.fetchQuery({ queryKey: ['tld', tldName], queryFn: () => tldsApi.get(tldName), staleTime: 5 * 60_000 }).then((t) => t.RyID);
}

// A page of the applications filed in a phase, by status.
export function useLaunchApplications(tldName: string, phaseName: string, status: LaunchApplicationStatus | '' = '', cursor = '', enabled = true) {
  const operator = useOperator(tldName);
  return useQuery({
    queryKey: ['launch-applications', tldName, phaseName, status, cursor],
    queryFn: async () => launchApplicationsApi.list(tldName, phaseName, await operator(), { status, cursor, pageSize: 25 }),
    enabled: enabled && !!tldName && !!phaseName,
  });
}

export type ApplicationDecision =
  | { kind: 'validate'; applicationId: string; valid: boolean; reason?: string }
  | { kind: 'allocate'; applicationId: string }
  | { kind: 'reject'; applicationId: string; reason: string };

// One mutation for the three decisions, refreshing the phase's applications
// and the TLD's domains (an allocation registers one).
export function useDecideApplication(tldName: string, phaseName: string) {
  const queryClient = useQueryClient();
  const operator = useOperator(tldName);
  return useMutation({
    mutationFn: async (decision: ApplicationDecision) => {
      const ryID = await operator();
      switch (decision.kind) {
        case 'validate':
          return launchApplicationsApi.validate(decision.applicationId, ryID, { valid: decision.valid, reason: decision.reason });
        case 'allocate':
          return launchApplicationsApi.allocate(decision.applicationId, ryID);
        case 'reject':
          return launchApplicationsApi.reject(decision.applicationId, ryID, decision.reason);
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['launch-applications', tldName, phaseName] });
      queryClient.invalidateQueries({ queryKey: ['domains'] });
    },
  });
}
