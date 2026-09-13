'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowRight, Loader2 } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  getEffectiveEscrowArrangement,
  inheritedFromLabel,
  listEscrowParties,
  type EscrowArrangementSide,
  type EscrowParty,
} from '@/lib/api/escrow-keys';

/**
 * Who deposits for a TLD and who receives, after inheritance, and where each
 * side came from. Most TLDs set nothing and inherit both.
 */
export function EffectiveArrangementCard({ tenantId, tld }: { tenantId: string; tld: string }) {
  const effective = useQuery({
    queryKey: ['escrow-effective', tenantId, tld],
    queryFn: () => getEffectiveEscrowArrangement(tenantId, tld),
    enabled: Boolean(tenantId && tld),
    retry: false,
  });
  const parties = useQuery({
    queryKey: ['escrow-parties', tenantId],
    queryFn: () => listEscrowParties({ tenantId }, { pagesize: 200 }),
    enabled: Boolean(tenantId),
    retry: false,
  });
  const byId = new Map((parties.data?.items ?? []).map((p) => [p.id, p]));

  return (
    <Card>
      <CardHeader>
        <CardTitle>Escrow</CardTitle>
        <CardDescription>The parties on each side of this TLD&apos;s deposits.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {effective.isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : effective.isError ? (
          <p className="text-sm text-muted-foreground">The escrow arrangement could not be loaded.</p>
        ) : (
          <dl className="grid gap-4 sm:grid-cols-2">
            <Side label="Depositor" hint="Signs the deposits; we verify with its keys." side={effective.data?.depositor} byId={byId} tenantId={tenantId} />
            <Side label="Receiver" hint="Our identity the deposits are encrypted to." side={effective.data?.receiver} byId={byId} tenantId={tenantId} />
          </dl>
        )}
        <Link
          href={`/escrow/arrangements?tenantId=${encodeURIComponent(tenantId)}`}
          className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
        >
          Manage arrangements <ArrowRight className="h-3.5 w-3.5" />
        </Link>
      </CardContent>
    </Card>
  );
}

function Side({
  label,
  hint,
  side,
  byId,
  tenantId,
}: {
  label: string;
  hint: string;
  side?: EscrowArrangementSide;
  byId: Map<string, EscrowParty>;
  tenantId: string;
}) {
  const party = side ? byId.get(side.partyId) : undefined;
  return (
    <div className="space-y-1">
      <dt className="text-xs uppercase tracking-wide text-muted-foreground" title={hint}>
        {label}
      </dt>
      <dd className="text-sm">
        {side ? (
          <>
            <Link href={`/escrow/keys/${side.partyId}?tenantId=${encodeURIComponent(tenantId)}`} className="font-medium hover:underline">
              {party?.name ?? side.partyId}
            </Link>
            <span className="block text-xs text-muted-foreground">{inheritedFromLabel(side.from)}</span>
          </>
        ) : (
          <span className="text-muted-foreground">Not set</span>
        )}
      </dd>
    </div>
  );
}
