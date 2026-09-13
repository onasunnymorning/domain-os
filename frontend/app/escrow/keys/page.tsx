'use client';

import { Suspense, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { ChevronRight, KeyRound, Loader2, Plus, Waypoints } from 'lucide-react';
import { DashboardLayout } from '@/components/layout/DashboardLayout';
import { KeyScopePicker } from '@/components/escrow/keys/KeyScopePicker';
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

/**
 * Escrow parties and their keys.
 *
 * Keys belong to parties — our own identities and the registry service
 * providers that deposit with us — never to a TLD. Which parties a TLD uses is
 * set under Arrangements.
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
  const [creating, setCreating] = useState<null | 'self-dea' | 'external-rsp'>(null);
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
              <KeyRound className="h-6 w-6" />
              Escrow keys
            </h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              Keys belong to parties: our own identities, and the registry service providers that send us deposits.
              A TLD only chooses which parties it uses.
            </p>
          </div>
          <Button variant="outline" asChild>
            <Link href={`/escrow/arrangements${scopeQuery}`} className="gap-1.5">
              <Waypoints className="h-4 w-4" />
              Arrangements
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

        {!chosen ? (
          <Empty>Choose a registry operator, or open the platform registry.</Empty>
        ) : isLoading ? (
          <div className="flex justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : error ? (
          <Empty>
            {isPermissionError(error) || platform
              ? 'The platform registry needs the escrow:platform-keys:admin permission.'
              : escrowKeyErrorMessage(error, 'Could not load the key registry.')}
          </Empty>
        ) : (
          <div className="space-y-8">
            <PartyGroup
              title="Our identities"
              description="Identities this installation operates. Their private keys live in the key store."
              parties={groups.ourIdentities}
              scopeQuery={scopeQuery}
              onAdd={platform ? () => setCreating('self-dea') : undefined}
              addLabel="Add DEA identity"
            />
            <PartyGroup
              title="Registry service providers"
              description="The providers that sign the deposits we receive. We only hold their public keys."
              parties={groups.registryProviders}
              scopeQuery={scopeQuery}
              onAdd={() => setCreating('external-rsp')}
              addLabel="Add provider"
            />
            <section className="space-y-2">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Escrow agents</h2>
              <p className="text-sm text-muted-foreground">
                Third-party escrow agents we deposit with arrive with escrow targets.
              </p>
            </section>
          </div>
        )}
      </div>

      <CreatePartyDialog
        scope={scope}
        kind={creating}
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
}: {
  title: string;
  description: string;
  parties: EscrowParty[];
  scopeQuery: string;
  onAdd?: () => void;
  addLabel: string;
}) {
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">{title}</h2>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        {onAdd && (
          <Button size="sm" variant="outline" onClick={onAdd} className="gap-1.5">
            <Plus className="h-4 w-4" />
            {addLabel}
          </Button>
        )}
      </div>
      {parties.length === 0 ? (
        <Empty>None yet.</Empty>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {parties.map((p) => (
            <li key={p.id}>
              <Link
                href={`/escrow/keys/${p.id}${scopeQuery}`}
                className="flex items-center justify-between gap-4 px-4 py-3 hover:bg-muted/40"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{p.name}</span>
                  <Badge variant="outline" className="font-mono text-[11px]">
                    {p.kind} · {p.side}
                  </Badge>
                  {p.owner.kind === 'platform' ? (
                    <Badge variant="secondary" className="text-[11px]">platform</Badge>
                  ) : (
                    <Badge variant="outline" className="text-[11px]">{p.owner.operator}</Badge>
                  )}
                  {!p.manageable && <span className="text-xs text-muted-foreground">read-only here</span>}
                </div>
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function CreatePartyDialog({
  scope,
  kind,
  onClose,
  queryKey,
}: {
  scope: EscrowKeyScope;
  kind: null | 'self-dea' | 'external-rsp';
  onClose: () => void;
  queryKey: string[];
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const mutation = useMutation({
    mutationFn: () =>
      createEscrowParty(scope, {
        name,
        kind: kind === 'self-dea' ? 'DEA' : 'RSP',
        side: kind === 'self-dea' ? 'self' : 'external',
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      setName('');
      onClose();
    },
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'Could not create the party')),
  });

  return (
    <Dialog open={kind !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{kind === 'self-dea' ? 'Add a DEA identity' : 'Add a registry service provider'}</DialogTitle>
          <DialogDescription>
            {kind === 'self-dea'
              ? 'An identity this installation operates to receive deposits, such as EVE.'
              : scope.tenantId
                ? `A provider that deposits for ${scope.tenantId}'s TLDs. Only ${scope.tenantId} will see it.`
                : 'A provider in the platform catalogue, usable by every operator.'}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5">
          <Label htmlFor="party-name">Name</Label>
          <Input id="party-name" value={name} onChange={(e) => setName(e.target.value)} />
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
