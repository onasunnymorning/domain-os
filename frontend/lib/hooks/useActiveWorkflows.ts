/**
 * useActiveWorkflows Hook
 * React Query hook that polls /workflows/active and syncs to the Zustand store.
 * Resolves final status for workflows that close between polls.
 */

'use client';

import { useQuery } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';
import { getActiveWorkflows, getWorkflowStatus } from '@/lib/api/workflows';
import { useWorkflowStore } from '@/lib/stores/useWorkflowStore';

export function useActiveWorkflows() {
  const runs = useWorkflowStore((s) => s.runs);
  const updateRun = useWorkflowStore((s) => s.updateRun);
  const hasRunning = useWorkflowStore((s) => s.hasRunning());
  const runsRef = useRef(runs);
  runsRef.current = runs;

  const { data, isLoading, error, dataUpdatedAt } = useQuery({
    queryKey: ['workflows', 'active'],
    queryFn: getActiveWorkflows,
    // Poll aggressively (3s) when workflows are running so short-lived
    // workflows get their completion detected quickly. Back off to 30s when idle.
    refetchInterval: hasRunning ? 3_000 : 30_000,
    // Always refetch on mount so newly launched workflows get checked immediately
    refetchOnMount: 'always',
    staleTime: 0,
    // Only poll when there are tracked runs in the store
    enabled: runs.length > 0,
  });

  // Sync active workflow data into the store.
  // IMPORTANT: We use a ref for `runs` to avoid the infinite loop where:
  //   updateRun → runs changes → effect re-fires → updateRun → ...
  useEffect(() => {
    if (!data?.items) return;

    const activeIds = new Set(data.items.map((item) => item.workflowId));

    // Update store with fresh data from Temporal
    for (const item of data.items) {
      updateRun(item.workflowId, {
        runId: item.runId,
        status: item.status as any,
        temporalUrl: item.url,
      });
    }

    // For store runs no longer in the active list: query their final status
    for (const run of runsRef.current) {
      if (run.status === 'RUNNING' && !activeIds.has(run.workflowId)) {
        getWorkflowStatus(run.workflowId)
          .then((status) => {
            updateRun(run.workflowId, {
              status: (status.status as any) || 'COMPLETED',
              closedAt: status.closeTime || new Date().toISOString(),
            });
          })
          .catch(() => {
            updateRun(run.workflowId, {
              status: 'COMPLETED',
              closedAt: new Date().toISOString(),
            });
          });
      }
    }
  // dataUpdatedAt ensures the effect fires on every poll, even if the response
  // payload is identical (e.g., empty active list on consecutive polls).
  }, [data, dataUpdatedAt, updateRun]);

  return { isLoading, error };
}
