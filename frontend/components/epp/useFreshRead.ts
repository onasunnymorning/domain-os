"use client";

import { useState } from "react";

/**
 * When an editor may mount its form. A cached copy is never enough: the
 * form waits for a fetch that completed after this mount (offline, a paused
 * fetch never completes and the form stays unmounted). Once mounted it stays
 * mounted: a later background refetch, failed or not, must not discard edits.
 */
export function useFreshRead(q: { data?: unknown; dataUpdatedAt: number; isFetching: boolean; error?: unknown }) {
  const [mountedAt] = useState(() => Date.now());
  const [fetchedAt, setFetchedAt] = useState<number | null>(null);
  if (fetchedAt === null && q.data && !q.isFetching && q.dataUpdatedAt >= mountedAt) {
    setFetchedAt(q.dataUpdatedAt);
  }
  return {
    /** The read the form is built on; null until a fresh read completed. */
    fetchedAt,
    /** An error before the first fresh read: the blocking case. */
    initialError: fetchedAt === null && !q.isFetching ? q.error : undefined,
  };
}
