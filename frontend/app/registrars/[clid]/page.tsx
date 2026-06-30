"use client";

import { useParams, useRouter } from "next/navigation";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { useRegistrar } from "@/lib/hooks/useRegistrars";
import { useRegistrarAccreditations, useAccreditRegistrar, useDeaccreditRegistrar } from "@/lib/hooks/useAccreditations";
import { formatCompactNumber } from "@/lib/utils/numberUtils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useState } from "react";
import Link from "next/link";
import { useDebounce } from "@/lib/hooks/useDebounce";
import { useTLDs } from "@/lib/hooks/useTLDs";
import type { TLD } from "@/lib/api/tlds";
import { RegistrarDomainCountWidget } from "@/components/registrars/RegistrarDomainCountWidget";
import { RegistrarLifecycleWidget } from "@/components/registrars/RegistrarLifecycleWidget";
import { RegistrarTLDCountWidget } from "@/components/registrars/RegistrarTLDCountWidget";
import posthog from "posthog-js";

export default function RegistrarDetailPage() {
  const params = useParams();
  const router = useRouter();
  const clid = typeof params?.clid === "string" ? params.clid : Array.isArray(params?.clid) ? params?.clid[0] : "";
  const { data, isLoading, error } = useRegistrar(clid, !!clid);
  const { data: accData, isLoading: accLoading } = useRegistrarAccreditations(clid, { pagesize: 100 });
  const accreditMutation = useAccreditRegistrar(clid);
  const deaccreditMutation = useDeaccreditRegistrar(clid);

  // Add accreditation modal state
  const [addOpen, setAddOpen] = useState(false);
  const [search, setSearch] = useState("");
  const debounced = useDebounce(search, 300);
  const { data: tldSearch, isLoading: tldLoading } = useTLDs({ name_like: debounced || undefined, pagesize: 20 });

  // De-accredit modal state
  const [deaccOpen, setDeaccOpen] = useState(false);
  const [selectedTLD, setSelectedTLD] = useState<TLD | null>(null);
  const [confirmText, setConfirmText] = useState("");
  const [deaccError, setDeaccError] = useState<string | null>(null);
  const canAccredit = (data?.Status || '').toString().toLowerCase() === 'ok';

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-2xl font-semibold">{isLoading ? <Skeleton className="h-8 w-48" /> : data?.Name ?? clid}</h1>
            <Badge variant="outline" className="font-mono mt-2">{clid}</Badge>
          </div>
          <div className="flex gap-2">
            <Button variant="default" onClick={() => router.push(`/registrars/${encodeURIComponent(clid)}/edit`)}>
              Edit
            </Button>
            <Button variant="outline" onClick={() => router.push("/registrars")}>
              Back to list
            </Button>
          </div>
        </div>

        {data && (
          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
            <div className="md:col-span-2">
              <RegistrarLifecycleWidget data={data} />
            </div>
            <RegistrarDomainCountWidget clid={clid} />
            <RegistrarTLDCountWidget 
              tlds={accData?.Data?.map((t: TLD) => t.Name) ?? []} 
              isLoading={accLoading} 
              onClick={() => {
                const el = document.getElementById("accreditations");
                if (el) {
                  const y = el.getBoundingClientRect().top + window.scrollY - 100;
                  window.scrollTo({ top: y, behavior: "smooth" });
                }
              }} 
            />
          </div>
        )}

        <Card>
          <CardHeader>
            <CardTitle>Details</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-4 sm:grid-cols-2">
            {isLoading ? (
              <>
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className="space-y-1">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-5 w-64" />
                  </div>
                ))}
              </>
            ) : error ? (
              <div className="text-red-600">Failed to load registrar: {(error as { message?: string } | undefined)?.message ?? "Unknown error"}</div>
            ) : (
              <>
                <Field label="Name" value={data?.Name} />
                <Field label="Client ID" value={data?.ClID} mono />
                <Field label="IANA ID" value={data?.GurID?.toString()} mono />
                <Field label="Email" value={data?.Email} />
                <Field label="URL" value={data?.URL} />
                <Field label="RDAP Base URL" value={data?.RdapBaseURL} />
              </>
            )}
          </CardContent>
        </Card>
      </div>
      {/* Accreditations */}
      <Card id="accreditations" className="mt-8">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <CardTitle>Accreditations</CardTitle>
            <CardDescription>
            {accLoading
              ? 'Loading accredited TLDs...'
              : `${accData?.Data?.length ?? 0} TLD${(accData?.Data?.length ?? 0) !== 1 ? 's' : ''} accredited`}
            </CardDescription>
          </div>
          <div className="pt-1 flex flex-col items-end text-right">
            <Button
              size="sm"
              onClick={() => canAccredit && setAddOpen(true)}
              disabled={!canAccredit}
              title={!canAccredit ? 'Registrar status prevents adding accreditations' : undefined}
            >
              Add accreditation
            </Button>
            {!canAccredit && (
              <div className="text-xs text-muted-foreground mt-1">This registrar is not in an eligible status to add accreditations.</div>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {accLoading ? (
            <div className="space-y-2">
              {[1,2,3,4].map(i => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : (accData?.Data?.length ?? 0) === 0 ? (
            <div className="text-center py-8 text-muted-foreground">No accreditations found</div>
          ) : (
            <div className="rounded-md border overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead className="w-28">Domains</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Unicode Name</TableHead>
                    <TableHead>Registry Operator</TableHead>
                    <TableHead className="w-[140px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {accData!.Data!.map((tld: TLD) => (
                    <TableRow key={tld.Name}>
                      <TableCell className="font-medium">
                        <Link href={`/tlds/${encodeURIComponent(tld.Name)}`} className="text-primary hover:underline">
                          {tld.Name}
                        </Link>
                      </TableCell>
                      <TableCell className="font-mono" title={(tld.DomainCount ?? 0).toLocaleString()}>
                        {formatCompactNumber(tld.DomainCount ?? 0)}
                      </TableCell>
                      <TableCell><TLDTypeBadge type={tld.Type} /></TableCell>
                      <TableCell>
                        {tld.UName ? (
                          <span className="text-muted-foreground">{tld.UName}</span>
                        ) : (
                          <span className="text-muted-foreground italic">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Link href={`/registry-operators/${encodeURIComponent(tld.RyID)}`} className="text-primary hover:underline">
                          {tld.RyID}
                        </Link>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => {
                            setSelectedTLD(tld);
                            setConfirmText("");
                            setDeaccError(null);
                            setDeaccOpen(true);
                          }}
                        >
                          De-accredit
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Add accreditation dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>Accredit registrar</DialogTitle>
            <DialogDescription>Search for a TLD to accredit this registrar for.</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Input
              placeholder="Search TLDs by name…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <div className="rounded-md border max-h-80 overflow-y-auto">
              {tldLoading ? (
                <div className="p-4 text-sm text-muted-foreground">Searching…</div>
              ) : (tldSearch?.Data?.length ?? 0) === 0 ? (
                <div className="p-4 text-sm text-muted-foreground">No TLDs found</div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Type</TableHead>
                      <TableHead className="w-[120px]"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {tldSearch!.Data!.map((tld: TLD) => (
                      <TableRow key={tld.Name}>
                        <TableCell className="font-medium">.{tld.Name}</TableCell>
                        <TableCell><TLDTypeBadge type={tld.Type} /></TableCell>
                        <TableCell className="text-right">
                          <Button
                            size="sm"
                            disabled={accreditMutation.isPending}
                            onClick={async () => {
                              await accreditMutation.mutateAsync(tld.Name);
                              posthog.capture('tld_accredited_to_registrar', {
                                registrar_clid: clid,
                                tld_name: tld.Name,
                                tld_type: tld.Type,
                              });
                              setAddOpen(false);
                              setSearch("");
                            }}
                          >
                            {accreditMutation.isPending ? 'Adding…' : 'Accredit'}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* De-accredit dialog */}
      <Dialog open={deaccOpen} onOpenChange={(v) => { setDeaccOpen(v); if (!v) { setDeaccError(null); setConfirmText(""); } }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>De-accredit registrar from .{selectedTLD?.Name}</DialogTitle>
            <DialogDescription>
              This will remove this registrar’s accreditation for .{selectedTLD?.Name}.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              To confirm, type: <span className="font-mono">{"delete "}{selectedTLD?.Name}</span>
            </p>
            <Input
              autoFocus
              placeholder={`delete ${selectedTLD?.Name ?? ''}`}
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
            />
            {deaccError && (
              <div className="text-sm text-red-600">{deaccError}</div>
            )}
            <div className="flex items-center justify-end gap-2 pt-2">
              <Button variant="outline" onClick={() => setDeaccOpen(false)} disabled={deaccreditMutation.isPending}>Cancel</Button>
              <Button
                variant="destructive"
                disabled={
                  deaccreditMutation.isPending || !selectedTLD || confirmText.trim() !== `delete ${selectedTLD?.Name}`
                }
                onClick={async () => {
                  if (!selectedTLD) return;
                  setDeaccError(null);
                  try {
                    await deaccreditMutation.mutateAsync(selectedTLD.Name);
                    posthog.capture('tld_deaccredited_from_registrar', {
                      registrar_clid: clid,
                      tld_name: selectedTLD.Name,
                    });
                    setDeaccOpen(false);
                    setConfirmText("");
                    setSelectedTLD(null);
                  } catch (err) {
                    const e = err as { response?: { data?: { error?: string } } };
                    setDeaccError(e?.response?.data?.error || 'Failed to de-accredit');
                  }
                }}
              >
                {deaccreditMutation.isPending ? 'Removing…' : 'Confirm de-accredit'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </DashboardLayout>
  );
}

function Field({ label, value, mono = false }: { label: string; value?: string | number | boolean; mono?: boolean }) {
  return (
    <div className="space-y-1">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={mono ? "font-mono" : ""}>{value ?? "-"}</div>
    </div>
  );
}

function TLDTypeBadge({ type }: { type: string }) {
  let variant: "default" | "secondary" | "outline" = "default";
  if (type === "country-code") variant = "secondary";
  if (type === "second-level") variant = "outline";
  return (
    <Badge variant={variant}>
      {type === 'generic' && 'gTLD'}
      {type === 'country-code' && 'ccTLD'}
      {type === 'second-level' && 'SLD'}
    </Badge>
  );
}
