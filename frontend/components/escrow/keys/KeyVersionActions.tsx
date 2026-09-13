'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Download, Loader2, MoreHorizontal } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  activateEscrowKeyVersion,
  deactivateEscrowKeyVersion,
  destroyEscrowKeyVersion,
  escrowKeyErrorMessage,
  getEscrowPublicKey,
  probeEscrowKeyVersion,
  revokeEscrowKeyVersion,
  type EscrowKeyScope,
  type EscrowKeyVersion,
} from '@/lib/api/escrow-keys';

type Dialogs = 'none' | 'replace' | 'revoke' | 'destroy';

interface KeyVersionActionsProps {
  scope: EscrowKeyScope;
  version: EscrowKeyVersion;
  /** Another version of the same key is ACTIVE and the purpose allows only one. */
  replacesActive: boolean;
  manageable: boolean;
}

/**
 * Lifecycle actions for one key version. The server decides what is allowed;
 * the menu only offers what makes sense in the current state, and every
 * destructive step asks for confirmation proportionate to its consequences.
 */
export function KeyVersionActions({ scope, version, replacesActive, manageable }: KeyVersionActionsProps) {
  const queryClient = useQueryClient();
  const [dialog, setDialog] = useState<Dialogs>('none');
  const [reason, setReason] = useState('');
  const [compromised, setCompromised] = useState(false);
  const [fingerprint, setFingerprint] = useState('');

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['escrow-party', scope.tenantId ?? '', version.partyId] });
  const run = useMutation({
    mutationFn: (fn: () => Promise<unknown>) => fn(),
    onSuccess: () => {
      refresh();
      setDialog('none');
    },
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'The action failed')),
  });

  const download = async () => {
    try {
      const armored = await getEscrowPublicKey(scope, version.id);
      const url = URL.createObjectURL(new Blob([armored], { type: 'application/pgp-keys' }));
      const a = document.createElement('a');
      a.href = url;
      a.download = `${version.fingerprint}.asc`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      toast.error(escrowKeyErrorMessage(err, 'Could not download the public key'));
    }
  };

  const live = version.state === 'STAGED' || version.state === 'ACTIVE' || version.state === 'HISTORICAL';
  const needsProbe = version.material !== 'openpgp-public';
  const activate = () =>
    replacesActive ? setDialog('replace') : run.mutate(() => activateEscrowKeyVersion(scope, version.id));

  return (
    <>
      <div className="flex items-center justify-end gap-1">
        {version.hasPublicKey && (
          <Button variant="ghost" size="icon" title="Download public key" onClick={download}>
            <Download className="h-4 w-4" />
          </Button>
        )}
        {manageable && live && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label="Key version actions" disabled={run.isPending}>
                {run.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <MoreHorizontal className="h-4 w-4" />}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {needsProbe && (
                <DropdownMenuItem
                  onClick={() =>
                    run.mutate(async () => {
                      await probeEscrowKeyVersion(scope, version.id);
                      toast.success('Probe started. Refresh in a moment to see the result.');
                    })
                  }
                >
                  Probe on a worker
                </DropdownMenuItem>
              )}
              {version.state === 'STAGED' && (
                <DropdownMenuItem
                  disabled={needsProbe && !version.lastProbeOk}
                  onClick={activate}
                  title={needsProbe && !version.lastProbeOk ? 'Needs a successful probe first' : undefined}
                >
                  Activate
                </DropdownMenuItem>
              )}
              {version.state === 'ACTIVE' && (
                <DropdownMenuItem onClick={() => run.mutate(() => deactivateEscrowKeyVersion(scope, version.id))}>
                  Deactivate (keep for older deposits)
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-red-600" onClick={() => setDialog('revoke')}>
                Revoke…
              </DropdownMenuItem>
              {version.state !== 'ACTIVE' && (
                <DropdownMenuItem className="text-red-600" onClick={() => setDialog('destroy')}>
                  Destroy…
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        {manageable && version.state === 'REVOKED' && (
          <Button variant="ghost" size="sm" className="text-red-600" onClick={() => setDialog('destroy')}>
            Destroy…
          </Button>
        )}
      </div>

      <Dialog open={dialog === 'replace'} onOpenChange={(o) => !o && setDialog('none')}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Replace the active pseudonymisation key?</DialogTitle>
            <DialogDescription>
              The current version stops being used for new sanitized copies. Tokens in copies made from now on will
              not join with tokens in earlier copies. Runs already in progress keep the key they started with.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialog('none')}>
              Cancel
            </Button>
            <Button onClick={() => run.mutate(() => activateEscrowKeyVersion(scope, version.id, true))}>Replace</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={dialog === 'revoke'}
        onOpenChange={(o) => {
          if (!o) {
            setDialog('none');
            setReason('');
            setCompromised(false);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Revoke version {version.version}</DialogTitle>
            <DialogDescription>
              A revoked key is withdrawn from every use immediately, including validations already in progress. To
              stop using a key for new deposits only, deactivate it instead.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="revoke-reason">Reason</Label>
              <Input id="revoke-reason" value={reason} onChange={(e) => setReason(e.target.value)} />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={compromised} onCheckedChange={(v) => setCompromised(v === true)} />
              The private key may be known to someone else
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialog('none')}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={reason.trim() === ''}
              onClick={() => run.mutate(() => revokeEscrowKeyVersion(scope, version.id, reason.trim(), compromised))}
            >
              Revoke
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={dialog === 'destroy'}
        onOpenChange={(o) => {
          if (!o) {
            setDialog('none');
            setFingerprint('');
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Destroy version {version.version}</DialogTitle>
            <DialogDescription>
              The key material is deleted from the key store after its recovery window. Deposits that only this key
              can open will no longer open. Type the fingerprint to confirm.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-1.5">
            <Label htmlFor="destroy-fp" className="font-mono text-xs">
              {version.fingerprint}
            </Label>
            <Input id="destroy-fp" className="font-mono" value={fingerprint} onChange={(e) => setFingerprint(e.target.value)} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialog('none')}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={fingerprint.replace(/\s/g, '').toUpperCase() !== version.fingerprint}
              onClick={() => run.mutate(() => destroyEscrowKeyVersion(scope, version.id, fingerprint))}
            >
              Destroy
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
