'use client';

import { Phase } from '@/lib/types/phase';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet';
import { isPhaseCurrent, isPhaseFuture } from '@/lib/utils/dateUtils';
import { Trash2, GitCompare, CalendarX } from 'lucide-react';

interface PhaseDetailHeaderProps {
  phase: Phase;
  hasPreviousPhase: boolean;
  showDiff: boolean;
  onToggleDiff: () => void;
  onDelete: () => void;
  onEndPhase: () => void;
}

export function PhaseDetailHeader({
  phase,
  hasPreviousPhase,
  showDiff,
  onToggleDiff,
  onDelete,
  onEndPhase,
}: PhaseDetailHeaderProps) {
  const isFuture = isPhaseFuture(phase.starts);
  const isCurrent = isPhaseCurrent(phase.starts, phase.ends);
  const canDelete = isFuture;
  const canEnd = (isCurrent || isFuture) && !phase.ends;

  return (
    <SheetHeader className="pb-6 border-b space-y-4">
      {/* Title and Badge */}
      <div className="space-y-3">
        <div className="flex items-baseline gap-3">
          <SheetTitle className="text-4xl font-bold">{phase.name}</SheetTitle>
          <Badge
            variant={phase.type === 'GA' ? 'default' : 'secondary'}
            className="text-xs"
          >
            {phase.type}
          </Badge>
        </div>
        <SheetDescription className="text-sm text-muted-foreground">
          {isCurrent
            ? '🟢 Currently active'
            : isFuture
            ? '🔵 Scheduled'
            : '⚫ Past'}
        </SheetDescription>
      </div>

      {/* Action Buttons */}
      <div className="flex flex-wrap gap-2">
        {hasPreviousPhase && (
          <Button
            size="sm"
            variant="outline"
            onClick={onToggleDiff}
          >
            <GitCompare className="h-4 w-4 mr-1.5" />
            {showDiff ? 'Hide' : 'Show'} Diff
          </Button>
        )}
        {canEnd && (
          <Button
            size="sm"
            variant="outline"
            onClick={onEndPhase}
          >
            <CalendarX className="h-3.5 w-3.5 mr-1" />
            Set End Date
          </Button>
        )}
        {canDelete && (
          <Button
            size="sm"
            variant="destructive"
            onClick={onDelete}
          >
            <Trash2 className="h-4 w-4 mr-1.5" />
            Delete
          </Button>
        )}
      </div>
    </SheetHeader>
  );
}
