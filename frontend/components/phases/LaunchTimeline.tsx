'use client';

import { Phase } from '@/lib/types/phase';
import { PhaseCard } from './PhaseCard';

interface LaunchTimelineProps {
  current: Phase[];
  past: Phase[];
  future: Phase[];
  onPhaseClick: (phase: Phase) => void;
}

export function LaunchTimeline({ current, past, future, onPhaseClick }: LaunchTimelineProps) {
  const allPhases = [...past, ...current, ...future];

  if (allPhases.length === 0) {
    return (
      <div className="space-y-2">
        <div className="text-sm font-semibold text-foreground">Launch Phases</div>
        <div className="text-sm text-muted-foreground italic px-2 py-4">
          No launch phases configured
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <div className="text-sm font-semibold text-foreground">Launch Phases</div>
        {current.length > 0 && (
          <div className="text-xs text-muted-foreground">
            ({current.length} active)
          </div>
        )}
      </div>
      
      <div className="relative">
        <div className="overflow-x-auto pb-2">
          <div className="flex items-center gap-3 min-w-max px-2">
            {/* Past phases */}
            {past.map((phase) => (
              <PhaseCard
                key={phase.id}
                phase={phase}
                status="past"
                onClick={() => onPhaseClick(phase)}
              />
            ))}
            
            {/* Current phases (can be multiple) */}
            {current.map((phase) => (
              <PhaseCard
                key={phase.id}
                phase={phase}
                status="current"
                onClick={() => onPhaseClick(phase)}
              />
            ))}
            
            {/* Future phases */}
            {future.map((phase) => (
              <PhaseCard
                key={phase.id}
                phase={phase}
                status="future"
                onClick={() => onPhaseClick(phase)}
              />
            ))}
          </div>
        </div>
        
        {/* Scroll hint */}
        {allPhases.length > 3 && (
          <div className="absolute right-0 top-0 bottom-2 w-8 bg-gradient-to-l from-background to-transparent pointer-events-none" />
        )}
      </div>
    </div>
  );
}
