'use client';

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { Globe, Building2, Wifi, WifiOff, ChevronDown, ChevronUp } from 'lucide-react';

// ---------------------------------------------------------------------------
// Types matching GetTLDOutput from the Go MCP package (camelCase JSON tags)
// ---------------------------------------------------------------------------

interface PhasePrice {
  currency: string;
  registrationAmount: number;
  renewalAmount: number;
  transferAmount: number;
  restoreAmount: number;
}

interface PhasePolicy {
  minLabelLength?: number;
  maxLabelLength?: number;
  registrationGP?: number;
  renewalGP?: number;
  autoRenewalGP?: number;
  transferGP?: number;
  redemptionGP?: number;
  pendingDeleteGP?: number;
  transferLockPeriod?: number;
  maxHorizon?: number;
  allowAutoRenew?: boolean;
  baseCurrency?: string;
}

interface TLDPhase {
  name: string;
  type: string;
  starts: string;
  ends?: string;
  prices?: PhasePrice[];
  policy?: PhasePolicy;
}

export interface TLDCardData {
  name: string;
  unicodeName?: string;
  type: string;
  registryOperatorId?: string;
  dnsEnabled?: boolean;
  createdDate?: string;
  updatedDate?: string;
  currentPhases?: TLDPhase[];
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function typeBadgeColor(type: string): string {
  switch (type.toLowerCase()) {
    case 'gtld':
    case 'generic':
      return 'bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/20';
    case 'cctld':
    case 'country-code':
      return 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/20';
    case 'newtld':
    case 'new-gtld':
      return 'bg-violet-500/15 text-violet-600 dark:text-violet-400 border-violet-500/20';
    default:
      return 'bg-muted text-muted-foreground border-border';
  }
}

function formatAmount(amount: number, currency: string): string {
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency,
      minimumFractionDigits: 2,
    }).format(amount);
  } catch {
    return `${currency} ${amount.toFixed(2)}`;
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function TLDPricingCard({ data }: { data: TLDCardData }) {
  const phases = data.currentPhases ?? [];
  const [expandedPhase, setExpandedPhase] = useState<string | null>(
    phases.length === 1 ? phases[0].name : null,
  );

  return (
    <Card className="overflow-hidden border-border/60 bg-gradient-to-br from-card to-card/80 shadow-md transition-shadow hover:shadow-lg">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Globe className="h-4 w-4" />
          </div>
          <div className="flex items-center gap-2">
            <CardTitle className="text-base font-semibold tracking-tight">
              .{data.name}
              {data.unicodeName && data.unicodeName !== data.name && (
                <span className="ml-1 text-muted-foreground">({data.unicodeName})</span>
              )}
            </CardTitle>
            <Badge variant="outline" className={cn('text-[11px]', typeBadgeColor(data.type))}>
              {data.type}
            </Badge>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-4">
        {/* Meta row */}
        <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
          {data.registryOperatorId && (
            <span className="inline-flex items-center gap-1">
              <Building2 className="h-3 w-3" />
              {data.registryOperatorId}
            </span>
          )}
          <span className={cn(
            'inline-flex items-center gap-1',
            data.dnsEnabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-500 dark:text-red-400',
          )}>
            {data.dnsEnabled ? <Wifi className="h-3 w-3" /> : <WifiOff className="h-3 w-3" />}
            DNS {data.dnsEnabled ? 'Enabled' : 'Disabled'}
          </span>
        </div>

        {/* Phases */}
        {phases.length > 0 && (
          <div className="space-y-2">
            {phases.map((phase) => {
              const isExpanded = expandedPhase === phase.name;
              const prices = phase.prices ?? [];
              return (
                <div
                  key={phase.name}
                  className="rounded-lg border border-border/50 bg-muted/20 transition-colors"
                >
                  {/* Phase header */}
                  <button
                    type="button"
                    onClick={() => setExpandedPhase(isExpanded ? null : phase.name)}
                    className="flex w-full items-center justify-between px-3 py-2 text-left text-xs font-medium text-foreground/90 hover:bg-muted/40 transition-colors rounded-lg"
                  >
                    <div className="flex items-center gap-2">
                      <span>{phase.name}</span>
                      <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                        {phase.type}
                      </Badge>
                    </div>
                    {isExpanded
                      ? <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
                      : <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
                    }
                  </button>

                  {/* Phase pricing table */}
                  {isExpanded && prices.length > 0 && (
                    <div className="border-t border-border/30 px-3 py-2 animate-in fade-in slide-in-from-top-1 duration-200">
                      <div className="overflow-x-auto">
                        <table className="w-full text-[11px]">
                          <thead>
                            <tr className="text-muted-foreground">
                              <th className="py-1 text-left font-medium">Currency</th>
                              <th className="py-1 text-right font-medium">Registration</th>
                              <th className="py-1 text-right font-medium">Renewal</th>
                              <th className="py-1 text-right font-medium">Transfer</th>
                              <th className="py-1 text-right font-medium">Restore</th>
                            </tr>
                          </thead>
                          <tbody>
                            {prices.map((price) => (
                              <tr key={price.currency} className="text-foreground/80 tabular-nums">
                                <td className="py-1 font-medium">{price.currency}</td>
                                <td className="py-1 text-right">{formatAmount(price.registrationAmount, price.currency)}</td>
                                <td className="py-1 text-right">{formatAmount(price.renewalAmount, price.currency)}</td>
                                <td className="py-1 text-right">{formatAmount(price.transferAmount, price.currency)}</td>
                                <td className="py-1 text-right">{formatAmount(price.restoreAmount, price.currency)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
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
