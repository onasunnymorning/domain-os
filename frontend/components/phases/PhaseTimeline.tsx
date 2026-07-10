'use client';

import { usePhases } from '@/lib/hooks/usePhases';
import { Phase } from '@/lib/types/phase';
import { useState, useEffect } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { PhaseDetailDrawer } from './PhaseDetailDrawer';
import { PhaseCreateConversation } from './PhaseCreateConversation';
import { GanttTimeline } from './GanttTimeline';
import { Button } from '@/components/ui/button';
import { Plus, Calendar, Layers, Import, ArrowRight } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { WorkflowShortcuts } from '@/components/shared/WorkflowShortcuts';

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
            <EmptyPhaseState
              tldName={tldName}
              onAddPhase={() => setShowCreateWizard(true)}
            />
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

      {/* Create Phase Conversation */}
      <PhaseCreateConversation
        tldName={tldName}
        open={showCreateWizard}
        onClose={() => setShowCreateWizard(false)}
        existingPhases={phases || []}
      />
    </>
  );
}

// ── Empty State with Import CTA ────────────────────────────────────────────

function EmptyPhaseState({ tldName, onAddPhase }: { tldName: string; onAddPhase: () => void }) {
  return (
    <div className="rounded-lg border-2 border-dashed border-border/60 bg-muted/20 p-8">
      <div className="max-w-lg mx-auto text-center space-y-5">
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
            this TLD at different points in time.
          </p>
        </div>

        {/* Import CTA */}
        <div className="rounded-lg border bg-muted/30 p-4 space-y-2">
          <div className="flex items-center gap-2 text-sm">
            <Import className="h-4 w-4 text-muted-foreground shrink-0" />
            <span>Need to import existing registry data first?</span>
          </div>
          <WorkflowShortcuts workflowKeys={['escrow-import']} />
        </div>

        {/* Create CTA */}
        <div className="space-y-2">
          <p className="text-xs text-muted-foreground">Or start fresh:</p>
          <Button onClick={onAddPhase} className="gap-1.5">
            <Plus className="h-4 w-4" />
            Create First Phase
          </Button>
        </div>
      </div>
    </div>
  );
}
