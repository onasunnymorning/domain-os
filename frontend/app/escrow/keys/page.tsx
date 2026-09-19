'use client';

import { Suspense, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  ChevronRight,
  Inbox,
  Loader2,
  Lock,
  Plus,
  Waypoints,
} from 'lucide-react';
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
import { partyReadinessFromList, type Readiness } from '@/lib/escrow/readiness';
import { cn } from '@/lib/utils';

/** What can be added, in the words of the thing being added rather than its record. */
type NewParty = 'receiving-identity' | 'source';

/**
 * Escrow setup, arranged by the direction deposits travel.
 *
 * Sources send deposits to us; our own identities are what those deposits are
 * encrypted to; destinations will receive the deposits we send. Keys hang off
 * each of those, which is the only place they mean anything — which TLD uses
 * which source is a separate question, answered under defaults and overrides.
 *
 * One glyph carries the direction in each section and the words say it again,
 * so the page can be read either way round: the arrows are never the only
 * thing carrying the meaning.
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

        {!chosen ? null : isLoading ? (
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
          <div>
            <Section
              glyph={{
                Icon: ArrowDownToLine,
                className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
              }}
              title="Escrow sources"
              chip={{
                label: 'deposits arrive here',
                className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
              }}
              description="Organisations and systems that send deposits to us. We hold the public key each one signs with, so we can check that a deposit really came from them."
              action={{ label: 'Add source', onClick: () => setCreating('source') }}
            >
              <PartyRows
                parties={groups.sources}
                scopeQuery={scopeQuery}
                Icon={ArrowDownToLine}
                ready="Their verification key is on file. Deposits can be checked."
                empty="No sources yet. Add the organisation that will deposit with you."
              />
            </Section>

            <Section
              glyph={{ Icon: Lock, className: 'border-transparent bg-foreground text-background' }}
              title="Our receiving identities"
              chip={{ label: 'private half stays here', className: 'border-border bg-muted text-muted-foreground' }}
              description="The identities deposits are encrypted to. Their private decryption keys stay in the key store; the matching public key is what a source needs from us."
              action={
                platform
                  ? { label: 'Add receiving identity', onClick: () => setCreating('receiving-identity') }
                  : undefined
              }
            >
              <PartyRows
                parties={groups.ourIdentities}
                scopeQuery={scopeQuery}
                Icon={Lock}
                ready="The public half is ready to hand to a source."
                empty={
                  platform
                    ? 'No receiving identity yet. Deposits cannot be opened until one exists.'
                    : 'None of your own. Deposits are received by a platform identity.'
                }
              />
            </Section>

            <Section
              last
              muted
              glyph={{ Icon: ArrowUpFromLine, className: 'border-dashed border-border bg-muted/40 text-muted-foreground' }}
              title="Escrow destinations"
              chip={{ label: 'deposits would leave here', className: 'border-border bg-muted text-muted-foreground' }}
              badge="Not built yet"
            >
              <DestinationsPlaceholder />
            </Section>
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

/**
 * One direction of travel, hung off a spine. The dotted line is what makes the
 * three sections read as one journey — deposits arrive, are opened by an
 * identity we hold, and will one day leave again — rather than three unrelated
 * lists that happen to share a page.
 */
function Section({
  glyph,
  title,
  chip,
  badge,
  description,
  action,
  muted,
  last,
  children,
}: {
  glyph: { Icon: typeof Lock; className: string };
  title: string;
  chip: { label: string; className: string };
  badge?: string;
  description?: string;
  action?: { label: string; onClick: () => void };
  muted?: boolean;
  last?: boolean;
  children: React.ReactNode;
}) {
  const { Icon } = glyph;
  return (
    <section className={cn('flex gap-4', !last && 'pb-5')}>
      <div className="flex w-10 shrink-0 flex-col items-center">
        <span className={cn('flex h-10 w-10 items-center justify-center rounded-xl border', glyph.className)}>
          <Icon className="h-5 w-5" aria-hidden />
        </span>
        {!last && <span className="mt-2 w-px flex-1 border-l border-dashed border-muted-foreground/40" aria-hidden />}
      </div>

      <div className="flex min-w-0 flex-1 flex-col gap-3">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <h2
            className={cn(
              'text-[13px] font-bold uppercase tracking-[0.08em]',
              muted && 'text-muted-foreground'
            )}
          >
            {title}
          </h2>
          <span className={cn('rounded-full border px-2.5 py-0.5 text-[11px] font-medium', chip.className)}>
            {chip.label}
          </span>
          {badge && (
            <span className="rounded-full border border-border bg-muted px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground">
              {badge}
            </span>
          )}
          {action && (
            <Button size="sm" variant="outline" onClick={action.onClick} className="ml-auto gap-1.5">
              <Plus className="h-4 w-4" />
              {action.label}
            </Button>
          )}
        </div>
        {description && <p className="max-w-3xl text-sm leading-relaxed text-muted-foreground">{description}</p>}
        {children}
      </div>
    </section>
  );
}

