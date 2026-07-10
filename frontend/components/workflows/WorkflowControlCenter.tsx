'use client';

import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import {
  Activity,
  CheckCircle2,
  XCircle,
  Clock,
  Rocket,
  AlertTriangle,
  Terminal,
  X,
  Trash2,
} from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
} from '@/components/ui/dialog';
import * as DialogPrimitive from '@radix-ui/react-dialog';
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

function getStatusIcon(status: WorkflowStatus, size = 'size-3.5') {
  switch (status) {
    case 'RUNNING':
      return <Activity className={`${size} animate-pulse text-blue-500`} />;
    case 'COMPLETED':
      return <CheckCircle2 className={`${size} text-green-500`} />;
    case 'FAILED':
    case 'TIMED_OUT':
    case 'TERMINATED':
      return <XCircle className={`${size} text-red-500`} />;
    case 'CANCELED':
      return <AlertTriangle className={`${size} text-amber-500`} />;
    default:
      return <Clock className={`${size} text-muted-foreground`} />;
  }
}

// Solarized Dark palette
const SOL = {
  base03: '#002b36',   // bg
  base02: '#073642',   // bg highlights
  base01: '#586e75',   // comments / muted
  base00: '#657b83',   // secondary text
  base0:  '#839496',   // body text
  base1:  '#93a1a1',   // emphasized text
  green:  '#859900',
  cyan:   '#2aa198',
  blue:   '#268bd2',
  yellow: '#b58900',
  orange: '#cb4b16',
  red:    '#dc322f',
} as const;

