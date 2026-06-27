/**
 * Events API Client
 * Endpoints for fetching domain events from the outbox
 */

import { apiClient } from './client';
import type { DomainEvent } from '@/lib/types/domain';

/**
 * Fetch the most recent events across all entities
 * GET /events?limit=N
 */
export async function getRecentEvents(limit: number = 20): Promise<DomainEvent[]> {
  const { data } = await apiClient.get('/events', { params: { limit } });
  return data;
}
