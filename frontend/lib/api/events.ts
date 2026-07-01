/**
 * Events API Client
 * Endpoints for fetching and searching domain events
 */

import { apiClient } from './client';
import type { DomainEvent } from '@/lib/types/domain';

/**
 * Event search parameters for the unified search API
 */
export interface EventSearchParams {
  subject?: string;
  type?: string;
  source?: string;
  actor?: string;
  roid?: string;
  trace_id?: string;
  correlation_id?: string;
  after?: string;  // ISO 8601
  before?: string; // ISO 8601
  limit?: number;
  cursor?: string;
}

/**
 * Event search result envelope
 */
export interface EventSearchResult {
  data: DomainEvent[];
  nextCursor?: string;
  totalCount: number;
  tier: 'hot' | 'warm' | 'mixed';
}

/**
 * Search events with filters and cursor-based pagination
 * GET /events/search?subject=&type=&source=&actor=&roid=&after=&before=&limit=&cursor=
 */
export async function searchEvents(params: EventSearchParams): Promise<EventSearchResult> {
  // Strip undefined/empty params to keep the URL clean
  const cleanParams = Object.fromEntries(
    Object.entries(params).filter(([, v]) => v !== undefined && v !== '' && v !== null)
  );
  const { data } = await apiClient.get('/events/search', { params: cleanParams });
  return data;
}

/**
 * Fetch the most recent events across all entities (backward-compatible)
 * GET /events?limit=N
 */
export async function getRecentEvents(limit: number = 20): Promise<DomainEvent[]> {
  const { data } = await apiClient.get('/events', { params: { limit } });
  return data;
}
