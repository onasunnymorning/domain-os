'use client';

import { useActivePhases } from '@/lib/hooks/usePhases';
import { Badge } from '@/components/ui/badge';
import { useRouter } from 'next/navigation';

interface TLDActivePhasesProps {
  tldName: string;
}

export function TLDActivePhases({ tldName }: TLDActivePhasesProps) {
  const router = useRouter();
  const { data: activePhases, isLoading } = useActivePhases(tldName);

  if (isLoading) {
    return (
      <div className="flex gap-1">
        <Badge variant="outline" className="animate-pulse">
          Loading...
        </Badge>
      </div>
    );
  }

  if (!activePhases || activePhases.length === 0) {
    return null;
  }

  const handlePhaseClick = (e: React.MouseEvent, phaseName: string) => {
    e.stopPropagation();
    // Navigate to TLD page with phase parameter
    router.push(`/tlds/${tldName}?phase=${encodeURIComponent(phaseName)}`);
  };

  return (
    <div className="flex flex-wrap gap-1">
      {activePhases.map((phase) => (
        <Badge
          key={phase.name}
          variant={phase.type === 'GA' ? 'default' : 'secondary'}
          className="cursor-pointer hover:opacity-80 transition-opacity"
          onClick={(e) => handlePhaseClick(e, phase.name)}
        >
          {phase.name}
          {phase.type === 'Launch' && ' 🚀'}
        </Badge>
      ))}
    </div>
  );
}
