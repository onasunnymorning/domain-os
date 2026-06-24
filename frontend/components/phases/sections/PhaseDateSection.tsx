'use client';

import { Phase } from '@/lib/types/phase';
import { formatPhaseDateLong, formatRelativeDate, isPhaseFuture, isPhaseCurrent } from '@/lib/utils/dateUtils';
import { Clock } from 'lucide-react';
import { format } from 'date-fns';

interface PhaseDateSectionProps {
  phase: Phase;
}

function formatDateWithTime(dateString: string) {
  const date = new Date(dateString);
  return {
    date: format(date, 'MMMM d, yyyy'),
    time: format(date, 'HH:mm:ss'),
  };
}

export function PhaseDateSection({ phase }: PhaseDateSectionProps) {
  const isFuture = isPhaseFuture(phase.starts);
  const isCurrent = isPhaseCurrent(phase.starts, phase.ends);

  return (
    <div className="space-y-4">
      {/* Start Date */}
      <div className="space-y-1.5">
        <div className="text-xs text-muted-foreground uppercase tracking-wide">
          {isFuture ? 'Starts' : 'Started'}
        </div>
        <div className="text-3xl font-bold">{formatDateWithTime(phase.starts).date}</div>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Clock className="h-3.5 w-3.5" />
          <span>{formatDateWithTime(phase.starts).time} UTC</span>
          <span>•</span>
          <span>{formatRelativeDate(phase.starts)}</span>
        </div>
      </div>

      {/* End Date */}
      <div className={`space-y-1.5 ${!phase.ends ? 'opacity-60' : ''}`}>
        <div className="text-xs text-muted-foreground uppercase tracking-wide">Ends</div>
        {phase.ends ? (
          <>
            <div className="text-3xl font-bold">{formatDateWithTime(phase.ends).date}</div>
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Clock className="h-3.5 w-3.5" />
              <span>{formatDateWithTime(phase.ends).time} UTC</span>
              <span>•</span>
              <span>{formatRelativeDate(phase.ends)}</span>
            </div>
          </>
        ) : (
          <div className="text-lg font-semibold italic text-muted-foreground">
            {isCurrent ? 'Ongoing (no end date)' : 'Not set'}
          </div>
        )}
      </div>
    </div>
  );
}
