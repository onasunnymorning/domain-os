/**
 * System Registrars Tab Component
 * Displays and manages system registrars
 * 
 * Note: This is a placeholder component. Full CRUD functionality will be implemented later.
 */

"use client";

import { useState } from "react";
import { useRegistrars, useRegistrarCount } from "@/lib/hooks/useRegistrars";
import { RegistrarListParams } from "@/lib/types/registrar";
import { Input } from "@/components/ui/input";
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
import { Search, Loader2 } from "lucide-react";

export function SystemRegistrarsTab() {
  const [searchQuery] = useState("");
  const [pageSize] = useState(50);

  // Build query parameters
  const queryParams: RegistrarListParams = {
    pagesize: pageSize,
  };

  // Fetch data
  const { data, isLoading, error } = useRegistrars(queryParams);
  const { data: countData } = useRegistrarCount();

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
          <div className="flex items-center justify-between">
            <div className="text-sm text-muted-foreground">
              Total System Registrars: <span className="font-semibold">{countData?.Count ?? "-"}</span>
            </div>
          </div>
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
                  disabled
                  className="pl-10"
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
            <div className="text-center py-8">
              <Loader2 className="h-8 w-8 animate-spin mx-auto text-muted-foreground" />
              <p className="mt-2 text-muted-foreground">Loading registrars...</p>
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
                        <TableRow key={registrar.ClID}>
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
    </div>
  );
}
