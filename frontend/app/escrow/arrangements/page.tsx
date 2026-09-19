'use client';

import { Suspense, useEffect, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { KeyRound, Loader2, Trash2, Waypoints } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { KeyScopePicker } from '@/components/escrow/keys/KeyScopePicker';
import { MissingKeyNotice, PartySelect } from '@/components/escrow/keys/PartySelect';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  ARRANGEMENT_SIDE_PURPOSE,
  escrowKeyErrorMessage,
  getDefaultEscrowArrangement,
  groupParties,
  listEscrowParties,
  listTLDEscrowArrangements,
  removeDefaultEscrowArrangement,
  removeTLDEscrowArrangement,
  setDefaultEscrowArrangement,
  setTLDEscrowArrangement,
  type EscrowArrangement,
  type EscrowKeyScope,
  type EscrowParty,
} from '@/lib/api/escrow-keys';

/**
 * Where each top-level domain's deposits come from, and which of our
 * identities receives them.
 *
 * Both sides resolve the same way and independently — platform default, then
 * operator default, then an override for one top-level domain — so a domain
 * can take a different source while still being received by the platform
 * identity. Most set nothing here and inherit both.
 */
export default function EscrowArrangementsPage() {
  return (
    <Suspense fallback={null}>
      <Arrangements />
    </Suspense>
  );
}

function Arrangements() {
  const search = useSearchParams();
  const [tenantId, setTenantId] = useState(search.get('tenantId') ?? '');
  const [platform, setPlatform] = useState(false);
  const scope: EscrowKeyScope = platform ? {} : { tenantId };
  const chosen = platform || tenantId !== '';
  const scopeKey = platform ? 'platform' : tenantId;

  const parties = useQuery({
    queryKey: ['escrow-parties', scopeKey],
    queryFn: () => listEscrowParties(scope, { pagesize: 200 }),
    enabled: chosen,
    retry: false,
  });
  const groups = groupParties(parties.data?.items ?? []);

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-semibold">
              <Waypoints className="h-6 w-6" />
              Escrow defaults &amp; overrides
            </h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Which source sends the deposits for a top-level domain, and which of our identities receives them. Set
              the default once; override a single top-level domain only when it differs.
            </p>
          </div>
          <Button variant="outline" asChild>
            <Link href={platform ? '/escrow/keys' : `/escrow/keys?tenantId=${encodeURIComponent(tenantId)}`} className="gap-1.5">
              <KeyRound className="h-4 w-4" />
              Sources &amp; keys
            </Link>
          </Button>
        </div>

        <KeyScopePicker
          tenantId={tenantId}
          platform={platform}
          onChange={(next) => {
            setTenantId(next.tenantId);
            setPlatform(next.platform);
          }}
        />

        {!chosen ? null : parties.isError ? (
          <p className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
            {escrowKeyErrorMessage(parties.error, 'Could not load the escrow setup for this scope.')}
          </p>
        ) : parties.isLoading ? (
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        ) : (
          <>
            <DefaultArrangement scope={scope} scopeKey={scopeKey} platform={platform} depositors={groups.sources} receivers={groups.ourIdentities} />
            {!platform && <TLDOverrides tenantId={tenantId} depositors={groups.sources} receivers={groups.ourIdentities} />}
          </>
        )}
      </div>
    </DashboardLayout>
  );
}

/** Where to go to fix a party that has no key: its own page, in scope. */
function partyHref(partyId: string | undefined, tenantId?: string): string {
  const query = tenantId ? `?tenantId=${encodeURIComponent(tenantId)}` : '';
  return `/escrow/keys/${partyId ?? ''}${query}`;
}

