'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { AlertTriangle, Copy, Download, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { CopyButton } from '@/components/ui/copy-button';
import {
  getDefaultEscrowArrangement,
  getEscrowParty,
  getEscrowPublicKey,
  escrowKeyErrorMessage,
  type EscrowKeyScope,
} from '@/lib/api/escrow-keys';
import { saveArmoredKey } from '@/lib/escrow/publicKey';

/**
 * The other half of setting up a source: what the source needs from us.
 *
 * A source cannot encrypt a deposit until it has the public key of the
 * identity that will open it, and the person configuring the source is the one
 * who has to send it. Leaving them to work out which identity that is, find
 * its page and export the right version is where first-time setup stalls — so
 * it is answered here, next to the key they have just pasted.
 *
 * It shows the default for this scope. A top-level domain that overrides its
 * receiver is a rarity, and the card says so rather than pretending otherwise.
 */
export function ReceivingIdentityCard({ scope }: { scope: EscrowKeyScope }) {
  const scopeKey = scope.tenantId ?? 'platform';
  const arrangement = useQuery({
    queryKey: ['escrow-arrangement-default', scopeKey],
    queryFn: () => getDefaultEscrowArrangement(scope),
    retry: false,
  });
  const receiverId = arrangement.data?.receiverPartyId;
  const receiver = useQuery({
    queryKey: ['escrow-party', scope.tenantId ?? '', receiverId],
    queryFn: () => getEscrowParty(scope, receiverId!),
    enabled: Boolean(receiverId),
    retry: false,
  });

  const active = (receiver.data?.versions ?? []).find((v) => v.purpose === 'decrypt-inbound' && v.state === 'ACTIVE');
  const partyHref = `/escrow/keys/${receiverId}${scope.tenantId ? `?tenantId=${encodeURIComponent(scope.tenantId)}` : ''}`;
  const arrangementsHref = `/escrow/arrangements${scope.tenantId ? `?tenantId=${encodeURIComponent(scope.tenantId)}` : ''}`;

  const copyKey = async () => {
    if (!active) return;
    try {
      await navigator.clipboard.writeText(await getEscrowPublicKey(scope, active.id));
      toast.success('Public key copied. Send it to the source.');
    } catch (err) {
      toast.error(escrowKeyErrorMessage(err, 'Could not copy the public key'));
    }
  };

  const download = async () => {
    if (!active) return;
    try {
      saveArmoredKey(await getEscrowPublicKey(scope, active.id), active.fingerprint);
    } catch (err) {
      toast.error(escrowKeyErrorMessage(err, 'Could not download the public key'));
    }
  };

  return (
    <section className="space-y-3 rounded-lg border border-border p-4">
      <div className="max-w-2xl">
        <h2 className="font-semibold">What this source needs from us</h2>
        <p className="text-sm text-muted-foreground">
          Give the source the public key below. It uses that key to encrypt deposits so that only we can open them.
        </p>
      </div>

      {arrangement.isLoading || receiver.isLoading ? (
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      ) : !receiverId ? (
        <Warning>
          No identity is set to receive deposits in this scope, so there is no key to hand out yet and nothing could
          open a deposit that arrived.{' '}
          <Link href={arrangementsHref} className="font-medium underline underline-offset-2">
            Choose a receiving identity
          </Link>
          .
        </Warning>
      ) : !active ? (
        <Warning>
          <Link href={partyHref} className="font-medium underline underline-offset-2">
            {receiver.data?.party.name ?? 'The receiving identity'}
          </Link>{' '}
          has no active decryption key, so there is no key to hand out and deposits could not be opened.
        </Warning>
      ) : (
        <div className="space-y-2 rounded-md border border-border bg-muted/30 p-3">
          <p className="text-sm">
            Deposits are encrypted for{' '}
            <Link href={partyHref} className="font-medium hover:underline">
              {receiver.data?.party.name}
            </Link>
          </p>
          <div className="flex items-start gap-1.5">
            <code className="min-w-0 flex-1 break-all font-mono text-xs text-muted-foreground">{active.fingerprint}</code>
            <CopyButton value={active.fingerprint} size="icon-sm" variant="ghost" tooltip="Copy fingerprint" />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" className="h-7 gap-1.5 text-xs" onClick={copyKey}>
              <Copy className="h-3.5 w-3.5" />
              Copy public key
            </Button>
            <Button variant="outline" size="sm" className="h-7 gap-1.5 text-xs" onClick={download}>
              <Download className="h-3.5 w-3.5" />
              Download .asc
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            This key is not secret — it is meant to be handed out. Ask the source to confirm the fingerprint above
            once they have loaded it. Individual top-level domains can be set to use a different identity.
          </p>
        </div>
      )}
    </section>
  );
}

function Warning({ children }: { children: React.ReactNode }) {
  return (
    <p className="flex items-start gap-2 rounded-md border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-600 dark:text-amber-400">
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{children}</span>
    </p>
  );
}
