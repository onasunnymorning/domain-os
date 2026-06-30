'use client';

import { useState, useMemo } from 'react';
import Link from 'next/link';
import { useEventSearch } from '@/lib/hooks/useEventSearch';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { formatDistanceToNow, parseISO } from 'date-fns';
import {
  CheckCircle2,
  RefreshCw,
  Lock,
  Unlock,
  Server,
  Trash2,
  Activity,
  ChevronDown,
  ChevronUp,
  Radio,
  Users,
  Target,
  Shield,
  AlertTriangle,
  RotateCcw,
  UserPlus,
  UserMinus,
  Settings,
  Filter,
  X,
  Loader2,
  User,
} from 'lucide-react';
import type { EventSearchParams } from '@/lib/api/events';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

interface LinkTarget {
  text: string;
  href: string;
}

function renderClickableText(text: string, event: any) {
  const targets: LinkTarget[] = [];

  // Extract domain name target
  if (event.type.startsWith('domain.')) {
    const isDeleted = event.type === 'domain.admin_deleted' || event.type === 'domain.purged';
    if (!isDeleted) {
      const domainName = event.data?.DomainName || event.data?.domainName || (event.subject && event.subject !== 'bulk' ? event.subject : undefined);
      if (domainName && typeof domainName === 'string') {
        targets.push({
          text: domainName,
          href: `/domains/${encodeURIComponent(domainName)}`,
        });
      }
    }
  }

  // Extract registrar client ID target
  const isRegistrarDeleted = event.type === 'registrar.deleted';
  if (!isRegistrarDeleted) {
    const clid = event.data?.ClientID || event.data?.clientId || event.data?.ClID || event.data?.clid || (event.type.startsWith('registrar.') && event.subject && event.subject !== 'bulk' ? event.subject : undefined);
    if (clid && typeof clid === 'string') {
      targets.push({
        text: clid,
        href: `/registrars/${encodeURIComponent(clid)}`,
      });
    }
  }

  // Deduplicate and filter out empty texts or text not present in display text
  const activeTargets: LinkTarget[] = [];
  const seenTexts = new Set<string>();

  for (const target of targets) {
    if (target.text && text.includes(target.text) && !seenTexts.has(target.text)) {
      activeTargets.push(target);
      seenTexts.add(target.text);
    }
  }

  if (activeTargets.length === 0) {
    return text;
  }

  // Sort by length of text descending to match longest first
  activeTargets.sort((a, b) => b.text.length - a.text.length);

  // Recursively split the text and insert Links
  const renderParts = (currentText: string, remainingTargets: LinkTarget[]): React.ReactNode[] => {
    if (!currentText) return [];
    if (remainingTargets.length === 0) return [currentText];

    const [first, ...rest] = remainingTargets;
    const index = currentText.indexOf(first.text);

    if (index === -1) {
      return renderParts(currentText, rest);
    }

    const before = currentText.substring(0, index);
    const match = currentText.substring(index, index + first.text.length);
    const after = currentText.substring(index + first.text.length);

    return [
      ...renderParts(before, rest),
      <Link
        key={`${match}-${index}`}
        href={first.href}
        onClick={(e) => e.stopPropagation()}
        className="text-primary hover:underline font-semibold transition-colors"
      >
        {match}
      </Link>,
      ...renderParts(after, remainingTargets),
    ];
  };

  return <>{renderParts(text, activeTargets)}</>;
}


// ---------------------------------------------------------------------------
// Event type → visual config
// ---------------------------------------------------------------------------

