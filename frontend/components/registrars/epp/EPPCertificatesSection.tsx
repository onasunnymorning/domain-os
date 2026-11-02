"use client";

import { useRef, useState } from "react";
import { toast } from "sonner";
import { differenceInCalendarDays, format } from "date-fns";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { EPPSectionHeader } from "@/components/epp/EPPSectionHeader";
import { CredentialStateBadge } from "@/components/epp/CredentialStateBadge";
import type { RetireTarget } from "@/components/epp/RetireCredentialDialog";
import { useAddEPPCertificatePin, eppErrorMessage } from "@/lib/hooks/useEPP";
import type { EPPCertificatePin, EPPLogin } from "@/lib/types/epp";
import { cn } from "@/lib/utils";
import { LastUsed } from "./LastUsed";

const MAX_USABLE = 3;

function expiryText(notAfter: string, now = new Date()) {
  const days = differenceInCalendarDays(new Date(notAfter), now);
  if (days < 0) return { text: `expired ${-days} day${days === -1 ? "" : "s"} ago`, tone: "text-red-600" };
  if (days === 0) return { text: "expires today", tone: "text-red-600" };
  if (days <= 7) return { text: `expires in ${days} day${days === 1 ? "" : "s"}`, tone: "text-red-600" };
  if (days <= 30) return { text: `expires in ${days} days`, tone: "text-amber-600" };
  return { text: `expires in ${days} days`, tone: "text-muted-foreground" };
}

export function EPPCertificatesSection({
  clid,
  pins,
  logins,
  onRetire,
}: {
  clid: string;
  pins: EPPCertificatePin[];
  logins: EPPLogin[];
  onRetire: (t: RetireTarget) => void;
}) {
  const add = useAddEPPCertificatePin(clid);
  const [open, setOpen] = useState(false);
  const [pem, setPem] = useState("");
  const [restrict, setRestrict] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const fileRef = useRef<HTMLInputElement>(null);
  const usable = pins.filter((p) => p.state !== "retired").length;

  const readFile = (file: File | undefined) => {
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => setPem(String(reader.result ?? ""));
    reader.readAsText(file);
  };

  const reset = () => {
    setPem("");
    setRestrict(false);
    setSelected([]);
  };

  const submit = async () => {
    try {
      await add.mutateAsync({ certificate_pem: pem.trim(), logins: restrict && selected.length ? selected : undefined });
      toast.success("Certificate pinned. The registrar can use it as soon as it is valid.");
      setOpen(false);
      reset();
    } catch (err) {
      toast.error(eppErrorMessage(err, "Could not pin the certificate"));
    }
  };

  return (
    <Card id="epp-certificates">
      <EPPSectionHeader
        title="Certificates"
        description="A registrar must connect with one of these client certificates. Only the public key's fingerprint is kept, so a certificate renewed on the same key needs nothing here. Expired certificates are refused with no grace period."
        count={`${usable} of ${MAX_USABLE} usable`}
        action={
          <Button size="sm" onClick={() => setOpen(true)} disabled={usable >= MAX_USABLE}>
            Add certificate
          </Button>
        }
      />
      <CardContent>
        {pins.length === 0 ? (
          <div className="rounded-md border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">
            No certificate yet. Ask the registrar for their client certificate (PEM) and add it here.
          </div>
        ) : (
          <div className="rounded-md border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Subject</TableHead>
                  <TableHead>Issuer</TableHead>
                  <TableHead>Valid</TableHead>
                  <TableHead>Logins</TableHead>
                  <TableHead>Last used</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead className="w-[100px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {pins.map((p) => {
                  const exp = expiryText(p.not_after);
                  return (
                    <TableRow key={p.id} className={p.state === "retired" ? "text-muted-foreground" : undefined}>
                      <TableCell>
                        <div className="font-medium">{p.subject || <span className="text-muted-foreground">(no subject)</span>}</div>
                        <div className="font-mono text-xs text-muted-foreground" title={p.spki_sha256}>
                          key {p.spki_sha256.slice(0, 16)}…
                        </div>
                      </TableCell>
                      <TableCell className="max-w-[200px] truncate" title={p.issuer}>
                        {p.issuer || "–"}
                      </TableCell>
                      <TableCell>
                        <div className="whitespace-nowrap">
                          {format(new Date(p.not_before), "MMM d, yyyy")} → {format(new Date(p.not_after), "MMM d, yyyy")}
                        </div>
                        {p.state !== "retired" && <div className={cn("text-xs", exp.tone)}>{exp.text}</div>}
                      </TableCell>
                      <TableCell>
                        {p.logins && p.logins.length > 0 ? (
                          <span className="font-mono text-xs">{p.logins.join(", ")}</span>
                        ) : (
                          <span className="text-muted-foreground">All logins</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <LastUsed at={p.last_used_at} from={p.last_used_from} />
                      </TableCell>
                      <TableCell>
                        <CredentialStateBadge credential={p} notBefore={p.not_before} notAfter={p.not_after} />
                      </TableCell>
                      <TableCell className="text-right">
                        {p.retire_kind !== "compromise" && (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() =>
                              onRetire({
                                kind: "certificate_pin",
                                id: p.id,
                                label: `certificate ${p.subject || p.spki_sha256.slice(0, 12)}`,
                                alreadyRetired: p.state !== "active",
                              })
                            }
                          >
                            {p.state === "active" ? "Retire" : "Revoke"}
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog
        open={open}
        onOpenChange={(v) => {
          setOpen(v);
          if (!v) reset();
        }}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>Add a client certificate</DialogTitle>
            <DialogDescription>
              Paste the registrar&apos;s certificate in PEM form, or pick the file. Any CA or self-signed certificate
              works. An expired one is refused; one that is not valid yet is accepted, so it can be ready before the
              registrar switches.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="epp-pem">Certificate (PEM)</Label>
                <input
                  ref={fileRef}
                  type="file"
                  accept=".pem,.crt,.cer,.txt"
                  className="hidden"
                  onChange={(e) => readFile(e.target.files?.[0])}
                />
                <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()}>
                  Choose file…
                </Button>
              </div>
              <Textarea
                id="epp-pem"
                value={pem}
                onChange={(e) => setPem(e.target.value)}
                placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----"}
                className="min-h-[160px] font-mono text-xs"
              />
            </div>
            <div className="space-y-2">
              <label className="flex items-center gap-2 text-sm">
                <Checkbox checked={restrict} onCheckedChange={(v) => setRestrict(v === true)} />
                Only some logins may use it
              </label>
              {restrict && (
                <div className="ml-6 space-y-1">
                  {logins.length === 0 && (
                    <p className="text-sm text-muted-foreground">No logins yet. Leave this unchecked.</p>
                  )}
                  {logins.map((l) => (
                    <label key={l.login_id} className="flex items-center gap-2 font-mono text-sm">
                      <Checkbox
                        checked={selected.includes(l.login_id)}
                        onCheckedChange={(v) =>
                          setSelected((s) => (v === true ? [...s, l.login_id] : s.filter((x) => x !== l.login_id)))
                        }
                      />
                      {l.login_id}
                    </label>
                  ))}
                </div>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={add.isPending}>
              Cancel
            </Button>
            <Button
              onClick={submit}
              disabled={add.isPending || !pem.includes("BEGIN CERTIFICATE") || (restrict && selected.length === 0)}
            >
              {add.isPending ? "Checking…" : "Pin certificate"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
