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
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <div className="text-sm font-semibold text-foreground">GA Phases</div>
        {current && (
          <div className="text-xs text-muted-foreground">
            (Current: {current.name})
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
              <div className="text-sm text-muted-foreground italic px-4">
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
        
        {/* Scroll hint */}
        {(past.length > 0 || future.length > 0) && (
          <div className="absolute right-0 top-0 bottom-2 w-8 bg-gradient-to-l from-background to-transparent pointer-events-none" />
        )}
      </div>
    </div>
  );
}
