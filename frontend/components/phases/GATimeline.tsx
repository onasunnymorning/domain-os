'use client';

import { Phase } from '@/lib/types/phase';
import { PhaseCard } from './PhaseCard';
import { useRef, useEffect } from 'react';

interface GATimelineProps {
  current: Phase | null;
  past: Phase[];
  future: Phase[];
  onPhaseClick: (phase: Phase) => void;
}

export function GATimeline({ current, past, future, onPhaseClick }: GATimelineProps) {
  const currentRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to current phase on mount
  useEffect(() => {
    if (currentRef.current) {
      currentRef.current.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
    }
  }, []);

  return (
    <div className="space-y-3">
      <div className="text-sm font-semibold text-foreground">GA Phases</div>
      
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
            
            {/* Current phase (focal point) */}
            {current ? (
              <div ref={currentRef}>
                <PhaseCard
                  phase={current}
                  status="current"
                  isFocal={true}
                  onClick={() => onPhaseClick(current)}
                />
              </div>
            ) : (
              <div className="text-sm text-muted-foreground italic px-6 py-8 border-2 border-dashed rounded-lg">
                No active GA phase
              </div>
            )}
            
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
