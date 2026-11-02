"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { useRetireEPPCredential, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPCredentialKind } from "@/lib/types/epp";
import { cn } from "@/lib/utils";

export interface RetireTarget {
  kind: EPPCredentialKind;
  id: string;
  /** Required for a password. */
  login?: string;
  /** What to call it in the dialog, e.g. "the password added Oct 3". */
  label: string;
  /** Already retiring or retired: only "revoke" still applies. */
  alreadyRetired?: boolean;
}

const nouns: Record<EPPCredentialKind, string> = {
  password: "password",
  certificate_pin: "certificate",
  allowed_network: "network",
};

/**
 * Two choices, one quiet and one loud, each saying in words what happens and
 * when. Maps to POST .../retire {"kind": "normal" | "compromise", "at"}.
 */
export function RetireCredentialDialog({
  clid,
  target,
  onOpenChange,
}: {
  clid: string;
  target: RetireTarget | null;
  onOpenChange: (open: boolean) => void;
}) {
  const retire = useRetireEPPCredential(clid);
  const [mode, setMode] = useState<"normal" | "compromise">("normal");
  const [earlier, setEarlier] = useState(false);
  const [at, setAt] = useState("");

  const open = target !== null;
  const noun = target ? nouns[target.kind] : "credential";
  const onlyRevoke = target?.alreadyRetired ?? false;
  const effectiveMode = onlyRevoke ? "compromise" : mode;

  const reset = () => {
    setMode("normal");
    setEarlier(false);
    setAt("");
  };

  const submit = async () => {
    if (!target) return;
    const body: { kind: "normal" | "compromise"; at?: string } = { kind: effectiveMode };
    if (effectiveMode === "normal" && earlier && at) {
      body.at = new Date(at).toISOString();
    }
    try {
      await retire.mutateAsync({ kind: target.kind, id: target.id, login: target.login, body });
      toast.success(
        effectiveMode === "compromise"
          ? `The ${noun} is revoked and its sessions are being ended.`
          : body.at
            ? `The ${noun} stops working at the time you chose.`
            : `The ${noun} keeps working for 7 days, then stops.`,
      );
      onOpenChange(false);
      reset();
    } catch (err) {
      toast.error(eppErrorMessage(err, `Could not retire the ${noun}`));
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
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {onlyRevoke ? `Revoke ${target?.label}` : `Retire ${target?.label}`}
          </DialogTitle>
          <DialogDescription>
            {onlyRevoke
              ? `This ${noun} is already retiring or retired. Marking it compromised ends every session that used it.`
              : `Choose how the ${noun} stops being accepted. Nothing is deleted: it stays on the list as a record.`}
          </DialogDescription>
        </DialogHeader>

        {!onlyRevoke && (
          <RadioGroup value={mode} onValueChange={(v) => setMode(v as "normal" | "compromise")} className="gap-3">
            <label
              htmlFor="retire-normal"
              className={cn(
                "flex cursor-pointer gap-3 rounded-md border p-3",
                mode === "normal" && "border-primary bg-accent/40",
              )}
            >
              <RadioGroupItem id="retire-normal" value="normal" className="mt-1" />
              <div className="space-y-1">
                <div className="font-medium">Retire</div>
                <p className="text-sm text-muted-foreground">
                  Keeps working for 7 more days, then stops. Sessions already open are not ended. Use this for a
                  planned rotation.
                </p>
                {mode === "normal" && (
                  <div className="pt-2 space-y-2" onClick={(e) => e.stopPropagation()}>
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={earlier}
                        onChange={(e) => setEarlier(e.target.checked)}
                        className="h-4 w-4"
                      />
                      Stop earlier, at a time I choose
                    </label>
                    {earlier && (
                      <Input
                        type="datetime-local"
                        value={at}
                        onChange={(e) => setAt(e.target.value)}
                        aria-label="Stop at"
                        className="max-w-xs"
                      />
                    )}
                  </div>
                )}
              </div>
            </label>

            <label
              htmlFor="retire-compromise"
              className={cn(
                "flex cursor-pointer gap-3 rounded-md border p-3",
                mode === "compromise" && "border-destructive bg-destructive/5",
              )}
            >
              <RadioGroupItem id="retire-compromise" value="compromise" className="mt-1" />
              <div className="space-y-1">
                <div className="font-medium text-destructive">Revoke, it is compromised</div>
                <p className="text-sm text-muted-foreground">
                  Stops now and ends every session that used it, within about 30 seconds. Use this when the {noun} has
                  leaked.
                </p>
              </div>
            </label>
          </RadioGroup>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={retire.isPending}>
            Cancel
          </Button>
          <Button
            variant={effectiveMode === "compromise" ? "destructive" : "default"}
            onClick={submit}
            disabled={retire.isPending || (effectiveMode === "normal" && earlier && !at)}
          >
            {retire.isPending ? "Working…" : effectiveMode === "compromise" ? "Revoke now" : "Retire"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
