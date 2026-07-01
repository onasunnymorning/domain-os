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
import { cn } from '@/lib/utils';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Calendar } from '@/components/ui/calendar';
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from '@/components/ui/tooltip';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useRegistrars } from '@/lib/hooks/useRegistrars';
import { searchEvents } from '@/lib/api/events';
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
  Download,
  Calendar as CalendarIcon,
  SlidersHorizontal,
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

  // Extract domain name target — always link, even for purged/deleted domains.
  // The domain detail page handles 404 → tombstone archive fallback.
  if (event.type.startsWith('domain.')) {
    const domainName = event.data?.DomainName || event.data?.domainName || (event.subject && event.subject !== 'bulk' ? event.subject : undefined);
    if (domainName && typeof domainName === 'string') {
      targets.push({
        text: domainName,
        href: `/domains/${encodeURIComponent(domainName)}`,
      });
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
  { value: 'domain.auto_renewed', label: 'Domain Auto-Renewed' },
  { value: 'domain.expired', label: 'Domain Expired' },
  { value: 'domain.purged', label: 'Domain Purged' },
  { value: 'domain.updated', label: 'Domain Updated' },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EventFeed() {
  const [showFilters, setShowFilters] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [isExporting, setIsExporting] = useState(false);

  // Filter States
  const [typeFilter, setTypeFilter] = useState('');
  const [subjectFilter, setSubjectFilter] = useState('');
  const [actorFilter, setActorFilter] = useState('');
  const [roidFilter, setRoidFilter] = useState('');
  const [openReg, setOpenReg] = useState(false);
  const [regSearch, setRegSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [traceIdFilter, setTraceIdFilter] = useState('');
  const [correlationIdFilter, setCorrelationIdFilter] = useState('');
  const [dateAfter, setDateAfter] = useState<Date | undefined>(undefined);
  const [dateBefore, setDateBefore] = useState<Date | undefined>(undefined);

  const [expandedId, setExpandedId] = useState<string | null>(null);

  // Fetch active registrars for the dropdown
  const { data: registrarData } = useRegistrars({ pagesize: 200 });
  const registrars = registrarData?.Data ?? [];

  const filteredRegistrars = useMemo(() => {
    if (!regSearch) return registrars;
    const searchLower = regSearch.toLowerCase();
    return registrars.filter((r) =>
      r.ClID.toLowerCase().includes(searchLower) ||
      (r.Name && r.Name.toLowerCase().includes(searchLower))
    );
  }, [registrars, regSearch]);

  // Build search params — only include non-empty values
  const searchParams = useMemo<Omit<EventSearchParams, 'cursor'>>(() => {
    const params: Omit<EventSearchParams, 'cursor'> = { limit: 25 };
    if (typeFilter) params.type = typeFilter;
    if (subjectFilter) params.subject = subjectFilter;
    if (actorFilter) params.actor = actorFilter;
    if (roidFilter) params.roid = roidFilter;
    if (sourceFilter) params.source = sourceFilter;
    if (traceIdFilter) params.trace_id = traceIdFilter;
    if (correlationIdFilter) params.correlation_id = correlationIdFilter;

    if (dateAfter) {
      const startOfDay = new Date(dateAfter);
      startOfDay.setHours(0, 0, 0, 0);
      params.after = startOfDay.toISOString();
    }
    if (dateBefore) {
      const endOfDay = new Date(dateBefore);
      endOfDay.setHours(23, 59, 59, 999);
      params.before = endOfDay.toISOString();
    }
    return params;
  }, [
    typeFilter,
    subjectFilter,
    actorFilter,
    roidFilter,
    sourceFilter,
    traceIdFilter,
    correlationIdFilter,
    dateAfter,
    dateBefore,
  ]);

  const hasActiveFilters = !!(
    typeFilter ||
    subjectFilter ||
    actorFilter ||
    roidFilter ||
    sourceFilter ||
    traceIdFilter ||
    correlationIdFilter ||
    dateAfter ||
    dateBefore
  );

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
    setRoidFilter('');
    setRegSearch('');
    setSourceFilter('');
    setTraceIdFilter('');
    setCorrelationIdFilter('');
    setDateAfter(undefined);
    setDateBefore(undefined);
    setShowAdvanced(false);
  };

  const exportToCSV = async () => {
    setIsExporting(true);
    try {
      const allEvents: any[] = [];
      let currentCursor: string | undefined = undefined;
      let pageCount = 0;
      const maxPages = 5; // Capped at 1,000 matches (5 pages of 200 events)

      // Copy active filters but request maximum page limit
      const exportParams = {
        ...searchParams,
        limit: 200,
      };

      while (pageCount < maxPages) {
        const result = await searchEvents({
          ...exportParams,
          cursor: currentCursor,
        });

        if (!result.data || result.data.length === 0) {
          break;
        }

        allEvents.push(...result.data);

        if (!result.nextCursor || result.data.length < 200) {
          break;
        }

        currentCursor = result.nextCursor;
        pageCount++;
      }

      // Formulate CSV file content
      const headers = ['ID', 'Time', 'Type', 'Subject', 'Actor', 'RoID', 'Source', 'TraceID', 'CorrelationID', 'Description'];
      const csvRows = [headers.join(',')];

      for (const evt of allEvents) {
        const row = [
          evt.id || '',
          evt.time || '',
          evt.type || '',
          `"${(evt.subject || '').replace(/"/g, '""')}"`,
          `"${(evt.actor || '').replace(/"/g, '""')}"`,
          `"${(evt.roid || '').replace(/"/g, '""')}"`,
          `"${(evt.source || '').replace(/"/g, '""')}"`,
          evt.trace_id || '',
          evt.correlation_id || '',
          `"${(evt.description || '').replace(/"/g, '""')}"`
        ];
        csvRows.push(row.join(','));
      }

      // Safe Blob-based download to support large files without URI size limitations
      const csvContent = "\uFEFF" + csvRows.join("\n");
      const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.setAttribute("href", url);
      link.setAttribute("download", `events_export_${new Date().toISOString().slice(0, 10)}.csv`);
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error("Event CSV Export Failed:", err);
    } finally {
      setIsExporting(false);
    }
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
              <span className="text-xs text-muted-foreground tabular-nums mr-1">
                {totalCount.toLocaleString()} total
              </span>
            )}

            {tier && (
              <Badge
                variant={tier === 'hot' ? 'secondary' : 'outline'}
                className={cn(
                  "text-[10px] capitalize font-medium mr-1 select-none",
                  tier === 'hot' && "bg-emerald-500/10 text-emerald-600 border-emerald-500/20 dark:bg-emerald-500/20 dark:text-emerald-400",
                  tier === 'warm' && "bg-amber-500/10 text-amber-600 border-amber-500/20 dark:bg-amber-500/20 dark:text-amber-400",
                  tier === 'mixed' && "bg-blue-500/10 text-blue-600 border-blue-500/20 dark:bg-blue-500/20 dark:text-blue-400"
                )}
              >
                {tier} tier
              </Badge>
            )}

            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    onClick={exportToCSV}
                    disabled={isExporting || events.length === 0}
                  >
                    {isExporting ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Download className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent align="end" className="max-w-[220px]">
                  <p className="text-[11px] leading-relaxed">
                    Export results to CSV. Exports are capped at the first 1,000 matches for performance and S3 safety.
                  </p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>

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
          <div className="mt-3 space-y-3 animate-in fade-in-0 slide-in-from-top-2 duration-200 p-3 bg-muted/30 border rounded-lg">
            {/* Main Filters Row 1 */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium text-muted-foreground uppercase px-1">Event Type</span>
                <select
                  value={typeFilter}
                  onChange={(e) => setTypeFilter(e.target.value)}
                  className="flex h-8 w-full rounded-md border border-input bg-background px-2 py-1 text-xs ring-offset-background focus:outline-none focus:ring-1 focus:ring-ring"
                >
                  {EVENT_TYPE_FILTERS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium text-muted-foreground uppercase px-1">Subject (Domain / ID)</span>
                <Input
                  placeholder="e.g. example.best"
                  value={subjectFilter}
                  onChange={(e) => setSubjectFilter(e.target.value)}
                  className="h-8 text-xs w-full"
                />
              </div>

              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium text-muted-foreground uppercase px-1">Registrar</span>
                <Popover open={openReg} onOpenChange={setOpenReg}>
                  <PopoverTrigger asChild>
                    <Button
                      variant="outline"
                      role="combobox"
                      aria-expanded={openReg}
                      className={cn(
                        "h-8 w-full justify-between text-left text-xs font-normal bg-background",
                        !roidFilter && "text-muted-foreground"
                      )}
                    >
                      <span className="truncate">
                        {roidFilter
                          ? (registrars.find((r) => r.ClID === roidFilter)?.Name
                            ? `${registrars.find((r) => r.ClID === roidFilter)?.Name} (${roidFilter})`
                            : roidFilter)
                          : "All Registrars"}
                      </span>
                      <ChevronDown className="ml-2 h-3.5 w-3.5 shrink-0 opacity-50" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-80 p-2" align="start">
                    <Input
                      placeholder="Search registrar..."
                      value={regSearch}
                      onChange={(e) => setRegSearch(e.target.value)}
                      className="mb-2 h-8 text-xs"
                    />
                    <ScrollArea className="h-52">
                      <div className="space-y-1">
                        <Button
                          variant={roidFilter === '' ? "secondary" : "ghost"}
                          className="w-full justify-start text-xs h-8 font-normal"
                          onClick={() => {
                            setRoidFilter('');
                            setOpenReg(false);
                          }}
                        >
                          All Registrars
                        </Button>
                        {filteredRegistrars.map((opt) => (
                          <Button
                            key={opt.ClID}
                            variant={opt.ClID === roidFilter ? "secondary" : "ghost"}
                            className="w-full justify-start text-xs h-8 font-normal"
                            onClick={() => {
                              setRoidFilter(opt.ClID);
                              setOpenReg(false);
                            }}
                          >
                            <span className="truncate">
                              {opt.Name ? `${opt.Name} (${opt.ClID})` : opt.ClID}
                            </span>
                          </Button>
                        ))}
                        {filteredRegistrars.length === 0 && (
                          <div className="text-xs text-muted-foreground px-2 py-1">No results</div>
                        )}
                      </div>
                    </ScrollArea>
                  </PopoverContent>
                </Popover>
              </div>
            </div>

            {/* Date Filters Row */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium text-muted-foreground uppercase px-1">From Date</span>
                <Popover>
                  <PopoverTrigger asChild>
                    <Button
                      variant={"outline"}
                      className={cn(
                        "h-8 w-full justify-start text-left text-xs font-normal bg-background",
                        !dateAfter && "text-muted-foreground"
                      )}
                    >
                      <CalendarIcon className="mr-2 h-3.5 w-3.5 shrink-0" />
                      {dateAfter ? dateAfter.toLocaleDateString() : <span>Pick start date</span>}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-0" align="start">
                    <Calendar
                      mode="single"
                      selected={dateAfter}
                      onSelect={setDateAfter}
                      initialFocus
                    />
                  </PopoverContent>
                </Popover>
              </div>

              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium text-muted-foreground uppercase px-1">To Date</span>
                <Popover>
                  <PopoverTrigger asChild>
                    <Button
                      variant={"outline"}
                      className={cn(
                        "h-8 w-full justify-start text-left text-xs font-normal bg-background",
                        !dateBefore && "text-muted-foreground"
                      )}
                    >
                      <CalendarIcon className="mr-2 h-3.5 w-3.5 shrink-0" />
                      {dateBefore ? dateBefore.toLocaleDateString() : <span>Pick end date</span>}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-0" align="start">
                    <Calendar
                      mode="single"
                      selected={dateBefore}
                      onSelect={setDateBefore}
                      initialFocus
                    />
                  </PopoverContent>
                </Popover>
              </div>
            </div>

            {/* Advanced Toggle */}
            <div className="pt-1">
              <Button
                variant="ghost"
                size="sm"
                className="h-6 text-[11px] gap-1 px-1 text-muted-foreground hover:text-foreground"
                onClick={() => setShowAdvanced(!showAdvanced)}
              >
                <SlidersHorizontal className="h-3 w-3" />
                {showAdvanced ? "Hide Advanced Filters" : "Show Advanced Filters"}
              </Button>
            </div>

            {/* Collapsible Advanced Filters */}
            {showAdvanced && (
              <div className="grid grid-cols-1 md:grid-cols-4 gap-2 pt-1 border-t border-dashed animate-in fade-in-0 duration-200">
                <div className="flex flex-col gap-1">
                  <span className="text-[10px] font-medium text-muted-foreground uppercase px-1">Actor</span>
                  <Input
                    placeholder="admin@test.com"
                    value={actorFilter}
                    onChange={(e) => setActorFilter(e.target.value)}
                    className="h-8 text-xs w-full"
                  />
                </div>

                <div className="flex flex-col gap-1">
                  <span className="text-[10px] font-medium text-muted-foreground uppercase px-1">Source</span>
                  <select
                    value={sourceFilter}
                    onChange={(e) => setSourceFilter(e.target.value)}
                    className="flex h-8 w-full rounded-md border border-input bg-background px-2 py-1 text-xs ring-offset-background focus:outline-none focus:ring-1 focus:ring-ring"
                  >
                    <option value="">All Sources</option>
                    <option value="domain-os/api">API</option>
                    <option value="domain-os/worker">Worker</option>
                    <option value="domain-os/epp">EPP</option>
                  </select>
                </div>

                <div className="flex flex-col gap-1">
                  <span className="text-[10px] font-medium text-muted-foreground uppercase px-1 font-mono">Trace ID</span>
                  <Input
                    placeholder="trace-xxx"
                    value={traceIdFilter}
                    onChange={(e) => setTraceIdFilter(e.target.value)}
                    className="h-8 text-xs w-full font-mono"
                  />
                </div>

                <div className="flex flex-col gap-1">
                  <span className="text-[10px] font-medium text-muted-foreground uppercase px-1 font-mono">Correlation ID</span>
                  <Input
                    placeholder="corr-xxx"
                    value={correlationIdFilter}
                    onChange={(e) => setCorrelationIdFilter(e.target.value)}
                    className="h-8 text-xs w-full font-mono"
                  />
                </div>
              </div>
            )}

            {/* Filter actions */}
            {hasActiveFilters && (
              <div className="flex justify-start pt-1">
                <Button variant="ghost" size="sm" onClick={clearFilters} className="h-7 text-xs gap-1 text-destructive hover:bg-destructive/10">
                  <X className="h-3 w-3" />
                  Clear all filters
                </Button>
              </div>
            )}
          </div>
        )}

        {totalCount > 1000 && showFilters && (
          <div className="mt-2 rounded-md border border-amber-500/30 bg-amber-500/5 p-2 text-[11px] text-amber-600 dark:text-amber-400 flex items-center gap-1.5 animate-in fade-in-0 duration-200">
            <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
            <span>Search matches {totalCount.toLocaleString()} events. Exporting will only download the first 1,000 matches.</span>
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
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              setTraceIdFilter(event.trace_id || '');
                              setShowFilters(true);
                              setShowAdvanced(true);
                            }}
                            className="inline-flex items-center rounded-md border px-1.5 py-0.5 text-[10px] font-mono font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 border-transparent bg-primary/10 text-primary hover:bg-primary/20 cursor-pointer"
                            title="Filter all events by this Trace ID"
                          >
                            trace:{event.trace_id.slice(0, 8)}... (Filter)
                          </button>
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
