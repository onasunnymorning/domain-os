/**
 * IANA Registrars Tab Component
 * Displays and manages IANA registrars with filtering and sync capabilities
 */

"use client";

import { useState } from "react";
import { useIANARegistrars, useIANARegistrarCount, useSyncIANARegistrars } from "@/lib/hooks/useRegistrars";
import { IANARegistrarStatus, IANARegistrarListParams } from "@/lib/types/registrar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { RefreshCw, Search, Loader2, ExternalLink, X, Download } from "lucide-react";
import { toast } from "sonner";
import { getIANARegistrars } from "@/lib/api/registrars";

export function IANARegistrarsTab() {
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [pageSize] = useState(50);

  // Build query parameters
  const queryParams: IANARegistrarListParams = {
    pagesize: pageSize,
  };

  if (searchQuery) {
    queryParams.name_like = searchQuery;
  }

  if (statusFilter && statusFilter !== "all") {
    queryParams.status = statusFilter;
  }

  // Fetch data
  const { data, isLoading, error, refetch } = useIANARegistrars(queryParams);
  const { data: countData } = useIANARegistrarCount();
  const syncMutation = useSyncIANARegistrars();

  const handleSync = async () => {
    try {
      await syncMutation.mutateAsync();
      toast.success("IANA registrar data has been updated");
      refetch();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Failed to sync IANA registrars"
      );
    }
  };

  const [exporting, setExporting] = useState(false);

  const handleExportCSV = async () => {
    setExporting(true);
    try {
      const exportParams: IANARegistrarListParams = {
        pagesize: 10000,
      };
      if (searchQuery) {
        exportParams.name_like = searchQuery;
      }
      if (statusFilter && statusFilter !== "all") {
        exportParams.status = statusFilter;
      }

      const res = await getIANARegistrars(exportParams);
      const registrars = res.Data || [];

      // 2. Generate CSV content
      const headers = ['IANA ID', 'Name', 'Status', 'RDAP URL', 'CreatedAt'];
      const rows = registrars.map(r => [
        r.GurID,
        r.Name,
        r.Status,
        r.RdapURL || '',
        r.CreatedAt || ''
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
      link.setAttribute('download', `iana_registrars.csv`);
      link.style.visibility = 'hidden';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (err) {
      console.error('Failed to export IANA registrars CSV:', err);
    } finally {
      setExporting(false);
    }
  };

  const hasActiveFilters = searchQuery !== "" || statusFilter !== "all";

  const handleReset = () => {
    setSearchQuery("");
    setStatusFilter("all");
  };

  const getStatusBadgeVariant = (status: string) => {
    switch (status) {
      case IANARegistrarStatus.Accredited:
        return "default";
      case IANARegistrarStatus.Terminated:
        return "destructive";
      case IANARegistrarStatus.Reserved:
        return "secondary";
      default:
        return "outline";
    }
  };

  return (
    <div className="space-y-4">
      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex-1 min-w-[280px] flex items-center gap-3">
              <div className="flex-1 relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search by name or IANA ID..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10 pr-9 h-9"
                />
                {searchQuery && (
                  <button
                    onClick={() => setSearchQuery("")}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    type="button"
                  >
                    <X className="h-4 w-4" />
                  </button>
                )}
              </div>
              <div className="w-48">
                <Select value={statusFilter} onValueChange={setStatusFilter}>
                  <SelectTrigger className="h-9">
                    <SelectValue placeholder="Filter by status" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Statuses</SelectItem>
                    <SelectItem value={IANARegistrarStatus.Accredited}>
                      Accredited
                    </SelectItem>
                    <SelectItem value={IANARegistrarStatus.Terminated}>
                      Terminated
                    </SelectItem>
                    <SelectItem value={IANARegistrarStatus.Reserved}>
                      Reserved
                    </SelectItem>
                    <SelectItem value={IANARegistrarStatus.Unknown}>
                      Unknown
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {hasActiveFilters && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleReset}
                  className="h-9 px-3 text-muted-foreground hover:text-foreground shrink-0 gap-1.5"
                  type="button"
                >
                  <X className="h-4 w-4" />
                  Clear
                </Button>
              )}
            </div>

            <div className="flex items-center gap-3">
              <Button
                onClick={handleSync}
                disabled={syncMutation.isPending}
                variant="outline"
                className="h-9 shrink-0"
                size="sm"
              >
                {syncMutation.isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Syncing...
                  </>
                ) : (
                  <>
                    <RefreshCw className="mr-2 h-4 w-4" />
                    Sync from IANA
                  </>
                )}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* IANA Source Link */}
      <div className="flex items-center gap-1.5 text-sm text-muted-foreground px-1">
        <ExternalLink className="h-3.5 w-3.5" />
        <span>Source:</span>
        <a
          href="https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml"
          target="_blank"
          rel="noopener noreferrer"
          className="text-blue-600 hover:underline"
        >
          IANA Registrar IDs Registry
        </a>
      </div>

      {/* Results Table */}
      <Card>
        <CardContent className="pt-6">
          {error && (
            <div className="text-center py-8 text-red-600">
              Error loading registrars: {error.message}
            </div>
          )}

          {isLoading && (
            <div className="text-center py-8">
              <Loader2 className="h-8 w-8 animate-spin mx-auto text-muted-foreground" />
              <p className="mt-2 text-muted-foreground">Loading registrars...</p>
            </div>
          )}

          {!isLoading && !error && data && (
            <>
              {!data.Data || data.Data.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  No registrars found matching your criteria
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="flex justify-end">
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
                          : "Export filtered IANA registrar list to CSV"
                      }
                    >
                      <Download className="h-4 w-4 mr-2" />
                      {exporting ? 'Exporting...' : 'Export CSV'}
                    </Button>
                  </div>
                  <div className="rounded-md border">
                    <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-24">IANA ID</TableHead>
                        <TableHead>Name</TableHead>
                        <TableHead className="w-32">Status</TableHead>
                        <TableHead>RDAP URL</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.Data.map((registrar) => (
                        <TableRow key={registrar.GurID}>
                          <TableCell className="font-mono">
                            {registrar.GurID}
                          </TableCell>
                          <TableCell className="font-medium">
                            {registrar.Name}
                          </TableCell>
                          <TableCell>
                            <Badge variant={getStatusBadgeVariant(registrar.Status)}>
                              {registrar.Status}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {registrar.RdapURL ? (
                              <a
                                href={registrar.RdapURL}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-blue-600 hover:underline text-sm"
                              >
                                {registrar.RdapURL}
                              </a>
                            ) : (
                              <span className="text-muted-foreground">-</span>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
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
    </div>
  );
}
