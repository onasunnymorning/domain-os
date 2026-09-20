'use client';

import { useQuery } from '@tanstack/react-query';
import {
  AlertTriangle,
  CheckCircle2,
  Circle,
  Loader2,
  Minus,
  XCircle,
  type LucideIcon,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import {
  getEscrowValidationSummary,
  type EscrowDeposit,
  type EscrowValidationRun,
} from '@/lib/api/escrow-runs';
import {
  depositHeading,
  depositKindLabel,
  formatUtc,
  objectCounts,
  pipelineSteps,
  reachedObjects,
  resendLabel,
  type ObjectCount,
  type PipelineStep,
  type StepState,
} from '@/lib/escrow/runOverview';
import { EscrowOutcomeBadge } from './EscrowOutcomeBadge';

/**
 * The top of a validation run: what deposit this is, how it fared, what was in
 * it, and how far through the pipeline it got. The detail sections stay below
 * for anyone who needs the digests and identifiers.
 *
 * Only the verdict is guaranteed. The object counts live in a separate
 * document, so they load on their own and their failure costs the reader the
 * tiles, not the page.
 */
export function EscrowRunOverview({
  run,
  deposit,
}: {
  run: EscrowValidationRun;
  deposit?: EscrowDeposit;
}) {
  const steps = pipelineSteps(run);

  return (
    <section
      aria-label="Deposit overview"
      className="space-y-5 rounded-xl border border-border bg-card p-5"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="font-mono text-xs text-muted-foreground">{run.tld}</p>
          <h1 className="text-2xl font-semibold">{depositHeading(run)}</h1>
          <p className="text-sm text-muted-foreground">
            {[
              depositKindLabel(run.rdeKind),
              `Resend: ${resendLabel(run.rdeResend)}`,
              deposit ? `Received ${formatUtc(deposit.receivedAt, false)}` : undefined,
            ]
              .filter(Boolean)
              .join(' · ')}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <EscrowOutcomeBadge outcome={run.outcome} className="px-3 py-1 text-sm" />
          <Badge variant="outline" className="font-mono text-xs">
            {run.profile}
          </Badge>
          {run.verified ? (
            <Badge
              variant="outline"
              className="border-emerald-500/30 text-emerald-600 dark:text-emerald-400"
            >
              cryptographically verified
            </Badge>
          ) : (
            <Badge
              variant="outline"
              title="Only a signed deposit that passed can be a verified pass."
              className="text-muted-foreground"
            >
              not verified
            </Badge>
          )}
        </div>
      </div>

      <ObjectCounts run={run} />

      <ol aria-label="Pipeline" className="flex flex-wrap gap-2">
        {steps.map((step) => (
          <StepChip key={step.key} step={step} />
        ))}
      </ol>
    </section>
  );
}

// =============================================================================
// Object counts
// =============================================================================

function ObjectCounts({ run }: { run: EscrowValidationRun }) {
  const key = run.summaryObjectKey;
  const { data, isLoading, isError } = useQuery({
    queryKey: ['escrow-validation-summary', key],
    queryFn: () => getEscrowValidationSummary(key!),
    enabled: Boolean(key),
    // A summary is written once and never changes, and a bucket that refuses
    // the browser will refuse again.
    staleTime: Infinity,
    retry: false,
  });

  if (isLoading) {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-5" aria-busy="true" aria-label="Loading object counts">
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className="h-16" />
        ))}
      </div>
    );
  }

  if (!key || isError || !data) {
    return (
      <Aside>
        {!key
          ? 'Object counts are not available for this run.'
          : 'Object counts could not be loaded. They are also in the findings summary document below.'}
      </Aside>
    );
  }

  const counts = data.deposit?.counts ?? [];
  if (counts.length === 0) {
    return (
      <Aside>
        {reachedObjects(run)
          ? 'No object counts were recorded for this run.'
          : `No objects were read: the run stopped at ${run.stageReached ?? 'an early stage'}, before the deposit's contents.`}
      </Aside>
    );
  }

  return (
    <dl className="grid grid-cols-2 gap-3 sm:grid-cols-5">
      {objectCounts(counts).map((c) => (
        <CountTile key={c.key} count={c} />
      ))}
    </dl>
  );
}

function CountTile({ count }: { count: ObjectCount }) {
  return (
    <div
      className={cn(
        'rounded-lg border px-3 py-2',
        count.mismatch ? 'border-amber-500/40 bg-amber-500/5' : 'border-border bg-muted/30'
      )}
      title={
        count.mismatch
          ? `The deposit header declares ${(count.declared ?? 0).toLocaleString()}, but ${count.observed.toLocaleString()} were found.`
          : undefined
      }
    >
      <dt className="text-xs uppercase tracking-wide text-muted-foreground">{count.label}</dt>
      <dd className="mt-0.5 text-xl font-semibold tabular-nums">
        {count.observed.toLocaleString()}
      </dd>
      {count.mismatch && (
        <p className="text-xs text-amber-600 dark:text-amber-400">
          header says {(count.declared ?? 0).toLocaleString()}
        </p>
      )}
    </div>
  );
}

function Aside({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-lg border border-dashed border-border px-3 py-2 text-sm text-muted-foreground">
      {children}
    </p>
  );
}

// =============================================================================
// Pipeline steps
// =============================================================================

const STATES: Record<
  StepState,
  { icon: LucideIcon; className: string; word: string }
> = {
  passed: {
    icon: CheckCircle2,
    className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
    word: 'passed',
  },
  failed: {
    icon: XCircle,
    className: 'border-red-500/30 bg-red-500/10 text-red-700 dark:text-red-400',
    word: 'failed',
  },
  // Amber, like the ERROR outcome: the service could not decide.
  undecided: {
    icon: AlertTriangle,
    className: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
    word: 'could not finish',
  },
  running: {
    icon: Loader2,
    className: 'border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-400',
    word: 'running',
  },
  'not-reached': {
    icon: Circle,
    className: 'border-dashed border-border text-muted-foreground',
    word: 'not reached',
  },
  'not-applicable': {
    icon: Minus,
    className: 'border-dashed border-border text-muted-foreground',
    word: 'not applicable',
  },
};

const plural = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`;

/** What to say beside the label: the finding counts if there are any, else the state. */
function stepDetail(step: PipelineStep): string | undefined {
  if (step.state === 'failed' || step.state === 'undecided') {
    return step.errors > 0 ? plural(step.errors, 'error') : STATES[step.state].word;
  }
  // A pass with warnings is still a pass, but the tick alone would hide them.
  if (step.state === 'passed') return step.warnings > 0 ? plural(step.warnings, 'warning') : undefined;
  if (step.state === 'not-applicable' || step.state === 'not-reached') return STATES[step.state].word;
  return undefined;
}

function StepChip({ step }: { step: PipelineStep }) {
  const meta = STATES[step.state];
  const Icon = meta.icon;
  const detail = stepDetail(step);
  // The icon carries the state visually, so the label must carry it for
  // everyone else — and carry the finding counts the detail text shows.
  const findings = [
    step.errors > 0 ? plural(step.errors, 'error') : undefined,
    step.warnings > 0 ? plural(step.warnings, 'warning') : undefined,
  ].filter(Boolean);
  return (
    <li
      title={step.hint}
      aria-label={`${step.label}: ${meta.word}${findings.length ? `, ${findings.join(', ')}` : ''}`}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium',
        meta.className
      )}
    >
      <Icon className={cn('h-4 w-4', step.state === 'running' && 'animate-spin')} aria-hidden />
      {step.label}
      {detail && <span className="text-xs font-normal opacity-80">· {detail}</span>}
    </li>
  );
}
