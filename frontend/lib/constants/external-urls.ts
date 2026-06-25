/**
 * External service URLs, configurable via NEXT_PUBLIC_* environment variables
 * with sensible local-development defaults.
 */

export const TEMPORAL_UI_URL =
  process.env.NEXT_PUBLIC_TEMPORAL_UI_URL || 'http://localhost:8081';

export const STORAGE_UI_URL =
  process.env.NEXT_PUBLIC_STORAGE_UI_URL || 'http://localhost:9001';

