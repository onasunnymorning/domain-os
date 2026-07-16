import { env } from 'next-runtime-env';

/**
 * Runtime environment accessors.
 *
 * Every NEXT_PUBLIC_* value is read here and nowhere else. Values resolve at
 * request time (server) or from window.__ENV (browser), never at build time —
 * so one image runs in every environment. Do not read process.env directly;
 * an ESLint rule enforces this.
 *
 * Each accessor is a function, not a const. Reading at module scope would
 * capture the value once per process and, in a server component, evaluate
 * outside a request scope.
 */

export const getApiUrl = () => env('NEXT_PUBLIC_API_URL') || 'http://localhost:8080';

export const getApiToken = () => env('NEXT_PUBLIC_API_TOKEN') || '';

/** Auth0 is on unless explicitly disabled, matching the backend's AUTH0_ENABLED. */
export const isAuthEnabled = () => env('NEXT_PUBLIC_AUTH0_ENABLED') !== 'false';

export const getAuth0Domain = () => env('NEXT_PUBLIC_AUTH0_DOMAIN') || '';

export const getAuth0ClientId = () => env('NEXT_PUBLIC_AUTH0_CLIENT_ID') || '';

export const getAuth0Audience = () => env('NEXT_PUBLIC_AUTH0_AUDIENCE') || '';

export const getAppVersion = () => env('NEXT_PUBLIC_APP_VERSION') || '1.0.0';

/**
 * External tool URLs have no localhost fallback: in a deployed environment an
 * unset value used to render silently broken localhost links. Callers must
 * treat '' as "not configured" and hide the corresponding control. Local dev
 * sets these explicitly (Tiltfile / .env.local).
 */
export const getTemporalUiUrl = () => env('NEXT_PUBLIC_TEMPORAL_UI_URL') || '';

/** 'default' is Temporal's actual default namespace, so it is a safe fallback. */
export const getTemporalNamespace = () => env('NEXT_PUBLIC_TEMPORAL_NAMESPACE') || 'default';

export const getStorageUiUrl = () => env('NEXT_PUBLIC_STORAGE_UI_URL') || '';

export const getPostHogToken = () => env('NEXT_PUBLIC_POSTHOG_PROJECT_TOKEN') || '';

export const getPostHogHost = () => env('NEXT_PUBLIC_POSTHOG_HOST') || 'https://us.i.posthog.com';
