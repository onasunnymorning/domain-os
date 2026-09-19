'use client';

import { Suspense } from 'react';
import Link from 'next/link';
import { useParams, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, History, Loader2 } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { KeyCard } from '@/components/escrow/keys/KeyCard';
import { ReadinessLine } from '@/components/escrow/keys/ReadinessBadge';
import { ReceivingIdentityCard } from '@/components/escrow/keys/ReceivingIdentityCard';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  ROLE_SUMMARY,
  getEscrowParty,
  getEscrowPartyAudit,
  partyRole,
  versionsFor,
  type EscrowKeyScope,
  type EscrowPartyDetail,
} from '@/lib/api/escrow-keys';
import { partyReadiness } from '@/lib/escrow/readiness';

/**
 * One source, or one of our own identities.
 *
 * It leads with whether the relationship works and what is missing, because
 * that is what someone opening the page is here to find out. The record behind
 * it — versions, fingerprints, test results, arrangement revisions — is a
 * click away under each key's technical details and under the tabs.
 */
export default function EscrowPartyPage() {
  return (
    <Suspense fallback={<DashboardLayout><Spinner /></DashboardLayout>}>
      <PartyPage />
    </Suspense>
  );
}

function PartyPage() {
  const { id } = useParams<{ id: string }>();
  const tenantId = useSearchParams().get('tenantId') ?? '';
  const scope: EscrowKeyScope = tenantId ? { tenantId } : {};
  const { data, isLoading, isError } = useQuery({
    queryKey: ['escrow-party', tenantId, id],
    queryFn: () => getEscrowParty(scope, id),
    enabled: Boolean(id),
    retry: false,
  });
  const back = tenantId ? `/escrow/keys?tenantId=${encodeURIComponent(tenantId)}` : '/escrow/keys';

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <Link href={back} className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft className="h-4 w-4" />
          Escrow setup
        </Link>
        {isLoading ? (
          <Spinner />
        ) : isError || !data ? (
          <p className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
            This is not visible in this scope.
          </p>
        ) : (
          <Loaded scope={scope} detail={data} />
        )}
      </div>
    </DashboardLayout>
  );
}

function Loaded({ scope, detail }: { scope: EscrowKeyScope; detail: EscrowPartyDetail }) {
  const { party, versions, usedBy } = detail;
  const role = partyRole(party);
  const readiness = partyReadiness(party, versions);

  return (
    <>
      <header className="space-y-2">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-2xl font-semibold">{party.name}</h1>
          {party.owner.kind === 'platform' ? (
            <Badge variant="secondary">platform</Badge>
          ) : (
            <Badge variant="outline">{party.owner.operator}</Badge>
          )}
        </div>
        <p className="max-w-3xl text-sm text-muted-foreground">{ROLE_SUMMARY[role]}</p>
        <ReadinessLine readiness={readiness} />
        {!party.manageable && (
          // Spelled out rather than left to a disabled button: the reason a
          // control is missing is a permission, not a bug, and saying which
          // one saves a support round trip (§16 of the escrow UX brief).
          <p className="rounded-md border border-border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
            <span className="font-medium text-foreground">
              Provided by {party.owner.kind === 'platform' ? 'the platform' : party.owner.operator} · Read-only
            </span>{' '}
            — you can use it here, but its keys are changed by{' '}
            {party.owner.kind === 'platform' ? 'platform administrators' : party.owner.operator} only.
          </p>
        )}
      </header>

      <Tabs defaultValue="keys">
        <TabsList>
          <TabsTrigger value="keys">Keys</TabsTrigger>
          <TabsTrigger value="usage">Used by ({usedBy.length})</TabsTrigger>
          <TabsTrigger value="audit">History</TabsTrigger>
        </TabsList>

        <TabsContent value="keys" className="mt-6 space-y-6">
          {party.purposes.map((purpose) => (
            <KeyCard
              key={purpose}
              scope={scope}
              partyId={party.id}
              purpose={purpose}
              versions={versionsFor(versions, purpose)}
              manageable={party.manageable}
            />
          ))}
          {role === 'source' && <ReceivingIdentityCard scope={scope} />}
        </TabsContent>

        <TabsContent value="usage" className="mt-6">
          {usedBy.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Nothing uses it yet. Deposits start flowing through it once it is set as a default, or for one top-level
              domain, under defaults and overrides.
            </p>
          ) : (
            <ul className="divide-y divide-border rounded-lg border">
              {usedBy.map((a) => (
                <li key={a.id} className="flex flex-wrap items-center gap-2 px-4 py-3 text-sm">
                  <span className="font-medium">
                    {a.level === 'tld'
                      ? `.${a.tld}`
                      : a.level === 'operator'
                        ? `Default for ${a.operator}`
                        : 'Platform default'}
                  </span>
                  <span className="text-muted-foreground">
                    {a.depositorPartyId === party.id ? 'deposits come from here' : 'deposits are received here'}
                  </span>
                  <span className="text-xs text-muted-foreground">· revision {a.revision}</span>
                </li>
              ))}
            </ul>
          )}
        </TabsContent>

        <TabsContent value="audit" className="mt-6">
          <AuditTrail scope={scope} partyId={party.id} />
        </TabsContent>
      </Tabs>
    </>
  );
}

function AuditTrail({ scope, partyId }: { scope: EscrowKeyScope; partyId: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['escrow-party-audit', scope.tenantId ?? '', partyId],
    queryFn: () => getEscrowPartyAudit(scope, partyId, { pagesize: 100 }),
  });
  if (isLoading) return <Spinner />;
  const items = data?.items ?? [];
  if (items.length === 0) return <p className="text-sm text-muted-foreground">No history.</p>;
  return (
    <ol className="space-y-2">
      {items.map((e) => (
        <li key={e.id} className="flex flex-wrap items-baseline gap-2 text-sm">
          <History className="h-3.5 w-3.5 self-center text-muted-foreground" />
          <span className="text-muted-foreground">{new Date(e.at).toLocaleString()}</span>
          <span className="font-mono text-xs">{e.action}</span>
          {e.version ? <span>v{e.version}</span> : null}
          {e.stateBefore && e.stateAfter && e.stateBefore !== e.stateAfter && (
            <span className="text-muted-foreground">
              {e.stateBefore} → {e.stateAfter}
            </span>
          )}
          {e.reason && <span className="text-muted-foreground">({e.reason})</span>}
          {e.actor && <span className="text-muted-foreground">by {e.actor}</span>}
        </li>
      ))}
    </ol>
  );
}

function Spinner() {
  return (
    <div className="flex justify-center py-12">
      <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
    </div>
  );
}
