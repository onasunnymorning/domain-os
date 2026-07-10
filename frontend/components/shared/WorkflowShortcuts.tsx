'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { WorkflowLaunchForm } from '@/components/workflows/WorkflowLaunchForm';
import { useWorkflowStore } from '@/lib/stores/useWorkflowStore';
import { getWorkflowRegistry, type WorkflowMeta } from '@/lib/api/workflows';
import type { WorkflowRun } from '@/lib/stores/useWorkflowStore';
import {
  Import,
  Users,
  Camera,
  Trash2,
  RefreshCw,
  Zap,
  TrendingDown,
  RotateCcw,
  ArrowDownToLine,
  ArrowUpFromLine,
  Shield,
  type LucideIcon,
} from 'lucide-react';

// ---------------------------------------------------------------------------
// Workflow → icon mapping
// ---------------------------------------------------------------------------

const WORKFLOW_ICONS: Record<string, LucideIcon> = {
  'escrow-import': Import,
  'tld-cleanup': Trash2,
  'sync-registrars': Users,
  'take-snapshot': Camera,
  'seed-from-snapshot': ArrowDownToLine,
  'update-fx': RefreshCw,
  'sync-spec5': ArrowUpFromLine,
  'expiry-loop': TrendingDown,
  'purge-loop': Trash2,
  'restore-workflow': RotateCcw,
  'spec5-sweep': Shield,
};

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface WorkflowShortcutsProps {
  /** Workflow registry keys to display as shortcut buttons */
  workflowKeys: string[];
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * WorkflowShortcuts — reusable row of workflow launch buttons.
 *
 * Renders a compact set of icon buttons, one per workflow key. Each button
 * shows a tooltip with the workflow name and opens the launch dialog on click.
 *
 * Usage:
 *   <WorkflowShortcuts workflowKeys={['escrow-import', 'tld-cleanup']} />
 */
export function WorkflowShortcuts({ workflowKeys }: WorkflowShortcutsProps) {
  const [launchTarget, setLaunchTarget] = useState<WorkflowMeta | null>(null);
  const addRun = useWorkflowStore((s) => s.addRun);
  const setModalOpen = useWorkflowStore((s) => s.setModalOpen);

  const { data: registry } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000,
  });

  const workflows = registry?.items ?? [];

  const handleLaunched = (run: WorkflowRun) => {
    addRun(run);
    setModalOpen(true);
  };

  // Only render workflows that exist in the registry
  const matched = workflowKeys
    .map((key) => workflows.find((w) => w.key === key))
    .filter((wf): wf is WorkflowMeta => wf !== undefined);

  if (matched.length === 0) return null;

  return (
    <>
      <div className="flex items-center gap-2">
        {matched.map((wf) => {
          const Icon = WORKFLOW_ICONS[wf.key] ?? Zap;
          return (
            <Button
              key={wf.key}
              variant="outline"
              size="sm"
              className="gap-1.5 text-muted-foreground hover:text-primary hover:border-primary/40 hover:bg-primary/5 transition-all duration-200"
              onClick={(e) => {
                e.stopPropagation();
                setLaunchTarget(wf);
              }}
            >
              <Icon className="h-3.5 w-3.5" />
              {wf.name}
            </Button>
          );
        })}
      </div>

      <WorkflowLaunchForm
        workflow={launchTarget}
        onClose={() => setLaunchTarget(null)}
        onLaunched={handleLaunched}
      />
    </>
  );
}
