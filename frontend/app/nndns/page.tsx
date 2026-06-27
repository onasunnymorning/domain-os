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
import { Switch } from "@/components/ui/switch";
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
import { useNNDNs, useNNDNCount } from "@/lib/hooks/useNNDNs";
import { useTLDs } from "@/lib/hooks/useTLDs";
import { NNDNListParams } from "@/lib/api/nndns";
import { useDebounce } from "@/lib/hooks/useDebounce";
import { ServerOff, Search, ChevronDown, ChevronLeft, ChevronRight, Eraser, SlidersHorizontal } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";

export default function NNDNsPage() {
  return (
    <Suspense fallback={<div />}>
      <NNDNsPageInner />
    </Suspense>
  );
}

function NNDNsPageInner() {
  const router = useRouter();
  const pathname = usePathname();
  const urlSearch = useSearchParams();

  // Basic Filter
  const [nameQuery, setNameQuery] = useState(() => urlSearch.get("q") || "");
  const debouncedName = useDebounce(nameQuery, 300);

  // Advanced Filters
  const [tldFilter, setTldFilter] = useState<string | undefined>(() => urlSearch.get("tld") || undefined);
  const [reasonLike, setReasonLike] = useState(() => urlSearch.get("reason_like") || "");
  const debouncedReasonLike = useDebounce(reasonLike, 300);
  
  const [exactReasonMatch, setExactReasonMatch] = useState(() => {
    const urlVal = urlSearch.get("exact_reason");
    return urlVal === "1";
  });

  const [showAdvanced, setShowAdvanced] = useState(false);

  // Pagination
  const [pageSize] = useState(50);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [cursorStack, setCursorStack] = useState<string[]>([]);

  // Sync state to URL
  useEffect(() => {
    const sp = new URLSearchParams();
    if (nameQuery) sp.set("q", nameQuery);
    if (tldFilter) sp.set("tld", tldFilter);
    if (reasonLike) sp.set("reason_like", reasonLike);
    if (exactReasonMatch) sp.set("exact_reason", "1");
    
    const q = sp.toString();
    router.replace(`${pathname}${q ? `?${q}` : ""}`, { scroll: false });
  }, [nameQuery, tldFilter, reasonLike, exactReasonMatch, pathname, router]);

  // Construct query parameters
  const params: NNDNListParams = useMemo(() => {
    const p: NNDNListParams = { pagesize: pageSize, cursor };
    
    const qName = (debouncedName || "").trim();
    if (qName) {
      p.name_like = qName;
    }

    if (tldFilter) {
      p.tld_equals = tldFilter;
    }

    const qReason = (debouncedReasonLike || "").trim();
    if (qReason) {
      if (exactReasonMatch) {
        p.reason_equals = qReason;
      } else {
        p.reason_like = qReason;
      }
    }

    return p;
  }, [pageSize, cursor, debouncedName, tldFilter, debouncedReasonLike, exactReasonMatch]);

  // Queries
  const { data, isLoading, error } = useNNDNs(params);
  const { data: countData } = useNNDNCount();
  const { data: tldData } = useTLDs({ pagesize: 200 });

  // Reset pagination when filters change
  useEffect(() => {
    setCursor(undefined);
    setCursorStack([]);
  }, [debouncedName, tldFilter, debouncedReasonLike, exactReasonMatch]);

  const tldOptions = (tldData?.Data ?? []).map((t) => ({ value: t.Name, label: t.Name }));
  const [tldSearch, setTldSearch] = useState("");
  const filteredTldOptions = tldOptions.filter((o) =>
    o.label.toLowerCase().includes(tldSearch.toLowerCase())
  );

  const PaginationButtons = () => (
    <div className="flex items-center gap-2">
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
  );

  const resetFilters = () => {
    setNameQuery("");
    setTldFilter(undefined);
    setReasonLike("");
    setExactReasonMatch(false);
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
              <ServerOff className="h-8 w-8" />
              Blocking
            </h1>
          </div>
        </div>

        {/* Info Box */}
        <Card>
          <CardHeader>
            <CardTitle>Blocked Names</CardTitle>
            <CardDescription>
              Registry-blocked and IDN mirrored domains
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-sm text-muted-foreground">
              Total: <span className="font-semibold">{countData?.Count ?? "-"}</span>
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
                </div>
                <div className="relative">
                  <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                  <Input
                    placeholder="Search blocked names..."
                    value={nameQuery}
                    onChange={(e) => setNameQuery(e.target.value)}
                    className="pl-9 h-9"
                  />
                </div>
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
              <div className="mt-4 pt-4 border-t grid grid-cols-1 md:grid-cols-2 gap-6">
                {/* Reason Search */}
                <div className="flex-1 flex flex-col gap-2">
                  <div className="flex items-center justify-between px-1 h-6">
                    <span className="text-sm font-medium text-muted-foreground">Reason</span>
                    <div className="flex items-center gap-2">
                      <label htmlFor="exact-reason-match" className="text-xs text-muted-foreground cursor-pointer">
                        Exact match
                      </label>
                      <Switch
                        id="exact-reason-match"
                        checked={exactReasonMatch}
                        onCheckedChange={setExactReasonMatch}
                      />
                    </div>
                  </div>
                  <div className="relative">
                    <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                      placeholder="Search block reason..."
                      value={reasonLike}
                      onChange={(e) => setReasonLike(e.target.value)}
                      className="pl-9 h-9"
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
                Error loading blocked names: {(error as any)?.message || "Unknown error"}
              </div>
            )}

            {!error && (
              <>
                {data?.Meta && data?.Data && data.Data.length > 0 && (
                  <div className="mb-4 flex justify-end">
                    <PaginationButtons />
                  </div>
                )}
                <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Domain Name</TableHead>
                      <TableHead>Unicode Name</TableHead>
                      <TableHead>TLD</TableHead>
                      <TableHead>State</TableHead>
                      <TableHead>Reason</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead>Updated</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {isLoading &&
                      Array.from({ length: 6 }).map((_, i) => (
                        <TableRow key={i}>
                          <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                          <TableCell><Skeleton className="h-5 w-16" /></TableCell>
                          <TableCell><Skeleton className="h-5 w-20" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-40" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                          <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                        </TableRow>
                      ))}
                    {!isLoading && (!data?.Data || data.Data.length === 0) && (
                      <TableRow>
                        <TableCell className="py-8 text-center text-muted-foreground" colSpan={7}>
                          No blocked names found
                        </TableCell>
                      </TableRow>
                    )}
                    {!isLoading &&
                      data?.Data?.map((d) => (
                        <TableRow key={d.Name} className="hover:bg-muted/50">
                          <TableCell className="font-medium">
                            {d.Name}
                          </TableCell>
                          <TableCell className="font-medium">
                            {d.UName || "-"}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{d.TLDName}</Badge>
                          </TableCell>
                          <TableCell>
                            <Badge className="capitalize">{d.NameState}</Badge>
                          </TableCell>
                          <TableCell className="max-w-[15rem] truncate" title={d.Reason}>
                            {d.Reason || "-"}
                          </TableCell>
                          <TableCell>
                            {d.CreatedAt ? formatDistanceToNow(new Date(d.CreatedAt), { addSuffix: true }) : "-"}
                          </TableCell>
                          <TableCell>
                            {d.UpdatedAt ? formatDistanceToNow(new Date(d.UpdatedAt), { addSuffix: true }) : "-"}
                          </TableCell>
                        </TableRow>
                      ))}
                  </TableBody>
                </Table>
              </div>
            </>
            )}

            {data?.Meta && data?.Data && data.Data.length > 0 && (
              <div className="mt-4 flex items-center justify-between">
                <div className="text-sm text-muted-foreground">
                  Showing {data.Data.length} blocked name{data.Data.length !== 1 ? "s" : ""}
                </div>
                <PaginationButtons />
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