function getStatusColor(status: WorkflowStatus): string {
  switch (status) {
    case 'RUNNING':
      return 'text-[#268bd2]';  // sol blue
    case 'COMPLETED':
      return 'text-[#859900]';  // sol green
    case 'FAILED':
    case 'TIMED_OUT':
    case 'TERMINATED':
      return 'text-[#dc322f]';  // sol red
    case 'CANCELED':
      return 'text-[#b58900]';  // sol yellow
    default:
      return 'text-[#586e75]';  // sol base01
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
// Run detail panel (terminal-style)
// =============================================================================

function RunDetailPanel({ run }: { run: WorkflowRun }) {
  const [runningState, setRunningState] = useState<any>(null);

  const { data: registry } = useQuery({
    queryKey: ['workflow-registry'],
    queryFn: getWorkflowRegistry,
    staleTime: 5 * 60 * 1000,
  });

  const meta = registry?.items?.find((item) => item.key === run.type);
  const signalName = meta?.signalName || run.params?.signalName;

  return (
    <div className="flex h-full flex-col">
      {/* Terminal title bar */}
      <div className="flex items-center justify-between px-4 py-2.5" style={{ borderBottom: `1px solid ${SOL.base02}` }}>
        <div className="flex items-center gap-2.5">
          {getStatusIcon(run.status, 'size-4')}
          <span className="font-medium text-sm" style={{ color: SOL.base1 }}>{run.displayName}</span>
          <Badge
            variant="outline"
            className="text-[10px] px-1.5 py-0 lowercase"
            style={{ borderColor: SOL.base02, color: SOL.base0 }}
          >
            {run.type}
          </Badge>
        </div>
        <WorkflowTemporalLink url={run.temporalUrl} className="text-[#586e75] hover:text-[#93a1a1]" />
      </div>

      {/* Terminal body */}
      <ScrollArea className="flex-1 min-h-0">
        <div className="p-4 font-mono text-sm">
          {/* Meta line */}
          <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs mb-4 pb-3" style={{ color: SOL.base01, borderBottom: `1px solid ${SOL.base02}80` }}>
            <span>
              <span style={{ color: SOL.base00 }}>started</span>{' '}
              <span style={{ color: SOL.base0 }}>
                {formatDistanceToNow(new Date(run.startedAt), { addSuffix: true })}
              </span>
            </span>
            <span>
              <span style={{ color: SOL.base00 }}>status</span>{' '}
              <span className={getStatusColor(run.status)}>
                {run.status.toLowerCase().replace('_', ' ')}
              </span>
            </span>
            {run.params?.tld && (
              <span>
                <span style={{ color: SOL.base00 }}>tld</span>{' '}
                <span style={{ color: SOL.base0 }}>.{run.params.tld.replace(/^\./, '')}</span>
              </span>
            )}
          </div>

          {/* Prompt indicator */}
          <div className="flex items-center gap-2 text-xs mb-3" style={{ color: SOL.base01 }}>
            <span style={{ color: SOL.green }}>❯</span>
            <span style={{ color: SOL.base0 }}>{run.type}</span>
            {run.status === 'RUNNING' && (
              <span className="inline-block w-2 h-4 animate-pulse rounded-sm" style={{ backgroundColor: SOL.base0 }} />
            )}
          </div>

          {/* WorkflowResult — existing component, just wrapped */}
          <div style={{ color: SOL.base0 }} className="[&_*]:border-[#073642] [&_table]:border-[#073642] [&_thead]:bg-[#073642]/50 [&_.text-muted-foreground]:text-[#586e75] [&_button]:border-[#073642]">
            <WorkflowResult
              workflowId={run.workflowId}
              workflowType={run.type}
              status={run.status}
              signalName={signalName}
              onStateLoaded={setRunningState}
            />
          </div>
        </div>
      </ScrollArea>
    </div>
  );
}

// =============================================================================
// Main Control Center Component
// =============================================================================

export function WorkflowControlCenter() {
  const { runs, modalOpen, setModalOpen, clearCompleted, runningCount, selectedRunId, selectRun } =
    useWorkflowStore();

  // Keep data fresh via polling
  useActiveWorkflows();

  const running = runningCount();

  // Auto-select first run if none selected or selected run was removed
  useEffect(() => {
    if (runs.length > 0 && (!selectedRunId || !runs.find((r) => r.workflowId === selectedRunId))) {
      selectRun(runs[0].workflowId);
    }
  }, [runs, selectedRunId, selectRun]);

  const selectedRun = runs.find((r) => r.workflowId === selectedRunId) ?? null;

  // Hide completely when there are no tracked workflows
  if (runs.length === 0) return null;

  return (
    <>
      {/* Collapsed pill — fixed in bottom-right */}
      {!modalOpen && (
        <button
          type="button"
          onClick={() => setModalOpen(true)}
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

      {/* Modal */}
      <Dialog open={modalOpen} onOpenChange={setModalOpen}>
        <DialogPrimitive.Portal>
          <DialogPrimitive.Overlay
            className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
          />
          <DialogPrimitive.Content
            className={cn(
              'fixed top-[50%] left-[50%] z-50 translate-x-[-50%] translate-y-[-50%]',
              'w-[calc(100vw-3rem)] max-w-5xl h-[calc(100vh-6rem)] max-h-[700px]',
              'rounded-xl shadow-2xl',
              'flex flex-col overflow-hidden',
              'data-[state=open]:animate-in data-[state=closed]:animate-out',
              'data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
              'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
              'duration-200'
            )}
            style={{ backgroundColor: `${SOL.base03}e0`, borderColor: SOL.base02, borderWidth: 1 }}
          >
            {/* Header */}
            <div className="flex items-center justify-between px-5 py-3" style={{ borderBottom: `1px solid ${SOL.base02}` }}>
              <div className="flex items-center gap-3">
                <Terminal className="size-5" style={{ color: SOL.base01 }} />
                <DialogTitle className="text-base font-semibold" style={{ color: SOL.base1 }}>
                  Workflow Control
                </DialogTitle>
                {running > 0 && (
                  <Badge className="bg-[#268bd2] text-white text-[10px] px-2 py-0 tabular-nums">
                    {running} running
                  </Badge>
                )}
              </div>
              <div className="flex items-center gap-1">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={clearCompleted}
                  disabled={!runs.some((r) => r.status !== 'RUNNING')}
                  className="text-[#586e75] hover:text-[#93a1a1] hover:bg-[#073642] text-xs gap-1.5"
                >
                  <Trash2 className="size-3" />
                  Clear Done
                </Button>
                <DialogPrimitive.Close className="rounded-sm p-1.5 transition-colors text-[#586e75] hover:text-[#93a1a1] hover:bg-[#073642]">
                  <X className="size-4" />
                  <span className="sr-only">Close</span>
                </DialogPrimitive.Close>
              </div>
            </div>

            {/* Body: split panel */}
            <div className="flex flex-1 min-h-0">
              {/* Left sidebar — run list */}
              <div className="w-56 shrink-0 overflow-y-auto" style={{ borderRight: `1px solid ${SOL.base02}` }}>
                {runs.map((run) => {
                  const isSelected = run.workflowId === selectedRunId;
                  return (
                    <button
                      key={run.workflowId}
                      type="button"
                      onClick={() => selectRun(run.workflowId)}
                      className="flex w-full items-start gap-2.5 px-3 py-3 text-left transition-colors"
                      style={{
                        borderBottom: `1px solid ${SOL.base02}80`,
                        backgroundColor: isSelected ? `${SOL.base02}cc` : 'transparent',
                      }}
                      onMouseEnter={(e) => { if (!isSelected) e.currentTarget.style.backgroundColor = `${SOL.base02}40`; }}
                      onMouseLeave={(e) => { if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent'; }}
                    >
                      <div className="mt-0.5 shrink-0">{getStatusIcon(run.status)}</div>
                      <div className="min-w-0 flex-1">
                        <p className="text-xs font-medium truncate" style={{ color: isSelected ? SOL.base1 : SOL.base0 }}>
                          {run.displayName}
                        </p>
                        <p className="text-[10px] mt-0.5" style={{ color: SOL.base01 }}>
                          {formatDistanceToNow(new Date(run.startedAt), { addSuffix: true })}
                        </p>
                      </div>
                    </button>
                  );
                })}
              </div>

              {/* Right panel — terminal-style result */}
              <div className="flex-1 min-w-0">
                {selectedRun ? (
                  <RunDetailPanel key={selectedRun.workflowId} run={selectedRun} />
                ) : (
                  <div className="flex h-full flex-col items-center justify-center text-sm gap-2" style={{ color: SOL.base01 }}>
                    <Rocket className="size-8 opacity-40" />
                    <p>Select a workflow</p>
                  </div>
                )}
              </div>
            </div>
          </DialogPrimitive.Content>
        </DialogPrimitive.Portal>
      </Dialog>
    </>
  );
}
