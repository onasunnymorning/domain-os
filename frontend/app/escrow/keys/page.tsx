'use client';

import { Suspense, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { ChevronRight, Inbox, Loader2, Plus, Send, Waypoints } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { EscrowBriefing } from '@/components/escrow/keys/EscrowBriefing';
import { KeyScopePicker } from '@/components/escrow/keys/KeyScopePicker';
import { ReadinessBadge } from '@/components/escrow/keys/ReadinessBadge';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  createEscrowParty,
  escrowKeyErrorMessage,
  groupParties,
  isPermissionError,
  listEscrowParties,
  type EscrowKeyScope,
  type EscrowParty,
} from '@/lib/api/escrow-keys';
import { partyReadinessFromList } from '@/lib/escrow/readiness';

/** What can be added, in the words of the thing being added rather than its record. */
type NewParty = 'receiving-identity' | 'source';

/**
 * Escrow setup, arranged by the direction deposits travel.
 *
 * Sources send deposits to us; our own identities are what those deposits are
 * encrypted to; destinations will receive the deposits we send. Keys hang off
 * each of those, which is the only place they mean anything — which TLD uses
 * which source is a separate question, answered under defaults and overrides.
 */
export default function EscrowKeysPage() {
  return (
    <Suspense fallback={null}>
      <EscrowKeys />
    </Suspense>
  );
}

function EscrowKeys() {
  const [tenantId, setTenantId] = useState(useSearchParams().get('tenantId') ?? '');
  const [platform, setPlatform] = useState(false);
  const [creating, setCreating] = useState<NewParty | null>(null);
  const scope: EscrowKeyScope = platform ? {} : { tenantId };
  const chosen = platform || tenantId !== '';

  const { data, isLoading, error } = useQuery({
    queryKey: ['escrow-parties', platform ? 'platform' : tenantId],
    queryFn: () => listEscrowParties(scope, { pagesize: 200 }),
    enabled: chosen,
    retry: false,
  });

  const groups = groupParties(data?.items ?? []);
  const scopeQuery = platform ? '' : `?tenantId=${encodeURIComponent(tenantId)}`;

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-semibold">
              <Inbox className="h-6 w-6" />
              Escrow setup
            </h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Who sends escrow deposits to us, which of our identities receives them, and the keys each side needs.
            </p>
          </div>
          <Button variant="outline" asChild>
            <Link href={`/escrow/arrangements${scopeQuery}`} className="gap-1.5">
              <Waypoints className="h-4 w-4" />
              Defaults &amp; overrides
            </Link>
          </Button>
        </div>

        <EscrowBriefing />

        <KeyScopePicker
          tenantId={tenantId}
          platform={platform}
          onChange={(next) => {
            setTenantId(next.tenantId);
            setPlatform(next.platform);
          }}
        />

        {!chosen ? (
          <Empty>Choose a registry operator, or open the platform registry.</Empty>
        ) : isLoading ? (
          <div className="flex justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <Empty>
            {/* Only a 403 is a permission problem: saying so for every failure
                sent an operator hunting for a missing role when the API was
                simply unreachable. */}
            {isPermissionError(error)
              ? platform
                ? 'The platform registry needs the escrow:platform-keys:admin permission.'
                : 'This registry needs the escrow:keys:admin permission.'
              : escrowKeyErrorMessage(error, 'Could not load the key registry.')}
          </Empty>
        ) : (
          <div className="space-y-8">
            <PartyGroup
              title="Escrow sources"
              description="Organisations and systems that send deposits to us. We hold the public key each one signs with, so we can check that a deposit really came from them."
              parties={groups.sources}
              scopeQuery={scopeQuery}
              onAdd={() => setCreating('source')}
              addLabel="Add source"
              empty="No sources yet. Add the organisation that will deposit with you."
            />
            <PartyGroup
              title="Our receiving identities"
              description="The identities deposits are encrypted to. Their private decryption keys stay in the key store; the matching public key is what a source needs from us."
              parties={groups.ourIdentities}
              scopeQuery={scopeQuery}
              onAdd={platform ? () => setCreating('receiving-identity') : undefined}
              addLabel="Add receiving identity"
              empty={
                platform
                  ? 'No receiving identity yet. Deposits cannot be opened until one exists.'
                  : 'None of your own. Deposits are received by a platform identity.'
              }
            />
            <section className="space-y-2">
              <h2 className="flex items-center gap-1.5 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                <Send className="h-3.5 w-3.5" />
                Escrow destinations
              </h2>
              <Empty>
                Destinations are organisations or systems that receive escrow deposits from us. Sending deposits out is
                not available yet.
              </Empty>
            </section>
          </div>
        )}
      </div>

      <CreatePartyDialog
        scope={scope}
        creating={creating}
        onClose={() => setCreating(null)}
        queryKey={['escrow-parties', platform ? 'platform' : tenantId]}
      />
    </DashboardLayout>
  );
}

