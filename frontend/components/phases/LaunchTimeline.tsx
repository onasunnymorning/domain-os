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
      <div className="space-y-3">
        <div className="text-sm font-semibold text-foreground">Launch Phases</div>
        <div className="text-sm text-muted-foreground italic px-6 py-8 border-2 border-dashed rounded-lg">
          No launch phases configured
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <div className="text-sm font-semibold text-foreground">Launch Phases</div>
        {current.length > 0 && (
          <div className="text-xs px-2 py-0.5 rounded-full bg-orange-100 text-orange-700 font-medium">
            {current.length} active
          </div>
        )}
      </div>
      
      <div className="relative -mx-6 px-6">
        <div className="overflow-x-auto pb-4 scrollbar-thin scrollbar-thumb-orange-300 scrollbar-track-transparent">
          <div className={`flex items-center gap-4 min-w-max py-2 ${past.length > 0 ? 'pl-6 pr-1' : 'pl-6 pr-1'}`}>
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
        
        {/* Scroll gradient hints */}
        {past.length > 0 && (
          <div className="absolute left-0 top-0 bottom-4 w-12 bg-gradient-to-r from-background via-background/80 to-transparent pointer-events-none" />
        )}
        {future.length > 0 && (
          <div className="absolute right-0 top-0 bottom-4 w-12 bg-gradient-to-l from-background via-background/80 to-transparent pointer-events-none" />
        )}
      </div>
    </div>
  );
}
