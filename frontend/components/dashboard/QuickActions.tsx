'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Card,
  CardContent,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { WorkflowLaunchForm } from '@/components/workflows/WorkflowLaunchForm';
import { useWorkflowStore } from '@/lib/stores/useWorkflowStore';
import { getWorkflowRegistry, type WorkflowMeta } from '@/lib/api/workflows';
import type { WorkflowRun } from '@/lib/stores/useWorkflowStore';
import {
  Import,
  Users,
  Camera,
  Zap,
} from 'lucide-react';

// ---------------------------------------------------------------------------
// Featured workflows — these appear as quick-action cards on the dashboard
// ---------------------------------------------------------------------------

const FEATURED_WORKFLOWS = [
  {
    key: 'escrow-import',
    icon: Import,
    gradient: 'from-orange-500/10 to-orange-600/5',
    iconColor: 'text-orange-600 dark:text-orange-400',
    iconBg: 'bg-orange-100 dark:bg-orange-950/40',
  },
  {
    key: 'sync-registrars',
    icon: Users,
    gradient: 'from-blue-500/10 to-blue-600/5',
    iconColor: 'text-blue-600 dark:text-blue-400',
    iconBg: 'bg-blue-100 dark:bg-blue-950/40',
  },
  {
    key: 'take-snapshot',
    icon: Camera,
    gradient: 'from-violet-500/10 to-violet-600/5',
    iconColor: 'text-violet-600 dark:text-violet-400',
    iconBg: 'bg-violet-100 dark:bg-violet-950/40',
  },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function QuickActions() {
  const [launchTarget, setLaunchTarget] = useState<WorkflowMeta | null>(null);
  const addRun = useWorkflowStore((s) => s.addRun);
  const setModalOpen = useWorkflowStore((s) => s.setModalOpen);

  const { data: registry, isLoading } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000,
  });

  const workflows = registry?.items ?? [];

  const handleLaunched = (run: WorkflowRun) => {
    addRun(run);
    setModalOpen(true);
  };

  if (isLoading) {
    return (
      <div className="space-y-3">
        <div className="flex items-center gap-2 mb-1">
          <Zap className="h-4 w-4 text-primary" />
          <h3 className="text-base font-semibold">Quick Actions</h3>
        </div>
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  return (
    <>
      <div className="space-y-3">
        <div className="flex items-center gap-2 mb-1">
          <Zap className="h-4 w-4 text-primary" />
          <h3 className="text-base font-semibold">Quick Actions</h3>
        </div>
        {FEATURED_WORKFLOWS.map((featured) => {
          const wf = workflows.find((w) => w.key === featured.key);
          if (!wf) return null;

          const Icon = featured.icon;

          return (
            <Card
              key={featured.key}
              className={`group cursor-pointer border transition-all duration-200 hover:border-primary/30 hover:shadow-sm bg-gradient-to-br ${featured.gradient}`}
              onClick={() => setLaunchTarget(wf)}
            >
              <CardContent className="flex items-center gap-4 py-4 px-5">
                <div
                  className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${featured.iconBg}`}
                >
                  <Icon className={`h-5 w-5 ${featured.iconColor}`} />
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium leading-tight">{wf.name}</p>
                  <p className="text-xs text-muted-foreground mt-0.5 line-clamp-1">
                    {wf.description}
                  </p>
                </div>
              </CardContent>
            </Card>
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
