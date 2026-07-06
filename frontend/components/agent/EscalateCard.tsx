'use client';

import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { AlertTriangle, UserCheck } from 'lucide-react';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface EscalateCardProps {
  reason: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EscalateCard({ reason }: EscalateCardProps) {
  return (
    <Card className="overflow-hidden border-amber-500/30 bg-gradient-to-br from-amber-500/5 to-amber-600/10 shadow-md transition-shadow hover:shadow-lg">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-500/15 text-amber-600 dark:text-amber-400">
            <AlertTriangle className="h-4 w-4" />
          </div>
          <CardTitle className="text-base font-semibold tracking-tight text-amber-700 dark:text-amber-300">
            Escalated to Human Representative
          </CardTitle>
        </div>
      </CardHeader>

      <CardContent className="space-y-3">
        <p className="text-sm leading-relaxed text-foreground/80">{reason}</p>
      </CardContent>

      <CardFooter>
        <div className="flex items-center gap-1.5 text-xs text-amber-600/70 dark:text-amber-400/60">
          <UserCheck className="h-3 w-3 shrink-0" />
          <span>A team member will follow up on this request.</span>
        </div>
      </CardFooter>
    </Card>
  );
}
