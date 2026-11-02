"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAddEPPPassword, eppErrorMessage } from "@/lib/hooks/useEPP";

// RFC 5730 pwType: 6 to 16 printable ASCII; no leading, trailing or repeated spaces.
const PASSWORD = /^(?! )(?!.* {2})[\x20-\x7e]{6,16}(?<! )$/;

/**
 * Adds a password to a login. The generated password is shown once; the
 * dialog cannot be dismissed by clicking outside or pressing Escape while it is
 * on screen, only by the "I have copied it" button.
 */
export function AddPasswordDialog({
  clid,
  login,
  onOpenChange,
}: {
  clid: string;
  login: string | null;
  onOpenChange: (open: boolean) => void;
}) {
  const add = useAddEPPPassword(clid);
  const [own, setOwn] = useState(false);
  const [password, setPassword] = useState("");
  const [shown, setShown] = useState<string | null>(null);

  const open = login !== null;
  const reset = () => {
    setOwn(false);
    setPassword("");
    setShown(null);
  };

  const submit = async () => {
    if (!login) return;
    try {
      const res = await add.mutateAsync({ login, body: own ? { password } : {} });
      if (res.password) {
        setShown(res.password);
      } else {
        toast.success(`Password added to ${login}.`);
        onOpenChange(false);
        reset();
      }
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not add the password"));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v && shown) return; // must press "I have copied it"
        onOpenChange(v);
        if (!v) reset();
      }}
    >
      <DialogContent
        className="max-w-md"
        onInteractOutside={(e) => shown && e.preventDefault()}
        onEscapeKeyDown={(e) => shown && e.preventDefault()}
        showCloseButton={!shown}
      >
        {shown ? (
          <>
            <DialogHeader>
              <DialogTitle>New password for {login}</DialogTitle>
              <DialogDescription>
                This is the only time it will be shown. Copy it now and give it to the registrar over a secure
                channel.
              </DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2 rounded-md border bg-muted p-3">
              <code className="flex-1 break-all font-mono text-base" data-testid="generated-password">
                {shown}
              </code>
              <CopyButton value={shown} tooltip="Copy password" showToast toastMessage="Password copied" />
            </div>
            <DialogFooter>
              <Button
                onClick={() => {
                  onOpenChange(false);
                  reset();
                }}
              >
                I have copied it
              </Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Add a password to {login}</DialogTitle>
              <DialogDescription>
                A login keeps up to two usable passwords, so the registrar can switch to the new one before the old
                one is retired.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" className="h-4 w-4" checked={own} onChange={(e) => setOwn(e.target.checked)} />
                Set my own instead of a generated one
              </label>
              {own ? (
                <div className="space-y-2">
                  <Label htmlFor="epp-password">Password</Label>
                  <Input
                    id="epp-password"
                    type="text"
                    autoComplete="off"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="font-mono"
                  />
                  <p className="text-xs text-muted-foreground">
                    6 to 16 printable ASCII characters, no leading, trailing or double spaces. A generated password is
                    stronger.
                  </p>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  The server will generate a 16-character password and show it once.
                </p>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => onOpenChange(false)} disabled={add.isPending}>
                Cancel
              </Button>
              <Button onClick={submit} disabled={add.isPending || (own && !PASSWORD.test(password))}>
                {add.isPending ? "Adding…" : own ? "Add password" : "Generate password"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
