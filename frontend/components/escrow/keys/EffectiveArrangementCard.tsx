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
 * Where this top-level domain's deposits come from and who receives them,
 * after inheritance, with the level each side was settled at. Most set nothing
 * and inherit both.
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
        <CardDescription>Who sends this top-level domain&apos;s deposits, and which of our identities receives them.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {effective.isLoading ? (
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        ) : effective.isError ? (
          <p className="text-sm text-muted-foreground">The escrow arrangement could not be loaded.</p>
        ) : (
          <dl className="grid gap-4 sm:grid-cols-2">
            <Side
              label="Deposits come from"
              hint="The source that signs the deposits. We check the signature with its public key."
              side={effective.data?.depositor}
              byId={byId}
              tenantId={tenantId}
            />
            <Side
              label="Received by"
              hint="Our identity the deposits are encrypted to, and whose private key opens them."
              side={effective.data?.receiver}
              byId={byId}
              tenantId={tenantId}
            />
          </dl>
        )}
        <Link
          href={`/escrow/arrangements?tenantId=${encodeURIComponent(tenantId)}`}
          className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
        >
          Change this, or the defaults it follows <ArrowRight className="h-3.5 w-3.5" />
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
          <span className="text-muted-foreground">Not set — deposits for this top-level domain cannot be processed</span>
        )}
      </dd>
    </div>
  );
}
