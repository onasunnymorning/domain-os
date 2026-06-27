/**
 * useRecentEvents Hook
 * React Query hook for fetching the global event stream.
 * Auto-refreshes every 30 seconds to show near-real-time activity.
 */

'use client';

import { useQuery } from '@tanstack/react-query';
import { getRecentEvents } from '@/lib/api/events';

export function useRecentEvents(limit: number = 20) {
  return useQuery({
    queryKey: ['events', 'recent', limit],
    queryFn: () => getRecentEvents(limit),
    refetchInterval: 30_000,
    staleTime: 10_000,
  });
}
