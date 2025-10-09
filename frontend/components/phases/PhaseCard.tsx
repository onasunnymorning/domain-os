'use client';

import { Phase, PhaseStatus } from '@/lib/types/phase';
import { formatPhaseDate, formatRelativeDate } from '@/lib/utils/dateUtils';
import { Check, Clock } from 'lucide-react';
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
      bgColor: 'bg-muted/40',
      borderColor: 'border-border/50',
      opacity: 'opacity-50',
      showCheck: false,
    },
    current: {
      bgColor: 'bg-gradient-to-br from-orange-50 to-orange-100/50',
      borderColor: 'border-orange-300',
      opacity: 'opacity-100',
      showCheck: true,
    },
    future: {
      bgColor: 'bg-background',
      borderColor: 'border-dashed border-border',
      opacity: 'opacity-70',
      showCheck: false,
    },
  };

  const config = statusConfig[status];

  const cardClasses = cn(
    'rounded-xl border-2 transition-all duration-300',
    config.bgColor,
    config.borderColor,
    config.opacity,
    isFocal && status === 'current' && 'shadow-xl shadow-orange-200/50 scale-105 border-orange-400',
    'hover:scale-105 hover:shadow-lg',
    'cursor-pointer',
    'relative overflow-hidden',
    // Larger sizes for carousel effect
    isFocal ? 'p-6 min-w-[280px]' : status === 'current' ? 'p-5 min-w-[240px]' : 'p-4 min-w-[200px]'
  );

  return (
    <div className={cardClasses} onClick={onClick}>
      {/* Status indicator circle */}
      <div className="flex items-start gap-4">
        <div className={cn(
          'flex-shrink-0 rounded-full flex items-center justify-center',
          isFocal ? 'h-12 w-12' : 'h-10 w-10',
          status === 'current' ? 'bg-orange-500 text-white' : status === 'past' ? 'bg-muted text-muted-foreground' : 'bg-background border-2 border-dashed border-muted-foreground/40 text-muted-foreground'
        )}>
          {status === 'current' ? (
            <Check className={cn('stroke-[3]', isFocal ? 'h-7 w-7' : 'h-6 w-6')} />
          ) : status === 'future' ? (
            <Clock className={isFocal ? 'h-5 w-5' : 'h-4 w-4'} />
          ) : (
            <Check className={cn('opacity-40', isFocal ? 'h-6 w-6' : 'h-5 w-5')} />
          )}
        </div>
        
        <div className="flex-1 min-w-0 space-y-2">
          <div className={cn(
            'font-semibold truncate',
            isFocal ? 'text-xl' : 'text-lg',
            status === 'current' && 'text-orange-900'
          )}>
            {phase.name}
          </div>
          
          <div className="space-y-1">
            <div className={cn(
              'text-muted-foreground',
              isFocal ? 'text-sm' : 'text-xs'
            )}>
              <span className="font-medium">{status === 'future' ? 'Starts:' : 'Started:'}</span> {formatPhaseDate(phase.starts)}
            </div>
            {phase.ends ? (
              <div className={cn(
                'text-muted-foreground',
                isFocal ? 'text-sm' : 'text-xs'
              )}>
                <span className="font-medium">Ends:</span> {formatPhaseDate(phase.ends)}
              </div>
            ) : (
              <div className={cn(
                'text-muted-foreground italic',
                isFocal ? 'text-sm' : 'text-xs'
              )}>
                {status === 'current' ? 'Ongoing' : 'No end date'}
              </div>
            )}
          </div>
          
          {isFocal && status === 'current' && (
            <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-orange-500 text-white text-xs font-medium mt-2">
              <div className="h-1.5 w-1.5 rounded-full bg-white animate-pulse" />
              Active {formatRelativeDate(phase.starts)}
            </div>
          )}
        </div>
      </div>
      
      {/* Decorative corner accent for current phase */}
      {status === 'current' && (
        <div className="absolute top-0 right-0 w-20 h-20 bg-gradient-to-br from-orange-400/20 to-transparent rounded-bl-full" />
      )}
    </div>
  );
}
