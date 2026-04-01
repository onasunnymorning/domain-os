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
import { RefreshCw, Search, Loader2 } from "lucide-react";
import { toast } from "sonner";

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
          <div className="flex gap-4">
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search by name or IANA ID..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>
            <div className="w-64">
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger>
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
            <Button
              onClick={handleSync}
              disabled={syncMutation.isPending}
              variant="outline"
              className="shrink-0"
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
