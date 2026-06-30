/**
 * System Registrars Tab Component
 * Displays and manages system registrars
 * 
 * Note: This is a placeholder component. Full CRUD functionality will be implemented later.
 */

"use client";

import { useEffect, useMemo, useState } from "react";
import { useRegistrars, useRegistrarCount, useStartRegistrarSyncWorkflow } from "@/lib/hooks/useRegistrars";
import { useTLDs } from "@/lib/hooks/useTLDs";
import { RegistrarListParams } from "@/lib/types/registrar";
import { RegistrarSearchFilters } from "./RegistrarSearchFilters";
import {
  Card,
  CardContent,
} from "@/components/ui/card";
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
import { Loader2, ChevronLeft, ChevronRight, Download } from "lucide-react";
import { getRegistrars } from "@/lib/api/registrars";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { WorkflowStartResponse } from "@/lib/api/workflows";
import { useDebounce } from "@/lib/hooks/useDebounce";
import { useRouter } from "next/navigation";
import { Skeleton } from "@/components/ui/skeleton";
import { formatCompactNumber } from "@/lib/utils/numberUtils";

export function SystemRegistrarsTab() {
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedQuery = useDebounce(searchQuery, 300);
  const [pageSize] = useState(50);
  const [statusFilter, setStatusFilter] = useState<string>("all");
  // New: dedicated IANA ID exact-match search input
  const [ianaIdQuery, setIanaIdQuery] = useState<string>("");
  // New: TLD filter and Sorting
  const [tldFilter, setTldFilter] = useState<string>("all");
  const [sortBy, setSortBy] = useState<string>("domains_desc");
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const router = useRouter();
  const [exporting, setExporting] = useState(false);

  const handleExportCSV = async () => {
    setExporting(true);
    try {
      // 1. Fetch filtered system registrars (large page size, matching current filters)
      const exportParams: RegistrarListParams = {
        pagesize: 10000,
      };
      const q = (debouncedQuery || "").trim();
      if (q) {
        exportParams.name_like = q;
      }
      if (statusFilter && statusFilter !== "all") {
        exportParams.status_equals = statusFilter.toLowerCase();
      }
      const iid = (ianaIdQuery || "").trim();
      if (iid && /^\d+$/.test(iid)) {
        exportParams.gurid_equals = parseInt(iid, 10);
      }
      if (tldFilter && tldFilter !== "all") {
        exportParams.tld = tldFilter;
      }
      if (sortBy === "domains_desc") {
        exportParams.sort_by = "domain_count";
        exportParams.sort_order = "desc";
      } else if (sortBy === "domains_asc") {
        exportParams.sort_by = "domain_count";
        exportParams.sort_order = "asc";
      }

      const res = await getRegistrars(exportParams);
      const registrars = res.Data || [];

      // 2. Generate CSV content
      const headers = ['Client ID', 'Name', 'IANA ID', 'Status', 'Auto-renew', 'Domains', 'TLDs'];
      const rows = registrars.map(r => [
        r.ClID,
        r.Name,
        r.GurID || '',
        r.Status,
        r.Autorenew ? 'Enabled' : 'Disabled',
        r.DomainCount ?? 0,
        (r.TLDList || []).join(' ')
      ]);

      const csvContent = [
        headers.join(','),
        ...rows.map(row => row.map(val => `"${String(val).replace(/"/g, '""')}"`).join(','))
      ].join('\n');

      // 3. Trigger download
      const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.setAttribute('href', url);
      link.setAttribute('download', `system_registrars.csv`);
      link.style.visibility = 'hidden';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (err) {
      console.error('Failed to export CSV:', err);
    } finally {
      setExporting(false);
    }
  };

  // Fetch TLDs for the dropdown filter
  const { data: tldData } = useTLDs({ pagesize: 1000 });

  // Build query parameters
  const queryParams: RegistrarListParams = useMemo(() => {
    const params: RegistrarListParams = {
      pagesize: pageSize,
      cursor,
    };
    const q = (debouncedQuery || "").trim();
    if (q) {
      // Filter by name only as requested to avoid overly restrictive AND filtering in backend
      params.name_like = q;
    }
    if (statusFilter && statusFilter !== "all") {
      // Backend stores registrar status in lowercase (ok, readonly, terminated)
      params.status_equals = statusFilter.toLowerCase();
    }
    // Apply exact IANA ID match when provided
    const iid = (ianaIdQuery || "").trim();
    if (iid && /^\d+$/.test(iid)) {
      params.gurid_equals = parseInt(iid, 10);
    }
    // Apply TLD filter
    if (tldFilter && tldFilter !== "all") {
      params.tld = tldFilter;
    }
    // Apply sorting params
    if (sortBy === "domains_desc") {
      params.sort_by = "domain_count";
      params.sort_order = "desc";
    } else if (sortBy === "domains_asc") {
      params.sort_by = "domain_count";
      params.sort_order = "asc";
    }
    return params;
  }, [pageSize, cursor, debouncedQuery, statusFilter, ianaIdQuery, tldFilter, sortBy]);

  // Fetch data
  const { data, isLoading, error } = useRegistrars(queryParams);
  const { data: countData } = useRegistrarCount();
  const startWorkflow = useStartRegistrarSyncWorkflow();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [workflowInfo, setWorkflowInfo] = useState<WorkflowStartResponse | null>(null);

  // Reset pagination when search or filters change
  useEffect(() => {
    setCursor(undefined);
    setCursorStack([]);
  }, [debouncedQuery, statusFilter, ianaIdQuery, tldFilter, sortBy]);

  const getStatusBadgeVariant = (status: string) => {
    switch (status.toLowerCase()) {
      case "ok":
        return "default";
      case "terminated":
        return "destructive";
      case "readonly":
        return "secondary";
      default:
        return "outline";
    }
  };

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

  return (
    <div className="space-y-4">
      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex-1 min-w-[280px]">
              <RegistrarSearchFilters
                searchQuery={searchQuery}
                setSearchQuery={setSearchQuery}
                ianaIdQuery={ianaIdQuery}
                setIanaIdQuery={setIanaIdQuery}
                statusFilter={statusFilter}
                setStatusFilter={setStatusFilter}
                tldFilter={tldFilter}
                setTldFilter={setTldFilter}
                tlds={tldData?.Data}
                sortBy={sortBy}
                setSortBy={setSortBy}
              />
            </div>
            <div className="flex items-center gap-3">
              <Button
                variant="outline"
                size="sm"
                onClick={handleExportCSV}
                disabled={exporting || (data?.Data?.length ?? 0) === 0}
                className="h-9 shrink-0 font-medium"
                title={
                  (data?.Data?.length ?? 0) === 0
                    ? "No registrars to export"
                    : exporting
                    ? "Exporting to CSV..."
                    : "Export filtered registrar list to CSV"
                }
              >
                <Download className="h-4 w-4 mr-2" />
                {exporting ? 'Exporting...' : 'Export CSV'}
              </Button>

              {countData?.Count === 0 && (
                <Button
                  variant="default"
                  size="sm"
                  onClick={() => {
                    startWorkflow.mutate(undefined, {
                      onSuccess: (data) => {
                        setWorkflowInfo(data);
                        setDialogOpen(true);
                      },
                    });
                  }}
                  disabled={startWorkflow.isPending}
                  className="h-9 shrink-0"
                >
                  {startWorkflow.isPending ? (
                    <span className="inline-flex items-center gap-2">
                      <Loader2 className="h-4 w-4 animate-spin" />
                    </span>
                  ) : (
                    "Pre-populate registrars"
                  )}
                </Button>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Results Table */}
      <Card>
        <CardContent className="pt-6">
          {error && (
            <div className="text-center py-8 text-red-600">
              Error loading registrars: {error.message}
            </div>
          )}

          {isLoading && (
            <div className="py-4">
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-28">Domains</TableHead>
                      <TableHead>Client ID</TableHead>
                      <TableHead>Name</TableHead>
                      <TableHead className="w-24">IANA ID</TableHead>
                      <TableHead className="w-32">Status</TableHead>
                      <TableHead className="w-32">Auto-renew</TableHead>
                      <TableHead className="w-64">TLDs</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {Array.from({ length: 5 }).map((_, i) => (
                      <TableRow key={i}>
                        <TableCell><Skeleton className="h-4 w-12" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-40" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-10" /></TableCell>
                        <TableCell><Skeleton className="h-5 w-24" /></TableCell>
                        <TableCell><Skeleton className="h-5 w-20" /></TableCell>
                        <TableCell><Skeleton className="h-5 w-32" /></TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          )}

          {!isLoading && !error && data && (
            <>
              {!data.Data || data.Data.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  No system registrars found
                </div>
              ) : (
                <>
                  <div className="mb-4 flex justify-end">
                    <PaginationButtons />
                  </div>
                  <div className="rounded-md border">
                    <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-28">Domains</TableHead>
                        <TableHead>Client ID</TableHead>
                        <TableHead>Name</TableHead>
                        <TableHead className="w-24">IANA ID</TableHead>
                        <TableHead className="w-32">Status</TableHead>
                        <TableHead className="w-32">Auto-renew</TableHead>
                        <TableHead className="w-64">TLDs</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.Data.map((registrar) => (
                        <TableRow
                          key={registrar.ClID}
                          className="cursor-pointer hover:bg-muted/40"
                          onClick={() => router.push(`/registrars/${registrar.ClID}`)}
                        >
                          <TableCell className="font-medium" title={(registrar.DomainCount ?? 0).toLocaleString()}>
                            {formatCompactNumber(registrar.DomainCount ?? 0)}
                          </TableCell>
                          <TableCell className="font-mono">
                            {registrar.ClID}
                          </TableCell>
                          <TableCell className="font-semibold text-base">
                            {registrar.Name}
                          </TableCell>
                          <TableCell className="font-mono">
                            {registrar.GurID || "-"}
                          </TableCell>
                          <TableCell>
                            <Badge variant={getStatusBadgeVariant(registrar.Status)}>
                              {registrar.Status}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Badge variant={registrar.Autorenew ? "default" : "outline"}>
                              {registrar.Autorenew ? "Enabled" : "Disabled"}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-1 max-w-[240px]">
                              {registrar.TLDList && registrar.TLDList.length > 0 ? (
                                registrar.TLDList.slice(0, 5).map((tld) => (
                                  <Badge key={tld} variant="outline" className="text-[10px] py-0 px-1.5 font-normal">
                                    .{tld}
                                  </Badge>
                                ))
                              ) : (
                                <span className="text-muted-foreground text-xs">-</span>
                              )}
                              {registrar.TLDList && registrar.TLDList.length > 5 && (
                                <Badge variant="secondary" className="text-[10px] py-0 px-1.5 font-normal">
                                  +{registrar.TLDList.length - 5} more
                                </Badge>
                              )}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </>
              )}

              {data.Meta && data.Data && data.Data.length > 0 && (
                <div className="mt-4 flex items-center justify-between">
                  <div className="text-sm text-muted-foreground">
                    Showing {data.Data.length} registrar{data.Data.length !== 1 ? "s" : ""}
                  </div>
                  <PaginationButtons />
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {/* Success Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Workflow started</DialogTitle>
            <DialogDescription>
              Registrar population has been triggered and may take a few seconds to complete.
            </DialogDescription>
            {workflowInfo?.workflowId && (
              <div className="mt-3 text-xs text-muted-foreground">
                Workflow ID: <span className="font-mono">{workflowInfo.workflowId}</span>
              </div>
            )}
          </DialogHeader>
        </DialogContent>
      </Dialog>
    </div>
  );
}
