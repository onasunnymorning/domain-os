'use client';

import { useState } from 'react';
import Link from 'next/link';
import { formatDistanceToNow, format } from 'date-fns';
import {
  Archive,
  Clock,
  Hash,
  Building2,
  Globe,
  CalendarPlus,
  CalendarX,
  Trash2,
  ShieldAlert,
  Target,
  ChevronDown,
  ChevronUp,
  FileCode,
  History,
  Loader2,
  GitCompare,
} from 'lucide-react';

import type { DomainTombstone } from '@/lib/api/tombstones';
import { useEventSearch } from '@/lib/hooks/useEventSearch';
import type { DomainEvent } from '@/lib/types/domain';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { CopyButton } from '@/components/ui/copy-button';
import { StateDiffModal } from '@/components/shared/StateDiffModal';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface ArchivedDomainViewProps {
  tombstones: DomainTombstone[];
  domainName: string;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatDate(dateStr: string) {
  const d = new Date(dateStr);
  return {
    relative: formatDistanceToNow(d, { addSuffix: true }),
    absolute: format(d, 'MMM d, yyyy HH:mm'),
  };
}

function purgeReasonVariant(reason: string): 'default' | 'destructive' | 'secondary' | 'outline' {
  switch (reason.toLowerCase()) {
    case 'expired':
    case 'lifecycle':
      return 'destructive';
    case 'admin_deleted':
    case 'admin':
      return 'secondary';
    default:
      return 'outline';
  }
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function ArchiveBanner({
  tombstone,
  totalIncarnations,
  domainName,
}: {
  tombstone: DomainTombstone;
  totalIncarnations: number;
  domainName: string;
}) {
  const purged = formatDate(tombstone.purged_at);

  return (
    <div className="rounded-lg border border-amber-300/40 dark:border-amber-700/40 bg-gradient-to-r from-amber-50/80 via-orange-50/50 to-amber-50/80 dark:from-amber-950/30 dark:via-orange-950/20 dark:to-amber-950/30 p-5">
      <div className="flex items-start gap-3">
        <div className="rounded-full bg-amber-100 dark:bg-amber-900/50 p-2 shrink-0 mt-0.5">
          <Archive className="h-5 w-5 text-amber-700 dark:text-amber-400" />
        </div>
        <div className="space-y-1 flex-1">
          <p className="text-sm font-medium text-amber-900 dark:text-amber-200">
            This domain was purged{' '}
            <span title={purged.absolute}>{purged.relative}</span>.
            Showing archived data.
          </p>
          {totalIncarnations > 1 && (
            <p className="text-xs text-amber-700/80 dark:text-amber-400/70">
              This domain has been registered {totalIncarnations} times.
            </p>
          )}
        </div>
        <Link
          href={`/domains/${encodeURIComponent(domainName)}/history`}
          className="shrink-0"
        >
          <Button variant="outline" size="sm" className="gap-1.5 text-amber-800 dark:text-amber-300 border-amber-300 dark:border-amber-700">
            <History className="h-3.5 w-3.5" />
            View full history
          </Button>
        </Link>
      </div>
    </div>
  );
}

function IncarnationPicker({
  tombstones,
  selectedIndex,
  onSelect,
}: {
  tombstones: DomainTombstone[];
  selectedIndex: number;
  onSelect: (index: number) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {tombstones.map((t, i) => {
        const purged = formatDate(t.purged_at);
        const isSelected = i === selectedIndex;
        return (
          <button
            key={t.roid}
            onClick={() => onSelect(i)}
            className={`
              inline-flex items-center gap-1.5 rounded-full px-3 py-1.5
              text-xs font-medium transition-all duration-150
              border cursor-pointer
              ${
                isSelected
                  ? 'bg-primary text-primary-foreground border-primary shadow-sm'
                  : 'bg-muted/50 text-muted-foreground border-muted-foreground/20 hover:bg-muted hover:border-muted-foreground/40'
              }
            `}
          >
            <Hash className="h-3 w-3" />
            Incarnation {tombstones.length - i}
            <span className="opacity-70">—</span>
            <span className="opacity-70" title={purged.absolute}>
              purged {purged.relative}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function MetadataItem({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-start gap-3 py-3">
      <div className="text-muted-foreground mt-0.5 shrink-0">{icon}</div>
      <div className="min-w-0">
        <p className="text-xs text-muted-foreground mb-0.5">{label}</p>
        <div className="text-sm font-medium break-all">{children}</div>
      </div>
    </div>
  );
}

function TombstoneMetadataCard({ tombstone }: { tombstone: DomainTombstone }) {
  const registered = tombstone.registered_at ? formatDate(tombstone.registered_at) : null;
  const expired = tombstone.expired_at ? formatDate(tombstone.expired_at) : null;
  const purged = formatDate(tombstone.purged_at);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Archived Metadata</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-8 gap-y-1 divide-y sm:divide-y-0">
          {/* ROID */}
          <MetadataItem icon={<Hash className="h-4 w-4" />} label="ROID">
            <span className="inline-flex items-center gap-1.5">
              <code className="font-mono text-xs">{tombstone.roid}</code>
              <CopyButton
                value={tombstone.roid}
                variant="ghost"
                size="icon-sm"
                iconClassName="h-3 w-3"
                tooltip="Copy ROID"
              />
            </span>
          </MetadataItem>

          {/* Registrar */}
          <MetadataItem icon={<Building2 className="h-4 w-4" />} label="Registrar">
            <Link
              href={`/registrars/${tombstone.registrar_clid}`}
              className="text-primary hover:underline underline-offset-2"
            >
              {tombstone.registrar_clid}
            </Link>
          </MetadataItem>

          {/* TLD */}
          <MetadataItem icon={<Globe className="h-4 w-4" />} label="TLD">
            {tombstone.tld_name}
          </MetadataItem>

          {/* Registered At */}
          {registered && (
            <MetadataItem icon={<CalendarPlus className="h-4 w-4" />} label="Registered">
              <span title={registered.absolute}>{registered.relative}</span>
              <span className="text-xs text-muted-foreground ml-1.5">
                ({registered.absolute})
              </span>
            </MetadataItem>
          )}

          {/* Expired At */}
          {expired && (
            <MetadataItem icon={<CalendarX className="h-4 w-4" />} label="Expired">
              <span title={expired.absolute}>{expired.relative}</span>
              <span className="text-xs text-muted-foreground ml-1.5">
                ({expired.absolute})
              </span>
            </MetadataItem>
          )}

          {/* Purged At */}
          <MetadataItem icon={<Trash2 className="h-4 w-4" />} label="Purged">
            <span title={purged.absolute}>{purged.relative}</span>
            <span className="text-xs text-muted-foreground ml-1.5">
              ({purged.absolute})
            </span>
          </MetadataItem>

          {/* Purge Reason */}
          <MetadataItem icon={<ShieldAlert className="h-4 w-4" />} label="Purge Reason">
            <Badge variant={purgeReasonVariant(tombstone.purge_reason)}>
              {tombstone.purge_reason}
            </Badge>
          </MetadataItem>

          {/* DropCatch */}
          <MetadataItem icon={<Target className="h-4 w-4" />} label="DropCatch">
            <Badge variant={tombstone.drop_catch ? 'default' : 'outline'}>
              {tombstone.drop_catch ? 'Yes' : 'No'}
            </Badge>
          </MetadataItem>
        </div>
      </CardContent>
    </Card>
  );
}

function ArchivedEventsSection({ tombstone }: { tombstone: DomainTombstone }) {
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useEventSearch({ subject: tombstone.name, roid: tombstone.roid, limit: 50 });

  const events = data?.pages.flatMap((page) => page.data) ?? [];
  const totalCount = data?.pages[0]?.totalCount ?? 0;

  const [expandedEventId, setExpandedEventId] = useState<string | null>(null);
  const [activeDiffEvent, setActiveDiffEvent] = useState<DomainEvent | null>(null);

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <History className="h-5 w-5 text-muted-foreground" />
            Activity History
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className="border-destructive/30">
        <CardHeader>
          <CardTitle className="text-lg text-destructive">
            Failed to load activity history
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            {(error as any)?.response?.data?.error ||
              (error as any)?.message ||
              'An unexpected error occurred.'}
          </p>
        </CardHeader>
      </Card>
    );
  }

  if (!events || events.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <History className="h-5 w-5 text-muted-foreground" />
            Activity History
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            No events recorded for this incarnation.
          </p>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <History className="h-5 w-5 text-primary" />
          Activity History
        </CardTitle>
        <p className="text-xs text-muted-foreground">
          {totalCount > 0 && (
            <span className="tabular-nums">{totalCount} events · </span>
          )}
          Filtered by ROID to show only this incarnation.
        </p>
      </CardHeader>
      <CardContent className="relative pl-6 border-l border-muted-foreground/25 ml-4 space-y-6">
        {events.map((event) => {
          const isExpanded = expandedEventId === event.id;
          const eventTime = new Date(event.time);
          const relativeTime = formatDistanceToNow(eventTime, { addSuffix: true });

          return (
            <div key={event.id} className="relative group">
              {/* Timeline dot */}
              <div className="absolute -left-[35px] top-1 p-1 rounded-full border bg-muted flex items-center justify-center shadow-sm">
                <Clock className="h-3.5 w-3.5 text-muted-foreground" />
              </div>

              <div className="flex flex-col space-y-2">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h4 className="font-semibold text-sm capitalize">
                      {event.type.replace('domain.', '').replaceAll('_', ' ')}
                    </h4>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      <code className="font-mono text-[11px]">{event.type}</code>
                    </p>
                  </div>
                  <span
                    className="text-xs font-medium text-muted-foreground shrink-0"
                    title={eventTime.toLocaleString()}
                  >
                    {relativeTime}
                  </span>
                </div>

                {/* Metadata badges */}
                <div className="flex flex-wrap gap-1.5 items-center text-xs">
                  {event.data?.ClientID && (
                    <Badge variant="outline" className="font-mono bg-muted/30 text-[11px]">
                      {event.data.ClientID}
                    </Badge>
                  )}
                  {event.actor && (
                    <Badge variant="outline" className="font-mono bg-amber-50 dark:bg-amber-950/30 text-[11px]">
                      {event.actor}
                    </Badge>
                  )}
                </div>

                 {/* Expand toggle */}
                <div className="flex justify-end gap-2">
                  {(event.before_state || event.after_state) && (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 text-xs font-medium gap-1 text-primary hover:text-primary hover:bg-muted cursor-pointer"
                      onClick={() => setActiveDiffEvent(event)}
                    >
                      <GitCompare className="h-3.5 w-3.5" />
                      Diff State
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 text-xs font-medium gap-1 text-muted-foreground hover:text-foreground cursor-pointer"
                    onClick={() =>
                      setExpandedEventId(isExpanded ? null : event.id)
                    }
                  >
                    <FileCode className="h-3.5 w-3.5" />
                    {isExpanded ? 'Hide Payload' : 'Show Payload'}
                    {isExpanded ? (
                      <ChevronUp className="h-3.5 w-3.5" />
                    ) : (
                      <ChevronDown className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </div>

                {/* Structured payload / Raw JSON block */}
                {isExpanded && (
                  <div className="mt-2 text-xs border rounded-md bg-muted/65 dark:bg-zinc-950/50 p-3 max-h-[300px] overflow-auto font-mono scrollbar-thin space-y-3">
                    {/* Show structured state diffs when available */}
                    {(event.command || event.before_state || event.after_state) && (
                      <div className="space-y-2 font-sans">
                        {event.actor && (
                          <div className="text-xs text-muted-foreground">
                            Performed by: <span className="font-medium text-foreground">{event.actor}</span>
                          </div>
                        )}
                        {event.command && (
                          <details className="group relative group/code">
                            <summary className="cursor-pointer text-xs font-medium text-muted-foreground hover:text-foreground transition-colors">Command</summary>
                            <div className="relative">
                              <pre className="mt-1 pl-3 border-l-2 border-primary/30 overflow-x-auto pr-10"><code>{JSON.stringify(event.command, null, 2)}</code></pre>
                              <CopyButton value={JSON.stringify(event.command, null, 2)} className="absolute right-2 top-2 opacity-0 group-hover/code:opacity-100 transition-opacity" />
                            </div>
                          </details>
                        )}
                        {event.before_state && (
                          <details className="group relative group/code">
                            <summary className="cursor-pointer text-xs font-medium text-muted-foreground hover:text-foreground transition-colors">Before State</summary>
                            <div className="relative">
                              <pre className="mt-1 pl-3 border-l-2 border-orange-400/30 overflow-x-auto pr-10"><code>{JSON.stringify(event.before_state, null, 2)}</code></pre>
                              <CopyButton value={JSON.stringify(event.before_state, null, 2)} className="absolute right-2 top-2 opacity-0 group-hover/code:opacity-100 transition-opacity" />
                            </div>
                          </details>
                        )}
                        {event.after_state && (
                          <details className="group relative group/code">
                            <summary className="cursor-pointer text-xs font-medium text-muted-foreground hover:text-foreground transition-colors">After State</summary>
                            <div className="relative">
                              <pre className="mt-1 pl-3 border-l-2 border-emerald-400/30 overflow-x-auto pr-10"><code>{JSON.stringify(event.after_state, null, 2)}</code></pre>
                              <CopyButton value={JSON.stringify(event.after_state, null, 2)} className="absolute right-2 top-2 opacity-0 group-hover/code:opacity-100 transition-opacity" />
                            </div>
                          </details>
                        )}
                      </div>
                    )}
                    {/* Always show full raw JSON */}
                    <details className={`relative group/code ${event.command || event.before_state || event.after_state ? 'group' : 'group open'}`}>
                      <summary className="cursor-pointer text-xs font-medium text-muted-foreground hover:text-foreground transition-colors font-sans font-medium">
                        Raw Event JSON
                      </summary>
                      <div className="relative">
                        <pre className="mt-1 overflow-x-auto pr-10"><code>{JSON.stringify(event, null, 2)}</code></pre>
                        <CopyButton value={JSON.stringify(event, null, 2)} className="absolute right-2 top-2 opacity-0 group-hover/code:opacity-100 transition-opacity" />
                      </div>
                    </details>
                  </div>
                )}
              </div>
            </div>
          );
        })}

        {hasNextPage && (
          <div className="pt-4 flex justify-center">
            <Button
              variant="outline"
              size="sm"
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
              className="text-xs gap-1.5"
            >
              {isFetchingNextPage ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  Loading...
                </>
              ) : (
                <>Load earlier events</>
              )}
            </Button>
          </div>
        )}
      </CardContent>

      <StateDiffModal
        isOpen={!!activeDiffEvent}
        onClose={() => setActiveDiffEvent(null)}
        before={activeDiffEvent?.before_state}
        after={activeDiffEvent?.after_state}
        title="State Difference"
        subtitle={`Comparing before/after state for event: ${activeDiffEvent ? activeDiffEvent.type.replace('domain.', '').replaceAll('_', ' ') : ""}`}
      />
    </Card>
  );
}

function LastSnapshotSection({ snapshot }: { snapshot: any }) {
  const [isOpen, setIsOpen] = useState(false);

  if (!snapshot) return null;

  return (
    <Card>
      <CardContent className="pt-6">
        <button
          onClick={() => setIsOpen(!isOpen)}
          className="flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground transition-colors w-full text-left cursor-pointer"
        >
          <FileCode className="h-4 w-4" />
          Last Known Snapshot
          {isOpen ? (
            <ChevronUp className="h-4 w-4 ml-auto" />
          ) : (
            <ChevronDown className="h-4 w-4 ml-auto" />
          )}
        </button>
        {isOpen && (
          <div className="mt-4 rounded-md border bg-muted/65 dark:bg-zinc-950/50 p-4 max-h-[500px] overflow-auto scrollbar-thin">
            <pre className="text-xs font-mono whitespace-pre-wrap break-all">
              <code>{JSON.stringify(snapshot, null, 2)}</code>
            </pre>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------

export function ArchivedDomainView({
  tombstones,
  domainName,
}: ArchivedDomainViewProps) {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const selected = tombstones[selectedIndex];

  if (!selected) return null;

  return (
    <div className="space-y-6">
      {/* Archive Banner */}
      <ArchiveBanner
        tombstone={selected}
        totalIncarnations={tombstones.length}
        domainName={domainName}
      />

      {/* Incarnation Picker (multi-incarnation only) */}
      {tombstones.length > 1 && (
        <IncarnationPicker
          tombstones={tombstones}
          selectedIndex={selectedIndex}
          onSelect={setSelectedIndex}
        />
      )}

      {/* Metadata */}
      <TombstoneMetadataCard tombstone={selected} />

      {/* Events — filtered by ROID */}
      <ArchivedEventsSection tombstone={selected} />

      {/* Last Snapshot */}
      <LastSnapshotSection snapshot={selected.last_snapshot} />
    </div>
  );
}
