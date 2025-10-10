'use client';

import { useTLDsByRyID } from '@/lib/hooks/useTLDs';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import Link from 'next/link';

interface TLDBadgesProps {
  ryid: string;
  maxDisplay?: number;
}

export function TLDBadges({ ryid, maxDisplay = 5 }: TLDBadgesProps) {
  const { data, isLoading } = useTLDsByRyID(ryid);

  if (isLoading) {
    return <Skeleton className="h-5 w-20" />;
  }

  const tlds = data?.Data || [];
  const displayTLDs = maxDisplay ? tlds.slice(0, maxDisplay) : tlds;
  const remainingCount = tlds.length - displayTLDs.length;

  if (tlds.length === 0) {
    return <span className="text-xs text-muted-foreground">No TLDs</span>;
  }

  return (
    <div className="flex flex-wrap gap-1">
      {displayTLDs.map((tld) => (
        <Link key={tld.Name} href={`/tlds/${tld.Name}`}>
          <Badge 
            variant="secondary" 
            className="text-xs hover:bg-primary/20 cursor-pointer"
          >
            .{tld.Name}
          </Badge>
        </Link>
      ))}
      {remainingCount > 0 && (
        <Badge variant="outline" className="text-xs">
          +{remainingCount} more
        </Badge>
      )}
    </div>
  );
}
