"use client";

import { useState } from "react";
import { useDomainEvents } from "@/lib/hooks/useDomains";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatDistanceToNow, parseISO } from "date-fns";
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
  History,
  FileCode
} from "lucide-react";

interface Props {
  domainName: string;
}

export function DomainEventsWidget({ domainName }: Props) {
  const { data: events, isLoading, error } = useDomainEvents(domainName);
  const [expandedEventId, setExpandedEventId] = useState<string | null>(null);

  const toggleExpand = (id: string) => {
    setExpandedEventId(expandedEventId === id ? null : id);
  };

  const getEventConfig = (type: string) => {
    switch (type) {
      case "domain.registered":
      case "domain.admin_created":
      case "domain.bulk_created":
        return {
          icon: <CheckCircle2 className="h-4 w-4 text-emerald-600" />,
          bgColor: "bg-emerald-50 dark:bg-emerald-950/30 border-emerald-200 dark:border-emerald-800/50",
          title: "Domain Created/Registered",
        };
      case "domain.renewed":
      case "domain.auto_renewed":
        return {
          icon: <RefreshCw className="h-4 w-4 text-blue-600" />,
          bgColor: "bg-blue-50 dark:bg-blue-950/30 border-blue-200 dark:border-blue-800/50",
          title: "Domain Renewed",
        };
      case "domain.status_set":
        return {
          icon: <Lock className="h-4 w-4 text-amber-600" />,
          bgColor: "bg-amber-50 dark:bg-amber-950/30 border-amber-200 dark:border-emerald-800/50",
          title: "Status Set (Lock)",
        };
      case "domain.status_unset":
        return {
          icon: <Unlock className="h-4 w-4 text-orange-600" />,
          bgColor: "bg-orange-50 dark:bg-orange-950/30 border-orange-200 dark:border-orange-800/50",
          title: "Status Unset (Unlock)",
        };
      case "domain.host_added":
        return {
          icon: <Server className="h-4 w-4 text-purple-600" />,
          bgColor: "bg-purple-50 dark:bg-purple-950/30 border-purple-200 dark:border-purple-800/50",
          title: "Nameserver Added",
        };
      case "domain.host_removed":
      case "domain.hosts_cleared":
        return {
          icon: <Server className="h-4 w-4 text-pink-600" />,
          bgColor: "bg-pink-50 dark:bg-pink-950/30 border-pink-200 dark:border-pink-800/50",
          title: "Nameserver Removed",
        };
      case "domain.purged":
      case "domain.marked_for_deletion":
      case "domain.admin_deleted":
      case "domain.expired":
        return {
          icon: <Trash2 className="h-4 w-4 text-destructive" />,
          bgColor: "bg-red-50 dark:bg-red-950/30 border-red-200 dark:border-red-800/50",
          title: "Domain Lifecycle Expiry/Deletion",
        };
      default:
        return {
          icon: <Activity className="h-4 w-4 text-muted-foreground" />,
          bgColor: "bg-muted border-muted-foreground/20",
          title: type.replace("domain.", "").replace("_", " "),
        };
    }
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <History className="h-5 w-5 text-muted-foreground" />
            Activity History
          </CardTitle>
          <CardDescription>Loading lifecycle events...</CardDescription>
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
          <CardTitle className="text-lg text-destructive">Failed to load activity history</CardTitle>
          <CardDescription>
            {(error as any)?.response?.data?.error || (error as any)?.message || "An unexpected error occurred."}
          </CardDescription>
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
          <CardDescription>No events recorded for this domain.</CardDescription>
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
        <CardDescription>
          Chronological audit log of lifecycle events, registrations, and updates
        </CardDescription>
      </CardHeader>
      <CardContent className="relative pl-6 border-l border-muted-foreground/25 ml-4 space-y-6">
        {events.map((event) => {
          const config = getEventConfig(event.type);
          const isExpanded = expandedEventId === event.id;
          const eventTime = parseISO(event.time);
          const relativeTime = formatDistanceToNow(eventTime, { addSuffix: true });
          
          return (
            <div key={event.id} className="relative group">
              {/* Timeline dot */}
              <div className={`absolute -left-[35px] top-1 p-1 rounded-full border ${config.bgColor} flex items-center justify-center shadow-sm`}>
                {config.icon}
              </div>

              {/* Event content */}
              <div className="flex flex-col space-y-2">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h4 className="font-semibold text-sm capitalize">{config.title}</h4>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      Event type: <code className="font-mono text-xs">{event.type}</code>
                    </p>
                  </div>
                  <div className="text-right">
                    <span className="text-xs font-medium text-muted-foreground" title={eventTime.toLocaleString()}>
                      {relativeTime}
                    </span>
                  </div>
                </div>

                {/* Data Fields */}
                <div className="flex flex-wrap gap-2 items-center text-xs">
                  {event.data?.ClientID && (
                    <Badge variant="outline" className="font-mono bg-muted/30">
                      Registrar: {event.data.ClientID}
                    </Badge>
                  )}
                  {event.data?.SKU && (
                    <Badge variant="secondary" className="font-mono">
                      SKU: {event.data.SKU}
                    </Badge>
                  )}
                  {event.data?.DomainYears > 0 && (
                    <Badge variant="outline">
                      Duration: {event.data.DomainYears} {event.data.DomainYears === 1 ? 'Year' : 'Years'}
                    </Badge>
                  )}
                  {event.data?.TransactionType && (
                    <Badge variant="outline" className="uppercase bg-slate-50 dark:bg-slate-900">
                      Action: {event.data.TransactionType}
                    </Badge>
                  )}
                </div>

                {/* Advanced Info & Toggle */}
                <div className="flex items-center justify-between text-xs pt-1">
                  <div className="flex gap-4 text-muted-foreground font-mono text-[10px]">
                    {event.trace_id && (
                      <span className="truncate max-w-[200px]" title={`Trace ID: ${event.trace_id}`}>
                        Trace: {event.trace_id.slice(0, 8)}...
                      </span>
                    )}
                    {event.correlation_id && (
                      <span className="truncate max-w-[200px]" title={`Correlation ID: ${event.correlation_id}`}>
                        Correlation: {event.correlation_id.slice(0, 8)}...
                      </span>
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 text-xs font-medium gap-1 text-muted-foreground hover:text-foreground"
                    onClick={() => toggleExpand(event.id)}
                  >
                    <FileCode className="h-3.5 w-3.5" />
                    {isExpanded ? "Hide Payload" : "Show Payload"}
                    {isExpanded ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
                  </Button>
                </div>

                {/* Raw JSON block */}
                {isExpanded && (
                  <div className="mt-2 text-xs border rounded-md bg-muted/65 dark:bg-zinc-950/50 p-3 max-h-[300px] overflow-auto font-mono scrollbar-thin">
                    <pre><code>{JSON.stringify(event, null, 2)}</code></pre>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
