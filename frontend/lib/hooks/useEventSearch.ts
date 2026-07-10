/**
 * useEventSearch Hook
 * React Query hook for searching events with filters and cursor-based pagination.
 * Supports infinite scroll / "load more" via nextCursor.
 */

'use client';

import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { searchEvents, type EventSearchParams, type EventSearchResult } from '@/lib/api/events';

/**
 * Hook for paginated event search with infinite scroll support.
 * Uses React Query's useInfiniteQuery for cursor-based pagination.
 *
 * @param params - Search filter parameters (excluding cursor, which is managed internally)
 * @param options - Additional options
 * @param options.enabled - Whether the query should execute (default: true)
 * @param options.refetchInterval - Auto-refresh interval in ms (default: undefined = no auto-refresh)
 */
export function useEventSearch(
  params: Omit<EventSearchParams, 'cursor'>,
  options?: { enabled?: boolean; refetchInterval?: number }
) {
  return useInfiniteQuery<EventSearchResult>({
    queryKey: ['events', 'search', params],
    queryFn: ({ pageParam }) =>
      searchEvents({ ...params, cursor: pageParam as string | undefined }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    staleTime: 10_000,
  });
}

/**
 * Hook for a single-page event search (no infinite scroll).
 * Useful for widgets that just need the first page of results.
 *
 * @param params - Search filter parameters
 * @param options - Additional options
 */
export function useEventSearchPage(
  params: EventSearchParams,
  options?: { enabled?: boolean; refetchInterval?: number }
) {
  return useQuery<EventSearchResult>({
    queryKey: ['events', 'search', 'page', params],
    queryFn: () => searchEvents(params),
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    staleTime: 10_000,
  });
}
