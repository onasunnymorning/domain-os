'use client';

import { Clock, ExternalLink } from 'lucide-react';
import { cn } from '@/lib/utils';

interface WorkflowTemporalLinkProps {
  url: string;
  className?: string;
}

export function WorkflowTemporalLink({ url, className }: WorkflowTemporalLinkProps) {
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        'text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-xs transition-colors',
        className
      )}
    >
      <Clock className="size-3" />
      <span>View in Temporal</span>
      <ExternalLink className="size-3" />
    </a>
  );
}