function DefaultArrangement({
  scope,
  scopeKey,
  platform,
  depositors,
  receivers,
}: {
  scope: EscrowKeyScope;
  scopeKey: string;
  platform: boolean;
  depositors: EscrowParty[];
  receivers: EscrowParty[];
}) {
  const queryClient = useQueryClient();
  const current = useQuery({
    queryKey: ['escrow-arrangement-default', scopeKey],
    queryFn: () => getDefaultEscrowArrangement(scope),
    retry: false,
  });
  const [depositor, setDepositor] = useState<string | undefined>();
  const [receiver, setReceiver] = useState<string | undefined>();
  useEffect(() => {
    setDepositor(current.data?.depositorPartyId);
    setReceiver(current.data?.receiverPartyId);
  }, [current.data]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['escrow-arrangement-default', scopeKey] });
    queryClient.invalidateQueries({ queryKey: ['escrow-effective'] });
  };
  const save = useMutation({
    mutationFn: () => setDefaultEscrowArrangement(scope, { depositorPartyId: depositor, receiverPartyId: receiver }),
    onSuccess: () => {
      invalidate();
      toast.success('Default saved');
    },
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'Could not save the default')),
  });
  const remove = useMutation({
    mutationFn: () => removeDefaultEscrowArrangement(scope),
    onSuccess: () => {
      invalidate();
      toast.success('Default removed');
    },
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'Could not remove the default')),
  });

  return (
    <section className="space-y-3 rounded-lg border border-border p-4">
      <div>
        <h2 className="font-semibold">{platform ? 'Platform default' : 'Operator default'}</h2>
        <p className="text-sm text-muted-foreground">
          {platform
            ? 'Applies to every operator that sets nothing itself. Only platform-owned sources and identities can be used here.'
            : 'Applies to every top-level domain of this operator that sets nothing itself.'}
          {current.data && ` Revision ${current.data.revision}.`}
        </p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label>Deposits come from</Label>
          <PartySelect
            parties={depositors}
            value={depositor}
            onChange={setDepositor}
            inheritLabel={platform ? 'Not set' : 'Use the platform default'}
            needsPurpose={ARRANGEMENT_SIDE_PURPOSE.depositor}
          />
          <MissingKeyNotice
            party={depositors.find((p) => p.id === depositor)}
            purpose={ARRANGEMENT_SIDE_PURPOSE.depositor}
            side="depositor"
            href={partyHref(depositor, scope.tenantId)}
          />
        </div>
        <div className="space-y-1.5">
          <Label>Received by (one of our identities)</Label>
          <PartySelect
            parties={receivers}
            value={receiver}
            onChange={setReceiver}
            inheritLabel={platform ? 'Not set' : 'Use the platform default'}
            needsPurpose={ARRANGEMENT_SIDE_PURPOSE.receiver}
          />
          <MissingKeyNotice
            party={receivers.find((p) => p.id === receiver)}
            purpose={ARRANGEMENT_SIDE_PURPOSE.receiver}
            side="receiver"
            href={partyHref(receiver, scope.tenantId)}
          />
        </div>
      </div>
      <div className="flex gap-2">
        <Button size="sm" onClick={() => save.mutate()} disabled={(!depositor && !receiver) || save.isPending}>
          Save
        </Button>
        {current.data && (
          <Button size="sm" variant="outline" onClick={() => remove.mutate()} disabled={remove.isPending}>
            Remove default
          </Button>
        )}
      </div>
    </section>
  );
}

function TLDOverrides({ tenantId, depositors, receivers }: { tenantId: string; depositors: EscrowParty[]; receivers: EscrowParty[] }) {
  const queryClient = useQueryClient();
  const overrides = useQuery({
    queryKey: ['escrow-arrangement-tlds', tenantId],
    queryFn: () => listTLDEscrowArrangements(tenantId),
  });
  const [tld, setTld] = useState('');
  const [depositor, setDepositor] = useState<string | undefined>();
  const [receiver, setReceiver] = useState<string | undefined>();
  const names = new Map([...depositors, ...receivers].map((p) => [p.id, p.name]));

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['escrow-arrangement-tlds', tenantId] });
    queryClient.invalidateQueries({ queryKey: ['escrow-effective'] });
  };
  const add = useMutation({
    mutationFn: () => setTLDEscrowArrangement(tenantId, tld.trim(), { depositorPartyId: depositor, receiverPartyId: receiver }),
    onSuccess: () => {
      invalidate();
      setTld('');
      setDepositor(undefined);
      setReceiver(undefined);
    },
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'Could not save the override')),
  });
  const remove = useMutation({
    mutationFn: (name: string) => removeTLDEscrowArrangement(tenantId, name),
    onSuccess: invalidate,
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'Could not remove the override')),
  });

  const side = (id?: string) => (id ? names.get(id) ?? id : 'the default');

  return (
    <section className="space-y-3 rounded-lg border border-border p-4">
      <div>
        <h2 className="font-semibold">Overrides for one top-level domain</h2>
        <p className="text-sm text-muted-foreground">
          Only for a top-level domain whose source, or the identity that receives for it, differs from the default
          above. Each side is overridden on its own; the other keeps following the default.
        </p>
      </div>
      {(overrides.data?.items ?? []).length > 0 && (
        <ul className="divide-y divide-border rounded-md border">
          {(overrides.data?.items ?? []).map((a: EscrowArrangement) => (
            <li key={a.id} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-sm">
              <span>
                <span className="font-medium">.{a.tld}</span>
                <span className="text-muted-foreground">
                  {' '}— from {side(a.depositorPartyId)}, received by {side(a.receiverPartyId)}
                </span>
                <span className="text-xs text-muted-foreground"> · revision {a.revision}</span>
              </span>
              <Button variant="ghost" size="icon" aria-label={`Remove the override for ${a.tld}`} onClick={() => remove.mutate(a.tld ?? '')}>
                <Trash2 className="h-4 w-4" />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <div className="grid gap-3 sm:grid-cols-4 sm:items-end">
        <div className="space-y-1.5">
          <Label htmlFor="override-tld">Top-level domain</Label>
          <Input id="override-tld" placeholder="example" value={tld} onChange={(e) => setTld(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label>Deposits come from</Label>
          <PartySelect
            parties={depositors}
            value={depositor}
            onChange={setDepositor}
            inheritLabel="Use the operator default"
            needsPurpose={ARRANGEMENT_SIDE_PURPOSE.depositor}
          />
        </div>
        <div className="space-y-1.5">
          <Label>Received by</Label>
          <PartySelect
            parties={receivers}
            value={receiver}
            onChange={setReceiver}
            inheritLabel="Use the operator default"
            needsPurpose={ARRANGEMENT_SIDE_PURPOSE.receiver}
          />
        </div>
        <Button onClick={() => add.mutate()} disabled={!tld.trim() || (!depositor && !receiver) || add.isPending}>
          Set override
        </Button>
      </div>
    </section>
  );
}
