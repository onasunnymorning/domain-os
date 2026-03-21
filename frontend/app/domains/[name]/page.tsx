"use client";

import { useParams, useRouter } from "next/navigation";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import Link from "next/link";
import { useDomain } from "@/lib/hooks/useDomains";
import { formatDistanceToNow } from "date-fns";
import type { DomainDetail } from "@/lib/types/domain";
import { HelpCircle, Copy, Eye, EyeOff, Server } from "lucide-react";
import { useMemo, useState } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { STATUS_LABELS, STATUS_DESCRIPTIONS, RGP_LABELS, RGP_DESCRIPTIONS } from "@/lib/constants/domainStatus";
import { DomainLifecycleWidget } from "@/components/domains/DomainLifecycleWidget";


function formatUTCString(d: Date) {
  // Example: 2025-10-29 14:32 UTC
  const yyyy = d.getUTCFullYear();
  const mm = String(d.getUTCMonth() + 1).padStart(2, "0");
  const dd = String(d.getUTCDate()).padStart(2, "0");
  const hh = String(d.getUTCHours()).padStart(2, "0");
  const min = String(d.getUTCMinutes()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd} ${hh}:${min} UTC`;
}

function formatLocalString(d: Date) {
  // Example: 2025-10-29 16:32 (local)
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const min = String(d.getMinutes()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd} ${hh}:${min}`;
}

function RelDate({ value }: { value?: string }) {
  if (!value) return <span className="text-muted-foreground">-</span>;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return <span className="text-muted-foreground">-</span>;
  const abs = formatUTCString(d);
  const local = formatLocalString(d);
  const rel = formatDistanceToNow(d, { addSuffix: true });
  return (
    <span title={d.toISOString()} className="inline-flex flex-col leading-tight">
      <span className="font-inherit">
        {abs}
        <span className="ml-2 text-xs text-muted-foreground">({local} local)</span>
      </span>
      <span className="text-xs text-muted-foreground font-normal">{rel}</span>
    </span>
  );
}

