'use client';

import { useState } from 'react';
import { AlertCircle, ChevronDown, ChevronRight } from 'lucide-react';
import { cn } from '@/lib/utils';
import { severityClass } from './EscrowOutcomeBadge';
import type { EscrowFinding, EscrowFindingTally } from '@/lib/api/escrow-runs';

/**
 * The findings of one run: the exact per-code tally first, the individual
 * findings second.
 *
 * The ordering is the point. A deposit can produce tens of thousands of
 * findings while the run record keeps only the first thousand, so the list is
 * a sample and the tally is the fact. Leading with the list would invite
 * reading "1,000 findings" as the total.
 */
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

  const sorted = [...tally].sort((a, b) => b.count - a.count || a.code.localeCompare(b.code));

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
              The run record stops at 1,000 findings. The counts below are exact and include
              the {suppressed.toLocaleString()} that were not kept.
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
                <th className="px-3 py-2 text-left font-medium">Severity</th>
                <th className="px-3 py-2 text-left font-medium">Stage</th>
                <th className="px-3 py-2 text-right font-medium">Count</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {sorted.map((row) => (
                <tr key={`${row.code}-${row.stage}-${row.severity}`} className="hover:bg-muted/30">
                  <td className="px-3 py-2 font-mono text-xs">{row.code}</td>
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
              {findings.map((f, i) => (
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
