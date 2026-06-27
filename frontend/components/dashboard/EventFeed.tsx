'use client';

import { useState } from 'react';
import { useRecentEvents } from '@/lib/hooks/useRecentEvents';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
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
} from 'lucide-react';

// ---------------------------------------------------------------------------
// Event type → visual config
// ---------------------------------------------------------------------------

function getEventConfig(type: string) {
  // Domain events
  if (type.startsWith('domain.')) {
    switch (type) {
      case 'domain.registered':
        return {
          icon: <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />,
          color: 'bg-emerald-500',
          label: 'Registered',
        };
      case 'domain.admin_created':
        return {
          icon: <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />,
          color: 'bg-emerald-500',
          label: 'Admin Created',
        };
      case 'domain.bulk_created':
        return {
          icon: <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />,
          color: 'bg-emerald-500',
          label: 'Bulk Created',
        };
      case 'domain.renewed':
        return {
          icon: <RefreshCw className="h-3.5 w-3.5 text-blue-600" />,
          color: 'bg-blue-500',
          label: 'Renewed',
        };
      case 'domain.auto_renewed':
        return {
          icon: <RefreshCw className="h-3.5 w-3.5 text-blue-600" />,
          color: 'bg-blue-500',
          label: 'Auto-Renewed',
        };
      case 'domain.updated':
        return {
          icon: <Settings className="h-3.5 w-3.5 text-sky-600" />,
          color: 'bg-sky-500',
          label: 'Updated',
        };
      case 'domain.status_set':
        return {
          icon: <Lock className="h-3.5 w-3.5 text-amber-600" />,
          color: 'bg-amber-500',
          label: 'Status Set',
        };
      case 'domain.status_unset':
        return {
          icon: <Unlock className="h-3.5 w-3.5 text-orange-600" />,
          color: 'bg-orange-500',
          label: 'Status Unset',
        };
      case 'domain.host_added':
        return {
          icon: <Server className="h-3.5 w-3.5 text-violet-600" />,
          color: 'bg-violet-500',
          label: 'NS Added',
        };
      case 'domain.host_removed':
        return {
          icon: <Server className="h-3.5 w-3.5 text-pink-600" />,
          color: 'bg-pink-500',
          label: 'NS Removed',
        };
      case 'domain.hosts_cleared':
        return {
          icon: <Server className="h-3.5 w-3.5 text-pink-600" />,
          color: 'bg-pink-500',
          label: 'NS Cleared',
        };
      case 'domain.dropcatch_updated':
        return {
          icon: <Target className="h-3.5 w-3.5 text-orange-600" />,
          color: 'bg-orange-500',
          label: 'Dropcatch',
        };
      case 'domain.transferred':
        return {
          icon: <RotateCcw className="h-3.5 w-3.5 text-purple-600" />,
          color: 'bg-purple-500',
          label: 'Transferred',
        };
      case 'domain.expired':
        return {
          icon: <AlertTriangle className="h-3.5 w-3.5 text-red-600" />,
          color: 'bg-red-500',
          label: 'Expired',
        };
      case 'domain.marked_for_deletion':
        return {
          icon: <Trash2 className="h-3.5 w-3.5 text-red-600" />,
          color: 'bg-red-500',
          label: 'Pending Delete',
        };
      case 'domain.admin_deleted':
        return {
          icon: <Trash2 className="h-3.5 w-3.5 text-red-600" />,
          color: 'bg-red-500',
          label: 'Deleted',
        };
      case 'domain.purged':
        return {
          icon: <Trash2 className="h-3.5 w-3.5 text-red-600" />,
          color: 'bg-red-500',
          label: 'Purged',
        };
      case 'domain.restored':
        return {
          icon: <RefreshCw className="h-3.5 w-3.5 text-teal-600" />,
          color: 'bg-teal-500',
          label: 'Restored',
        };
      default:
        return {
          icon: <Activity className="h-3.5 w-3.5 text-muted-foreground" />,
          color: 'bg-muted-foreground',
          label: type.replace('domain.', '').replaceAll('_', ' '),
        };
    }
  }

  // Registrar events
  if (type.startsWith('registrar.')) {
    switch (type) {
      case 'registrar.created':
        return {
          icon: <UserPlus className="h-3.5 w-3.5 text-emerald-600" />,
          color: 'bg-emerald-500',
          label: 'Registrar Created',
        };
      case 'registrar.bulk_created':
        return {
          icon: <UserPlus className="h-3.5 w-3.5 text-emerald-600" />,
          color: 'bg-emerald-500',
          label: 'Registrars Imported',
        };
      case 'registrar.updated':
        return {
          icon: <Settings className="h-3.5 w-3.5 text-blue-600" />,
          color: 'bg-blue-500',
          label: 'Registrar Updated',
        };
      case 'registrar.deleted':
        return {
          icon: <UserMinus className="h-3.5 w-3.5 text-red-600" />,
          color: 'bg-red-500',
          label: 'Registrar Deleted',
        };
      case 'registrar.status_updated':
        return {
          icon: <Shield className="h-3.5 w-3.5 text-amber-600" />,
          color: 'bg-amber-500',
          label: 'Registrar Status',
        };
      case 'registrar.iana_status_updated':
        return {
          icon: <Shield className="h-3.5 w-3.5 text-amber-600" />,
          color: 'bg-amber-500',
          label: 'IANA Status',
        };
      default:
        return {
          icon: <Users className="h-3.5 w-3.5 text-blue-600" />,
          color: 'bg-blue-500',
          label: type.replace('registrar.', '').replaceAll('_', ' '),
        };
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
// Component
// ---------------------------------------------------------------------------

export function EventFeed() {
  const { data: events, isLoading, error } = useRecentEvents(25);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  return (
    <Card className="h-full">
      <CardHeader className="pb-4">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-base">
            <Radio className="h-4 w-4 text-primary" />
            Latest Events
          </CardTitle>
          {events && events.length > 0 && (
            <span className="text-xs text-muted-foreground tabular-nums">
              {events.length} events
            </span>
          )}
        </div>
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

        {!isLoading && !error && events && events.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-sm text-muted-foreground">
            <Activity className="h-8 w-8 mb-2 opacity-40" />
            No events recorded yet
          </div>
        )}

        {!isLoading && !error && events && events.length > 0 && (
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
                  <button
                    type="button"
                    className="flex w-full items-center gap-3 rounded-md px-2 py-2 text-left transition-colors hover:bg-accent/50"
                    onClick={() => setExpandedId(isExpanded ? null : event.id)}
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
                          {displayText}
                        </span>
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
                  </button>

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
          </div>
        )}
      </CardContent>
    </Card>
  );
}