export default function DomainDetailPage() {
  const params = useParams<{ name: string }>();
  const router = useRouter();
  const name = decodeURIComponent(params.name || "");
  const { data, isLoading, error } = useDomain(name, !!name);
  const [showAuth, setShowAuth] = useState(false);
  const domain = (data || {}) as DomainDetail;
  const activeRGPLabels = useMemo(() => {
    const r: any = domain?.RGPStatus || {};
    const inFuture = (v?: string) => {
      if (!v) return false;
      const t = new Date(v).getTime();
      return Number.isFinite(t) && t > Date.now();
    };
    const labels: string[] = [];
    if (inFuture(r.addPeriodEnd)) labels.push(RGP_LABELS.addPeriodEnd);
    if (inFuture(r.autoRenewPeriodEnd)) labels.push(RGP_LABELS.autoRenewPeriodEnd);
    if (inFuture(r.renewPeriodEnd)) labels.push(RGP_LABELS.renewPeriodEnd);
    if (inFuture(r.transferLockPeriodEnd)) labels.push(RGP_LABELS.transferLockPeriodEnd);
    if (inFuture(r.redemptionPeriodEnd)) labels.push(RGP_LABELS.redemptionPeriodEnd);
    if (inFuture(r.purgeDate)) labels.push(RGP_LABELS.purgeDate);
    return labels;
  }, [domain?.RGPStatus]);

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-3xl font-bold tracking-tight font-mono">{name || "Domain"}</h1>
              {name && (
                <Button
                  variant="outline"
                  size="icon"
                  className="h-8 w-8"
                  aria-label="Copy domain name"
                  onClick={async () => {
                    try { await navigator.clipboard.writeText(name); } catch {}
                  }}
                  title="Copy domain name"
                >
                  <Copy className="h-4 w-4" />
                </Button>
              )}
            </div>
            <div className="mt-2 flex items-center gap-2">
              {isLoading ? (
                <Skeleton className="h-5 w-32" />
              ) : (
                <>
                  {domain?.ClID ? (
                    <Link href={`/registrars/${encodeURIComponent(domain.ClID)}`} title="Current registrar" aria-label="Current registrar">
                      <Badge variant="outline" className="cursor-pointer hover:bg-muted">{domain.ClID}</Badge>
                    </Link>
                  ) : (
                    <span className="text-sm text-muted-foreground">-</span>
                  )}
                  {domain?.CrRr && domain.CrRr !== domain.ClID && (
                    <div className="flex items-center">
                      <span className="text-muted-foreground text-sm mx-2">←</span>
                      <Link href={`/registrars/${encodeURIComponent(domain.CrRr)}`} title="Created at registrar" aria-label="Created at registrar">
                        <Badge variant="secondary" className="cursor-pointer opacity-80 hover:opacity-100 transition-opacity">{domain.CrRr}</Badge>
                      </Link>
                    </div>
                  )}
                </>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              onClick={() => {
                if (typeof window !== "undefined" && window.history.length > 1) {
                  router.back();
                } else {
                  router.push("/domains");
                }
              }}
              aria-label="Back to results"
              title="Return to your results (preserves filters)"
            >
              Back to results
            </Button>
            <Button asChild aria-label="All Domains" title="Go to all domains">
              <Link href="/domains">All Domains</Link>
            </Button>
          </div>
        </div>

        {/* Lifecycle Visualizer */}
        {!isLoading && !error && domain && (
          <DomainLifecycleWidget domain={domain} />
        )}

        <Card>
          <CardHeader>
            <CardTitle>Overview</CardTitle>
            <CardDescription>Basic information about this domain</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading && (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Skeleton className="h-6 w-48" />
                <Skeleton className="h-6 w-32" />
                <Skeleton className="h-6 w-40" />
                <Skeleton className="h-6 w-56" />
              </div>
            )}

            {error && (
              <div className="text-red-600">Failed to load domain: {(error as any)?.message || "Unknown error"}</div>
            )}

            {!isLoading && !error && domain && (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground">Domain</div>
                  <div className="font-medium flex items-center gap-2 font-mono">
                    {domain.Name}
                    {domain.UName && domain.UName !== domain.Name && (
                      <Badge variant="secondary" title="IDN Unicode label">{domain.UName}</Badge>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground">TLD</div>
                  <div className="font-medium">
                    {domain.TLDName ? (
                      <Link href={`/tlds/${encodeURIComponent(domain.TLDName)}`} title={`View TLD ${domain.TLDName}`} aria-label={`View TLD ${domain.TLDName}`} className="hover:underline">
                        {domain.TLDName}
                      </Link>
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground">Registrar</div>
                  <div className="font-medium">
                    {domain.ClID ? (
                      <Link href={`/registrars/${encodeURIComponent(domain.ClID)}`} title={`View registrar ${domain.ClID}`} aria-label={`View registrar ${domain.ClID}`}>
                        <Badge variant="outline">{domain.ClID}</Badge>
                      </Link>
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground">RoID</div>
                  <div className="font-medium font-mono text-sm">{domain.RoID || '-'}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground">
                    {domain.CreatedAt ? (
                      <>
                        Created{" "}
                        <span className="text-xs text-muted-foreground">
                          {formatDistanceToNow(new Date(domain.CreatedAt), { addSuffix: true })}
                        </span>
                      </>
                    ) : (
                      <>Created</>
                    )}
                  </div>
                  <div className="font-medium">
                    {domain.CreatedAt ? (
                      (() => {
                        const d = new Date(domain.CreatedAt!);
                        return (
                          <span title={d.toISOString()}>
                            {formatUTCString(d)}
                            <span className="ml-2 text-xs text-muted-foreground">({formatLocalString(d)} local)</span>
                          </span>
                        );
                      })()
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground">
                    {domain.UpdatedAt ? (
                      <>
                        Updated{" "}
                        <span className="text-xs text-muted-foreground">
                          {formatDistanceToNow(new Date(domain.UpdatedAt), { addSuffix: true })}
                        </span>
                      </>
                    ) : (
                      <>Updated</>
                    )}
                  </div>
                  <div className="font-medium">
                    {domain.UpdatedAt ? (
                      (() => {
                        const d = new Date(domain.UpdatedAt!);
                        return (
                          <span title={d.toISOString()}>
                            {formatUTCString(d)}
                            <span className="ml-2 text-xs text-muted-foreground">({formatLocalString(d)} local)</span>
                          </span>
                        );
                      })()
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs text-muted-foreground">
                    {domain.ExpiryDate ? (
                      <>
                        Expires{" "}
                        <span className="text-xs text-muted-foreground">
                          {formatDistanceToNow(new Date(domain.ExpiryDate), { addSuffix: true })}
                        </span>
                      </>
                    ) : (
                      <>Expires</>
                    )}
                  </div>
                  <div className="font-medium">
                    {domain.ExpiryDate ? (
                      (() => {
                        const d = new Date(domain.ExpiryDate!);
                        return (
                          <span title={d.toISOString()}>
                            {formatUTCString(d)}
                            <span className="ml-2 text-xs text-muted-foreground">({formatLocalString(d)} local)</span>
                          </span>
                        );
                      })()
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </div>
                </div>
              </div>
            )}

            {!isLoading && !error && !data && (
              <div className="text-muted-foreground">Domain not found.</div>
            )}
          </CardContent>
        </Card>

        {/* Status & Grace Periods */}
        {!isLoading && !error && domain && (
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2">
                <CardTitle>Status & Grace Period</CardTitle>
              </div>
              <CardDescription>Operational state and active grace windows</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-6 sm:grid-cols-2">
              <div>
                <div className="text-xs text-muted-foreground mb-2">
                  <div className="flex items-center gap-1">
                    Status
                    <Popover>
                      <PopoverTrigger asChild>
                        <button aria-label="Status help" className="inline-flex items-center text-muted-foreground hover:text-foreground">
                          <HelpCircle className="h-4 w-4" />
                        </button>
                      </PopoverTrigger>
                      <PopoverContent className="w-80 p-3 text-sm" align="start">
                        <div className="mb-2 font-medium">What do these mean?</div>
                        <div className="space-y-2 max-h-64 overflow-auto pr-2">
                          {Object.keys(STATUS_LABELS).map((k) => (
                            <div key={k} className="flex items-start gap-2">
                              <Badge variant="outline" className="text-xs whitespace-nowrap">{STATUS_LABELS[k]}</Badge>
                              <span className="text-muted-foreground">{STATUS_DESCRIPTIONS[k]}</span>
                            </div>
                          ))}
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                </div>
                <div className="flex gap-1 flex-wrap">
                  {(() => {
                    const s: any = (domain as any).Status || {};
                    const entries = Object.entries(s).filter(([, v]) => Boolean(v));
                    if (entries.length === 0) return <span className="text-muted-foreground">-</span>;
                    return entries.map(([k]) => {
                      const label = STATUS_LABELS[k] || (k === 'OK' ? 'OK' : k.replace(/([a-z])([A-Z])/g, '$1 $2'));
                      const title = STATUS_DESCRIPTIONS[k] || label;
                      return (
                        <Badge key={k} variant="outline" className="text-xs" title={title}>
                          {label}
                        </Badge>
                      );
                    });
                  })()}
                </div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground mb-2">
                  <div className="flex items-center gap-1">
                    Grace Period
                    <Popover>
                      <PopoverTrigger asChild>
                        <button aria-label="Grace period help" className="inline-flex items-center text-muted-foreground hover:text-foreground">
                          <HelpCircle className="h-4 w-4" />
                        </button>
                      </PopoverTrigger>
                      <PopoverContent className="w-80 p-3 text-sm" align="start">
                        <div className="mb-2 font-medium">Grace periods shown only if active</div>
                        <div className="space-y-2">
                          {Object.values(RGP_LABELS).map((lbl) => (
                            <div key={lbl} className="flex items-start gap-2">
                              <Badge variant="secondary" className="text-xs whitespace-nowrap">{lbl}</Badge>
                              <span className="text-muted-foreground">{RGP_DESCRIPTIONS[lbl]}</span>
                            </div>
                          ))}
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                </div>
                <div className="flex gap-1 flex-wrap">
                  {activeRGPLabels.length === 0 ? (
                    <span className="text-muted-foreground">-</span>
                  ) : (
                    activeRGPLabels.map(lbl => (
                      <Badge key={lbl} variant="secondary" className="text-xs" title={RGP_DESCRIPTIONS[lbl] || lbl}>
                        {lbl}
                      </Badge>
                    ))
                  )}
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Auth & Contacts */}
        {!isLoading && !error && domain && (
          <Card>
            <CardHeader>
              <CardTitle>Contacts & Access</CardTitle>
              <CardDescription>Auth info and contact handles</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-6 sm:grid-cols-2">
              <div className="space-y-1">
                <div className="text-xs text-muted-foreground">AuthInfo</div>
                <div className="flex items-center gap-2">
                  <code className="px-2 py-1 bg-muted rounded text-sm">
                    {domain.AuthInfo ? (showAuth ? domain.AuthInfo : '••••••••••••••') : '-'}
                  </code>
                  {domain.AuthInfo && (
                    <>
                      <Button variant="outline" size="icon" aria-label={showAuth ? 'Hide AuthInfo' : 'Show AuthInfo'} onClick={() => setShowAuth((v) => !v)}>
                        {showAuth ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                      </Button>
                      <Button
                        variant="outline"
                        size="icon"
                        aria-label="Copy AuthInfo"
                        onClick={async () => {
                          try { await navigator.clipboard.writeText(domain.AuthInfo!); } catch {}
                        }}
                      >
                        <Copy className="h-4 w-4" />
                      </Button>
                    </>
                  )}
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <div className="text-xs text-muted-foreground">Registrant</div>
                  <div className="font-medium"><Badge variant="outline">{domain.RegistrantID || '-'}</Badge></div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Admin</div>
                  <div className="font-medium"><Badge variant="outline">{domain.AdminID || '-'}</Badge></div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Tech</div>
                  <div className="font-medium"><Badge variant="outline">{domain.TechID || '-'}</Badge></div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Billing</div>
                  <div className="font-medium"><Badge variant="outline">{domain.BillingID || '-'}</Badge></div>
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Registrars & Flags */}
        {!isLoading && !error && domain && (
          <Card>
            <CardHeader>
              <CardTitle>Registry Metadata</CardTitle>
              <CardDescription>Registrar actions and domain flags</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-6 sm:grid-cols-3">
              <div>
                <div className="text-xs text-muted-foreground">Create Registrar</div>
                <div className="font-medium font-mono text-sm">{domain.CrRr || '-'}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Update Registrar</div>
                <div className="font-medium font-mono text-sm">{domain.UpRr || '-'}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Drop-catch</div>
                <div className="font-medium">{domain.DropCatch ? 'Yes' : 'No'}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Renewed years</div>
                <div className="font-medium">{typeof domain.RenewedYears === 'number' ? domain.RenewedYears : '-'}</div>
              </div>
              <div className="col-span-2">
                <div className="text-xs text-muted-foreground">Grandfathering</div>
                {domain.GrandFathering ? (
                  <div className="flex flex-wrap items-center gap-2 mt-1">
                    <Badge variant="secondary">{domain.GrandFathering.Amount} {domain.GrandFathering.Currency}</Badge>
                    <Badge variant="outline">{domain.GrandFathering.ExpiryCondition || 'n/a'}</Badge>
                    <span className="text-sm text-muted-foreground">
                      {domain.GrandFathering.VoidDate ? `voids ${formatDistanceToNow(new Date(domain.GrandFathering.VoidDate), { addSuffix: true })}` : 'no void date'}
                    </span>
                  </div>
                ) : (
                  <span className="text-muted-foreground">-</span>
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Hosts */}
        {!isLoading && !error && domain && (
          <Card>
            <CardHeader>
              <CardTitle>Nameservers</CardTitle>
              <CardDescription>Associated hosts</CardDescription>
            </CardHeader>
            <CardContent>
              {!domain.Hosts || domain.Hosts.length === 0 ? (
                <div className="text-muted-foreground">No hosts associated</div>
              ) : (
                <div className="rounded-md border overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead className="bg-muted/50">
                      <tr>
                        <th className="text-left p-2 font-medium">Host</th>
                        <th className="text-left p-2 font-medium">Addresses</th>
                        <th className="text-left p-2 font-medium">In-bailiwick</th>
                        <th className="text-left p-2 font-medium">Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {domain.Hosts!.map((h) => (
                        <tr key={h.Name} className="border-t">
                          <td className="p-2 font-medium flex items-center gap-1">
                            <Server className="h-4 w-4 text-muted-foreground" /> {h.Name}
                          </td>
                          <td className="p-2">{h.Addresses?.length ?? 0}</td>
                          <td className="p-2">{h.InBailiwick ? 'Yes' : 'No'}</td>
                          <td className="p-2">
                            <div className="flex gap-1 flex-wrap">
                              {(() => {
                                const s: any = h.Status || {};
                                const entries = Object.entries(s).filter(([, v]) => Boolean(v));
                                if (entries.length === 0) return <span className="text-muted-foreground">-</span>;
                                return entries.map(([k]) => {
                                  const label = STATUS_LABELS[k] || (k === 'OK' ? 'OK' : k.replace(/([a-z])([A-Z])/g, '$1 $2'));
                                  return (
                                    <Badge key={k} variant="outline" className="text-xs">
                                      {label}
                                    </Badge>
                                  );
                                });
                              })()}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </div>
    </DashboardLayout>
  );
}
