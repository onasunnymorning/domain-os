'use client';

import { AlertTriangle, CheckCircle2, HelpCircle, Loader2, ShieldAlert, XCircle } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

/**
 * Outcome vocabularies of both escrow workflows, rendered the same way.
 *
 * ERROR and QUARANTINED are deliberately not styled as failures. A FAIL is a
 * verdict about the deposit; an ERROR means the service could not reach one,
 * and a QUARANTINED derivative means the source carried something the profile
 * refuses to transform. Reading either as "the deposit is bad" is exactly the
 * confusion these colours exist to prevent.
 */
const OUTCOMES: Record<
  string,
  { icon: typeof CheckCircle2; className: string; hint: string }
> = {
  PASS: {
    icon: CheckCircle2,
    className: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30',
    hint: 'The deposit was validated and accepted.',
  },
  FAIL: {
    icon: XCircle,
    className: 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30',
    hint: 'The deposit was validated and rejected.',
  },
  ERROR: {
    icon: AlertTriangle,
    className: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30',
    hint: 'The service could not decide. This is about us, not about the deposit.',
  },
  QUARANTINED: {
    icon: ShieldAlert,
    className: 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/30',
    hint: 'The source carried something the sanitisation profile refuses to transform.',
  },
  RUNNING: {
    icon: Loader2,
    className: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30',
    hint: 'The run has not reached a terminal state yet.',
  },
};

export function EscrowOutcomeBadge({
  outcome,
  className,
}: {
  outcome: string;
  className?: string;
}) {
  const meta = OUTCOMES[outcome] ?? {
    icon: HelpCircle,
    className: 'bg-muted text-muted-foreground border-border',
    hint: '',
  };
  const Icon = meta.icon;
  return (
    <Badge
      variant="outline"
      title={meta.hint}
      className={cn('gap-1 font-medium', meta.className, className)}
    >
      <Icon className={cn('h-3 w-3', outcome === 'RUNNING' && 'animate-spin')} />
      {outcome}
    </Badge>
  );
}

/** Severity colouring, shared by the tally and the sample findings list. */
export function severityClass(severity: string): string {
  switch (severity) {
    case 'ERROR':
      return 'text-red-600 dark:text-red-400';
    case 'WARNING':
      return 'text-amber-600 dark:text-amber-400';
    case 'INFO':
      return 'text-blue-600 dark:text-blue-400';
    default:
      return 'text-muted-foreground';
  }
}
