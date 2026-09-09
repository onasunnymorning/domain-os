'use client';

import { useState } from 'react';
import { AlertCircle, ChevronDown, ChevronRight } from 'lucide-react';
import { cn } from '@/lib/utils';
import { severityClass } from './EscrowOutcomeBadge';
import type { EscrowFinding, EscrowFindingTally } from '@/lib/api/escrow-runs';

/**
 * The findings of one run: the exact tally first, the individual findings
 * second.
 *
 * The ordering is the point. A deposit can produce tens of thousands of
 * findings while the run record keeps only worked examples, so the list is a
 * sample and the tally is the fact. Leading with the list would invite reading
 * its length as the total.
 *
 * Both are ordered by severity before count. A single ERROR among sixty
 * thousand warnings is what decided the run, and sorting by count alone puts
 * it at the bottom of the table.
 */
/** Loudest first. An unrecognised severity sorts last, never ahead of ERROR. */
function severityRank(severity: string): number {
  switch (severity) {
    case 'ERROR':
      return 0;
    case 'WARNING':
      return 1;
    case 'INFO':
      return 2;
    default:
      return 3;
  }
}

export function EscrowFindingsPanel({
  tally,
  findings,
  className,
}: {
  /** Exact counts. Empty for runs recorded before the tally existed. */
  tally: EscrowFindingTally[];
  /** The findings the run record retained. */
  findings: EscrowFinding[];
  className?: string;
}) {
  const [expanded, setExpanded] = useState(false);

  const tallied = tally.reduce((n, row) => n + row.count, 0);
  const total = tallied || findings.length;
  // The truncation notice is itself a finding but is not tallied, so retained
  // can exceed the tally by one on a truncated run. Never report a negative.
  const suppressed = Math.max(0, total - findings.length);

  // The retained findings are in document order, which on a deposit that trips
  // one warning per object means the ERROR is at the end. Lead with severity.
  const orderedFindings = [...findings].sort(
    (a, b) => severityRank(a.severity) - severityRank(b.severity)
  );

  if (total === 0) {
    return (
      <div
        className={cn(
          'rounded-lg border border-border bg-muted/30 p-4 text-sm text-muted-foreground',
          className
        )}
      >
        No findings. The deposit passed every check without comment.
      </div>
    );
  }

  const sorted = [...tally].sort(
    (a, b) =>
      severityRank(a.severity) - severityRank(b.severity) ||
      b.count - a.count ||
      a.code.localeCompare(b.code) ||
      (a.rule ?? '').localeCompare(b.rule ?? '') ||
      (a.objectType ?? '').localeCompare(b.objectType ?? '')
  );

  return (
    <div className={cn('space-y-4', className)}>
      {suppressed > 0 && (
        <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
          <div>
            <p className="font-medium text-amber-700 dark:text-amber-300">
              {total.toLocaleString()} findings, {findings.length.toLocaleString()} retained
            </p>
            <p className="text-muted-foreground">
              The run record keeps a limited number of examples of each finding. The counts
              below are exact and include the {suppressed.toLocaleString()} that were not kept.
            </p>
          </div>
        </div>
      )}

      {sorted.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="bg-muted/50 text-xs uppercase tracking-wide text-muted-foreground">
              <tr>
                <th className="px-3 py-2 text-left font-medium">Reason code</th>
                <th className="px-3 py-2 text-left font-medium">Object</th>
                <th className="px-3 py-2 text-left font-medium">What failed</th>
                <th className="px-3 py-2 text-left font-medium">Severity</th>
                <th className="px-3 py-2 text-left font-medium">Stage</th>
                <th className="px-3 py-2 text-right font-medium">Count</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {sorted.map((row) => (
                <tr
                  key={`${row.code}-${row.stage}-${row.severity}-${row.objectType ?? ''}-${row.rule ?? ''}`}
                  className="hover:bg-muted/30"
                >
                  <td className="px-3 py-2 font-mono text-xs">{row.code}</td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {row.objectType || <span className="text-muted-foreground/60">—</span>}
                  </td>
                  <td className="px-3 py-2">
                    {row.rule || <span className="text-muted-foreground/60">—</span>}
                  </td>
                  <td className={cn('px-3 py-2 font-medium', severityClass(row.severity))}>
                    {row.severity}
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">{row.stage}</td>
                  <td className="px-3 py-2 text-right font-mono tabular-nums">
                    {row.count.toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {findings.length > 0 && (
        <div className="rounded-lg border border-border">
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            className="flex w-full items-center gap-2 px-3 py-2 text-sm font-medium hover:bg-muted/30"
          >
            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            Individual findings ({findings.length.toLocaleString()}
            {suppressed > 0 ? ` of ${total.toLocaleString()}` : ''})
          </button>
          {expanded && (
            <ul className="max-h-96 divide-y divide-border overflow-y-auto border-t border-border">
              {orderedFindings.map((f, i) => (
                <li key={i} className="px-3 py-2 text-sm">
                  <div className="flex flex-wrap items-baseline gap-2">
                    <span className="font-mono text-xs">{f.code}</span>
                    <span className={cn('text-xs font-medium', severityClass(f.severity))}>
                      {f.severity}
                    </span>
                    <span className="text-xs text-muted-foreground">{f.stage}</span>
                  </div>
                  <p className="mt-0.5 text-muted-foreground">{f.message}</p>
                  {f.locator && (
                    <p className="mt-0.5 font-mono text-xs text-muted-foreground/70">{f.locator}</p>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
