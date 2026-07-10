import { useQuery } from '@tanstack/react-query';
import {
  getTombstoneByRoID,
  getTombstonesByName,
  countTombstones,
} from '@/lib/api/tombstones';
import type { DomainTombstone } from '@/lib/api/tombstones';

/**
 * Fetch all tombstones (past incarnations) for a given domain name.
 * Returns an array ordered by purged_at DESC (most recent first).
 */
export function useTombstonesByName(name: string, enabled = true) {
  return useQuery<DomainTombstone[]>({
    queryKey: ['tombstones', 'by-name', name],
    queryFn: () => getTombstonesByName(name),
    enabled: !!name && enabled,
  });
}

/**
 * Fetch a single tombstone by its ROID.
 */
export function useTombstoneByRoID(roid: string, enabled = true) {
  return useQuery<DomainTombstone>({
    queryKey: ['tombstones', 'by-roid', roid],
    queryFn: () => getTombstoneByRoID(roid),
    enabled: !!roid && enabled,
  });
}

/**
 * Fetch the total count of domain tombstones (optionally filtered).
 */
export function useTombstoneCount(
  params: Parameters<typeof countTombstones>[0] = {},
) {
  return useQuery<number>({
    queryKey: ['tombstones', 'count', params],
    queryFn: () => countTombstones(params),
  });
}
