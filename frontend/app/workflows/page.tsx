'use client';

import { useState, useMemo, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Loader2, Zap } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { WorkflowCard } from '@/components/workflows/WorkflowCard';
import { WorkflowTagFilter } from '@/components/workflows/WorkflowTagFilter';
import { WorkflowLaunchForm } from '@/components/workflows/WorkflowLaunchForm';
import { SerialDriftDialog } from '@/components/workflows/SerialDriftDialog';
import { useWorkflowStore } from '@/lib/stores/useWorkflowStore';
import { getWorkflowRegistry, type WorkflowMeta } from '@/lib/api/workflows';
import type { WorkflowRun } from '@/lib/stores/useWorkflowStore';

export default function WorkflowsPage() {
  return (
    <Suspense
      fallback={
        <DashboardLayout>
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        </DashboardLayout>
      }
    >
      <WorkflowsPageContent />
    </Suspense>
  );
}

function WorkflowsPageContent() {
  const searchParams = useSearchParams();
  const highlightKey = searchParams.get('highlight');

  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [launchTarget, setLaunchTarget] = useState<WorkflowMeta | null>(null);

  const addRun = useWorkflowStore((s) => s.addRun);
  const setModalOpen = useWorkflowStore((s) => s.setModalOpen);
  const selectRun = useWorkflowStore((s) => s.selectRun);

  const { data: registry, isLoading } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000, // 5 minutes — registry rarely changes
  });

  const workflows = registry?.items ?? [];

  // Extract all unique tags with counts
  const { allTags, tagCounts } = useMemo(() => {
    const counts: Record<string, number> = {};
    workflows.forEach((wf) => {
      wf.tags.forEach((tag) => {
        counts[tag] = (counts[tag] || 0) + 1;
      });
    });
    return {
      allTags: Object.keys(counts).sort(),
      tagCounts: counts,
    };
  }, [workflows]);

  // Filter workflows by selected tags (OR logic)
  const filteredWorkflows = useMemo(() => {
    if (selectedTags.length === 0) return workflows;
    return workflows.filter((wf) =>
      wf.tags.some((tag) => selectedTags.includes(tag))
    );
  }, [workflows, selectedTags]);

  const handleTagToggle = (tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  };

  const handleLaunched = (run: WorkflowRun) => {
    addRun(run);
    selectRun(run.workflowId);
    setModalOpen(true);
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
              <Zap className="h-5 w-5 text-primary" />
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-tight">Workflow Launchpad</h1>
              <p className="text-sm text-muted-foreground">
                Launch, monitor, and manage Temporal workflows
              </p>
            </div>
          </div>
        </div>

        {/* Tag Filters */}
        {allTags.length > 0 && (
          <WorkflowTagFilter
            allTags={allTags}
            selectedTags={selectedTags}
            onTagToggle={handleTagToggle}
            onClearAll={() => setSelectedTags([])}
            tagCounts={tagCounts}
          />
        )}

        {/* Loading State */}
        {isLoading && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        )}

        {/* Workflow Grid */}
        {!isLoading && filteredWorkflows.length > 0 && (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {filteredWorkflows.map((wf) => (
              <div
                key={wf.key}
                className={
                  highlightKey === wf.key
                    ? 'rounded-lg ring-2 ring-primary ring-offset-2 ring-offset-background transition-all'
                    : ''
                }
              >
                <WorkflowCard
                  workflow={wf}
                  onLaunch={setLaunchTarget}
                />
              </div>
            ))}
          </div>
        )}

        {/* Empty filtered state */}
        {!isLoading && filteredWorkflows.length === 0 && workflows.length > 0 && (
          <div className="rounded-lg border border-dashed p-8 text-center">
            <p className="text-sm text-muted-foreground">
              No workflows match the selected filters.
            </p>
          </div>
        )}

        {/* Empty state (no workflows at all) */}
        {!isLoading && workflows.length === 0 && (
          <div className="rounded-lg border border-dashed p-12 text-center">
            <Zap className="mx-auto h-8 w-8 text-muted-foreground/50" />
            <p className="mt-2 text-sm text-muted-foreground">
              No workflows found. Check that the backend is running.
            </p>
          </div>
        )}
      </div>

      {/* Launch Form Dialog — routes serial-drift to dedicated dialog */}
      <WorkflowLaunchForm
        workflow={launchTarget?.key === 'serial-drift' ? null : launchTarget}
        onClose={() => setLaunchTarget(null)}
        onLaunched={handleLaunched}
      />
      <SerialDriftDialog
        workflow={launchTarget?.key === 'serial-drift' ? launchTarget : null}
        onClose={() => setLaunchTarget(null)}
        onLaunched={handleLaunched}
      />
    </DashboardLayout>
  );
}
