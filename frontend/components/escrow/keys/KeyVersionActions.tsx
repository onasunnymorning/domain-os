'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Download, Loader2, MoreHorizontal } from 'lucide-react';
import { Button } from '@/components/ui/button';
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
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import {
  activateEscrowKeyVersion,
  deactivateEscrowKeyVersion,
  destroyEscrowKeyVersion,
  escrowKeyErrorMessage,
  probeEscrowKeyVersion,
  revokeEscrowKeyVersion,
  type EscrowKeyScope,
  type EscrowKeyVersion,
} from '@/lib/api/escrow-keys';
import { downloadPublicKey } from '@/lib/escrow/publicKey';

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
 *
 * Retiring and revoking are kept apart on purpose. Retiring is the routine end
 * of a rotation and keeps older deposits readable; revoking is the emergency
 * one and takes effect even mid-run. They are one menu apart, so the copy has
 * to carry the difference.
 */
export function KeyVersionActions({ scope, version, replacesActive, manageable }: KeyVersionActionsProps) {
  const queryClient = useQueryClient();
  const [dialog, setDialog] = useState<Dialogs>('none');
  const [reason, setReason] = useState('');
  const [compromised, setCompromised] = useState<'yes' | 'no' | ''>('');
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
      await downloadPublicKey(scope, version.id, version.fingerprint);
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
                      toast.success('Testing the key. Refresh in a moment to see the result.');
                    })
                  }
                >
                  Test on a worker
                </DropdownMenuItem>
              )}
              {version.state === 'STAGED' && (
                <DropdownMenuItem
                  disabled={needsProbe && !version.lastProbeOk}
                  onClick={activate}
                  title={needsProbe && !version.lastProbeOk ? 'A worker has to read and use the key before it can be activated' : undefined}
                >
                  Activate
                </DropdownMenuItem>
              )}
              {version.state === 'ACTIVE' && (
                <DropdownMenuItem onClick={() => run.mutate(() => deactivateEscrowKeyVersion(scope, version.id))}>
                  Retire (keep for older deposits)
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
            setCompromised('');
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Revoke version {version.version}</DialogTitle>
            <DialogDescription>
              Revoking blocks this key immediately, everywhere, including validations already in progress. To stop
              using it for new deposits while older deposits it opened stay readable, retire it instead.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="revoke-reason">Reason</Label>
              <Input id="revoke-reason" value={reason} onChange={(e) => setReason(e.target.value)} />
              <p className="text-xs text-muted-foreground">Kept in the history, and visible to anyone reviewing this key later.</p>
            </div>
            {/* Asked as a question with two answers rather than a checkbox:
                whether the material is in someone else's hands decides what
                has to happen next, and an unticked box is not an answer. */}
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium">Is the key material compromised?</legend>
              <RadioGroup value={compromised} onValueChange={(v) => setCompromised(v === 'yes' ? 'yes' : 'no')} className="gap-2">
                <label className="flex items-start gap-2 text-sm" htmlFor="revoke-not-compromised">
                  <RadioGroupItem id="revoke-not-compromised" value="no" className="mt-0.5" />
                  <span>
                    No — withdrawing it as a precaution or because it is no longer used.
                  </span>
                </label>
                <label className="flex items-start gap-2 text-sm" htmlFor="revoke-compromised">
                  <RadioGroupItem id="revoke-compromised" value="yes" className="mt-0.5" />
                  <span>
                    Yes — it may be known to someone else. Whoever holds the matching key has to be told.
                  </span>
                </label>
              </RadioGroup>
            </fieldset>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialog('none')}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={reason.trim() === '' || compromised === ''}
              onClick={() => run.mutate(() => revokeEscrowKeyVersion(scope, version.id, reason.trim(), compromised === 'yes'))}
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
