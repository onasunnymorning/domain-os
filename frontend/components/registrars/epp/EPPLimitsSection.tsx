"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { useAssignRegistrarEPPPolicy, useEPPPolicies, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPPolicy, EPPRegistrarPolicy } from "@/lib/types/epp";

function Limit({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <div className="space-y-1">
      <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className="text-lg font-medium">{value}</div>
      <div className="text-xs text-muted-foreground">{hint}</div>
    </div>
  );
}

export function EPPLimitsSection({
  clid,
  policy,
  isLoading,
}: {
  clid: string;
  policy?: EPPRegistrarPolicy;
  isLoading: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [choice, setChoice] = useState<string>("");
  const policies = useEPPPolicies(open);
  const assign = useAssignRegistrarEPPPolicy(clid);

  const current = policy?.policy;
  const limits = current?.limits;

  const submit = async () => {
    try {
      await assign.mutateAsync(choice === "default" ? "" : choice);
      toast.success(`This registrar is now on the ${choice || "default"} policy. Applies within about 30 seconds.`);
      setOpen(false);
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not assign the policy"));
    }
  };

  return (
    <Card id="epp-limits">
      <EPPSectionHeader
        title="Limits"
        description="How much this registrar may do over EPP: sessions, names per check, command rates and creates per day. The limits come from the policy it is assigned; a registrar that needs different limits gets a different policy, never a one-off override."
        action={
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setChoice(current?.name ?? "default");
              setOpen(true);
            }}
            disabled={isLoading}
          >
            Assign policy
          </Button>
        }
      />
      <CardContent className="space-y-6">
        {isLoading || !current || !limits ? (
          <Skeleton className="h-24 w-full" />
        ) : (
          <>
            <div className="space-y-1">
              <div className="text-sm">
                On the <span className="font-semibold">{current.name}</span> policy
                {current.description && <span className="text-muted-foreground"> · {current.description}</span>}
              </div>
              {policy?.family && (
                <div className="text-sm text-muted-foreground">
                  Member of family <span className="font-semibold text-foreground">{policy.family.name}</span>
                  {policy.family_policy && <> (family policy {policy.family_policy.name})</>}: its sessions and
                  commands are also counted together with the other members.
                </div>
              )}
            </div>
            <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
              <Limit
                label="Sessions"
                value={`${limits.sessions_per_registrar}`}
                hint={`across all logins · ${limits.sessions_per_login} per login unless a login has its own cap`}
              />
              <Limit label="Names per check" value={`${limits.check_names}`} hint="in one check command" />
              <Limit
                label="Creates per day"
                value={limits.creates_per_day === 0 ? "unlimited" : `${limits.creates_per_day}`}
                hint="successful domain creates, per UTC day"
              />
              <Limit
                label="Command rates"
                value={`${limits.rates.transform.per_second}/s`}
                hint={`changes · ${limits.rates.check.per_second} names/s checked · ${limits.rates.info.per_second}/s reads`}
              />
            </div>
          </>
        )}
      </CardContent>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Assign a policy</DialogTitle>
            <DialogDescription>
              Takes effect within about 30 seconds, for the next command and the next login. Sessions already open
              stay open.
            </DialogDescription>
          </DialogHeader>
          {policies.isLoading ? (
            <Skeleton className="h-10 w-full" />
          ) : (
            <Select value={choice} onValueChange={setChoice}>
              <SelectTrigger aria-label="Policy">
                <SelectValue placeholder="Choose a policy" />
              </SelectTrigger>
              <SelectContent>
                {(policies.data ?? []).map((p: EPPPolicy) => (
                  <SelectItem key={p.name} value={p.name}>
                    <span className="font-medium">{p.name}</span>
                    {p.description && <span className="text-muted-foreground"> · {p.description}</span>}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={assign.isPending}>
              Cancel
            </Button>
            <Button onClick={submit} disabled={assign.isPending || !choice || choice === current?.name}>
              {assign.isPending ? "Assigning…" : "Assign"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
