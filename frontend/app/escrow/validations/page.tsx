'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ChevronRight, FileSearch, Loader2 } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { OperatorScopeSelect } from '@/components/workflows/OperatorScopeSelect';
import { EscrowOutcomeBadge } from '@/components/escrow/EscrowOutcomeBadge';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { listEscrowValidations, type EscrowValidationRun } from '@/lib/api/escrow-runs';

const OUTCOMES = ['PASS', 'FAIL', 'ERROR', 'RUNNING'];

/**
 * Validation runs in one operator scope.
 *
 * Scope is not a filter here, it is the query: the endpoint is tenant-scoped
 * and derives everything from the X-Tenant-ID header (ADR-0006), so nothing
 * can be listed until an operator is chosen.
 */
export default function EscrowValidationsPage() {
  const [tenantId, setTenantId] = useState('');
  const [tld, setTld] = useState('');
  const [outcome, setOutcome] = useState('');

  const { data, isLoading, isError } = useQuery({
    queryKey: ['escrow-validations', tenantId, tld, outcome],
    queryFn: () =>
      listEscrowValidations(tenantId, {
        pagesize: 50,
        tld: tld.trim() || undefined,
        outcome: outcome || undefined,
      }),
    enabled: tenantId !== '',
  });

  const runs = data?.items ?? [];

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <FileSearch className="h-6 w-6" />
            Escrow validation runs
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Every deposit this registry has validated, with its findings and the documents
            it produced.
          </p>
        </div>

        <div className="grid gap-4 sm:grid-cols-3">
          <div className="space-y-1.5">
            <Label>Registry operator</Label>
            <OperatorScopeSelect value={tenantId} onChange={setTenantId} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="tld-filter">TLD</Label>
            <Input
              id="tld-filter"
              placeholder="All TLDs"
              value={tld}
              onChange={(e) => setTld(e.target.value)}
              disabled={!tenantId}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Outcome</Label>
            <Select
              value={outcome || 'ALL'}
              onValueChange={(v) => setOutcome(v === 'ALL' ? '' : v)}
              disabled={!tenantId}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ALL">All outcomes</SelectItem>
                {OUTCOMES.map((o) => (
                  <SelectItem key={o} value={o}>
                    {o}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        {!tenantId ? (
          <EmptyState>Choose a registry operator to list its validation runs.</EmptyState>
        ) : isLoading ? (
          <div className="flex justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : isError ? (
          <EmptyState>Could not load validation runs for this operator.</EmptyState>
        ) : runs.length === 0 ? (
          <EmptyState>No validation runs match these filters.</EmptyState>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="px-4 py-2.5 text-left font-medium">TLD</th>
                  <th className="px-4 py-2.5 text-left font-medium">Outcome</th>
                  <th className="px-4 py-2.5 text-left font-medium">Profile</th>
                  <th className="px-4 py-2.5 text-right font-medium">Findings</th>
                  <th className="px-4 py-2.5 text-left font-medium">Started</th>
                  <th className="w-8" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {runs.map((run) => (
                  <RunRow key={run.id} run={run} tenantId={tenantId} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </DashboardLayout>
  );
}

function RunRow({ run, tenantId }: { run: EscrowValidationRun; tenantId: string }) {
  const total = run.findingTally?.reduce((n, t) => n + t.count, 0) ?? run.findings.length;
  return (
    <tr className="hover:bg-muted/30">
      <td className="px-4 py-2.5 font-medium">{run.tld}</td>
      <td className="px-4 py-2.5">
        <div className="flex items-center gap-2">
          <EscrowOutcomeBadge outcome={run.outcome} />
          {run.verified && (
            <Badge
              variant="outline"
              title="Cryptographically verified: a signed deposit that passed."
              className="border-emerald-500/30 text-emerald-600 dark:text-emerald-400"
            >
              verified
            </Badge>
          )}
        </div>
      </td>
      <td className="px-4 py-2.5 font-mono text-xs text-muted-foreground">{run.profile}</td>
      <td className="px-4 py-2.5 text-right font-mono tabular-nums">
        {total.toLocaleString()}
      </td>
      <td className="px-4 py-2.5 text-muted-foreground">
        {new Date(run.startedAt).toLocaleString()}
      </td>
      <td className="px-4 py-2.5">
        <Link
          href={`/escrow/validations/${run.id}?tenantId=${encodeURIComponent(tenantId)}`}
          className="text-muted-foreground hover:text-foreground"
          aria-label={`Open validation run for ${run.tld}`}
        >
          <ChevronRight className="h-4 w-4" />
        </Link>
      </td>
    </tr>
  );
}

function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed border-border py-12 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
