'use client';

import { useState } from 'react';
import Link from 'next/link';
import { toast } from 'sonner';
import { ChevronDown, Download, Plus } from 'lucide-react';
import { AddKeyVersionDialog } from '@/components/escrow/keys/AddKeyVersionDialog';
import { KeyStateBadge } from '@/components/escrow/keys/KeyStateBadge';
import { KeyVersionActions } from '@/components/escrow/keys/KeyVersionActions';
import { ReadinessLine } from '@/components/escrow/keys/ReadinessBadge';
import { Button } from '@/components/ui/button';
import { CopyButton } from '@/components/ui/copy-button';
import { cn } from '@/lib/utils';
import {
  PURPOSE_LABELS,
  escrowKeyErrorMessage,
  type EscrowKeyPurpose,
  type EscrowKeyScope,
  type EscrowKeyVersion,
} from '@/lib/api/escrow-keys';
import { keyReadiness } from '@/lib/escrow/readiness';
import { downloadPublicKey } from '@/lib/escrow/publicKey';

const date = (iso: string) => new Date(iso).toLocaleDateString();

/**
 * One key of a party: what it is for, whether it works, and the versions
 * behind it.
 *
 * Routine work needs one line — is this key doing its job, and if not, what is
 * missing — so that is what the card leads with. Fingerprints, probe results
 * and the full version timeline are what incident and audit work needs, and
 * they sit one click away rather than in front of every reader.
 */
export function KeyCard({
  scope,
  partyId,
  purpose,
  versions,
  manageable,
}: {
  scope: EscrowKeyScope;
  partyId: string;
  purpose: EscrowKeyPurpose;
  /** Every version of this purpose, newest first. */
  versions: EscrowKeyVersion[];
  manageable: boolean;
}) {
  const [adding, setAdding] = useState(false);
  const [details, setDetails] = useState(false);
  const label = PURPOSE_LABELS[purpose];
  const readiness = keyReadiness(purpose, versions);
  const active = versions.filter((v) => v.state === 'ACTIVE');
  const singleActive = purpose === 'pseudonymise';
  const addLabel = singleActive
    ? active.length > 0
      ? 'Rotate'
      : 'Generate'
    : active.length > 0
      ? 'Add replacement'
      : 'Add key';

  return (
    <section className="space-y-4 rounded-lg border border-border p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="max-w-2xl">
          <h2 className="font-semibold">{label.title}</h2>
          <p className="text-sm text-muted-foreground">{label.description}</p>
        </div>
        {manageable && (
          <Button size="sm" variant={active.length > 0 ? 'outline' : 'default'} className="gap-1.5" onClick={() => setAdding(true)}>
            <Plus className="h-4 w-4" />
            {addLabel}
          </Button>
        )}
      </div>

      <ReadinessLine readiness={readiness} />

      {active.map((v) => (
        <ActiveVersion key={v.id} scope={scope} version={v} purpose={purpose} />
      ))}

      {versions.length > 0 && (
        <div>
          <button
            type="button"
            onClick={() => setDetails((o) => !o)}
            aria-expanded={details}
            className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
          >
            <ChevronDown className={cn('h-4 w-4 transition-transform', details && 'rotate-180')} />
            Technical details and key history ({versions.length}{' '}
            {versions.length === 1 ? 'version' : 'versions'})
          </button>
          {details && <VersionTable scope={scope} purpose={purpose} versions={versions} manageable={manageable} />}
        </div>
      )}

      <AddKeyVersionDialog scope={scope} partyId={partyId} purpose={purpose} open={adding} onOpenChange={setAdding} />
    </section>
  );
}

/** The key that is doing the work, with the two things people come here for. */
function ActiveVersion({
  scope,
  version,
  purpose,
}: {
  scope: EscrowKeyScope;
  version: EscrowKeyVersion;
  purpose: EscrowKeyPurpose;
}) {
  const download = async () => {
    try {
      await downloadPublicKey(scope, version.id, version.fingerprint);
    } catch (err) {
      toast.error(escrowKeyErrorMessage(err, 'Could not download the public key'));
    }
  };

  return (
    <div className="space-y-2 rounded-md border border-border bg-muted/30 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm font-medium">
          In use{version.activatedAt ? ` since ${date(version.activatedAt)}` : ''}
        </span>
        {version.hasPublicKey && (
          <Button variant="outline" size="sm" className="h-7 gap-1.5 text-xs" onClick={download}>
            <Download className="h-3.5 w-3.5" />
            {purpose === 'decrypt-inbound' ? 'Download the public key to give out' : 'Download public key'}
          </Button>
        )}
      </div>
      <div className="flex items-start gap-1.5">
        <code className="min-w-0 flex-1 break-all font-mono text-xs text-muted-foreground">{version.fingerprint}</code>
        <CopyButton value={version.fingerprint} size="icon-sm" variant="ghost" tooltip="Copy fingerprint" />
      </div>
      <p className="text-xs text-muted-foreground">
        {purpose === 'verify-inbound'
          ? 'Check this fingerprint with the source over a channel you trust — a matching fingerprint is what proves you were given the right key.'
          : purpose === 'decrypt-inbound'
            ? 'Sources need the public key above to encrypt deposits for us. The private half never leaves the key store.'
            : 'Sanitized copies made from now on are tokenised with this version.'}
      </p>
    </div>
  );
}

function VersionTable({
  scope,
  purpose,
  versions,
  manageable,
}: {
  scope: EscrowKeyScope;
  purpose: EscrowKeyPurpose;
  versions: EscrowKeyVersion[];
  manageable: boolean;
}) {
  const hasActive = versions.some((v) => v.state === 'ACTIVE');
  return (
    <div className="mt-3 overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-xs uppercase tracking-wide text-muted-foreground">
          <tr>
            <th className="py-2 pr-4 text-left font-medium">Version</th>
            <th className="py-2 pr-4 text-left font-medium">State</th>
            <th className="py-2 pr-4 text-left font-medium">Fingerprint</th>
            <th className="py-2 pr-4 text-left font-medium">Test</th>
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
                <KeyStateBadge state={v.state} compromised={v.compromised} exact />
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
                  <span className="text-muted-foreground">not tested yet</span>
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
                  replacesActive={purpose === 'pseudonymise' && hasActive && v.state === 'STAGED'}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Lifecycle({ v }: { v: EscrowKeyVersion }) {
  const parts: string[] = [];
  if (v.activatedAt) parts.push(`activated ${date(v.activatedAt)}`);
  if (v.deactivatedAt) parts.push(`retired ${date(v.deactivatedAt)}`);
  if (v.revokedAt) parts.push(`revoked ${date(v.revokedAt)}${v.revocationReason ? ` — ${v.revocationReason}` : ''}`);
  if (v.destroyedAt) parts.push(`destroyed ${date(v.destroyedAt)}`);
  if (v.keyExpiresAt) parts.push(`key expires ${date(v.keyExpiresAt)}`);
  if (parts.length === 0) parts.push(`added ${date(v.createdAt)}`);
  return <>{parts.join(' · ')}</>;
}
