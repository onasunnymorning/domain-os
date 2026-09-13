'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { KeyRound, Loader2 } from 'lucide-react';
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
import { Textarea } from '@/components/ui/textarea';
import {
  PURPOSE_LABELS,
  addEscrowPublicKey,
  escrowKeyErrorMessage,
  generateEscrowKey,
  importEscrowPrivateKey,
  type EscrowKeyPurpose,
  type EscrowKeyScope,
} from '@/lib/api/escrow-keys';

const MATERIAL: Record<EscrowKeyPurpose, 'private' | 'public' | 'generate'> = {
  'decrypt-inbound': 'private',
  'verify-inbound': 'public',
  pseudonymise: 'generate',
};

interface AddKeyVersionDialogProps {
  scope: EscrowKeyScope;
  partyId: string;
  purpose: EscrowKeyPurpose;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Adds a key version. What it asks for follows from the purpose: a private key
 * and its passphrase, a public key, or nothing at all when the service
 * generates the key itself.
 *
 * Secret fields live only in this component's state and are cleared the moment
 * the dialog closes or the request is sent, successful or not. They are never
 * written to storage, the query cache, or a log.
 */
export function AddKeyVersionDialog({ scope, partyId, purpose, open, onOpenChange }: AddKeyVersionDialogProps) {
  const queryClient = useQueryClient();
  const mode = MATERIAL[purpose];
  const [armored, setArmored] = useState('');
  const [passphrase, setPassphrase] = useState('');

  const clearSecrets = () => {
    setArmored('');
    setPassphrase('');
  };

  // The material is passed as mutation variables, captured at submit time, so
  // the component state can be cleared immediately without the request ever
  // reading the cleared values.
  const mutation = useMutation({
    mutationFn: (material: { armored: string; passphrase: string }) => {
      switch (mode) {
        case 'private':
          return importEscrowPrivateKey(scope, partyId, {
            purpose,
            armoredPrivateKey: material.armored,
            passphrase: material.passphrase || undefined,
          });
        case 'public':
          return addEscrowPublicKey(scope, partyId, { purpose, armoredPublicKey: material.armored });
        default:
          return generateEscrowKey(scope, partyId, purpose);
      }
    },
    onSuccess: (version) => {
      queryClient.invalidateQueries({ queryKey: ['escrow-party', scope.tenantId ?? '', partyId] });
      toast.success(
        mode === 'public'
          ? `Added version ${version.version}. Activate it when the registry starts signing with it.`
          : `Added version ${version.version}. A worker is probing it; activate it once the probe passes.`
      );
      onOpenChange(false);
    },
    onError: (err) => toast.error(escrowKeyErrorMessage(err, 'Could not add the key')),
  });

  const handleOpenChange = (next: boolean) => {
    if (!next) clearSecrets();
    onOpenChange(next);
  };

  const label = PURPOSE_LABELS[purpose];
  const canSubmit = mode === 'generate' || armored.trim() !== '';

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <KeyRound className="h-5 w-5" />
            {mode === 'generate' ? `Generate a new ${label.title.toLowerCase()}` : `Add a ${label.title.toLowerCase().replace(/s$/, '')}`}
          </DialogTitle>
          <DialogDescription>{label.description}</DialogDescription>
        </DialogHeader>

        {mode === 'private' && (
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="armored-private">ASCII-armored private key</Label>
              <Textarea
                id="armored-private"
                className="min-h-40 font-mono text-xs"
                autoComplete="off"
                spellCheck={false}
                placeholder="-----BEGIN PGP PRIVATE KEY BLOCK-----"
                value={armored}
                onChange={(e) => setArmored(e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="passphrase">Passphrase</Label>
              <Input
                id="passphrase"
                type="password"
                autoComplete="new-password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Sent once to the key store with the key, which stays protected by it there. Neither is ever shown again.
              </p>
            </div>
          </div>
        )}

        {mode === 'public' && (
          <div className="space-y-1.5">
            <Label htmlFor="armored-public">ASCII-armored public key</Label>
            <Textarea
              id="armored-public"
              className="min-h-40 font-mono text-xs"
              spellCheck={false}
              placeholder="-----BEGIN PGP PUBLIC KEY BLOCK-----"
              value={armored}
              onChange={(e) => setArmored(e.target.value)}
            />
          </div>
        )}

        {mode === 'generate' && (
          <p className="text-sm text-muted-foreground">
            The key is created in the key store and never leaves it. It starts as a staged version; activating it
            replaces the current one.
          </p>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={() => {
              const material = { armored, passphrase };
              clearSecrets();
              mutation.mutate(material);
            }}
            disabled={!canSubmit || mutation.isPending}
          >
            {mutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {mode === 'generate' ? 'Generate' : mode === 'private' ? 'Import' : 'Add'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
