/**
 * System Registrars Tab Component
 * Displays and manages system registrars
 * 
 * Note: This is a placeholder component. Full CRUD functionality will be implemented later.
 */

"use client";

import { useEffect, useMemo, useState } from "react";
import { useRegistrars, useRegistrarCount, useStartRegistrarSyncWorkflow } from "@/lib/hooks/useRegistrars";
import { RegistrarListParams, RegistrarStatus } from "@/lib/types/registrar";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
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
import { Search, Loader2, ChevronLeft, ChevronRight } from "lucide-react";
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

export function SystemRegistrarsTab() {
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedQuery = useDebounce(searchQuery, 300);
  const [pageSize] = useState(50);
  const [statusFilter, setStatusFilter] = useState<string>("all");
  // New: dedicated IANA ID exact-match search input
  const [ianaIdQuery, setIanaIdQuery] = useState<string>("");
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const router = useRouter();

  // Build query parameters
  const queryParams: RegistrarListParams = useMemo(() => {
    const params: RegistrarListParams = {
      pagesize: pageSize,
      cursor,
    };
    const q = (debouncedQuery || "").trim();
    if (q) {
      // Always perform fuzzy search by name and ClID for better UX
      params.name_like = q;
      params.clid_like = q;
      // Note: We intentionally avoid switching to gurid_equals-only on numeric input
      // to preserve fuzzy search behavior highlighted by product feedback.
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
    return params;
  }, [pageSize, cursor, debouncedQuery, statusFilter, ianaIdQuery]);

  // Fetch data
  const { data, isLoading, error } = useRegistrars(queryParams);
  const { data: countData } = useRegistrarCount();
  const startWorkflow = useStartRegistrarSyncWorkflow();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [workflowInfo, setWorkflowInfo] = useState<WorkflowStartResponse | null>(null);

  // Reset pagination when search changes
  useEffect(() => {
    setCursor(undefined);
    setCursorStack([]);
  }, [debouncedQuery, statusFilter, ianaIdQuery]);

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

  return (
    <div className="space-y-4">
      {/* Info Card */}
      <Card>
        <CardHeader>
          <CardTitle>System Registrars</CardTitle>
          <CardDescription>
            System registrars are configured in your registry. These can be created,
            updated, and managed directly through this interface.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between gap-4">
            <div className="text-sm text-muted-foreground">
              Total System Registrars: <span className="font-semibold">{countData?.Count ?? "-"}</span>
            </div>

            {countData?.Count === 0 && (
              <Button
                variant="default"
                onClick={() => {
                  startWorkflow.mutate(undefined, {
                    onSuccess: (data) => {
                      setWorkflowInfo(data);
                      setDialogOpen(true);
                    },
                  });
                }}
                disabled={startWorkflow.isPending}
              >
                {startWorkflow.isPending ? (
                  <span className="inline-flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Starting…
                  </span>
                ) : (
                  "Pre-populate registrars"
                )}
              </Button>
            )}
          </div>

          {startWorkflow.isError && (
            <div className="text-sm text-red-600 mt-2">
              Failed to trigger workflow. Please try again.
            </div>
          )}
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex gap-4">
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search registrars..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>
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
              <div className="w-48">
                <Select value={statusFilter} onValueChange={setStatusFilter}>
                  <SelectTrigger>
                    <SelectValue placeholder="All Statuses" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Statuses</SelectItem>
                    <SelectItem value={RegistrarStatus.OK}>OK</SelectItem>
                    <SelectItem value={RegistrarStatus.Readonly}>Readonly</SelectItem>
                    <SelectItem value={RegistrarStatus.Terminated}>Terminated</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {/* IANA ID exact match */}
              <div className="w-28">
                <Input
                  placeholder="IANA ID"
                  inputMode="numeric"
                  value={ianaIdQuery}
                  onChange={(e) => setIanaIdQuery(e.target.value)}
                />
              </div>
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
                      <TableHead>Client ID</TableHead>
                      <TableHead>Name</TableHead>
                      <TableHead className="w-24">IANA ID</TableHead>
                      <TableHead className="w-32">Status</TableHead>
                      <TableHead className="w-32">Auto-renew</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {Array.from({ length: 5 }).map((_, i) => (
                      <TableRow key={i}>
                        <TableCell><Skeleton className="h-4 w-28" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-40" /></TableCell>
                        <TableCell><Skeleton className="h-4 w-10" /></TableCell>
                        <TableCell><Skeleton className="h-5 w-24" /></TableCell>
                        <TableCell><Skeleton className="h-5 w-20" /></TableCell>
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
                <div className="rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Client ID</TableHead>
                        <TableHead>Name</TableHead>
                        <TableHead className="w-24">IANA ID</TableHead>
                        <TableHead className="w-32">Status</TableHead>
                        <TableHead className="w-32">Auto-renew</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.Data.map((registrar) => (
                        <TableRow
                          key={registrar.ClID}
                          className="cursor-pointer hover:bg-muted/40"
                          onClick={() => router.push(`/registrars/${registrar.ClID}`)}
                        >
                          <TableCell className="font-mono">
                            {registrar.ClID}
                          </TableCell>
                          <TableCell className="font-medium">
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
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}

              {data.Meta && data.Data && data.Data.length > 0 && (
                <div className="mt-4 text-sm text-muted-foreground text-center">
                  Showing {data.Data.length} registrar{data.Data.length !== 1 ? "s" : ""}
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
