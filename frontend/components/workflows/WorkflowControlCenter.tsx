'use client';

import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import {
  Activity,
  CheckCircle2,
  XCircle,
  Clock,
  ChevronDown,
  ChevronRight,
  Rocket,
  AlertTriangle,
} from 'lucide-react';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetFooter,
} from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';
import { useWorkflowStore, type WorkflowRun, type WorkflowStatus } from '@/lib/stores/useWorkflowStore';
import { useActiveWorkflows } from '@/lib/hooks/useActiveWorkflows';
import { WorkflowResult } from './WorkflowResult';
import { WorkflowTemporalLink } from './WorkflowTemporalLink';

import { getWorkflowRegistry } from '@/lib/api/workflows';

// =============================================================================
// Status styling helpers
// =============================================================================

function getStatusIcon(status: WorkflowStatus) {
  switch (status) {
    case 'RUNNING':
      return <Activity className="size-3.5 animate-pulse text-blue-500" />;
    case 'COMPLETED':
      return <CheckCircle2 className="size-3.5 text-green-500" />;
    case 'FAILED':
    case 'TIMED_OUT':
    case 'TERMINATED':
      return <XCircle className="size-3.5 text-red-500" />;
    case 'CANCELED':
      return <AlertTriangle className="size-3.5 text-amber-500" />;
    default:
      return <Clock className="size-3.5 text-muted-foreground" />;
  }
}

function getPillColor(runs: WorkflowRun[]): string {
  const hasRunning = runs.some((r) => r.status === 'RUNNING');
  const hasFailed = runs.some(
    (r) => r.status === 'FAILED' || r.status === 'TIMED_OUT' || r.status === 'TERMINATED'
  );

  if (hasRunning) return 'bg-blue-600 hover:bg-blue-700 text-white';
  if (hasFailed) return 'bg-amber-500 hover:bg-amber-600 text-white';
  return 'bg-green-600 hover:bg-green-700 text-white';
}

// =============================================================================
// Workflow Item Row
// =============================================================================

function WorkflowRunItem({ run }: { run: WorkflowRun }) {
  const [expanded, setExpanded] = useState(false);
  const [runningState, setRunningState] = useState<any>(null);

  const selectedRunId = useWorkflowStore((s) => s.selectedRunId);

  useEffect(() => {
    if (selectedRunId === run.workflowId) {
      setExpanded(true);
    }
  }, [selectedRunId, run.workflowId]);

  const { data: registry } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000,
  });

  const meta = registry?.items?.find((item) => item.key === run.type);
  const signalName = meta?.signalName || run.params?.signalName;

  return (
    <div className="border-b last:border-b-0">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className={cn(
          'flex w-full items-start gap-3 p-3 text-left transition-colors',
          'hover:bg-muted/50 cursor-pointer'
        )}
      >
        {/* Status icon */}
        <div className="mt-0.5 shrink-0">{getStatusIcon(run.status)}</div>

        {/* Main content */}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium">{run.displayName}</span>
            <Badge variant="outline" className="shrink-0 text-[10px] px-1.5 py-0 lowercase">
              {run.type === 'tld-cleanup' && run.params?.tld
                ? `.${run.params.tld.replace(/^\./, '')}`
                : run.type === 'escrow-import' && run.params?.tld
                ? `.${run.params.tld.replace(/^\./, '')}`
                : run.type}
            </Badge>
          </div>

          <div className="text-muted-foreground mt-1 flex items-center gap-3 text-xs">
            <span>
              {formatDistanceToNow(new Date(run.startedAt), { addSuffix: true })}
            </span>
            <WorkflowTemporalLink url={run.temporalUrl} />
          </div>
        </div>

        {/* Expand chevron */}
        <div className="text-muted-foreground mt-0.5 shrink-0">
          {expanded ? (
            <ChevronDown className="size-4" />
          ) : (
            <ChevronRight className="size-4" />
          )}
        </div>
      </button>

      {/* Expanded content: workflow result + HITL actions */}
      {expanded && (
        <div className="border-t bg-muted/30 px-3 py-3">
          <WorkflowResult
            workflowId={run.workflowId}
            workflowType={run.type}
            status={run.status}
            signalName={signalName}
            onStateLoaded={setRunningState}
          />
        </div>
      )}
    </div>
  );
}

// =============================================================================
// Main Control Center Component
// =============================================================================

export function WorkflowControlCenter() {
  const { runs, drawerOpen, setDrawerOpen, clearCompleted, runningCount } =
    useWorkflowStore();

  // Keep data fresh via polling
  useActiveWorkflows();

  const running = runningCount();

  // Hide completely when there are no tracked workflows
  if (runs.length === 0) return null;

  return (
    <>
      {/* Collapsed pill — fixed in bottom-right */}
      {!drawerOpen && (
        <button
          type="button"
          onClick={() => setDrawerOpen(true)}
          className={cn(
            'fixed bottom-6 right-6 z-50 flex items-center gap-2 rounded-full px-4 py-2.5 shadow-lg transition-all duration-200',
            getPillColor(runs),
            running > 0 && 'animate-pulse'
          )}
        >
          <Activity className="size-4" />
          <span className="text-sm font-medium">
            {running > 0
              ? `${running} running`
              : 'Workflows'}
          </span>
          {runs.length > running && running > 0 && (
            <span className="text-xs opacity-75">
              · {runs.length - running} done
            </span>
          )}
        </button>
      )}

      {/* Expanded Sheet drawer */}
      <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
        <SheetContent side="right" className="flex w-full flex-col overflow-hidden sm:max-w-md">
          <SheetHeader>
            <div className="flex items-center gap-2">
              <SheetTitle>Workflow Control</SheetTitle>
              {running > 0 && (
                <Badge variant="default" className="tabular-nums">
                  {running} running
                </Badge>
              )}
            </div>
          </SheetHeader>

          {/* Body: scrollable list of tracked workflows */}
          <ScrollArea className="flex-1 -mx-4 min-h-0 px-4">
            <div className="divide-y rounded-lg border">
              {runs.length === 0 ? (
                <div className="text-muted-foreground flex flex-col items-center gap-2 p-6 text-center text-sm">
                  <Rocket className="size-8 opacity-40" />
                  <p>No workflows tracked yet</p>
                </div>
              ) : (
                runs.map((run) => (
                  <WorkflowRunItem key={run.workflowId} run={run} />
                ))
              )}
            </div>
          </ScrollArea>

          {/* Footer */}
          <SheetFooter className="flex-row justify-between border-t pt-4">
            <Button
              variant="ghost"
              size="sm"
              onClick={clearCompleted}
              disabled={!runs.some((r) => r.status !== 'RUNNING')}
            >
              Clear Completed
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  );
}
