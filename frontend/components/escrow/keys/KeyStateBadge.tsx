'use client';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { EscrowKeyState } from '@/lib/api/escrow-keys';

/**
 * A key version's lifecycle, said the way an operator would say it.
 *
 * The distinctions matter and are easy to lose: retired is not broken — it
 * still opens deposits received before it was retired — and revoked is not
 * merely old, it stops being used everywhere at once (ADR-0009). The exact
 * state name stays available for support and audit work, in the tooltip and
 * next to the badge wherever technical detail is being shown.
 */
const STATES: Record<EscrowKeyState, { label: string; className: string; hint: string }> = {
  STAGED: {
    label: 'Added',
    className: 'bg-slate-500/10 text-slate-600 dark:text-slate-300 border-slate-500/30',
    hint: 'STAGED — added but not in use. Activate it once it is ready.',
  },
  ACTIVE: {
    label: 'Active',
    className: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30',
    hint: 'ACTIVE — used for new deposits.',
  },
  HISTORICAL: {
    label: 'Retired',
    className: 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/30',
    hint: 'HISTORICAL — no longer used for new deposits; still opens deposits received before it was retired.',
  },
  REVOKED: {
    label: 'Revoked',
    className: 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30',
    hint: 'REVOKED — withdrawn from every use, including runs already in progress.',
  },
  DESTROYED: {
    label: 'Destroyed',
    className: 'bg-zinc-500/10 text-zinc-500 border-zinc-500/30 line-through',
    hint: 'DESTROYED — the material has been deleted. The record is kept so earlier runs stay explainable.',
  },
};

export function KeyStateBadge({
  state,
  compromised,
  exact,
}: {
  state: EscrowKeyState;
  compromised?: boolean;
  /** Also print the registry's own state name, for technical and audit views. */
  exact?: boolean;
}) {
  const s = STATES[state];
  return (
    <span className="inline-flex items-center gap-1.5">
      <Badge variant="outline" title={s.hint} className={cn('text-[11px]', s.className)}>
        {s.label}
      </Badge>
      {exact && <span className="font-mono text-[10px] uppercase text-muted-foreground">{state}</span>}
      {compromised && (
        <Badge variant="outline" className="border-red-500/40 text-red-600 dark:text-red-400 text-[11px]">
          compromised
        </Badge>
      )}
    </span>
  );
}