function PartyRows({
  parties,
  scopeQuery,
  Icon,
  ready,
  empty,
}: {
  parties: EscrowParty[];
  scopeQuery: string;
  Icon: typeof Lock;
  /** What to say on a row that is working, where readiness has nothing to add. */
  ready: string;
  empty: string;
}) {
  if (parties.length === 0) return <Empty>{empty}</Empty>;
  return (
    <ul className="flex flex-col gap-2">
      {parties.map((p) => {
        const readiness = partyReadinessFromList(p);
        const look = ROW_LOOK[rowTone(readiness)];
        return (
          <li key={p.id}>
            <Link
              href={`/escrow/keys/${p.id}${scopeQuery}`}
              className={cn('flex items-center gap-4 rounded-xl border p-4 transition-colors', look.row)}
            >
              <span className={cn('flex h-9 w-9 shrink-0 items-center justify-center rounded-lg', look.tile)}>
                <Icon className="h-4 w-4" aria-hidden />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-[15px] font-semibold">{p.name}</span>
                  <ReadinessBadge readiness={readiness} />
                  {p.owner.kind === 'platform' ? (
                    <Badge variant="secondary" className="text-[11px]">
                      {p.manageable ? 'platform' : 'provided by platform · read-only'}
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-[11px]">{p.owner.operator}</Badge>
                  )}
                </div>
                <p className="mt-1 text-xs text-muted-foreground">{readiness.detail ?? ready}</p>
              </div>
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * How alarming a row looks. Only two tones beyond neutral, because a page
 * where several things shout at once is a page nobody triages: something is
 * missing, or something is broken.
 */
type RowTone = 'neutral' | 'incomplete' | 'broken';

const ROW_LOOK: Record<RowTone, { row: string; tile: string }> = {
  neutral: {
    row: 'border-border bg-card hover:border-primary/40 hover:bg-muted/40',
    tile: 'bg-muted text-muted-foreground',
  },
  incomplete: {
    row: 'border-amber-500/30 bg-amber-500/[0.06] hover:bg-amber-500/10',
    tile: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
  },
  broken: {
    row: 'border-red-500/30 bg-red-500/[0.06] hover:bg-red-500/10',
    tile: 'bg-red-500/15 text-red-700 dark:text-red-400',
  },
};

function rowTone(readiness: Readiness): RowTone {
  switch (readiness.status) {
    case 'setup-incomplete':
      return 'incomplete';
    case 'action-required':
    case 'revoked':
      return 'broken';
    default:
      return 'neutral';
  }
}

/**
 * The unbuilt half, shown rather than hidden: an operator who has just read
 * the outbound rail above will come looking for it, and finding nothing at all
 * reads as something broken.
 */
function DestinationsPlaceholder() {
  return (
    <div className="flex flex-col gap-5 rounded-xl border border-dashed border-border bg-muted/20 p-5 lg:flex-row lg:items-center lg:gap-6">
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <p className="text-sm leading-relaxed">
          Destinations are organisations or systems that receive escrow deposits from us.
        </p>
        <p className="text-xs leading-relaxed text-muted-foreground">
          Sending deposits out is not available yet. When it opens, each destination will hold their public key — the
          mirror of a source holding ours.
        </p>
      </div>
      <span className="hidden w-px self-stretch bg-border lg:block" aria-hidden />
      <div className="flex shrink-0 flex-col gap-2 lg:w-96">
        <span className="text-[10px] font-bold uppercase tracking-[0.09em] text-muted-foreground">
          What a destination row will look like
        </span>
        <div className="flex items-center gap-3 rounded-xl border border-dashed border-border bg-card p-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-dashed border-border bg-muted/40 text-muted-foreground">
            <ArrowUpFromLine className="h-4 w-4" aria-hidden />
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-muted-foreground">Destination name</span>
              <span className="rounded-full border border-border bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
                Their public key on file
              </span>
            </div>
            <p className="mt-0.5 text-xs text-muted-foreground">we sign it, we seal it for them</p>
          </div>
        </div>
      </div>
    </div>
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
    <div className="rounded-xl border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
      {children}
    </div>
  );
}
