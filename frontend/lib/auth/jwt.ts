/**
 * Minimal JWT payload decoding.
 *
 * DISPLAY ONLY. Nothing here verifies a signature, and no authorization
 * decision may be made from it. The access token is validated server-side by
 * Auth0Middleware (internal/interface/rest/auth_middleware.go) — the frontend
 * reads the payload purely to tell the user what their session carries.
 *
 * Field names mirror what the backend reads: `permissions` is Auth0's RBAC
 * claim (internal/interface/rest/authz.go) and `sub` becomes the gin context's
 * `userid` (auth_middleware.go).
 */

export interface JwtPayload {
    sub?: string;
    exp?: number;
    iat?: number;
    iss?: string;
    aud?: string | string[];
    /** Space-delimited OAuth scopes. */
    scope?: string;
    /** Auth0 RBAC permissions, present only when the API has RBAC enabled. */
    permissions?: string[];
}

/**
 * Decodes the payload segment of a JWT. Returns null for anything that is not a
 * decodable three-segment token — callers render a degraded view rather than
 * failing, so this never throws.
 */
export function decodeJwtPayload(token: string | null | undefined): JwtPayload | null {
    if (!token) return null;

    const segments = token.split('.');
    if (segments.length !== 3) return null;

    try {
        // base64url → base64, then pad to a multiple of 4 for atob().
        const base64 = segments[1].replace(/-/g, '+').replace(/_/g, '/');
        const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=');
        const parsed: unknown = JSON.parse(atob(padded));

        // A payload that is not a JSON object (e.g. a bare string or array) is
        // not a usable claims set.
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
            return null;
        }
        return parsed as JwtPayload;
    } catch {
        return null;
    }
}

/** Splits the space-delimited `scope` claim into individual scopes. */
export function scopesFromPayload(payload: JwtPayload | null): string[] {
    if (!payload?.scope) return [];
    return payload.scope.split(' ').filter(Boolean);
}

/** The `exp` claim as a Date, or null when absent or non-numeric. */
export function expiryFromPayload(payload: JwtPayload | null): Date | null {
    if (typeof payload?.exp !== 'number') return null;
    return new Date(payload.exp * 1000);
}
