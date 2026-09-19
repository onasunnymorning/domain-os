'use client';

import { Building2, Globe2, Info } from 'lucide-react';
import { OperatorScopeSelect } from '@/components/workflows/OperatorScopeSelect';
import { cn } from '@/lib/utils';

interface KeyScopePickerProps {
  /** '' with platform=false means nothing chosen yet. */
  tenantId: string;
  platform: boolean;
  onChange: (next: { tenantId: string; platform: boolean }) => void;
}

/**
 * Chooses whose key registry to look at: one operator's (which includes the
 * platform's parties, read-only) or the platform's own, which only staff with
 * the platform permission can open.
 *
 * It is drawn as two facing choices rather than a control because the page
 * beneath it changes meaning entirely with the answer — editing the platform
 * default reaches every operator that has none of its own, and there is no way
 * to tell that from a dropdown that has quietly kept its last value.
 */
export function KeyScopePicker({ tenantId, platform, onChange }: KeyScopePickerProps) {
  return (
    <section className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <h2 className="text-[11px] font-bold uppercase tracking-[0.1em]">Setup scope</h2>
        <span className="text-xs text-muted-foreground">whose escrow setup you are looking at</span>
      </div>

      <div className="flex flex-col items-stretch gap-2 sm:flex-row sm:items-stretch">
        <button
          type="button"
          onClick={() => onChange({ tenantId: '', platform: true })}
          aria-pressed={platform}
          className={cn(
            'flex flex-1 items-center gap-3 rounded-xl border p-3 text-left transition-colors',
            platform ? 'border-primary bg-primary/5' : 'border-border bg-background hover:bg-muted/50'
          )}
        >
          <span
            className={cn(
              'flex h-10 w-10 shrink-0 items-center justify-center rounded-lg',
              platform ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'
            )}
          >
            <Globe2 className="h-5 w-5" aria-hidden />
          </span>
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="flex flex-wrap items-center gap-2">
              <span className="text-[15px] font-semibold">Registry platform</span>
              {platform && (
                <span className="rounded-full bg-primary px-2 py-0.5 text-[11px] font-medium text-primary-foreground">
                  Editing now
                </span>
              )}
            </span>
            <span className="text-xs leading-snug text-muted-foreground">
              The default setup every registry operator starts from.
            </span>
          </span>
        </button>

        <span className="self-center text-xs text-muted-foreground">or</span>

        <div
          className={cn(
            'flex flex-1 items-center gap-3 rounded-xl border p-3',
            !platform && tenantId !== '' ? 'border-primary bg-primary/5' : 'border-border bg-background'
          )}
        >
          <span
            className={cn(
              'flex h-10 w-10 shrink-0 items-center justify-center rounded-lg',
              !platform && tenantId !== '' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'
            )}
          >
            <Building2 className="h-5 w-5" aria-hidden />
          </span>
          <div className="flex min-w-0 flex-1 flex-col gap-1.5">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-[15px] font-semibold">One registry operator</span>
              {!platform && tenantId !== '' && (
                <span className="rounded-full bg-primary px-2 py-0.5 text-[11px] font-medium text-primary-foreground">
                  Editing now
                </span>
              )}
            </div>
            <OperatorScopeSelect
              value={platform ? '' : tenantId}
              onChange={(ryid) => onChange({ tenantId: ryid, platform: false })}
            />
          </div>
        </div>
      </div>

      <p className="flex items-center gap-3 rounded-lg border border-border bg-muted/40 px-3 py-2 text-[13px] leading-relaxed">
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-background text-muted-foreground">
          <Info className="h-3.5 w-3.5" aria-hidden />
        </span>
        <ScopeNote platform={platform} tenantId={tenantId} />
      </p>
    </section>
  );
}

/** What the choice above means for everything on the page below it. */
function ScopeNote({ platform, tenantId }: { platform: boolean; tenantId: string }) {
  if (platform) {
    return (
      <span>
        <strong className="font-semibold">Everything below is the platform default.</strong> Every registry operator
        uses it as-is, unless you open that operator and give it a setup of its own.
      </span>
    );
  }
  if (tenantId === '') {
    return <span>Choose a registry operator, or open the platform registry.</span>;
  }
  return (
    <span>
      <strong className="font-semibold">You are looking at {tenantId}&apos;s setup.</strong> The platform&apos;s own
      sources and identities appear here too, read-only. Anything you add belongs to {tenantId} alone.
    </span>
  );
}
