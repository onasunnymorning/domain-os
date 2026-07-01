"use client";

import { useParams } from "next/navigation";
import Link from "next/link";
import { format, formatDistanceToNow } from "date-fns";
import { DashboardLayout } from "@/components/layout/DashboardLayout";
import { useTombstonesByName } from "@/lib/hooks/useTombstones";
import { useDomain } from "@/lib/hooks/useDomains";
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
import type { DomainDetail } from "@/lib/types/domain";
import type { DomainTombstone } from "@/lib/api/tombstones";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function fmtDate(v?: string) {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return "—";
  return format(d, "MMM d, yyyy");
}

function fmtRel(v?: string) {
  if (!v) return null;
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return null;
  return formatDistanceToNow(d, { addSuffix: true });
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function CurrentIncarnationCard({ domain }: { domain: DomainDetail }) {
  return (
    <Card className="border-green-200 dark:border-green-900/40">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-lg">Current Incarnation</CardTitle>
          <Badge className="bg-green-100 text-green-800 border-green-300 dark:bg-green-900/30 dark:text-green-400 dark:border-green-800">
            Active
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <DomainLifecycleVisualizer
          registeredAt={domain.CreatedAt!}
          expiredAt={
            domain.ExpiryDate && new Date(domain.ExpiryDate) <= new Date()
              ? domain.ExpiryDate
              : undefined
          }
          registrar={domain.ClID}
          roid={domain.RoID || "—"}
          isActive
        />

        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-2">
          <div>
            <div className="text-xs text-muted-foreground">Registrar</div>
            <div className="font-medium text-sm">
              <Link href={`/registrars/${encodeURIComponent(domain.ClID)}`}>
                <Badge variant="outline" className="cursor-pointer hover:bg-muted">
                  {domain.ClID}
                </Badge>
              </Link>
            </div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Registered</div>
            <div className="font-medium text-sm">{fmtDate(domain.CreatedAt)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Expires</div>
            <div className="font-medium text-sm">{fmtDate(domain.ExpiryDate)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">RoID</div>
            <div className="font-medium text-sm font-mono">{domain.RoID || "—"}</div>
          </div>
        </div>

        <div className="pt-1">
          <Button asChild variant="outline" size="sm">
            <Link href={`/domains/${encodeURIComponent(domain.Name)}`}>
              View domain details
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function TombstoneCard({ tombstone }: { tombstone: DomainTombstone }) {
  const rel = fmtRel(tombstone.purged_at);

  return (
    <Card className="border-muted/60">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-mono">{tombstone.roid}</CardTitle>
          <Badge variant="secondary" className="text-xs">
            Purged {rel || fmtDate(tombstone.purged_at)}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <DomainLifecycleVisualizer
          registeredAt={tombstone.registered_at}
          expiredAt={tombstone.expired_at}
          purgedAt={tombstone.purged_at}
          registrar={tombstone.registrar_clid}
          roid={tombstone.roid}
        />

        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-2">
          <div>
            <div className="text-xs text-muted-foreground">Registrar</div>
            <div className="font-medium text-sm">
              <Link href={`/registrars/${encodeURIComponent(tombstone.registrar_clid)}`}>
                <Badge variant="outline" className="cursor-pointer hover:bg-muted">
                  {tombstone.registrar_clid}
                </Badge>
              </Link>
            </div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Registered</div>
            <div className="font-medium text-sm">{fmtDate(tombstone.registered_at)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Expired</div>
            <div className="font-medium text-sm">{fmtDate(tombstone.expired_at)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">Purge Reason</div>
            <div className="font-medium text-sm capitalize">{tombstone.purge_reason || "—"}</div>
          </div>
        </div>

        <div className="pt-1">
          <Button asChild variant="outline" size="sm">
            <Link href={`/domains/archive/${encodeURIComponent(tombstone.roid)}`}>
              View archive record
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-8 w-64" />
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
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function DomainHistoryPage() {
  const params = useParams<{ name: string }>();
  const name = decodeURIComponent(params.name || "");

  const {
    data: domainData,
    isLoading: domainLoading,
    error: domainError,
  } = useDomain(name, !!name);

  const {
    data: tombstones,
    isLoading: tombstonesLoading,
    error: tombstonesError,
  } = useTombstonesByName(name, !!name);

  const domain = domainData as DomainDetail | undefined;
  const isLoading = domainLoading || tombstonesLoading;
  const hasDomain = !!domain?.Name;
  const hasTombstones = !!tombstones?.length;

  return (
    <DashboardLayout>
      <div className="space-y-6">
        {/* Breadcrumb */}
        <nav className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Link href="/domains" className="hover:text-foreground transition-colors">
            Domains
          </Link>
          <span>/</span>
          <Link
            href={`/domains/${encodeURIComponent(name)}`}
            className="hover:text-foreground transition-colors font-mono"
          >
            {name}
          </Link>
          <span>/</span>
          <span className="text-foreground font-medium">History</span>
        </nav>

        {/* Title */}
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold tracking-tight">
            <span className="font-mono">{name}</span>
            <span className="text-muted-foreground font-normal ml-2">
              — Domain History
            </span>
          </h1>
          <Button asChild variant="outline" size="sm">
            <Link href={`/domains/${encodeURIComponent(name)}`}>
              Back to domain
            </Link>
          </Button>
        </div>

        {/* Loading */}
        {isLoading && <LoadingSkeleton />}

        {/* Error */}
        {!isLoading && (domainError || tombstonesError) && (
          <Card>
            <CardContent className="py-8 text-center text-destructive">
              Failed to load domain history:{" "}
              {(domainError as any)?.message ||
                (tombstonesError as any)?.message ||
                "Unknown error"}
            </CardContent>
          </Card>
        )}

        {/* Current incarnation */}
        {!isLoading && !domainError && hasDomain && (
          <CurrentIncarnationCard domain={domain!} />
        )}

        {/* Past incarnations */}
        {!isLoading && !tombstonesError && hasTombstones && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold text-muted-foreground">
              Past Incarnations
              <Badge variant="secondary" className="ml-2">
                {tombstones!.length}
              </Badge>
            </h2>
            {tombstones!.map((ts) => (
              <TombstoneCard key={ts.roid} tombstone={ts} />
            ))}
          </div>
        )}

        {/* Empty state */}
        {!isLoading &&
          !domainError &&
          !tombstonesError &&
          !hasDomain &&
          !hasTombstones && (
            <Card>
              <CardContent className="py-16 text-center">
                <div className="text-muted-foreground text-lg">
                  No previous incarnations found
                </div>
                <div className="text-sm text-muted-foreground mt-2">
                  This domain name has no current registration or archived
                  tombstone records.
                </div>
              </CardContent>
            </Card>
          )}
      </div>
    </DashboardLayout>
  );
}