function getEventConfig(type: string) {
  // Domain events
  if (type.startsWith('domain.')) {
    switch (type) {
      case 'domain.registered':
        return { icon: <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />, color: 'bg-emerald-500', label: 'Registered' };
      case 'domain.admin_created':
        return { icon: <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />, color: 'bg-emerald-500', label: 'Admin Created' };
      case 'domain.bulk_created':
        return { icon: <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />, color: 'bg-emerald-500', label: 'Bulk Created' };
      case 'domain.renewed':
        return { icon: <RefreshCw className="h-3.5 w-3.5 text-blue-600" />, color: 'bg-blue-500', label: 'Renewed' };
      case 'domain.auto_renewed':
        return { icon: <RefreshCw className="h-3.5 w-3.5 text-blue-600" />, color: 'bg-blue-500', label: 'Auto-Renewed' };
      case 'domain.updated':
        return { icon: <Settings className="h-3.5 w-3.5 text-sky-600" />, color: 'bg-sky-500', label: 'Updated' };
      case 'domain.status_set':
        return { icon: <Lock className="h-3.5 w-3.5 text-amber-600" />, color: 'bg-amber-500', label: 'Status Set' };
      case 'domain.status_unset':
        return { icon: <Unlock className="h-3.5 w-3.5 text-orange-600" />, color: 'bg-orange-500', label: 'Status Unset' };
      case 'domain.host_added':
        return { icon: <Server className="h-3.5 w-3.5 text-violet-600" />, color: 'bg-violet-500', label: 'NS Added' };
      case 'domain.host_removed':
        return { icon: <Server className="h-3.5 w-3.5 text-pink-600" />, color: 'bg-pink-500', label: 'NS Removed' };
      case 'domain.hosts_cleared':
        return { icon: <Server className="h-3.5 w-3.5 text-pink-600" />, color: 'bg-pink-500', label: 'NS Cleared' };
      case 'domain.dropcatch_updated':
        return { icon: <Target className="h-3.5 w-3.5 text-orange-600" />, color: 'bg-orange-500', label: 'Dropcatch' };
      case 'domain.transferred':
        return { icon: <RotateCcw className="h-3.5 w-3.5 text-purple-600" />, color: 'bg-purple-500', label: 'Transferred' };
      case 'domain.expired':
        return { icon: <AlertTriangle className="h-3.5 w-3.5 text-red-600" />, color: 'bg-red-500', label: 'Expired' };
      case 'domain.marked_for_deletion':
        return { icon: <Trash2 className="h-3.5 w-3.5 text-red-600" />, color: 'bg-red-500', label: 'Pending Delete' };
      case 'domain.admin_deleted':
        return { icon: <Trash2 className="h-3.5 w-3.5 text-red-600" />, color: 'bg-red-500', label: 'Deleted' };
      case 'domain.purged':
        return { icon: <Trash2 className="h-3.5 w-3.5 text-red-600" />, color: 'bg-red-500', label: 'Purged' };
      case 'domain.restored':
        return { icon: <RefreshCw className="h-3.5 w-3.5 text-teal-600" />, color: 'bg-teal-500', label: 'Restored' };
      default:
        return { icon: <Activity className="h-3.5 w-3.5 text-muted-foreground" />, color: 'bg-muted-foreground', label: type.replace('domain.', '').replaceAll('_', ' ') };
    }
  }

  // Registrar events
  if (type.startsWith('registrar.')) {
    switch (type) {
      case 'registrar.created':
        return { icon: <UserPlus className="h-3.5 w-3.5 text-emerald-600" />, color: 'bg-emerald-500', label: 'Registrar Created' };
      case 'registrar.bulk_created':
        return { icon: <UserPlus className="h-3.5 w-3.5 text-emerald-600" />, color: 'bg-emerald-500', label: 'Registrars Imported' };
      case 'registrar.updated':
        return { icon: <Settings className="h-3.5 w-3.5 text-blue-600" />, color: 'bg-blue-500', label: 'Registrar Updated' };
      case 'registrar.deleted':
        return { icon: <UserMinus className="h-3.5 w-3.5 text-red-600" />, color: 'bg-red-500', label: 'Registrar Deleted' };
      case 'registrar.status_updated':
        return { icon: <Shield className="h-3.5 w-3.5 text-amber-600" />, color: 'bg-amber-500', label: 'Registrar Status' };
      case 'registrar.iana_status_updated':
        return { icon: <Shield className="h-3.5 w-3.5 text-amber-600" />, color: 'bg-amber-500', label: 'IANA Status' };
      default:
        return { icon: <Users className="h-3.5 w-3.5 text-blue-600" />, color: 'bg-blue-500', label: type.replace('registrar.', '').replaceAll('_', ' ') };
    }
  }

  // Fallback
  return {
    icon: <Activity className="h-3.5 w-3.5 text-muted-foreground" />,
    color: 'bg-muted-foreground',
    label: type,
  };
}

