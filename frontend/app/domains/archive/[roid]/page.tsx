"use client";

import { useParams } from "next/navigation";
import Link from "next/link";
import { format, formatDistanceToNow } from "date-fns";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { useTombstoneByRoID } from "@/lib/hooks/useTombstones";
import { DomainLifecycleVisualizer } from "@/components/domains/DomainLifecycleVisualizer";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function fmtDate(v?: string) {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return "—";
  return format(d, "MMM d, yyyy 'at' HH:mm 'UTC'");
}

function fmtRel(v?: string) {
  if (!v) return null;
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return null;
  return formatDistanceToNow(d, { addSuffix: true });
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function ArchiveDetailPage() {
  const params = useParams<{ roid: string }>();
  const roid = decodeURIComponent(params.roid || "");

  const { data: tombstone, isLoading, error } = useTombstoneByRoID(roid, !!roid);

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Breadcrumb */}
        <nav className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Link href="/domains" className="hover:text-foreground transition-colors">
            Domains
          </Link>
          <span>/</span>
          <span className="text-foreground font-medium">Archive</span>
          <span>/</span>
          <span className="text-foreground font-mono font-medium">
            {roid || "…"}
          </span>
        </nav>

        {/* Loading */}
        {isLoading && (
          <div className="space-y-6">
            <Skeleton className="h-8 w-72" />
            <Card>
              <CardContent className="space-y-4 pt-6">
                <Skeleton className="h-4 w-full" />
                <div className="grid grid-cols-4 gap-4">
                  <Skeleton className="h-10 w-full" />
                  <Skeleton className="h-10 w-full" />
                  <Skeleton className="h-10 w-full" />
                  <Skeleton className="h-10 w-full" />
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Error */}
        {!isLoading && error && (
          <Card>
            <CardContent className="py-8 text-center text-destructive">
              Failed to load tombstone ({roid}):{" "}
              {(error as any)?.message || "Unknown error"}
            </CardContent>
          </Card>
        )}

        {/* Not found */}
        {!isLoading && !error && !tombstone && (
          <Card>
            <CardContent className="py-16 text-center">
              <div className="text-muted-foreground text-lg">
                Tombstone not found
              </div>
              <div className="text-sm text-muted-foreground mt-2">
                No archived domain record exists for ROID{" "}
                <code className="px-1 py-0.5 bg-muted rounded text-xs">
                  {roid}
                </code>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Tombstone detail */}
        {!isLoading && !error && tombstone && (
          <>
            {/* Title */}
            <div className="flex items-center justify-between">
              <h1 className="text-2xl font-bold tracking-tight">
                <span className="font-mono">{tombstone.name}</span>
                <span className="text-muted-foreground font-normal ml-2">
                  — Archived Record
                </span>
              </h1>
              <div className="flex items-center gap-2">
                <Button asChild variant="outline" size="sm">
                  <Link
                    href={`/domains/${encodeURIComponent(tombstone.name)}/history`}
                  >
                    Full history
                  </Link>
                </Button>
                <Button asChild variant="outline" size="sm">
                  <Link href="/domains">All Domains</Link>
                </Button>
              </div>
            </div>

            {/* Lifecycle visualizer */}
            <Card>
              <CardContent className="pt-6">
                <DomainLifecycleVisualizer
                  registeredAt={tombstone.registered_at}
                  expiredAt={tombstone.expired_at}
                  purgedAt={tombstone.purged_at}
                  registrar={tombstone.registrar_clid}
                  roid={tombstone.roid}
                />
              </CardContent>
            </Card>

            {/* Metadata */}
            <Card>
              <CardHeader>
                <CardTitle>Archive Metadata</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">RoID</div>
                    <div className="font-medium font-mono text-sm">
                      {tombstone.roid}
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">Domain</div>
                    <div className="font-medium font-mono text-sm">
                      <Link
                        href={`/domains/${encodeURIComponent(tombstone.name)}`}
                        className="hover:underline"
                      >
                        {tombstone.name}
                      </Link>
                      {tombstone.uname &&
                        tombstone.uname !== tombstone.name && (
                          <Badge variant="secondary" className="ml-2">
                            {tombstone.uname}
                          </Badge>
                        )}
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">TLD</div>
                    <div className="font-medium text-sm">
                      <Link
                        href={`/tlds/${encodeURIComponent(tombstone.tld_name)}`}
                        className="hover:underline"
                      >
                        {tombstone.tld_name}
                      </Link>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">
                      Registrar
                    </div>
                    <div className="font-medium text-sm">
                      <Link
                        href={`/registrars/${encodeURIComponent(tombstone.registrar_clid)}`}
                      >
                        <Badge
                          variant="outline"
                          className="cursor-pointer hover:bg-muted"
                        >
                          {tombstone.registrar_clid}
                        </Badge>
                      </Link>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">
                      Registered
                    </div>
                    <div className="font-medium text-sm">
                      {fmtDate(tombstone.registered_at)}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {fmtRel(tombstone.registered_at)}
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">Expired</div>
                    <div className="font-medium text-sm">
                      {fmtDate(tombstone.expired_at)}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {fmtRel(tombstone.expired_at)}
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">Purged</div>
                    <div className="font-medium text-sm">
                      {fmtDate(tombstone.purged_at)}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {fmtRel(tombstone.purged_at)}
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">
                      Purge Reason
                    </div>
                    <div className="font-medium text-sm capitalize">
                      {tombstone.purge_reason || "—"}
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs text-muted-foreground">
                      Drop-catch
                    </div>
                    <div className="font-medium text-sm">
                      {tombstone.drop_catch ? "Yes" : "No"}
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Last Snapshot (if available) */}
            {tombstone.last_snapshot && (
              <Card>
                <CardHeader>
                  <CardTitle>Last Snapshot</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="bg-muted text-xs p-4 rounded-md overflow-x-auto max-h-96">
                    <pre>
                      <code>
                        {JSON.stringify(tombstone.last_snapshot, null, 2)}
                      </code>
                    </pre>
                  </div>
                </CardContent>
              </Card>
            )}
          </>
        )}
      </div>
    </DashboardLayout>
  );
}
