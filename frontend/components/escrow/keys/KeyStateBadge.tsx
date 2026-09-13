'use client';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { EscrowKeyState } from '@/lib/api/escrow-keys';

/**
 * Key version lifecycle, rendered so HISTORICAL never reads as broken and
 * REVOKED never reads as merely old: "not for new deposits" and "unusable"
 * are different states (ADR-0009).
 */
const STATES: Record<EscrowKeyState, { className: string; hint: string }> = {
  STAGED: {
    className: 'bg-slate-500/10 text-slate-600 dark:text-slate-300 border-slate-500/30',
    hint: 'Registered but not in use. Activate it once it is ready.',
  },
  ACTIVE: {
    className: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30',
    hint: 'Used for new deposits.',
  },
  HISTORICAL: {
    className: 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/30',
    hint: 'No longer used for new deposits; still opens deposits received before it was deactivated.',
  },
  REVOKED: {
    className: 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30',
    hint: 'Withdrawn from every use, including runs already in progress.',
  },
  DESTROYED: {
    className: 'bg-zinc-500/10 text-zinc-500 border-zinc-500/30 line-through',
    hint: 'The material has been deleted. The record is kept so earlier runs stay explainable.',
  },
};

export function KeyStateBadge({ state, compromised }: { state: EscrowKeyState; compromised?: boolean }) {
  const s = STATES[state];
  return (
    <span className="inline-flex items-center gap-1.5">
      <Badge variant="outline" title={s.hint} className={cn('font-mono text-[11px]', s.className)}>
        {state}
      </Badge>
      {compromised && (
        <Badge variant="outline" className="border-red-500/40 text-red-600 dark:text-red-400 text-[11px]">
          compromised
        </Badge>
      )}
    </span>
  );
}
