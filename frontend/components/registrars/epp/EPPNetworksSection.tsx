"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { CredentialStateBadge } from "@/components/epp/CredentialStateBadge";
import type { RetireTarget } from "@/components/epp/RetireCredentialDialog";
import { useAddEPPAllowedNetwork, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPAllowedNetwork } from "@/lib/types/epp";
import { LastUsed } from "./LastUsed";

const MAX_USABLE = 20;

export function EPPNetworksSection({
  clid,
  networks,
  onRetire,
}: {
  clid: string;
  networks: EPPAllowedNetwork[];
  onRetire: (t: RetireTarget) => void;
}) {
  const add = useAddEPPAllowedNetwork(clid);
  const [open, setOpen] = useState(false);
  const [network, setNetwork] = useState("");
  const [note, setNote] = useState("");
  const usable = networks.filter((n) => n.state !== "retired").length;

  const submit = async () => {
    try {
      await add.mutateAsync({ network: network.trim(), note: note.trim() || undefined });
      toast.success(`${network.trim()} is allowed. EPP servers pick it up within about 30 seconds.`);
      setOpen(false);
      setNetwork("");
      setNote("");
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not add the network"));
    }
  };

  return (
    <Card id="epp-networks">
      <EPPSectionHeader
        title="Allowed networks"
        description="Connections are only accepted from these addresses. Everything else is dropped before the TLS handshake, so a registrar with no network here cannot log in at all."
        count={`${usable} of ${MAX_USABLE} usable`}
        action={
          <Button size="sm" onClick={() => setOpen(true)} disabled={usable >= MAX_USABLE}>
            Add network
          </Button>
        }
      />
      <CardContent>
        {networks.length === 0 ? (
          <div className="rounded-md border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">
            No allowed network yet. Until one is added, every connection from this registrar is refused.
          </div>
        ) : (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Network</TableHead>
                  <TableHead>Note</TableHead>
                  <TableHead>Last used</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead className="w-[100px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {networks.map((n) => (
                  <TableRow key={n.id} className={n.state === "retired" ? "text-muted-foreground" : undefined}>
                    <TableCell className="font-mono">{n.prefix}</TableCell>
                    <TableCell>{n.note || <span className="text-muted-foreground">–</span>}</TableCell>
                    <TableCell>
                      <LastUsed at={n.last_used_at} from={n.last_used_from} />
                    </TableCell>
                    <TableCell>
                      <CredentialStateBadge credential={n} />
                    </TableCell>
                    <TableCell className="text-right">
                      {n.retire_kind !== "compromise" && (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() =>
                            onRetire({
                              kind: "allowed_network",
                              id: n.id,
                              label: `network ${n.prefix}`,
                              alreadyRetired: n.state !== "active",
                            })
                          }
                        >
                          {n.state === "active" ? "Retire" : "Revoke"}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Allow a network</DialogTitle>
            <DialogDescription>
              A single IP address, or a range no wider than /24 (IPv4) or /48 (IPv6). Takes effect within about 30
              seconds.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="epp-network">Network</Label>
              <Input
                id="epp-network"
                placeholder="192.0.2.0/24"
                value={network}
                onChange={(e) => setNetwork(e.target.value)}
                className="font-mono"
                autoFocus
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="epp-network-note">Note (optional)</Label>
              <Input
                id="epp-network-note"
                placeholder="Production EPP client, Frankfurt"
                value={note}
                onChange={(e) => setNote(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={add.isPending}>
              Cancel
            </Button>
            <Button onClick={submit} disabled={add.isPending || !network.trim()}>
              {add.isPending ? "Adding…" : "Allow network"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
