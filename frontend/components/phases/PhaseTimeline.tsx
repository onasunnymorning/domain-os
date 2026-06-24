'use client';

import { usePhases } from '@/lib/hooks/usePhases';
import { Phase } from '@/lib/types/phase';
import { useState, useEffect } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { PhaseDetailDrawer } from './PhaseDetailDrawer';
import { PhaseCreateWizard } from './PhaseCreateWizard';
import { GanttTimeline } from './GanttTimeline';
import { Button } from '@/components/ui/button';
import { Plus, Calendar, Layers, Info } from 'lucide-react';
import { Card } from '@/components/ui/card';

interface PhaseTimelineProps {
  tldName: string;
  initialPhaseName?: string;
}

export function PhaseTimeline({ tldName, initialPhaseName }: PhaseTimelineProps) {
  const router = useRouter();
  const pathname = usePathname();
  const { data: phases, isLoading, error } = usePhases(tldName);
  const [selectedPhase, setSelectedPhase] = useState<Phase | null>(null);
  const [showCreateWizard, setShowCreateWizard] = useState(false);
  const [hasOpenedInitialPhase, setHasOpenedInitialPhase] = useState(false);

  // Open the phase drawer if initialPhaseName is provided (only once)
  useEffect(() => {
    if (initialPhaseName && phases && phases.length > 0 && !hasOpenedInitialPhase) {
      const phase = phases.find(p => p.name === initialPhaseName);
      if (phase) {
        setSelectedPhase(phase);
        setHasOpenedInitialPhase(true);
      }
    }
  }, [initialPhaseName, phases, hasOpenedInitialPhase]);

  // Handle closing the phase drawer and removing query parameter
  const handleCloseDrawer = () => {
    setSelectedPhase(null);
    // Remove the phase query parameter from the URL
    if (initialPhaseName) {
      router.replace(pathname);
    }
  };

  if (isLoading) {
    return (
      <Card className="p-6">
        <div className="animate-pulse space-y-4">
          <div className="flex items-center justify-between">
            <div className="h-5 bg-muted rounded w-32" />
            <div className="h-8 bg-muted rounded w-24" />
          </div>
          <div className="h-6 bg-muted/50 rounded w-full" />
          <div className="space-y-2">
            <div className="flex">
              <div className="w-24 h-8" />
              <div className="flex-1 h-8 bg-muted/40 rounded" />
            </div>
            <div className="flex">
              <div className="w-24 h-8" />
              <div className="flex-1 h-8 bg-muted/40 rounded" />
            </div>
          </div>
        </div>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className="p-6">
        <div className="text-sm text-destructive">
          Failed to load phases: {error.message}
        </div>
      </Card>
    );
  }

  const gaPhases = (phases || []).filter(p => p.type === 'GA');
  const launchPhases = (phases || []).filter(p => p.type === 'Launch');
  const hasAnyPhases = (phases || []).length > 0;

  return (
    <>
      <Card className="p-6">
        <div className="space-y-4">
          {/* Header */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <h2 className="text-lg font-semibold">Phase Timeline</h2>
              {hasAnyPhases && (
                <span className="text-xs text-muted-foreground tabular-nums">
                  {phases!.length} phase{phases!.length !== 1 ? 's' : ''}
                </span>
              )}
            </div>
            <Button size="sm" variant="outline" onClick={() => setShowCreateWizard(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Add Phase
            </Button>
          </div>

          {/* Content */}
          {hasAnyPhases ? (
            <GanttTimeline
              gaPhases={gaPhases}
              launchPhases={launchPhases}
              onPhaseClick={setSelectedPhase}
            />
          ) : (
            <EmptyPhaseState onAddPhase={() => setShowCreateWizard(true)} />
          )}
        </div>
      </Card>

      {/* Phase Detail Drawer */}
      <PhaseDetailDrawer
        phase={selectedPhase}
        open={!!selectedPhase}
        onClose={handleCloseDrawer}
        tldName={tldName}
        allPhases={phases || []}
      />

      {/* Create Phase Wizard */}
      <PhaseCreateWizard
        tldName={tldName}
        open={showCreateWizard}
        onClose={() => setShowCreateWizard(false)}
        existingPhases={phases || []}
      />
    </>
  );
}

// ── Self-Documenting Empty State ───────────────────────────────────────────

function EmptyPhaseState({ onAddPhase }: { onAddPhase: () => void }) {
  return (
    <div className="rounded-lg border-2 border-dashed border-border/60 bg-muted/20 p-8">
      <div className="max-w-lg mx-auto text-center space-y-4">
        {/* Icon cluster */}
        <div className="flex items-center justify-center gap-3">
          <div className="rounded-full bg-orange-100 dark:bg-orange-900/30 p-3">
            <Calendar className="h-6 w-6 text-orange-600" />
          </div>
          <div className="rounded-full bg-muted p-3">
            <Layers className="h-6 w-6 text-muted-foreground" />
          </div>
        </div>

        {/* Explanation */}
        <div className="space-y-2">
          <h3 className="text-base font-semibold">No phases configured</h3>
          <p className="text-sm text-muted-foreground leading-relaxed">
            Phases define the <strong>pricing, policies, and registration rules</strong> for
            this TLD at different points in time. Each phase specifies grace periods, label
            constraints, contact data requirements, and per-currency pricing.
          </p>
        </div>

        {/* Phase type explanation */}
        <div className="grid grid-cols-2 gap-3 text-left">
          <div className="rounded-lg bg-background p-3 border">
            <div className="text-xs font-semibold text-orange-700 dark:text-orange-400 mb-1">GA Phase</div>
            <p className="text-xs text-muted-foreground">
              General Availability — only one can be active at a time. GA phases form a continuous
              timeline with no gaps or overlaps.
            </p>
          </div>
          <div className="rounded-lg bg-background p-3 border">
            <div className="text-xs font-semibold text-muted-foreground mb-1">Launch Phase</div>
            <p className="text-xs text-muted-foreground">
              Time-limited promotional or launch programs (e.g., Sunrise, Landrush). Multiple
              launch phases can run simultaneously.
            </p>
          </div>
        </div>

        {/* CTA */}
        <Button onClick={onAddPhase} className="mt-2">
          <Plus className="h-4 w-4 mr-2" />
          Create First Phase
        </Button>

        {/* Hint */}
        <div className="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground/70">
          <Info className="h-3 w-3" />
          <span>Pricing and policies can be configured after creating a phase</span>
        </div>
      </div>
    </div>
  );
}
