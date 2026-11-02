"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useUpdateEPPLogin, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPLogin, EPPLoginRole } from "@/lib/types/epp";

const MAX_CAP = 100;

export function EditLoginDialog({
  clid,
  login,
  onOpenChange,
}: {
  clid: string;
  login: EPPLogin | null;
  onOpenChange: (open: boolean) => void;
}) {
  const update = useUpdateEPPLogin(clid);
  const [role, setRole] = useState<EPPLoginRole>("transform");
  const [cap, setCap] = useState("0");

  useEffect(() => {
    if (login) {
      setRole(login.role);
      setCap(String(login.session_cap));
    }
  }, [login]);

  const capNum = Number(cap);
  const capValid = Number.isInteger(capNum) && capNum >= 0 && capNum <= MAX_CAP;

  const submit = async () => {
    if (!login) return;
    try {
      await update.mutateAsync({ login: login.login_id, body: { role, session_cap: capNum } });
      toast.success(`${login.login_id} updated.`);
      onOpenChange(false);
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not change the login"));
    }
  };

  return (
    <Dialog open={login !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Edit {login?.login_id}</DialogTitle>
          <DialogDescription>
            Changes apply to new logins. Sessions already open keep the role they logged in with until they end;
            to restrict one at once, end its session.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Role</Label>
            <Select value={role} onValueChange={(v) => setRole(v as EPPLoginRole)}>
              <SelectTrigger aria-label="Role">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="transform">Transform: may change the registry</SelectItem>
                <SelectItem value="readonly">Read-only: check, read and poll only</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="epp-session-cap">Session cap</Label>
            <Input
              id="epp-session-cap"
              type="number"
              min={0}
              max={MAX_CAP}
              value={cap}
              onChange={(e) => setCap(e.target.value)}
              className="max-w-[120px]"
            />
            <p className="text-xs text-muted-foreground">
              Sessions this one login may hold at once. 0 uses the policy&apos;s per-login limit; up to {MAX_CAP}.
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={update.isPending}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={update.isPending || !capValid}>
            {update.isPending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
