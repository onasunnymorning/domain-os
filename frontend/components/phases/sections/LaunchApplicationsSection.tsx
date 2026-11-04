'use client';

import { useState } from 'react';
import { toast } from 'sonner';
import { FileStack } from 'lucide-react';
import { Phase, LaunchApplication, LaunchApplicationStatus, APPLICATION_STATUSES, isFinalApplication } from '@/lib/types/phase';
import { useLaunchApplications, useDecideApplication } from '@/lib/hooks/useLaunchApplications';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';

interface LaunchApplicationsSectionProps {
  phase: Phase;
  tldName: string;
}

const statusStyle: Record<LaunchApplicationStatus, string> = {
  pendingValidation: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-amber-200 dark:border-amber-700/50',
  pendingAllocation: 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 border-blue-200 dark:border-blue-700/50',
  allocated: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-400 border-green-200 dark:border-green-800/50',
  rejected: 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-400 border-red-200 dark:border-red-700/50',
  invalid: 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-400 border-red-200 dark:border-red-700/50',
};

function quote(a: LaunchApplication): string {
  if (!a.Quote?.Currency) return '—';
  return `${(a.Quote.Amount / 100).toFixed(2)} ${a.Quote.Currency}`;
}

/**
 * The applications registrars filed in an application phase (ADR 0024 §3),
 * and the operator's decisions about them: validate (or not, with a reason),
 * allocate the name, reject with a reason. Allocation registers the name to
 * the applicant at the price quoted when it was filed and rejects the other
 * applications for the name; every applicant is told by poll message.
 */
export function LaunchApplicationsSection({ phase, tldName }: LaunchApplicationsSectionProps) {
  const [status, setStatus] = useState<LaunchApplicationStatus | ''>('');
  const [cursor, setCursor] = useState('');
  const [reasonFor, setReasonFor] = useState<{ id: string; action: 'reject' | 'invalid' } | null>(null);
  const [reason, setReason] = useState('');
  const page = useLaunchApplications(tldName, phase.name, status, cursor);
  const decide = useDecideApplication(tldName, phase.name);

  const apps = page.data?.Data ?? [];
  const nextCursor = page.data?.Meta?.PageCursor ?? '';

  const run = (decision: Parameters<typeof decide.mutate>[0], done: string) =>
    decide.mutate(decision, {
      onSuccess: () => {
        toast.success(done);
        setReasonFor(null);
        setReason('');
      },
      onError: (err: unknown) => {
        const e = err as { response?: { data?: { error?: string } } };
        toast.error(e.response?.data?.error ?? 'The decision was not recorded');
      },
    });

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <FileStack className="h-4 w-4 text-orange-600" />
          <span className="text-sm font-semibold">Applications</span>
        </div>
        <select
          aria-label="Status"
          value={status}
          onChange={(e) => { setStatus(e.target.value as LaunchApplicationStatus | ''); setCursor(''); }}
          className="px-2 py-1 border rounded text-xs bg-background"
        >
          <option value="">Every status</option>
          {APPLICATION_STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
      </div>

      {page.isLoading && <p className="text-xs text-muted-foreground">Loading applications…</p>}
      {page.error && <p className="text-xs text-red-700 dark:text-red-400">Could not read the applications.</p>}
      {!page.isLoading && !page.error && apps.length === 0 && (
        <p className="text-xs text-muted-foreground">No applications{status ? ` that are ${status}` : ''} in this phase.</p>
      )}

      {apps.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="text-xs">Name</TableHead>
              <TableHead className="text-xs">Registrar</TableHead>
              <TableHead className="text-xs">Status</TableHead>
              <TableHead className="text-xs">Quote</TableHead>
              <TableHead className="text-xs">Filed</TableHead>
              <TableHead className="text-xs text-right">Decision</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {apps.map((a) => (
              <TableRow key={a.ID}>
                <TableCell className="text-xs font-mono">
                  <div>{a.Name}</div>
                  <div className="text-[10px] text-muted-foreground">{a.ID}</div>
                </TableCell>
                <TableCell className="text-xs">{a.ClID}</TableCell>
                <TableCell>
                  <Badge variant="outline" className={cn('text-[10px]', statusStyle[a.Status])}>{a.Status}</Badge>
                  {a.Reason && <div className="text-[10px] text-muted-foreground mt-0.5">{a.Reason}</div>}
                </TableCell>
                <TableCell className="text-xs">{quote(a)} · {a.Years}y</TableCell>
                <TableCell className="text-xs">{new Date(a.CreatedAt).toLocaleDateString()}</TableCell>
                <TableCell className="text-right">
                  {isFinalApplication(a.Status) ? (
                    a.DomainRoID ? <span className="text-[10px] text-muted-foreground font-mono">{a.DomainRoID}</span> : null
                  ) : reasonFor?.id === a.ID ? (
                    <div className="flex items-center gap-1 justify-end">
                      <Input
                        aria-label="Reason"
                        value={reason}
                        onChange={(e) => setReason(e.target.value)}
                        placeholder="Reason"
                        className="h-7 w-40 text-xs"
                      />
                      <Button
                        size="sm"
                        variant="destructive"
                        className="h-7 text-xs"
                        disabled={!reason.trim() || decide.isPending}
                        onClick={() => run(
                          reasonFor.action === 'reject'
                            ? { kind: 'reject', applicationId: a.ID, reason: reason.trim() }
                            : { kind: 'validate', applicationId: a.ID, valid: false, reason: reason.trim() },
                          reasonFor.action === 'reject' ? `${a.Name}: application rejected.` : `${a.Name}: application invalid.`,
                        )}
                      >
                        Confirm
                      </Button>
                      <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => { setReasonFor(null); setReason(''); }}>Cancel</Button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-1 justify-end">
                      {a.Status === 'pendingValidation' && (
                        <>
                          <Button size="sm" variant="outline" className="h-7 text-xs" disabled={decide.isPending}
                            onClick={() => run({ kind: 'validate', applicationId: a.ID, valid: true }, `${a.Name}: application validated.`)}>
                            Valid
                          </Button>
                          <Button size="sm" variant="ghost" className="h-7 text-xs" disabled={decide.isPending}
                            onClick={() => { setReasonFor({ id: a.ID, action: 'invalid' }); setReason(''); }}>
                            Invalid…
                          </Button>
                        </>
                      )}
                      {a.Status === 'pendingAllocation' && (
                        <Button size="sm" className="h-7 text-xs" disabled={decide.isPending}
                          onClick={() => run({ kind: 'allocate', applicationId: a.ID }, `${a.Name} allocated to ${a.ClID}.`)}>
                          Allocate
                        </Button>
                      )}
                      <Button size="sm" variant="ghost" className="h-7 text-xs text-destructive" disabled={decide.isPending}
                        onClick={() => { setReasonFor({ id: a.ID, action: 'reject' }); setReason(''); }}>
                        Reject…
                      </Button>
                    </div>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {(cursor || nextCursor) && (
        <div className="flex items-center justify-end gap-2">
          {cursor && (
            <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => setCursor('')}>First page</Button>
          )}
          {nextCursor && apps.length >= 25 && (
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setCursor(nextCursor)}>Next page</Button>
          )}
        </div>
      )}

      <p className="text-[11px] text-muted-foreground">
        Allocating registers the name to the applicant at the price quoted when the application was filed and rejects the other applications for it. Every applicant is told by poll message.
      </p>
    </div>
  );
}
