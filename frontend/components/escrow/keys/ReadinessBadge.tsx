'use client';

import { AlertTriangle, Check, CircleSlash, Loader2, PlayCircle, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { Readiness, ReadinessStatus } from '@/lib/escrow/readiness';

/**
 * Colour carries the same meaning here as everywhere else in the product:
 * green works, amber is unfinished, red is broken. A rotation is deliberately
 * not amber — two active keys is how a correct rotation looks, and flagging it
 * as a problem would teach people to ignore the colour.
 */
const LOOK: Record<Exclude<ReadinessStatus, 'unknown'>, { className: string; Icon: typeof Check }> = {
  ready: { className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400', Icon: Check },
  rotating: { className: 'border-sky-500/30 bg-sky-500/10 text-sky-600 dark:text-sky-400', Icon: RefreshCw },
  'ready-to-activate': { className: 'border-sky-500/30 bg-sky-500/10 text-sky-600 dark:text-sky-400', Icon: PlayCircle },
  testing: { className: 'border-slate-500/30 bg-slate-500/10 text-slate-600 dark:text-slate-300', Icon: Loader2 },
  'setup-incomplete': { className: 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400', Icon: AlertTriangle },
  'action-required': { className: 'border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400', Icon: AlertTriangle },
  revoked: { className: 'border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400', Icon: CircleSlash },
};

export function ReadinessBadge({ readiness, className }: { readiness: Readiness; className?: string }) {
  if (readiness.status === 'unknown') return null;
  const { className: look, Icon } = LOOK[readiness.status];
  return (
    <Badge variant="outline" className={cn('gap-1 text-[11px] font-medium', look, className)} title={readiness.detail}>
      <Icon className={cn('h-3 w-3', readiness.status === 'testing' && 'animate-spin')} />
      {readiness.label}
    </Badge>
  );
}

/**
 * The badge with its reason spelled out, for a page that has room to say what
 * is missing rather than making the reader hover to find out.
 */
export function ReadinessLine({ readiness, className }: { readiness: Readiness; className?: string }) {
  if (readiness.status === 'unknown') return null;
  return (
    <div className={cn('flex flex-wrap items-center gap-2', className)}>
      <ReadinessBadge readiness={readiness} />
      {readiness.detail && <span className="text-sm text-muted-foreground">{readiness.detail}</span>}
    </div>
  );
}
