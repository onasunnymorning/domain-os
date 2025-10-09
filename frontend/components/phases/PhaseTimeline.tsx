'use client';

import { useCategorizedPhases } from '@/lib/hooks/usePhases';
import { GATimeline } from './GATimeline';
import { LaunchTimeline } from './LaunchTimeline';
import { Phase } from '@/lib/types/phase';
import { useState, useEffect } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { PhaseDetailDrawer } from './PhaseDetailDrawer';
import { PhaseCreateWizard } from './PhaseCreateWizard';
import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-react';
import { Card } from '@/components/ui/card';

interface PhaseTimelineProps {
  tldName: string;
  initialPhaseName?: string;
}

export function PhaseTimeline({ tldName, initialPhaseName }: PhaseTimelineProps) {
  const router = useRouter();
  const pathname = usePathname();
  const { categorized, phases, isLoading, error } = useCategorizedPhases(tldName);
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
