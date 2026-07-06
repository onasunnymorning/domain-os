'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { Globe, Calendar, Server, Building2, AlertTriangle } from 'lucide-react';

// ---------------------------------------------------------------------------
// Types matching GetDomainOutput from the Go MCP package
// ---------------------------------------------------------------------------

export interface DomainCardData {
  name: string;
  statuses?: string[];
  createdDate?: string;
  expiryDate?: string;
  rgpPhase?: string;
  rgpPhaseEndDate?: string;
  nameservers?: string[];
  sponsoringRegistrar?: string;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function statusColor(status: string): string {
  const s = status.toLowerCase();
  if (s.includes('serverhold') || s.includes('clienthold')) {
    return 'bg-red-500/15 text-red-600 dark:text-red-400 border-red-500/20';
  }
  if (s.includes('pendingdelete') || s.includes('pendingtransfer')) {
    return 'bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/20';
  }
  if (s === 'ok' || s === 'active') {
    return 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/20';
  }
  if (s.includes('prohibited')) {
    return 'bg-orange-500/15 text-orange-600 dark:text-orange-400 border-orange-500/20';
  }
  return 'bg-muted text-muted-foreground border-border';
}

function formatDate(iso: string): string {
  if (!iso) return '—';
  try {
    return new Date(iso).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return iso;
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function DomainCard({ data }: { data: DomainCardData }) {
  const hasRGP = !!data.rgpPhase && data.rgpPhase !== '';
  const statuses = data.statuses ?? [];
  const nameservers = data.nameservers ?? [];

  return (
    <Card className="overflow-hidden border-border/60 bg-gradient-to-br from-card to-card/80 shadow-md transition-shadow hover:shadow-lg">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Globe className="h-4 w-4" />
          </div>
          <CardTitle className="text-base font-semibold tracking-tight">{data.name}</CardTitle>
        </div>
      </CardHeader>

      <CardContent className="space-y-4">
        {/* Status badges */}
        {statuses.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {statuses.map((status) => (
              <Badge
                key={status}
                variant="outline"
                className={cn('text-[11px] font-medium', statusColor(status))}
              >
                {status}
              </Badge>
            ))}
          </div>
        )}

        {/* RGP phase warning */}
        {hasRGP && (
          <div className="flex items-center gap-2 rounded-lg border border-amber-500/20 bg-amber-500/10 px-3 py-2 text-xs text-amber-600 dark:text-amber-400">
            <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
            <span>
              <span className="font-medium">{data.rgpPhase}</span>
              {data.rgpPhaseEndDate && (
                <span className="text-amber-600/70 dark:text-amber-400/70"> — ends {formatDate(data.rgpPhaseEndDate)}</span>
              )}
            </span>
          </div>
        )}

        {/* Dates */}
        <div className="grid grid-cols-2 gap-3 text-xs">
          <div className="flex items-center gap-1.5 text-muted-foreground">
            <Calendar className="h-3 w-3" />
            <span>Created</span>
          </div>
          <div className="text-right text-foreground/80 tabular-nums">{formatDate(data.createdDate ?? '')}</div>

          <div className="flex items-center gap-1.5 text-muted-foreground">
            <Calendar className="h-3 w-3" />
            <span>Expires</span>
          </div>
          <div className="text-right text-foreground/80 tabular-nums">{formatDate(data.expiryDate ?? '')}</div>
        </div>

        {/* Registrar */}
        {data.sponsoringRegistrar && (
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Building2 className="h-3 w-3 shrink-0" />
            <span className="truncate">{data.sponsoringRegistrar}</span>
          </div>
        )}

        {/* Nameservers */}
        {nameservers.length > 0 && (
          <div className="space-y-1">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Server className="h-3 w-3" />
              <span>Nameservers</span>
            </div>
            <div className="flex flex-col gap-0.5 pl-4.5">
              {nameservers.map((ns) => (
                <span key={ns} className="font-mono text-[11px] text-foreground/70">{ns}</span>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
