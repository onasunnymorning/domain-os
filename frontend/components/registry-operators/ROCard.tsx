'use client';

import { useRouter } from 'next/navigation';
import { useTLDsByRyID } from '@/lib/hooks/useTLDs';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { RegistryOperator } from '@/lib/api/types';
import type { TLD } from '@/lib/api/tlds';

// ---------------------------------------------------------------------------
// Color palette for TLD blocks — warm, earthy tones that fit the sunset theme
// ---------------------------------------------------------------------------
const BLOCK_COLORS = [
  'bg-amber-500/15 hover:bg-amber-500/25 border-amber-500/20',
  'bg-orange-500/15 hover:bg-orange-500/25 border-orange-500/20',
  'bg-rose-500/15 hover:bg-rose-500/25 border-rose-500/20',
  'bg-violet-500/15 hover:bg-violet-500/25 border-violet-500/20',
  'bg-blue-500/15 hover:bg-blue-500/25 border-blue-500/20',
  'bg-emerald-500/15 hover:bg-emerald-500/25 border-emerald-500/20',
  'bg-teal-500/15 hover:bg-teal-500/25 border-teal-500/20',
  'bg-cyan-500/15 hover:bg-cyan-500/25 border-cyan-500/20',
];

function getBlockColor(index: number) {
  return BLOCK_COLORS[index % BLOCK_COLORS.length];
}

function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(n >= 10_000 ? 0 : 1)}K`;
  return n.toLocaleString();
}

// ---------------------------------------------------------------------------
// TLD Block Grid — proportionally sized blocks
// ---------------------------------------------------------------------------

function TLDBlockGrid({ tlds }: { tlds: TLD[] }) {
  const router = useRouter();

  if (tlds.length === 0) {
    return (
      <div className="flex items-center justify-center rounded-lg border border-dashed py-6 text-sm text-muted-foreground">
        No TLDs assigned
      </div>
    );
  }

  // Sort by domain count descending
  const sorted = [...tlds].sort((a, b) => (b.DomainCount ?? 0) - (a.DomainCount ?? 0));
  const totalDomains = sorted.reduce((sum, t) => sum + (t.DomainCount ?? 0), 0);
  const hasNonZero = totalDomains > 0;

  return (
    <div className="flex flex-wrap gap-1.5">
      {sorted.map((tld, i) => {
        const count = tld.DomainCount ?? 0;
        // Proportional width: min 20%, max 100%, based on share of total
        const share = hasNonZero && count > 0 ? count / totalDomains : 0;
        // For zero-count TLDs, use a fixed small width
        const widthPercent = count > 0
          ? Math.max(20, Math.round(share * 100))
          : 0;

        // Use flex-grow for proportional sizing within a flex-wrap container
        const flexBasis = count > 0
          ? `calc(${widthPercent}% - 6px)`
          : undefined;

        return (
          <button
            key={tld.Name}
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              router.push(`/tlds/${tld.Name}`);
            }}
            className={cn(
              'rounded-lg border px-3 py-2.5 text-left transition-all duration-200 cursor-pointer',
              'flex flex-col gap-0.5',
              getBlockColor(i),
              count === 0 && 'opacity-60'
            )}
            style={{
              flexGrow: count > 0 ? Math.max(1, Math.round(share * 10)) : 0,
              flexBasis: count > 0 ? flexBasis : 'auto',
              minWidth: count > 0 ? '80px' : undefined,
            }}
          >
            <span className="text-sm font-semibold truncate">.{tld.Name}</span>
            {count > 0 ? (
              <span className="text-xs text-muted-foreground tabular-nums">
                {formatCount(count)} domains
              </span>
            ) : (
              <span className="text-[10px] text-muted-foreground">empty</span>
            )}
          </button>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ROCard
// ---------------------------------------------------------------------------

interface ROCardProps {
  operator: RegistryOperator;
}

export function ROCard({ operator }: ROCardProps) {
  const router = useRouter();
  const { data, isLoading } = useTLDsByRyID(operator.RyID);
  const tlds = data?.Data ?? [];
  const totalDomains = tlds.reduce((sum, t) => sum + (t.DomainCount ?? 0), 0);

  return (
    <div
      onClick={() => router.push(`/registry-operators/${operator.RyID}`)}
      className={cn(
        'group rounded-xl border bg-card p-5 transition-all duration-200 cursor-pointer',
        'hover:shadow-md hover:border-primary/20 hover:-translate-y-0.5'
      )}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-4">
        <div className="min-w-0 flex-1">
          <h3 className="text-lg font-semibold truncate group-hover:text-primary transition-colors">
            {operator.Name}
          </h3>
          <Badge variant="outline" className="text-[10px] px-1.5 py-0 mt-1 font-mono">
            {operator.RyID}
          </Badge>
        </div>
      </div>

      {/* Stats */}
      <div className="flex items-center gap-4 mb-4 text-sm text-muted-foreground">
        {isLoading ? (
          <Skeleton className="h-4 w-32" />
        ) : (
          <>
            {totalDomains > 0 && (
              <span className="tabular-nums">{formatCount(totalDomains)} domains</span>
            )}
          </>
        )}
      </div>

      {/* TLD blocks */}
      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-12 w-full rounded-lg" />
          <Skeleton className="h-12 w-3/4 rounded-lg" />
        </div>
      ) : (
        <TLDBlockGrid tlds={tlds} />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Helper: aggregate total domain count for sorting
// ---------------------------------------------------------------------------

export function useROTotalDomains(ryid: string): number {
  const { data } = useTLDsByRyID(ryid);
  return (data?.Data ?? []).reduce((sum, t) => sum + (t.DomainCount ?? 0), 0);
}