// ---------------------------------------------------------------------------
// Event type categories for filter dropdown
// ---------------------------------------------------------------------------
const EVENT_TYPE_FILTERS = [
  { value: '', label: 'All Types' },
  { value: 'domain.*', label: 'All Domain Events' },
  { value: 'registrar.*', label: 'All Registrar Events' },
  { value: 'contact.*', label: 'All Contact Events' },
  { value: 'host.*', label: 'All Host Events' },
  { value: 'tld.*', label: 'All TLD Events' },
  { value: 'domain.registered', label: 'Domain Registered' },
  { value: 'domain.renewed', label: 'Domain Renewed' },
  { value: 'domain.expired', label: 'Domain Expired' },
  { value: 'domain.purged', label: 'Domain Purged' },
  { value: 'domain.updated', label: 'Domain Updated' },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EventFeed() {
  const [showFilters, setShowFilters] = useState(false);
  const [typeFilter, setTypeFilter] = useState('');
  const [subjectFilter, setSubjectFilter] = useState('');
  const [actorFilter, setActorFilter] = useState('');
  const [expandedId, setExpandedId] = useState<string | null>(null);

  // Build search params — only include non-empty values
  const searchParams = useMemo<Omit<EventSearchParams, 'cursor'>>(() => {
    const params: Omit<EventSearchParams, 'cursor'> = { limit: 25 };
    if (typeFilter) params.type = typeFilter;
    if (subjectFilter) params.subject = subjectFilter;
    if (actorFilter) params.actor = actorFilter;
    return params;
  }, [typeFilter, subjectFilter, actorFilter]);

  const hasActiveFilters = typeFilter || subjectFilter || actorFilter;

  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useEventSearch(searchParams, {
    refetchInterval: hasActiveFilters ? undefined : 30_000, // Only auto-refresh when unfiltered
  });

  // Flatten paginated results
  const events = data?.pages.flatMap(page => page.data) ?? [];
  const totalCount = data?.pages[0]?.totalCount ?? 0;
  const tier = data?.pages[0]?.tier;

  const clearFilters = () => {
    setTypeFilter('');
    setSubjectFilter('');
    setActorFilter('');
  };

  return (
    <Card className="h-full">
      <CardHeader className="pb-4">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-base">
            <Radio className="h-4 w-4 text-primary" />
            Latest Events
          </CardTitle>
          <div className="flex items-center gap-2">
            {totalCount > 0 && (
              <span className="text-xs text-muted-foreground tabular-nums">
                {totalCount.toLocaleString()} total
              </span>
            )}
            {tier && tier !== 'hot' && (
              <Badge variant="outline" className="text-[10px]">
                {tier}
              </Badge>
            )}
            <Button
              variant={showFilters ? 'secondary' : 'ghost'}
              size="icon"
              className="h-7 w-7"
              onClick={() => setShowFilters(!showFilters)}
              title="Toggle filters"
            >
              <Filter className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>

        {/* Filter bar */}
        {showFilters && (
          <div className="mt-3 space-y-2 animate-in fade-in-0 slide-in-from-top-2 duration-200">
            <div className="flex gap-2">
              <select
                value={typeFilter}
                onChange={(e) => setTypeFilter(e.target.value)}
                className="flex h-8 rounded-md border border-input bg-background px-2 py-1 text-xs ring-offset-background focus:outline-none focus:ring-1 focus:ring-ring"
              >
                {EVENT_TYPE_FILTERS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
              <Input
                placeholder="Subject (domain, registrar...)"
                value={subjectFilter}
                onChange={(e) => setSubjectFilter(e.target.value)}
                className="h-8 text-xs"
              />
              <Input
                placeholder="Actor"
                value={actorFilter}
                onChange={(e) => setActorFilter(e.target.value)}
                className="h-8 text-xs w-32"
              />
            </div>
            {hasActiveFilters && (
              <Button variant="ghost" size="sm" onClick={clearFilters} className="h-6 text-xs gap-1 text-muted-foreground">
                <X className="h-3 w-3" />
                Clear filters
              </Button>
            )}
          </div>
        )}
      </CardHeader>
      <CardContent>
        {isLoading && (
          <div className="space-y-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3">
                <Skeleton className="h-2 w-2 rounded-full shrink-0" />
                <Skeleton className="h-4 flex-1" />
                <Skeleton className="h-4 w-16" />
              </div>
            ))}
          </div>
        )}

        {error && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
            Failed to load events: {(error as any)?.response?.data?.error || (error as any)?.message || 'Unknown error'}
          </div>
        )}

        {!isLoading && !error && events.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-sm text-muted-foreground">
            <Activity className="h-8 w-8 mb-2 opacity-40" />
            {hasActiveFilters ? 'No events match your filters' : 'No events recorded yet'}
          </div>
        )}

        {!isLoading && !error && events.length > 0 && (
          <div className="space-y-1">
            {events.map((event) => {
              const config = getEventConfig(event.type);
              const isExpanded = expandedId === event.id;
              const eventTime = parseISO(event.time);
              const relative = formatDistanceToNow(eventTime, { addSuffix: true });

              // Use description when available, fall back to subject for legacy events
              const displayText = event.description || event.subject;

              return (
                <div key={event.id} className="group">
                  <div
                    role="button"
                    tabIndex={0}
                    className="flex w-full items-center gap-3 rounded-md px-2 py-2 text-left transition-colors hover:bg-accent/50 cursor-pointer focus:outline-none focus:ring-1 focus:ring-ring"
                    onClick={() => setExpandedId(isExpanded ? null : event.id)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setExpandedId(isExpanded ? null : event.id);
                      }
                    }}
                  >
                    {/* Status dot */}
                    <span
                      className={`h-2 w-2 rounded-full shrink-0 ${config.color}`}
                    />

                    {/* Event info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <Badge
                          variant="secondary"
                          className="text-[10px] font-medium px-1.5 py-0 capitalize shrink-0"
                        >
                          {config.label}
                        </Badge>
                        <span className="text-sm truncate text-foreground">
                          {renderClickableText(displayText, event)}
                        </span>
                        {event.actor && (
                          <span className="text-[10px] text-muted-foreground flex items-center gap-0.5 shrink-0" title={`Actor: ${event.actor}`}>
                            <User className="h-2.5 w-2.5" />
                            {event.actor.split('@')[0]}
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Timestamp + expand icon */}
                    <span
                      className="text-[11px] text-muted-foreground whitespace-nowrap tabular-nums shrink-0"
                      title={eventTime.toLocaleString()}
                    >
                      {relative}
                    </span>
                    {isExpanded ? (
                      <ChevronUp className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    ) : (
                      <ChevronDown className="h-3.5 w-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
                    )}
                  </div>

                  {/* Expanded payload */}
                  {isExpanded && (
                    <div className="ml-7 mr-2 mb-2 mt-1 rounded-md border bg-muted/50 p-3 text-xs font-mono max-h-[200px] overflow-auto animate-in fade-in-0 slide-in-from-top-1 duration-200">
                      <div className="flex flex-wrap gap-2 mb-2 font-sans">
                        <Badge variant="outline" className="text-[10px]">
                          {event.type}
                        </Badge>
                        <Badge variant="outline" className="text-[10px]">
                          {event.source}
                        </Badge>
                        {event.subject && (
                          <Badge variant="outline" className="text-[10px]">
                            {event.subject}
                          </Badge>
                        )}
                        {event.actor && (
                          <Badge variant="outline" className="text-[10px] bg-amber-50 dark:bg-amber-950/30">
                            <User className="h-2.5 w-2.5 mr-1" />
                            {event.actor}
                          </Badge>
                        )}
                        {event.trace_id && (
                          <Badge variant="outline" className="text-[10px] font-mono">
                            trace:{event.trace_id.slice(0, 8)}
                          </Badge>
                        )}
                      </div>
                      <pre className="text-muted-foreground leading-relaxed">
                        {JSON.stringify(event.data, null, 2)}
                      </pre>
                    </div>
                  )}
                </div>
              );
            })}

            {/* Load more button */}
            {hasNextPage && (
              <div className="pt-3 flex justify-center">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => fetchNextPage()}
                  disabled={isFetchingNextPage}
                  className="text-xs gap-1.5 h-7"
                >
                  {isFetchingNextPage ? (
                    <><Loader2 className="h-3 w-3 animate-spin" />Loading...</>
                  ) : (
                    'Load more events'
                  )}
                </Button>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
