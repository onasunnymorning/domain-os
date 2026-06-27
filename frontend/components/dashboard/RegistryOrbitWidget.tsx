'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useRegistryOperators } from '@/lib/hooks/useRegistryOperators';
import { useTLDs } from '@/lib/hooks/useTLDs';
import { RegistryOrbitMap } from '@/components/registry-operators/TLDOrbitVisualization';
import { Network } from 'lucide-react';

/**
 * Dashboard widget showing all Registry Operators and their TLDs
 * as an interactive force-directed hub-spoke graph.
 */
export function RegistryOrbitWidget() {
  const { data: roData, isLoading: loadingROs } = useRegistryOperators({ pagesize: 100 });
  const { data: tldData, isLoading: loadingTLDs } = useTLDs({ pagesize: 500 });

  const isLoading = loadingROs || loadingTLDs;
  const operators = roData?.Data ?? [];
  const tlds = tldData?.Data ?? [];

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-base">
          <Network className="h-4 w-4 text-muted-foreground" />
          Registry Landscape
        </CardTitle>
      </CardHeader>
      <CardContent className="p-2 pt-0">
        <RegistryOrbitMap
          operators={operators}
          tlds={tlds}
          isLoading={isLoading}
        />
      </CardContent>
    </Card>
  );
}
