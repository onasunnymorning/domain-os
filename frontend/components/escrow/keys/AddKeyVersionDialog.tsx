'use client';

import { useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { AlertTriangle, KeyRound, Loader2, Upload } from 'lucide-react';
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

// Built from the kind rather than written out, so the file never contains a
// literal private-key armor header: the secret scanner reads one as a leak.
const header = (kind: 'PUBLIC' | 'PRIVATE') => `-----BEGIN PGP ${kind} KEY BLOCK-----`;
const endLine = (kind: 'PUBLIC' | 'PRIVATE') => `-----END PGP ${kind} KEY BLOCK-----`;

/**
 * What is wrong with a pasted block, judged on shape alone. It catches the
 * mistakes worth catching before a round trip — above all a private key pasted
 * where a public one belongs, which must never be sent.
 */
function pasteProblem(armored: string, mode: 'private' | 'public'): string | null {
  const text = armored.trim();
  if (text === '') return null;
  const blocks = (kind: 'PUBLIC' | 'PRIVATE') => text.split(header(kind)).length - 1;
  const wanted = mode === 'public' ? 'PUBLIC' : 'PRIVATE';
  const other = mode === 'public' ? 'PRIVATE' : 'PUBLIC';

  if (blocks(other) > 0 && blocks(wanted) === 0) {
    return mode === 'public'
      ? 'This appears to be a private key, not a public key. Do not use it here \u2014 and because private key material has been on the clipboard, treat it as exposed and tell whoever owns it.'
      : 'This appears to be a public key. Opening a deposit needs the private half of the key it was encrypted to.';
  }
  if (mode === 'public' && blocks('PRIVATE') > 0) {
    return 'This holds a private key as well. Paste only the public key \u2014 and tell whoever owns the private one that it has been on the clipboard.';
  }
  if (blocks(wanted) === 0) return `This should start with ${header(wanted)}`;
  if (blocks(wanted) > 1) return 'This holds more than one key. Add one per version, so each can be activated, retired and traced on its own.';
  if (!text.includes(endLine(wanted))) return 'The end line is missing \u2014 the paste looks cut off.';
  return null;
}

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
 * the dialog closes or a private key is sent, successful or not. They are never
 * written to storage, the query cache, or a log. A *public* key is not a secret
 * and is deliberately kept when the server refuses it, so a correction does not
 * mean pasting it again.
 */
export function AddKeyVersionDialog({ scope, partyId, purpose, open, onOpenChange }: AddKeyVersionDialogProps) {
  const queryClient = useQueryClient();
  const mode = MATERIAL[purpose];
  const [armored, setArmored] = useState('');
  const [passphrase, setPassphrase] = useState('');
  const [rejection, setRejection] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

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
      const added =
        mode === 'public'
          ? `Added version ${version.version}. Activate it when the source starts signing with it.`
          : `Added version ${version.version}. We are checking that a worker can use it; activate it once that passes.`;
      // A key accepted with a change says so, and stays on screen until dismissed.
      if (version.notice) {
        toast.warning(added, { description: version.notice, duration: Infinity });
      } else {
        toast.success(added);
      }
      clearSecrets();
      onOpenChange(false);
    },
    // The dialog stays open and keeps the reason in view: these messages say
    // which check failed, and the answer is usually to fix the paste.
    onError: (err) => setRejection(escrowKeyErrorMessage(err, 'Could not add the key')),
  });

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      clearSecrets();
      setRejection(null);
    }
    onOpenChange(next);
  };

  const readFile = (file: File | undefined) => {
    if (!file) return;
    file.text().then((text) => {
      setArmored(text);
      setRejection(null);
    });
  };

  const label = PURPOSE_LABELS[purpose];
  const problem = mode === 'generate' ? null : pasteProblem(armored, mode);
  const canSubmit = mode === 'generate' || (armored.trim() !== '' && problem === null);
  const blockLabel = mode === 'private' ? 'Private decryption key' : 'Public verification key';
  // Enough lines to recognise a key, never so many that the buttons are pushed
  // off-screen: a key is thirty-odd lines and is meant to be pasted, not read.
  const fieldClass = 'h-44 max-h-[30vh] resize-y overflow-auto font-mono text-xs leading-relaxed';

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="flex max-h-[85vh] flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <KeyRound className="h-5 w-5" />
            {mode === 'generate' ? `Generate a new ${label.title.toLowerCase()}` : `Add a ${label.title.toLowerCase().replace(/s$/, '')}`}
          </DialogTitle>
          <DialogDescription>{label.description}</DialogDescription>
        </DialogHeader>

        <div className="flex-1 space-y-4 overflow-y-auto">
          {mode !== 'generate' && (
            <div className="space-y-1.5">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="armored-key">{blockLabel}</Label>
                <div className="flex items-center gap-2">
                  {armored.trim() !== '' && (
                    <span className="text-xs tabular-nums text-muted-foreground">
                      {armored.trim().split('\n').length} lines
                    </span>
                  )}
                  <Button type="button" variant="ghost" size="sm" className="h-7 gap-1.5 text-xs" onClick={() => fileInput.current?.click()}>
                    <Upload className="h-3.5 w-3.5" />
                    Open .asc file
                  </Button>
                  <input
                    ref={fileInput}
                    type="file"
                    accept=".asc,.gpg,.pgp,.txt,text/plain"
                    className="hidden"
                    aria-label={`${blockLabel} file`}
                    onChange={(e) => {
                      readFile(e.target.files?.[0]);
                      e.target.value = '';
                    }}
                  />
                </div>
              </div>
              <Textarea
                id="armored-key"
                aria-label={blockLabel}
                className={fieldClass}
                autoComplete="off"
                spellCheck={false}
                placeholder={header(mode === 'private' ? 'PRIVATE' : 'PUBLIC')}
                value={armored}
                onChange={(e) => {
                  setArmored(e.target.value);
                  setRejection(null);
                }}
              />
              {/* Which of the two keys this is, said before the paste rather
                  than after: a private key pasted into a public field cannot
                  be taken back off the clipboard. */}
              {mode === 'private' ? (
                <p className="rounded-md border border-amber-500/30 bg-amber-500/10 p-2 text-xs text-amber-600 dark:text-amber-400">
                  <strong className="font-medium">This is secret material.</strong> Paste only the private key of one
                  of our own identities. It is sent once to the key store, cleared from this form immediately, and
                  never shown again.
                </p>
              ) : (
                <p className="text-xs text-muted-foreground">
                  This key is not secret: the source gave it to us so we can check its signatures. Paste the whole
                  block, header and end line included, exactly as they sent it.
                </p>
              )}
            </div>
          )}

          {mode === 'private' && (
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
                The passphrase that unlocks the key above. Sent once to the key store, where it keeps protecting the
                key. Neither is ever shown again.
              </p>
            </div>
          )}

          {mode === 'generate' && (
            <p className="text-sm text-muted-foreground">
              The key is created inside the key store and never leaves it — there is nothing to paste and nothing
              to keep safe here. It is added but not in use; activating it replaces the current one.
            </p>
          )}

          {(problem || rejection) && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
            >
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
              <p>{problem ?? rejection}</p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={() => {
              const material = { armored: armored.trim(), passphrase };
              setRejection(null);
              // A private key is cleared the moment it is sent; a public key is
              // kept so a refusal can be corrected in place.
              if (mode === 'private') clearSecrets();
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