function PartyGroup({
  title,
  description,
  parties,
  scopeQuery,
  onAdd,
  addLabel,
  empty,
}: {
  title: string;
  description: string;
  parties: EscrowParty[];
  scopeQuery: string;
  onAdd?: () => void;
  addLabel: string;
  empty: string;
}) {
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">{title}</h2>
          <p className="max-w-3xl text-sm text-muted-foreground">{description}</p>
        </div>
        {onAdd && (
          <Button size="sm" variant="outline" onClick={onAdd} className="gap-1.5">
            <Plus className="h-4 w-4" />
            {addLabel}
          </Button>
        )}
      </div>
      {parties.length === 0 ? (
        <Empty>{empty}</Empty>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {parties.map((p) => {
            const readiness = partyReadinessFromList(p);
            return (
              <li key={p.id}>
                <Link
                  href={`/escrow/keys/${p.id}${scopeQuery}`}
                  className="flex items-center justify-between gap-4 px-4 py-3 hover:bg-muted/40"
                >
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{p.name}</span>
                      <ReadinessBadge readiness={readiness} />
                      {p.owner.kind === 'platform' ? (
                        <Badge variant="secondary" className="text-[11px]">
                          {p.manageable ? 'platform' : 'provided by platform · read-only'}
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-[11px]">{p.owner.operator}</Badge>
                      )}
                    </div>
                    {readiness.detail && (
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">{readiness.detail}</p>
                    )}
                  </div>
                  <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function CreatePartyDialog({
  scope,
  creating,
  onClose,
  queryKey,
}: {
  scope: EscrowKeyScope;
  creating: NewParty | null;
  onClose: () => void;
  queryKey: string[];
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const identity = creating === 'receiving-identity';
  const mutation = useMutation({
    mutationFn: () =>
      createEscrowParty(scope, {
        name,
        kind: identity ? 'DEA' : 'RSP',
        side: identity ? 'self' : 'external',
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setName('');
      onClose();
    },
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'Could not create it')),
  });

  return (
    <Dialog open={creating !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{identity ? 'Add a receiving identity' : 'Add an escrow source'}</DialogTitle>
          <DialogDescription>
            {identity
              ? 'An identity this installation operates to receive deposits. Sources encrypt to its public key, and we open the deposits with the private half.'
              : scope.tenantId
                ? `The organisation or system that will send escrow deposits to ${scope.tenantId}'s top-level domains. Only ${scope.tenantId} will see it.`
                : 'The organisation or system that sends escrow deposits to us. Added here, every operator can use it.'}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5">
          <Label htmlFor="party-name">Name</Label>
          <Input
            id="party-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={identity ? 'Our escrow identity' : 'The organisation that deposits'}
          />
          <p className="text-xs text-muted-foreground">
            You add its keys next. Nothing is used for deposits until a key is added and activated.
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={() => mutation.mutate()} disabled={name.trim() === '' || mutation.isPending}>
            {mutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
