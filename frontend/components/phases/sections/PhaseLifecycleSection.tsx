'use client';

import Link from 'next/link';
import { useState } from 'react';
import { toast } from 'sonner';
import { Clock, ExternalLink } from 'lucide-react';
import { Phase } from '@/lib/types/phase';
import type { ResolvedLifecycle } from '@/lib/types/lifecycle';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useLifecyclePolicies, useSetPhaseLifecyclePolicy } from '@/lib/hooks/useLifecyclePolicies';
import { LIFECYCLE_DURATIONS, STANDARD_LIFECYCLE, isStandardLifecyclePolicy } from '@/lib/constants/lifecycleDefaults';

interface PhaseLifecycleSectionProps {
  phase: Phase;
  tldName: string;
  onRefetch: () => void;
}

/**
 * The phase's lifecycle policy (ADR 0023): a selector for which named policy
 * the phase follows, and a read-only view of that policy resolved. The
 * values themselves are edited under Lifecycle Policies, since a policy
 * reaches every phase that names it.
 */
export function PhaseLifecycleSection({ phase, tldName, onRefetch }: PhaseLifecycleSectionProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [picked, setPicked] = useState(phase.lifecyclePolicyName);
  const policies = useLifecyclePolicies(isEditing);
  const { mutate: setPolicy, isPending: isSaving } = useSetPhaseLifecyclePolicy(tldName, phase.name);

  const resolved: ResolvedLifecycle = phase.lifecyclePolicy?.resolved ?? STANDARD_LIFECYCLE;
  const canEdit = !phase.ends || new Date(phase.ends) > new Date();

  const handleSave = () => {
    setPolicy(picked, {
      onSuccess: () => {
        setIsEditing(false);
        toast.success(`${phase.name} now follows ${picked}.`);
        onRefetch();
      },
      onError: (err: unknown) => {
        const e = err as { response?: { data?: { error?: string } } };
        toast.error(e.response?.data?.error ?? 'Could not change the lifecycle policy');
      },
    });
  };

  const maxDays = Math.max(...LIFECYCLE_DURATIONS.map((d) => resolved[d.key]), 1);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-orange-600" />
          <span className="text-sm font-semibold">Lifecycle policy</span>
        </div>
        {!isEditing ? (
          canEdit && (
            <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={() => { setPicked(phase.lifecyclePolicyName); setIsEditing(true); }}>
              Change
            </Button>
          )
        ) : (
          <div className="flex gap-1.5">
            <Button size="sm" className="h-7 text-xs" onClick={handleSave} disabled={isSaving || picked === phase.lifecyclePolicyName}>
              {isSaving ? 'Saving...' : 'Save'}
            </Button>
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => setIsEditing(false)} disabled={isSaving}>
              Cancel
            </Button>
          </div>
        )}
      </div>

      <div className="rounded-lg bg-muted/30 border border-border/40 p-3 space-y-2">
        <div className="text-[10px] text-muted-foreground uppercase tracking-wide">Policy</div>
        {!isEditing ? (
          <div className="flex items-center gap-2 flex-wrap">
            <Link href={`/lifecycle-policies/${encodeURIComponent(phase.lifecyclePolicyName)}`} className="font-mono font-semibold text-primary hover:underline inline-flex items-center gap-1">
              {phase.lifecyclePolicyName} <ExternalLink className="h-3 w-3" />
            </Link>
            {isStandardLifecyclePolicy(phase.lifecyclePolicyName) && <Badge variant="secondary" className="text-[10px]">Built-in</Badge>}
            {phase.lifecyclePolicy?.description && (
              <span className="text-xs text-muted-foreground">{phase.lifecyclePolicy.description}</span>
            )}
          </div>
        ) : (
          <div className="space-y-1.5">
            <Select value={picked} onValueChange={setPicked}>
              <SelectTrigger className="h-8 text-sm" aria-label="Lifecycle policy"><SelectValue /></SelectTrigger>
              <SelectContent>
                {(policies.data ?? [{ name: phase.lifecyclePolicyName, description: '' }]).map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    {p.name}{p.description ? ` — ${p.description}` : ''}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-[11px] text-muted-foreground">
              Applies to lifecycle decisions made from now on; dates already written to domains stay. The values of a policy are edited under Lifecycle Policies.
            </p>
          </div>
        )}
      </div>

      {/* Resolved durations, read-only */}
      <div className="space-y-1.5">
        {LIFECYCLE_DURATIONS.map((d) => {
          const val = resolved[d.key];
          return (
            <div key={d.key} className="flex items-center gap-2" title={d.help}>
              <div className="w-32 text-xs text-muted-foreground truncate">{d.label}</div>
              <div className="flex-1 h-4 bg-muted/40 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-orange-400/70 to-orange-500/70 rounded-full transition-all duration-300"
                  style={{ width: `${(val / maxDays) * 100}%` }}
                />
              </div>
              <div className="w-12 text-xs font-mono text-right tabular-nums">{val}d</div>
            </div>
          );
        })}
      </div>

      <div className="grid grid-cols-3 gap-3">
        <ReadOnlyField label="Max horizon" value={`${resolved.maxHorizon} years`} />
        <ReadOnlyField label="Auto-renew" value={resolved.allowAutorenew ? 'Allowed' : 'Not allowed'} />
        <ReadOnlyField label="Restore report" value={resolved.restoreReportRequired ? 'Required' : 'Not required'} />
      </div>
    </div>
  );
}

function ReadOnlyField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg bg-muted/30 border border-border/40 p-3 space-y-1">
      <div className="text-[10px] text-muted-foreground uppercase tracking-wide">{label}</div>
      <div className="font-semibold text-sm">{value}</div>
    </div>
  );
}
