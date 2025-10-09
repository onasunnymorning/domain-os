'use client';

import { useCategorizedPhases } from '@/lib/hooks/usePhases';
import { GATimeline } from './GATimeline';
import { LaunchTimeline } from './LaunchTimeline';
import { Phase } from '@/lib/types/phase';
import { useState } from 'react';
import { PhaseDetailDrawer } from './PhaseDetailDrawer';
import { PhaseCreateWizard } from './PhaseCreateWizard';
import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-react';
import { Card } from '@/components/ui/card';

interface PhaseTimelineProps {
  tldName: string;
}

export function PhaseTimeline({ tldName }: PhaseTimelineProps) {
  const { categorized, phases, isLoading, error } = useCategorizedPhases(tldName);
  const [selectedPhase, setSelectedPhase] = useState<Phase | null>(null);
  const [showCreateWizard, setShowCreateWizard] = useState(false);

  if (isLoading) {
    return (
      <Card className="p-6">
        <div className="animate-pulse space-y-4">
          <div className="h-4 bg-muted rounded w-32"></div>
          <div className="h-24 bg-muted rounded"></div>
          <div className="h-4 bg-muted rounded w-32"></div>
          <div className="h-24 bg-muted rounded"></div>
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

  return (
    <>
      <Card className="p-6">
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold">Phase Timeline</h2>
            <Button size="sm" variant="outline" onClick={() => setShowCreateWizard(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Add Phase
            </Button>
          </div>

          {/* GA Timeline */}
          <GATimeline
            current={categorized.ga.current}
            past={categorized.ga.past}
            future={categorized.ga.future}
            onPhaseClick={setSelectedPhase}
          />

          {/* Divider */}
          <div className="border-t" />

          {/* Launch Timeline */}
          <LaunchTimeline
            current={categorized.launch.current}
            past={categorized.launch.past}
            future={categorized.launch.future}
            onPhaseClick={setSelectedPhase}
          />
        </div>
      </Card>

      {/* Phase Detail Drawer */}
      <PhaseDetailDrawer
        phase={selectedPhase}
        open={!!selectedPhase}
        onClose={() => setSelectedPhase(null)}
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
