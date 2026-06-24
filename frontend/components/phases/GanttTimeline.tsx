'use client';

import { Phase } from '@/lib/types/phase';
import { useMemo } from 'react';
import { formatPhaseDate, formatRelativeDate } from '@/lib/utils/dateUtils';
import { cn } from '@/lib/utils';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { Check, Clock, DollarSign, Shield, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';

// ── Types ──────────────────────────────────────────────────────────────────

export interface GanttTimelineProps {
  gaPhases: Phase[];
  launchPhases: Phase[];
  onPhaseClick: (phase: Phase) => void;
}

interface TimeRange {
  start: Date;
  end: Date;
  totalMs: number;
}

type PhaseStatus = 'past' | 'current' | 'future';

// ── Helpers ────────────────────────────────────────────────────────────────

function getPhaseStatus(phase: Phase): PhaseStatus {
  const now = new Date();
  const start = new Date(phase.starts);
  const end = phase.ends ? new Date(phase.ends) : null;
  if (end && end < now) return 'past';
  if (start <= now && (!end || end > now)) return 'current';
  return 'future';
}

/** Compute the visible time window that spans all phases + some padding */
function computeTimeRange(allPhases: Phase[]): TimeRange {
  const now = new Date();

  if (allPhases.length === 0) {
    // Default: 6 months before → 6 months after today
    const start = new Date(now);
    start.setMonth(start.getMonth() - 6);
    const end = new Date(now);
    end.setMonth(end.getMonth() + 6);
    return { start, end, totalMs: end.getTime() - start.getTime() };
  }

  let earliest = Infinity;
  let latest = -Infinity;

  allPhases.forEach((p) => {
    const s = new Date(p.starts).getTime();
    if (s < earliest) earliest = s;
    const e = p.ends
      ? new Date(p.ends).getTime()
      : Math.max(now.getTime() + 90 * 24 * 60 * 60 * 1000, s + 30 * 24 * 60 * 60 * 1000); // ongoing: extend 90d from now or 30d from start
    if (e > latest) latest = e;
  });

  // Add 5% padding on each side
  const span = latest - earliest;
  const padding = Math.max(span * 0.05, 7 * 24 * 60 * 60 * 1000); // min 1 week padding
  const start = new Date(earliest - padding);
  const end = new Date(latest + padding);

  return { start, end, totalMs: end.getTime() - start.getTime() };
}

/** Convert a date to a percentage position within the time range */
function dateToPercent(date: Date, range: TimeRange): number {
  const offset = date.getTime() - range.start.getTime();
  return Math.max(0, Math.min(100, (offset / range.totalMs) * 100));
}

/** Get currency symbol for compact display */
function getCurrencySymbol(currency: string): string {
  const symbols: Record<string, string> = {
    USD: '$', EUR: '€', GBP: '£', JPY: '¥', CHF: 'Fr', CAD: 'C$', AUD: 'A$',
  };
  return symbols[currency?.toUpperCase()] || '$';
}

/** Format a price in compact form (e.g., "$10.00") */
function formatCompactPrice(phase: Phase): string | null {
  if (!phase.prices || phase.prices.length === 0) return null;
  // Use first price (usually base currency)
  const p = phase.prices[0];
  return `${getCurrencySymbol(p.currency)}${(p.registrationAmount / 100).toFixed(2)}`;
}

// ── Time Axis ──────────────────────────────────────────────────────────────

function TimeAxis({ range }: { range: TimeRange }) {
  const ticks = useMemo(() => {
    const result: { date: Date; label: string; percent: number }[] = [];
    const spanDays = range.totalMs / (24 * 60 * 60 * 1000);

    // Determine tick interval based on span
    let intervalMonths: number;
    if (spanDays < 90) intervalMonths = 1;
    else if (spanDays < 365) intervalMonths = 2;
    else if (spanDays < 730) intervalMonths = 3;
    else if (spanDays < 1825) intervalMonths = 6;
    else intervalMonths = 12;

    // Start from first month boundary
    const cursor = new Date(range.start);
    cursor.setDate(1);
    cursor.setHours(0, 0, 0, 0);
    if (cursor < range.start) cursor.setMonth(cursor.getMonth() + 1);

    // Round to interval
    const monthOffset = cursor.getMonth() % intervalMonths;
    if (monthOffset !== 0) cursor.setMonth(cursor.getMonth() + (intervalMonths - monthOffset));

    while (cursor <= range.end) {
      const percent = dateToPercent(cursor, range);
      const label =
        intervalMonths >= 12
          ? cursor.toLocaleDateString('en-US', { year: 'numeric' })
          : cursor.toLocaleDateString('en-US', { month: 'short', year: '2-digit' });
      result.push({ date: new Date(cursor), label, percent });
      cursor.setMonth(cursor.getMonth() + intervalMonths);
    }
    return result;
  }, [range]);

  return (
    <div className="relative h-6 border-b border-border/50">
      {ticks.map((tick, i) => (
        <div
          key={i}
          className="absolute top-0 flex flex-col items-center"
          style={{ left: `${tick.percent}%` }}
        >
          <div className="h-3 w-px bg-border/60" />
          <span className="text-[10px] text-muted-foreground whitespace-nowrap -translate-x-1/2 mt-0.5">
            {tick.label}
          </span>
        </div>
      ))}
    </div>
  );
}

// ── Today Marker ───────────────────────────────────────────────────────────

function TodayMarker({ range }: { range: TimeRange }) {
  const now = new Date();
  const percent = dateToPercent(now, range);
  if (percent <= 0 || percent >= 100) return null;

  return (
    <div
      className="absolute top-0 bottom-0 z-20 pointer-events-none"
      style={{ left: `${percent}%` }}
    >
      <div className="w-px h-full bg-primary/60" />
      <div className="absolute -top-0.5 -translate-x-1/2">
        <div className="px-1.5 py-0.5 rounded text-[9px] font-medium bg-primary text-primary-foreground whitespace-nowrap">
          Today
        </div>
      </div>
    </div>
  );
}

// ── Phase Bar ──────────────────────────────────────────────────────────────

interface PhaseBarProps {
  phase: Phase;
  range: TimeRange;
  status: PhaseStatus;
  onClick: () => void;
}

function PhaseBar({ phase, range, status, onClick }: PhaseBarProps) {
  const now = new Date();
  const start = new Date(phase.starts);
  const end = phase.ends
    ? new Date(phase.ends)
    : new Date(Math.max(now.getTime() + 30 * 24 * 60 * 60 * 1000, start.getTime() + 14 * 24 * 60 * 60 * 1000));

  const leftPercent = dateToPercent(start, range);
  const rightPercent = dateToPercent(end, range);
  const widthPercent = Math.max(rightPercent - leftPercent, 2); // min 2% so it's always visible

  const compactPrice = formatCompactPrice(phase);

  const barClasses = cn(
    'absolute top-1 bottom-1 rounded-md cursor-pointer transition-all duration-200',
    'flex items-center gap-1.5 px-2 overflow-hidden',
    'hover:shadow-lg hover:brightness-110 hover:scale-y-110',
    'group',
    status === 'current' && 'bg-gradient-to-r from-orange-500/90 to-orange-600/90 text-white shadow-md shadow-orange-500/20 ring-1 ring-orange-400/50',
    status === 'past' && 'bg-muted/60 text-muted-foreground/80 ring-1 ring-border/40',
    status === 'future' && 'bg-gradient-to-r from-orange-200/40 to-orange-300/40 text-foreground/70 ring-1 ring-orange-300/40 border border-dashed border-orange-300/50',
  );

  // Policy indicator icons
  const hasAutorenew = phase.policy?.allowAutorenew;
  const hasValidation = phase.policy?.requiresValidation;

  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>
          <div
            className={barClasses}
            style={{
              left: `${leftPercent}%`,
              width: `${widthPercent}%`,
            }}
            onClick={onClick}
          >
            {/* Status dot for current */}
            {status === 'current' && (
              <div className="flex-shrink-0 h-1.5 w-1.5 rounded-full bg-white animate-pulse" />
            )}

            {/* Phase name */}
            <span className={cn(
              'text-xs font-semibold truncate',
              widthPercent < 8 && 'hidden', // hide text on very narrow bars
            )}>
              {phase.name}
            </span>

            {/* Compact price badge */}
            {compactPrice && widthPercent > 12 && (
              <span className={cn(
                'text-[10px] font-mono flex-shrink-0 px-1 py-0.5 rounded',
                status === 'current' ? 'bg-white/20' : 'bg-muted/60',
              )}>
                {compactPrice}
              </span>
            )}

            {/* Policy icons - only on wider bars */}
            {widthPercent > 18 && (
              <div className="flex items-center gap-0.5 flex-shrink-0 ml-auto">
                {hasAutorenew && (
                  <RefreshCw className={cn('h-3 w-3', status === 'current' ? 'text-white/70' : 'text-muted-foreground/50')} />
                )}
                {hasValidation && (
                  <Shield className={cn('h-3 w-3', status === 'current' ? 'text-white/70' : 'text-muted-foreground/50')} />
                )}
              </div>
            )}

            {/* Ongoing indicator (wavy right edge) */}
            {!phase.ends && (
              <div className="absolute right-0 top-0 bottom-0 w-4 bg-gradient-to-r from-transparent to-background/50" />
            )}
          </div>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs">
          <PhaseTooltip phase={phase} status={status} />
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

// ── Phase Tooltip ──────────────────────────────────────────────────────────

function PhaseTooltip({ phase, status }: { phase: Phase; status: PhaseStatus }) {
  return (
    <div className="space-y-2 py-1">
      <div className="flex items-center gap-2">
        <span className="font-semibold text-sm">{phase.name}</span>
        <Badge variant={phase.type === 'GA' ? 'default' : 'secondary'} className="text-[10px] h-4">
          {phase.type}
        </Badge>
        <Badge
          variant="outline"
          className={cn(
            'text-[10px] h-4',
            status === 'current' && 'border-green-500 text-green-600',
            status === 'past' && 'border-muted-foreground text-muted-foreground',
            status === 'future' && 'border-blue-500 text-blue-600',
          )}
        >
          {status === 'current' ? '● Active' : status === 'past' ? 'Ended' : 'Scheduled'}
        </Badge>
      </div>

      {/* Dates */}
      <div className="text-xs space-y-0.5">
        <div className="flex items-center gap-1.5">
          <Clock className="h-3 w-3 text-muted-foreground" />
          <span>{status === 'future' ? 'Starts' : 'Started'}: {formatPhaseDate(phase.starts)}</span>
          <span className="text-muted-foreground">({formatRelativeDate(phase.starts)})</span>
        </div>
        {phase.ends ? (
          <div className="flex items-center gap-1.5">
            <Clock className="h-3 w-3 text-muted-foreground" />
            <span>Ends: {formatPhaseDate(phase.ends)}</span>
            <span className="text-muted-foreground">({formatRelativeDate(phase.ends)})</span>
          </div>
        ) : (
          <div className="text-muted-foreground italic">Ongoing — no end date</div>
        )}
      </div>

      {/* Pricing summary */}
      {phase.prices && phase.prices.length > 0 && (
        <div className="border-t pt-1.5 mt-1.5">
          <div className="flex items-center gap-1 text-[10px] text-muted-foreground mb-1">
            <DollarSign className="h-3 w-3" />
            Pricing ({phase.prices.length} {phase.prices.length === 1 ? 'currency' : 'currencies'})
          </div>
          <div className="grid grid-cols-4 gap-x-3 gap-y-0.5 text-[10px]">
            <span className="text-muted-foreground">Reg</span>
            <span className="text-muted-foreground">Renew</span>
            <span className="text-muted-foreground">Transfer</span>
            <span className="text-muted-foreground">Restore</span>
            {phase.prices.slice(0, 2).map((p) => (
              <>
                <span key={`${p.currency}-reg`} className="font-mono">{getCurrencySymbol(p.currency)}{(p.registrationAmount / 100).toFixed(0)}</span>
                <span key={`${p.currency}-ren`} className="font-mono">{getCurrencySymbol(p.currency)}{(p.renewalAmount / 100).toFixed(0)}</span>
                <span key={`${p.currency}-tr`} className="font-mono">{getCurrencySymbol(p.currency)}{(p.transferAmount / 100).toFixed(0)}</span>
                <span key={`${p.currency}-res`} className="font-mono">{getCurrencySymbol(p.currency)}{(p.restoreAmount / 100).toFixed(0)}</span>
              </>
            ))}
          </div>
          {phase.prices.length > 2 && (
            <div className="text-[10px] text-muted-foreground mt-0.5">+{phase.prices.length - 2} more</div>
          )}
        </div>
      )}

      {/* Policy summary */}
      <div className="flex gap-2 text-[10px] text-muted-foreground">
        {phase.policy?.allowAutorenew && (
          <span className="flex items-center gap-0.5"><RefreshCw className="h-2.5 w-2.5" /> Autorenew</span>
        )}
        {phase.policy?.requiresValidation && (
          <span className="flex items-center gap-0.5"><Shield className="h-2.5 w-2.5" /> Validation</span>
        )}
        {phase.policy?.baseCurrency && (
          <span className="font-mono">{phase.policy.baseCurrency}</span>
        )}
      </div>

      <div className="text-[10px] text-muted-foreground/60 pt-0.5">Click to view details</div>
    </div>
  );
}

// ── Swim Lane ──────────────────────────────────────────────────────────────

interface SwimLaneProps {
  label: string;
  phases: Phase[];
  range: TimeRange;
  onPhaseClick: (phase: Phase) => void;
  allowOverlap?: boolean;
}

function SwimLane({ label, phases, range, onPhaseClick, allowOverlap = false }: SwimLaneProps) {
  // Sort phases by start date
  const sortedPhases = useMemo(
    () => [...phases].sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime()),
    [phases]
  );

  // For overlapping phases (Launch), compute row assignments
  const phaseRows = useMemo(() => {
    if (!allowOverlap || sortedPhases.length === 0) {
      return sortedPhases.map((p) => ({ phase: p, row: 0 }));
    }

    const now = new Date();
    const assignments: { phase: Phase; row: number }[] = [];
    const rowEnds: number[] = []; // track the end time of the last phase in each row

    sortedPhases.forEach((phase) => {
      const start = new Date(phase.starts).getTime();
      const end = phase.ends
        ? new Date(phase.ends).getTime()
        : Math.max(now.getTime() + 30 * 24 * 60 * 60 * 1000, start + 14 * 24 * 60 * 60 * 1000);

      // Find the first row where this phase doesn't overlap
      let assignedRow = -1;
      for (let r = 0; r < rowEnds.length; r++) {
        if (start >= rowEnds[r]) {
          assignedRow = r;
          rowEnds[r] = end;
          break;
        }
      }
      if (assignedRow === -1) {
        assignedRow = rowEnds.length;
        rowEnds.push(end);
      }
      assignments.push({ phase, row: assignedRow });
    });

    return assignments;
  }, [sortedPhases, allowOverlap]);

  const maxRow = Math.max(0, ...phaseRows.map((p) => p.row));
  const laneHeight = (maxRow + 1) * 36; // 36px per row

  const hasPhases = phases.length > 0;
  const activeCount = phases.filter((p) => getPhaseStatus(p) === 'current').length;

  return (
    <div className="flex">
      {/* Lane label */}
      <div className="w-24 flex-shrink-0 flex items-center pr-3">
        <div className="space-y-0.5">
          <div className="text-xs font-semibold text-foreground">{label}</div>
          {activeCount > 0 && (
            <div className="text-[10px] text-orange-600 font-medium">
              {activeCount} active
            </div>
          )}
        </div>
      </div>

      {/* Lane content */}
      <div className="flex-1 relative border-l border-border/30" style={{ minHeight: `${Math.max(laneHeight, 36)}px` }}>
        {hasPhases ? (
          phaseRows.map(({ phase, row }) => (
            <div
              key={phase.id}
              className="absolute left-0 right-0"
              style={{
                top: `${row * 36}px`,
                height: '32px',
              }}
            >
              <PhaseBar
                phase={phase}
                range={range}
                status={getPhaseStatus(phase)}
                onClick={() => onPhaseClick(phase)}
              />
            </div>
          ))
        ) : (
          <div className="flex items-center h-full px-4">
            <span className="text-xs text-muted-foreground italic">
              No {label.toLowerCase()} configured
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Gap Warning Indicators ─────────────────────────────────────────────────

function GapIndicators({ phases, range }: { phases: Phase[]; range: TimeRange }) {
  const gaps = useMemo(() => {
    const sorted = [...phases]
      .sort((a, b) => new Date(a.starts).getTime() - new Date(b.starts).getTime());

    const result: { start: Date; end: Date }[] = [];
    for (let i = 0; i < sorted.length - 1; i++) {
      const currentEnd = sorted[i].ends ? new Date(sorted[i].ends!) : null;
      const nextStart = new Date(sorted[i + 1].starts);
      if (currentEnd && currentEnd < nextStart) {
        result.push({ start: currentEnd, end: nextStart });
      }
    }
    return result;
  }, [phases]);

  return (
    <>
      {gaps.map((gap, i) => {
        const leftPercent = dateToPercent(gap.start, range);
        const rightPercent = dateToPercent(gap.end, range);
        const widthPercent = rightPercent - leftPercent;
        if (widthPercent < 0.5) return null;

        return (
          <TooltipProvider key={i} delayDuration={200}>
            <Tooltip>
              <TooltipTrigger asChild>
                <div
                  className="absolute top-1 bottom-1 rounded border-2 border-dashed border-warning/50 bg-warning/5 z-10"
                  style={{
                    left: `${leftPercent}%`,
                    width: `${widthPercent}%`,
                  }}
                />
              </TooltipTrigger>
              <TooltipContent side="top">
                <div className="text-xs">
                  <div className="font-semibold text-warning">⚠ Coverage Gap</div>
                  <div>{formatPhaseDate(gap.start.toISOString())} → {formatPhaseDate(gap.end.toISOString())}</div>
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        );
      })}
    </>
  );
}

// ── Main Component ─────────────────────────────────────────────────────────

export function GanttTimeline({ gaPhases, launchPhases, onPhaseClick }: GanttTimelineProps) {
  const allPhases = [...gaPhases, ...launchPhases];
  const range = useMemo(() => computeTimeRange(allPhases), [allPhases]);
  const nowPercent = dateToPercent(new Date(), range);

  return (
    <div className="space-y-0">
      {/* Time Axis */}
      <div className="flex">
        <div className="w-24 flex-shrink-0" />
        <div className="flex-1 relative">
          <TimeAxis range={range} />
        </div>
      </div>

      {/* Gantt Body */}
      <div className="flex">
        <div className="w-24 flex-shrink-0" />
        <div className="flex-1 relative">
          <TodayMarker range={range} />
        </div>
      </div>

      {/* GA Swim Lane */}
      <div className="relative">
        <SwimLane
          label="GA Phases"
          phases={gaPhases}
          range={range}
          onPhaseClick={onPhaseClick}
          allowOverlap={false}
        />
        {/* Gap warnings for GA phases (they should be continuous) */}
        <div className="absolute left-24 right-0 top-0 bottom-0 pointer-events-none">
          <div className="relative h-full">
            <GapIndicators phases={gaPhases} range={range} />
          </div>
        </div>
      </div>

      {/* Separator */}
      <div className="flex">
        <div className="w-24 flex-shrink-0" />
        <div className="flex-1 border-t border-border/30" />
      </div>

      {/* Launch Swim Lane */}
      <SwimLane
        label="Launch Phases"
        phases={launchPhases}
        range={range}
        onPhaseClick={onPhaseClick}
        allowOverlap={true}
      />
    </div>
  );
}
