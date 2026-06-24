/**
 * Workflow Launchpad Store
 * Zustand store for tracking active workflow runs globally.
 * Persisted to localStorage so state survives page refreshes.
 */

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type WorkflowStatus =
  | 'RUNNING'
  | 'COMPLETED'
  | 'FAILED'
  | 'TIMED_OUT'
  | 'CANCELED'
  | 'CONTINUED_AS_NEW'
  | 'TERMINATED';

export interface WorkflowStep {
  key: string;
  label: string;
}

export interface WorkflowRun {
  workflowId: string;
  runId: string;
  type: string;            // Registry key
  displayName: string;     // e.g., "Escrow Staging — .com"
  status: WorkflowStatus;
  temporalUrl: string;
  startedAt: string;
  closedAt?: string;
  params?: Record<string, any>;
}

interface WorkflowStore {
  runs: WorkflowRun[];
  drawerOpen: boolean;
  selectedRunId: string | null;
  addRun: (run: WorkflowRun) => void;
  updateRun: (workflowId: string, patch: Partial<WorkflowRun>) => void;
  removeRun: (workflowId: string) => void;
  clearCompleted: () => void;
  setDrawerOpen: (open: boolean) => void;
  selectRun: (workflowId: string | null) => void;
  hasRunning: () => boolean;
  runningCount: () => number;
}

export const useWorkflowStore = create<WorkflowStore>()(
  persist(
    (set, get) => ({
      runs: [],
      drawerOpen: false,
      selectedRunId: null,

      addRun: (run) =>
        set((state) => {
          // Prevent duplicates by workflowId
          if (state.runs.some((r) => r.workflowId === run.workflowId)) {
            return state;
          }
          return { runs: [...state.runs, run] };
        }),

      updateRun: (workflowId, patch) =>
        set((state) => ({
          runs: state.runs.map((r) =>
            r.workflowId === workflowId ? { ...r, ...patch } : r
          ),
        })),

      removeRun: (workflowId) =>
        set((state) => ({
          runs: state.runs.filter((r) => r.workflowId !== workflowId),
        })),

      clearCompleted: () =>
        set((state) => ({
          runs: state.runs.filter((r) => r.status === 'RUNNING'),
        })),

      setDrawerOpen: (open) => set({ drawerOpen: open }),

      selectRun: (workflowId) => set({ selectedRunId: workflowId }),

      hasRunning: () => get().runs.some((r) => r.status === 'RUNNING'),

      runningCount: () =>
        get().runs.filter((r) => r.status === 'RUNNING').length,
    }),
    {
      name: 'workflow-launchpad-runs',
    }
  )
);
