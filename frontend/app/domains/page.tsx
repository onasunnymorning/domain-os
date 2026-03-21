"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { formatDistanceToNow } from "date-fns";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useDomains, useDomainCount } from "@/lib/hooks/useDomains";
import { useRegistrars } from "@/lib/hooks/useRegistrars";
import { useTLDs } from "@/lib/hooks/useTLDs";
import { DomainListParams } from "@/lib/types/domain";
import { useDebounce } from "@/lib/hooks/useDebounce";
import { Server, Search, ChevronDown, ChevronLeft, ChevronRight, HelpCircle, Eraser, SlidersHorizontal } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import Link from "next/link";

// Shared labels and help text
import { STATUS_LABELS, STATUS_DESCRIPTIONS, RGP_LABELS, RGP_DESCRIPTIONS } from "@/lib/constants/domainStatus";

export default function DomainsPage() {
  return (
    <Suspense fallback={<div />}> 
      <DomainsPageInner />
    </Suspense>
  );
}

function DomainsPageInner() {
  // Routing & query syncing
  const router = useRouter();
  const pathname = usePathname();
  const urlSearch = useSearchParams();
  // Search and filters
  const [nameQuery, setNameQuery] = useState(() => urlSearch.get("q") || "");
  const debouncedName = useDebounce(nameQuery, 300);
  const [exactMatch, setExactMatch] = useState(() => {
    const urlVal = urlSearch.get("exact");
    if (urlVal !== null) return urlVal === "1";
    if (typeof window !== "undefined") {
      return sessionStorage.getItem("domainExactMatch") === "1";
    }
    return false;
  });

  useEffect(() => {
    if (typeof window !== "undefined") {
      sessionStorage.setItem("domainExactMatch", exactMatch ? "1" : "0");
    }
  }, [exactMatch]);

  const [clidFilter, setClidFilter] = useState<string | undefined>(() => urlSearch.get("clid") || undefined);
  const [tldFilter, setTldFilter] = useState<string | undefined>(() => urlSearch.get("tld") || undefined);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [roidMin, setRoidMin] = useState(() => urlSearch.get("roidMin") || "");
  const [roidMax, setRoidMax] = useState(() => urlSearch.get("roidMax") || "");
  const [createdAfter, setCreatedAfter] = useState(() => urlSearch.get("createdAfter") || "");
  const [createdBefore, setCreatedBefore] = useState(() => urlSearch.get("createdBefore") || "");
  const [expiresAfter, setExpiresAfter] = useState(() => urlSearch.get("expiresAfter") || "");
  const [expiresBefore, setExpiresBefore] = useState(() => urlSearch.get("expiresBefore") || "");
  const [pageSize] = useState(50);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [cursorStack, setCursorStack] = useState<string[]>([]);

  useEffect(() => {
    const sp = new URLSearchParams();
    if (nameQuery) sp.set("q", nameQuery);
    if (exactMatch) sp.set("exact", "1");
    if (clidFilter) sp.set("clid", clidFilter);
    if (tldFilter) sp.set("tld", tldFilter);
    if (roidMin) sp.set("roidMin", roidMin);
    if (roidMax) sp.set("roidMax", roidMax);
    if (createdAfter) sp.set("createdAfter", createdAfter);
    if (createdBefore) sp.set("createdBefore", createdBefore);
    if (expiresAfter) sp.set("expiresAfter", expiresAfter);
    if (expiresBefore) sp.set("expiresBefore", expiresBefore);
    const q = sp.toString();
    // Replace to avoid stacking history entries while typing/filtering
    router.replace(`${pathname}${q ? `?${q}` : ""}`, { scroll: false });
  }, [nameQuery, exactMatch, clidFilter, tldFilter, roidMin, roidMax, createdAfter, createdBefore, expiresAfter, expiresBefore, pathname, router]);

  // Build domain query params
  const params: DomainListParams = useMemo(() => {
    const p: DomainListParams = { pagesize: pageSize, cursor };
    const q = (debouncedName || "").trim();
    if (q) {
      if (exactMatch) {
        p.name_equals = q;
      } else {
        p.name_like = q;
      }
    }
    if (clidFilter) p.clid_equals = clidFilter;
    if (tldFilter) p.tld_equals = tldFilter;
    if (roidMin) p.roid_greater_than = roidMin;
    if (roidMax) p.roid_less_than = roidMax;
    if (createdAfter) p.created_after = `${createdAfter}T00:00:00Z`;
    if (createdBefore) p.created_before = `${createdBefore}T23:59:59Z`;
    if (expiresAfter) p.expires_after = `${expiresAfter}T00:00:00Z`;
    if (expiresBefore) p.expires_before = `${expiresBefore}T23:59:59Z`;
    return p;
  }, [pageSize, cursor, debouncedName, exactMatch, clidFilter, tldFilter, roidMin, roidMax, createdAfter, createdBefore, expiresAfter, expiresBefore]);

  // Data queries
  const { data, isLoading, error } = useDomains(params);
  const { data: countData } = useDomainCount();
  const { data: registrarData } = useRegistrars({ pagesize: 200 });
  const { data: tldData } = useTLDs({ pagesize: 200 });

  useEffect(() => {
    setCursor(undefined);
    setCursorStack([]);
  }, [debouncedName, exactMatch, clidFilter, tldFilter, roidMin, roidMax, createdAfter, createdBefore, expiresAfter, expiresBefore]);

  // Derived option lists
  const registrarOptions = (registrarData?.Data ?? []).map((r) => ({
    value: r.ClID,
    label: r.Name ? `${r.Name} (${r.ClID})` : r.ClID,
    nameTerm: r.Name ? r.Name : r.ClID,
  }));
  const tldOptions = (tldData?.Data ?? []).map((t) => ({ value: t.Name, label: t.Name }));

  // Simple client-side search text for dropdowns
  const [searchRegistrarByClid, setSearchRegistrarByClid] = useState(false);
  const [clidSearch, setClidSearch] = useState("");
  const [tldSearch, setTldSearch] = useState("");

  const filteredRegistrarOptions = registrarOptions.filter((o) => {
    if (!clidSearch) return true;
    const term = clidSearch.toLowerCase();
    if (searchRegistrarByClid) {
      return o.value.toLowerCase().includes(term);
    }
    return o.nameTerm.toLowerCase().includes(term);
  });
  const filteredTldOptions = tldOptions.filter((o) =>
    o.label.toLowerCase().includes(tldSearch.toLowerCase())
  );

  const resetFilters = () => {
    setNameQuery("");
    setExactMatch(false);
    setClidFilter(undefined);
    setTldFilter(undefined);
    setRoidMin("");
    setRoidMax("");
    setCreatedAfter("");
    setCreatedBefore("");
    setExpiresAfter("");
    setExpiresBefore("");
    setClidSearch("");
    setTldSearch("");
    setCursor(undefined);
    setCursorStack([]);
    router.replace(pathname, { scroll: false });
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <Server className="h-8 w-8" />
              Domains
            </h1>
            <p className="text-muted-foreground mt-2">
              Browse and search domains by name, registrar, and TLD
            </p>
          </div>
          <div>
            <Link href="/domains/create">
              <Button>
                Create Domain
              </Button>
            </Link>
          </div>
        </div>

        {/* Info */}
        <Card>
          <CardHeader>
            <CardTitle>Domain Directory</CardTitle>
            <CardDescription>
              Use the filters below to narrow results. Toggle exact match to search by full domain name.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-sm text-muted-foreground">
              Total Domains: <span className="font-semibold">{countData?.Count ?? "-"}</span>
            </div>
          </CardContent>
        </Card>

        {/* Filters */}
        <Card>
          <CardContent className="pt-6">
            <div className="flex flex-col gap-4 md:flex-row md:items-end">
              {/* Name search */}
              <div className="flex-1 flex flex-col gap-2">
                <div className="flex items-center justify-between px-1 h-6">
                  <span className="text-sm font-medium text-muted-foreground">Domain Name</span>
                  <div className="flex items-center gap-2">
                    <label htmlFor="exact-match" className="text-xs text-muted-foreground cursor-pointer">
                      Exact match
                    </label>
                    <Switch
                      id="exact-match"
                      checked={exactMatch}
                      onCheckedChange={setExactMatch}
                    />
                  </div>
                </div>
                <div className="relative">
                  <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                  <Input
                    placeholder="Search domains..."
                    value={nameQuery}
                    onChange={(e) => setNameQuery(e.target.value)}
                    className="pl-9 h-9"
                  />
                </div>
              </div>

              {/* Registrar combobox */}
              <div className="w-full md:w-72 flex flex-col gap-2">
                <div className="h-6"></div> {/* Spacer to align with Search input label */}
                <Popover>
                  <PopoverTrigger asChild>
                    <Button variant="outline" role="combobox" className="w-full justify-between h-9">
                      {clidFilter ? registrarOptions.find((o) => o.value === clidFilter)?.label : "Filter by Registrar"}
                      <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-72 p-2" align="start">
                    <div className="flex items-center justify-between mb-2 px-1">
                      <span className="text-xs font-medium text-muted-foreground">Search by {searchRegistrarByClid ? "ClID" : "Name"}</span>
                      <div className="flex items-center gap-1.5">
                        <label htmlFor="search-clid" className="text-[10px] text-muted-foreground cursor-pointer uppercase tracking-wider">
                          Use ClID
                        </label>
                        <Switch
                          id="search-clid"
                          checked={searchRegistrarByClid}
                          onCheckedChange={setSearchRegistrarByClid}
                          className="scale-75 origin-right"
                        />
                      </div>
                    </div>
                    <Input
                      placeholder={searchRegistrarByClid ? "Search ClID..." : "Search name..."}
                      value={clidSearch}
                      onChange={(e) => setClidSearch(e.target.value)}
                      className="mb-2"
                    />
                    <ScrollArea className="h-52">
                      <div className="space-y-1">
                        <Button
                          variant="ghost"
                          className="w-full justify-start"
                          onClick={() => setClidFilter(undefined)}
                        >
                          All registrars
                        </Button>
                        {filteredRegistrarOptions.map((opt) => (
                          <Button
                            key={opt.value}
                            variant={opt.value === clidFilter ? "secondary" : "ghost"}
                            className="w-full justify-start"
                            onClick={() => setClidFilter(opt.value)}
                          >
                            {opt.label}
                          </Button>
                        ))}
                        {filteredRegistrarOptions.length === 0 && (
                          <div className="text-xs text-muted-foreground px-2 py-1">No results</div>
                        )}
                      </div>
                    </ScrollArea>
                  </PopoverContent>
                </Popover>
              </div>

              {/* TLD combobox */}
              <div className="w-full md:w-56 flex flex-col gap-2">
                <div className="h-6"></div> {/* Spacer to align */}
                <Popover>
                  <PopoverTrigger asChild>
                    <Button variant="outline" role="combobox" className="w-full justify-between h-9">
                      {tldFilter || "Filter by TLD"}
                      <ChevronDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-56 p-2" align="start">
                    <Input
                      placeholder="Search TLD..."
                      value={tldSearch}
                      onChange={(e) => setTldSearch(e.target.value)}
                      className="mb-2"
                    />
                    <ScrollArea className="h-52">
                      <div className="space-y-1">
                        <Button
                          variant="ghost"
                          className="w-full justify-start"
                          onClick={() => setTldFilter(undefined)}
                        >
                          All TLDs
                        </Button>
                        {filteredTldOptions.map((opt) => (
                          <Button
                            key={opt.value}
                            variant={opt.value === tldFilter ? "secondary" : "ghost"}
                            className="w-full justify-start"
                            onClick={() => setTldFilter(opt.value)}
                          >
                            {opt.label}
                          </Button>
                        ))}
                        {filteredTldOptions.length === 0 && (
                          <div className="text-xs text-muted-foreground px-2 py-1">No results</div>
                        )}
                      </div>
                    </ScrollArea>
                  </PopoverContent>
                </Popover>
              </div>

              {/* Pagination controls */}
              <div className="flex items-center gap-2 ml-auto">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    if (cursorStack.length > 0) {
                      const prev = [...cursorStack];
                      const c = prev.pop();
                      setCursorStack(prev);
                      setCursor(c);
                    }
                  }}
                  disabled={isLoading || cursorStack.length === 0}
                >
                  <ChevronLeft className="h-4 w-4" /> Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    const nextCursor = data?.Meta?.PageCursor;
                    if (nextCursor) {
                      setCursorStack((s) => (cursor ? [...s, cursor] : s));
                      setCursor(nextCursor);
                    }
                  }}
                  disabled={isLoading || !data?.Meta?.PageCursor}
                >
                  Next <ChevronRight className="h-4 w-4 ml-1" />
                </Button>
              </div>

              {/* Reset filters */}
              <div className="w-full md:w-auto md:ml-auto flex flex-wrap items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowAdvanced(!showAdvanced)}
                  className={showAdvanced ? "bg-muted" : ""}
                >
                  <SlidersHorizontal className="h-4 w-4 mr-2" />
                  Advanced Filters
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={resetFilters}
                  aria-label="Reset filters"
                  title="Clear all filters and search"
                  className="justify-start"
                >
                  <Eraser className="h-4 w-4 mr-2" /> Reset filters
                </Button>
              </div>
            </div>

            {/* Advanced Filters */}
            {showAdvanced && (
              <div className="mt-4 pt-4 border-t grid grid-cols-1 md:grid-cols-3 gap-6">
                {/* Created Range */}
                <div className="space-y-2">
                  <span className="text-sm font-medium text-muted-foreground">Created Date</span>
                  <div className="flex items-center gap-2">
                    <Input
                      type="date"
                      value={createdAfter}
                      onChange={(e) => setCreatedAfter(e.target.value)}
                      className="text-sm h-9"
                      title="Created After"
                    />
                    <span className="text-sm text-muted-foreground">to</span>
                    <Input
                      type="date"
                      value={createdBefore}
                      onChange={(e) => setCreatedBefore(e.target.value)}
                      className="text-sm h-9"
                      title="Created Before"
                    />
                  </div>
                </div>
                
                {/* Expires Range */}
                <div className="space-y-2">
                  <span className="text-sm font-medium text-muted-foreground">Expiry Date</span>
                  <div className="flex items-center gap-2">
                    <Input
                      type="date"
                      value={expiresAfter}
                      onChange={(e) => setExpiresAfter(e.target.value)}
                      className="text-sm h-9"
                      title="Expires After"
                    />
                    <span className="text-sm text-muted-foreground">to</span>
                    <Input
                      type="date"
                      value={expiresBefore}
                      onChange={(e) => setExpiresBefore(e.target.value)}
                      className="text-sm h-9"
                      title="Expires Before"
                    />
                  </div>
                </div>

                {/* RoID Range */}
                <div className="space-y-2">
                  <span className="text-sm font-medium text-muted-foreground">RoID Range</span>
                  <div className="flex items-center gap-2">
                    <Input
                      placeholder="Min RoID"
                      value={roidMin}
                      onChange={(e) => setRoidMin(e.target.value)}
                      className="text-sm h-9"
                    />
                    <span className="text-sm text-muted-foreground">to</span>
                    <Input
                      placeholder="Max RoID"
                      value={roidMax}
                      onChange={(e) => setRoidMax(e.target.value)}
                      className="text-sm h-9"
                    />
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Results */}
        <Card>
          <CardContent className="pt-6">
            {error && (
              <div className="text-center py-8 text-red-600">
                Error loading domains: {(error as any)?.message || "Unknown error"}
              </div>
            )}

            {!error && (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Domain</TableHead>
                      <TableHead className="w-40">Registrar</TableHead>
                      <TableHead className="w-40">Created</TableHead>
                      <TableHead className="w-40">Updated</TableHead>
                      <TableHead className="w-48">Expires</TableHead>
                      <TableHead className="w-[26rem]">
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
                      </TableHead>
                      <TableHead className="w-[22rem]">
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
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {isLoading &&
                      Array.from({ length: 6 }).map((_, i) => (
                        <TableRow key={i}>
                          <TableCell>
                            <Skeleton className="h-4 w-44" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-5 w-24" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-24" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-28" />
                          </TableCell>
                          <TableCell>
                            <Skeleton className="h-4 w-28" />
                          </TableCell>
                          <TableCell>
                            <div className="flex gap-1 flex-wrap py-1">
                              <Skeleton className="h-5 w-14" />
                              <Skeleton className="h-5 w-16" />
                              <Skeleton className="h-5 w-20" />
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex gap-1 flex-wrap py-1">
                              <Skeleton className="h-5 w-20" />
                              <Skeleton className="h-5 w-24" />
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    {!isLoading && (!data?.Data || data.Data.length === 0) && (
                      <TableRow>
                        <TableCell className="py-8 text-center text-muted-foreground" colSpan={7}>
                          No domains found
                        </TableCell>
                      </TableRow>
                    )}
                    {!isLoading &&
                      data?.Data?.map((d) => (
                        <TableRow
                          key={`${d.Name}-${d.ClID}`}
                          className="cursor-pointer hover:bg-muted/50"
                          onClick={() => router.push(`/domains/${encodeURIComponent(d.Name)}`)}
                          role="link"
                          tabIndex={0}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              router.push(`/domains/${encodeURIComponent(d.Name)}`);
                            }
                          }}
                          title={`Open ${d.Name}`}
                        >
                          <TableCell className="font-medium">
                            <Link className="hover:underline" href={`/domains/${encodeURIComponent(d.Name)}`}>
                              {d.Name}
                            </Link>
                          </TableCell>
                          <TableCell onClick={(e) => e.stopPropagation()}>
                            <Link href={`/registrars/${encodeURIComponent(d.ClID)}`} className="inline-block">
                              <Badge variant="outline" className="hover:underline" title={`View registrar ${d.ClID}`}>
                                {d.ClID}
                              </Badge>
                            </Link>
                          </TableCell>
                          <TableCell>
                            {d.CreatedAt ? formatDistanceToNow(new Date(d.CreatedAt), { addSuffix: true }) : "-"}
                          </TableCell>
                          <TableCell>
                            {d.UpdatedAt ? formatDistanceToNow(new Date(d.UpdatedAt), { addSuffix: true }) : "-"}
                          </TableCell>
                          <TableCell>
                            {d.ExpiryDate ? formatDistanceToNow(new Date(d.ExpiryDate), { addSuffix: true }) : "-"}
                          </TableCell>
                          <TableCell>
                            <div className="flex gap-1 flex-wrap">
                              {(() => {
                                const s: any = (d as any).Status || {};
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
                          </TableCell>
                          <TableCell>
                            <div className="flex gap-1 flex-wrap">
                              {(() => {
                                const r: any = (d as any).RGPStatus || {};
                                const labels: string[] = [];
                                const inFuture = (v?: string) => {
                                  if (!v) return false;
                                  const t = new Date(v).getTime();
                                  return Number.isFinite(t) && t > Date.now();
                                };
                                if (inFuture(r.addPeriodEnd)) labels.push(RGP_LABELS.addPeriodEnd);
                                if (inFuture(r.autoRenewPeriodEnd)) labels.push(RGP_LABELS.autoRenewPeriodEnd);
                                if (inFuture(r.renewPeriodEnd)) labels.push(RGP_LABELS.renewPeriodEnd);
                                if (inFuture(r.transferLockPeriodEnd)) labels.push(RGP_LABELS.transferLockPeriodEnd);
                                if (inFuture(r.redemptionPeriodEnd)) labels.push(RGP_LABELS.redemptionPeriodEnd);
                                if (inFuture(r.purgeDate)) labels.push(RGP_LABELS.purgeDate);
                                if (labels.length === 0) return <span className="text-muted-foreground">-</span>;
                                return labels.map((lbl) => (
                                  <Badge key={lbl} variant="secondary" className="text-xs" title={RGP_DESCRIPTIONS[lbl] || lbl}>
                                    {lbl}
                                  </Badge>
                                ));
                              })()}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                  </TableBody>
                </Table>
              </div>
            )}

            {data?.Meta && data?.Data && data.Data.length > 0 && (
              <div className="mt-4 text-sm text-muted-foreground text-center">
                Showing {data.Data.length} domain{data.Data.length !== 1 ? "s" : ""}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
