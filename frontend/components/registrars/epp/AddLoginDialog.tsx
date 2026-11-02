"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { useCreateEPPLogin, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPLoginRole } from "@/lib/types/epp";
import { cn } from "@/lib/utils";

const LOGIN_ID = /^[\x21-\x7e]{3,16}$/;

export function AddLoginDialog({
  clid,
  open,
  onOpenChange,
  onCreated,
}: {
  clid: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called with the new login ID, so the caller can go straight to adding a password. */
  onCreated: (loginId: string) => void;
}) {
  const create = useCreateEPPLogin(clid);
  const [loginId, setLoginId] = useState("");
  const [role, setRole] = useState<EPPLoginRole>("transform");
  const valid = LOGIN_ID.test(loginId);

  const reset = () => {
    setLoginId("");
    setRole("transform");
  };

  const submit = async () => {
    try {
      const l = await create.mutateAsync({ login_id: loginId, role });
      toast.success(`Login ${l.login_id} added. Now give it a password.`);
      onOpenChange(false);
      reset();
      onCreated(l.login_id);
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not add the login"));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v);
        if (!v) reset();
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Add a login</DialogTitle>
          <DialogDescription>
            The client ID the registrar&apos;s system sends when it logs in. Once added it cannot be renamed, only
            disabled.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="epp-login-id">Login ID</Label>
            <Input
              id="epp-login-id"
              value={loginId}
              onChange={(e) => setLoginId(e.target.value)}
              placeholder={clid}
              className="font-mono"
              autoFocus
            />
            <p className="text-xs text-muted-foreground">
              3 to 16 characters, no spaces, unique across all registrars. It may be this registrar&apos;s own client
              ID ({clid}).
            </p>
          </div>
          <div className="space-y-2">
            <Label>Role</Label>
            <RadioGroup value={role} onValueChange={(v) => setRole(v as EPPLoginRole)} className="gap-2">
              <label htmlFor="role-transform" className={cn("flex cursor-pointer gap-3 rounded-md border p-3", role === "transform" && "border-primary bg-accent/40")}>
                <RadioGroupItem id="role-transform" value="transform" className="mt-1" />
                <div>
                  <div className="font-medium">Transform</div>
                  <p className="text-sm text-muted-foreground">Can create, change, renew, delete and transfer domains, hosts and contacts.</p>
                </div>
              </label>
              <label htmlFor="role-readonly" className={cn("flex cursor-pointer gap-3 rounded-md border p-3", role === "readonly" && "border-primary bg-accent/40")}>
                <RadioGroupItem id="role-readonly" value="readonly" className="mt-1" />
                <div>
                  <div className="font-medium">Read-only</div>
                  <p className="text-sm text-muted-foreground">Can only check, read and poll. Anything that changes the registry is refused.</p>
                </div>
              </label>
            </RadioGroup>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={create.isPending}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={create.isPending || !valid}>
            {create.isPending ? "Adding…" : "Add login"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
