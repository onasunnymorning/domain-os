'use client';

import { Phase } from '@/lib/types/phase';
import { Info } from 'lucide-react';

interface PhaseMetadataSectionProps {
  phase: Phase;
}

export function PhaseMetadataSection({ phase }: PhaseMetadataSectionProps) {
  return (
    <div className="pt-4 border-t">
      <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-6 text-xs text-muted-foreground">
        <div className="flex items-center gap-2">
          <Info className="h-3 w-3" />
          <span>Created: {new Date(phase.createdAt).toLocaleString()}</span>
        </div>
        <div className="flex items-center gap-2 sm:pl-0 pl-5">
          <span>Updated: {new Date(phase.updatedAt).toLocaleString()}</span>
        </div>
      </div>
    </div>
  );
}
