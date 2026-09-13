'use client';

import { Suspense, useState } from 'react';
import Link from 'next/link';
import { useParams, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, History, Loader2, Plus } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { AddKeyVersionDialog } from '@/components/escrow/keys/AddKeyVersionDialog';
import { KeyStateBadge } from '@/components/escrow/keys/KeyStateBadge';
import { KeyVersionActions } from '@/components/escrow/keys/KeyVersionActions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  PURPOSE_LABELS,
  getEscrowParty,
  getEscrowPartyAudit,
  versionsFor,
  type EscrowKeyPurpose,
  type EscrowKeyScope,
  type EscrowKeyVersion,
  type EscrowPartyDetail,
} from '@/lib/api/escrow-keys';

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
          All parties
        </Link>
        {isLoading ? (
          <Spinner />
        ) : isError || !data ? (
          <p className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
            This party is not visible in this scope.
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
  return (
    <>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold">{party.name}</h1>
        <Badge variant="outline" className="font-mono text-xs">
          {party.kind} · {party.side}
        </Badge>
        {party.owner.kind === 'platform' ? (
          <Badge variant="secondary">platform</Badge>
        ) : (
          <Badge variant="outline">{party.owner.operator}</Badge>
        )}
        {!party.manageable && (
          <span className="text-sm text-muted-foreground">Read-only in this scope: it belongs to {party.owner.kind === 'platform' ? 'the platform' : party.owner.operator}.</span>
        )}
      </div>

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
        </TabsContent>

        <TabsContent value="usage" className="mt-6">
          {usedBy.length === 0 ? (
            <p className="text-sm text-muted-foreground">No arrangement in this scope uses this party yet.</p>
          ) : (
            <ul className="divide-y divide-border rounded-lg border">
              {usedBy.map((a) => (
                <li key={a.id} className="flex flex-wrap items-center gap-2 px-4 py-3 text-sm">
                  <Badge variant="outline">{a.level}</Badge>
                  <span className="font-medium">
                    {a.level === 'tld' ? `.${a.tld}` : a.level === 'operator' ? `${a.operator} default` : 'Platform default'}
                  </span>
                  <span className="text-muted-foreground">
                    as {a.depositorPartyId === party.id ? 'depositor' : 'receiver'} · revision {a.revision}
                  </span>
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

function KeyCard({
  scope,
  partyId,
  purpose,
  versions,
  manageable,
}: {
  scope: EscrowKeyScope;
  partyId: string;
  purpose: EscrowKeyPurpose;
  versions: EscrowKeyVersion[];
  manageable: boolean;
}) {
  const [adding, setAdding] = useState(false);
  const label = PURPOSE_LABELS[purpose];
  const hasActive = versions.some((v) => v.state === 'ACTIVE');
  const singleActive = purpose === 'pseudonymise';

  return (
    <section className="space-y-3 rounded-lg border border-border p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="font-semibold">{label.title}</h2>
          <p className="text-sm text-muted-foreground">{label.description}</p>
        </div>
        {manageable && (
          <Button size="sm" variant="outline" className="gap-1.5" onClick={() => setAdding(true)}>
            <Plus className="h-4 w-4" />
            {purpose === 'pseudonymise' ? (hasActive ? 'Rotate' : 'Generate') : hasActive ? 'Add next version' : 'Add key'}
          </Button>
        )}
      </div>

      {versions.length === 0 ? (
        <p className="text-sm text-muted-foreground">No versions yet.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="text-xs uppercase tracking-wide text-muted-foreground">
              <tr>
                <th className="py-2 pr-4 text-left font-medium">Version</th>
                <th className="py-2 pr-4 text-left font-medium">State</th>
                <th className="py-2 pr-4 text-left font-medium">Fingerprint</th>
                <th className="py-2 pr-4 text-left font-medium">Probe</th>
                <th className="py-2 pr-4 text-left font-medium">Lifecycle</th>
                <th className="py-2 text-right font-medium">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {versions.map((v) => (
                <tr key={v.id}>
                  <td className="py-2 pr-4 font-mono">v{v.version}</td>
                  <td className="py-2 pr-4">
                    <KeyStateBadge state={v.state} compromised={v.compromised} />
                  </td>
                  <td className="py-2 pr-4 font-mono text-xs break-all">{v.fingerprint}</td>
                  <td className="py-2 pr-4 text-xs">
                    {v.material === 'openpgp-public' ? (
                      <span className="text-muted-foreground">not needed</span>
                    ) : v.lastProbeAt ? (
                      <span className={v.lastProbeOk ? 'text-emerald-600' : 'text-red-600'}>
                        {v.lastProbeOk ? 'passed' : 'failed'} {new Date(v.lastProbeAt).toLocaleString()}
                      </span>
                    ) : (
                      <span className="text-muted-foreground">pending</span>
                    )}
                  </td>
                  <td className="py-2 pr-4 text-xs text-muted-foreground">
                    <Lifecycle v={v} />
                    {purpose !== 'pseudonymise' && scope.tenantId && (
                      <>
                        {' · '}
                        <Link
                          className="underline"
                          href={`/escrow/validations?tenantId=${encodeURIComponent(scope.tenantId)}&keyVersionId=${v.id}`}
                        >
                          runs
                        </Link>
                      </>
                    )}
                  </td>
                  <td className="py-2">
                    <KeyVersionActions
                      scope={scope}
                      version={v}
                      manageable={manageable}
                      replacesActive={singleActive && hasActive && v.state === 'STAGED'}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <AddKeyVersionDialog scope={scope} partyId={partyId} purpose={purpose} open={adding} onOpenChange={setAdding} />
    </section>
  );
}

function Lifecycle({ v }: { v: EscrowKeyVersion }) {
  const parts: string[] = [];
  if (v.activatedAt) parts.push(`activated ${new Date(v.activatedAt).toLocaleDateString()}`);
  if (v.deactivatedAt) parts.push(`deactivated ${new Date(v.deactivatedAt).toLocaleDateString()}`);
  if (v.revokedAt) parts.push(`revoked ${new Date(v.revokedAt).toLocaleDateString()}${v.revocationReason ? ` — ${v.revocationReason}` : ''}`);
  if (v.destroyedAt) parts.push(`destroyed ${new Date(v.destroyedAt).toLocaleDateString()}`);
  if (v.keyExpiresAt) parts.push(`key expires ${new Date(v.keyExpiresAt).toLocaleDateString()}`);
  if (parts.length === 0) parts.push(`added ${new Date(v.createdAt).toLocaleDateString()}`);
  return <>{parts.join(' · ')}</>;
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
