'use client';

import { Phase, PhaseStatus } from '@/lib/types/phase';
import { formatPhaseDate, formatRelativeDate } from '@/lib/utils/dateUtils';
import { CheckCircle2, Circle, Clock } from 'lucide-react';
import { cn } from '@/lib/utils';

interface PhaseCardProps {
  phase: Phase;
  status: PhaseStatus;
  isFocal?: boolean;
  onClick?: () => void;
}

export function PhaseCard({ phase, status, isFocal = false, onClick }: PhaseCardProps) {
  const statusConfig = {
    past: {
      icon: CheckCircle2,
      iconColor: 'text-[oklch(0.6_0.12_45)]',
      bgColor: 'bg-muted/30',
      borderColor: 'border-border/50',
      opacity: 'opacity-60',
    },
    current: {
      icon: Circle,
      iconColor: 'text-[oklch(0.65_0.18_45)]',
      bgColor: 'bg-gradient-to-br from-[oklch(0.98_0.02_45)] to-[oklch(0.95_0.05_45)]',
      borderColor: 'border-[oklch(0.7_0.15_45)]',
      opacity: 'opacity-100',
    },
    future: {
      icon: Clock,
      iconColor: 'text-muted-foreground',
      bgColor: 'bg-background',
      borderColor: 'border-dashed border-border',
      opacity: 'opacity-75',
    },
  };

  const config = statusConfig[status];
  const Icon = config.icon;

  const cardClasses = cn(
    'rounded-lg border-2 transition-all duration-200',
    config.bgColor,
    config.borderColor,
    config.opacity,
    isFocal && status === 'current' && 'shadow-[0_0_20px_rgba(255,149,0,0.3)] scale-105',
    'hover:scale-105 hover:shadow-md',
    'cursor-pointer',
    // Size based on status and focal
    isFocal ? 'p-4 min-w-[200px]' : status === 'current' ? 'p-3 min-w-[160px]' : 'p-2 min-w-[120px]'
  );

  return (
    <div className={cardClasses} onClick={onClick}>
      <div className="flex items-start gap-2">
        <Icon className={cn('flex-shrink-0', config.iconColor, isFocal ? 'h-5 w-5' : 'h-4 w-4')} />
        <div className="flex-1 min-w-0">
          <div className={cn(
            'font-medium truncate',
            isFocal ? 'text-base' : 'text-sm'
          )}>
            {phase.name}
          </div>
          <div className={cn(
            'text-muted-foreground truncate',
            isFocal ? 'text-sm' : 'text-xs'
          )}>
            {formatPhaseDate(phase.starts)}
          </div>
          {phase.ends && (
            <div className={cn(
              'text-muted-foreground truncate',
              isFocal ? 'text-sm' : 'text-xs'
            )}>
              → {formatPhaseDate(phase.ends)}
            </div>
          )}
          {isFocal && status === 'current' && (
            <div className="text-xs text-[oklch(0.65_0.18_45)] font-medium mt-1">
              Active {formatRelativeDate(phase.starts)}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
