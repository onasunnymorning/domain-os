/**
 * useActiveWorkflows Hook
 * React Query hook that polls /workflows/active and syncs to the Zustand store.
 */

'use client';

import { useQuery } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';
import { getActiveWorkflows } from '@/lib/api/workflows';
import { useWorkflowStore } from '@/lib/stores/useWorkflowStore';

export function useActiveWorkflows() {
  const runs = useWorkflowStore((s) => s.runs);
  const updateRun = useWorkflowStore((s) => s.updateRun);
  const hasRunning = useWorkflowStore((s) => s.hasRunning());
  const runsRef = useRef(runs);
  runsRef.current = runs;

  const { data, isLoading, error } = useQuery({
    queryKey: ['workflows', 'active'],
    queryFn: getActiveWorkflows,
    refetchInterval: hasRunning ? 10_000 : 30_000,
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

    // Mark store runs no longer in the active list as COMPLETED
    // (they finished between polls)
    for (const run of runsRef.current) {
      if (run.status === 'RUNNING' && !activeIds.has(run.workflowId)) {
        updateRun(run.workflowId, {
          status: 'COMPLETED',
          closedAt: new Date().toISOString(),
        });
      }
    }
  }, [data, updateRun]); // ← `runs` removed from deps, read from ref instead

  return { isLoading, error };
}
