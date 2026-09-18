'use client';

import { useAuth0 } from '@auth0/auth0-react';
import { useEffect, useState } from 'react';
import { isAuthEnabled } from '@/lib/env';
import { decodeJwtPayload, expiryFromPayload, scopesFromPayload } from '@/lib/auth/jwt';

/**
 * The static-token identity the backend assigns when Auth0 is disabled.
 * See internal/interface/rest/auth_middleware.go — the legacy token path sets
 * `userid` to this value.
 */
export const LEGACY_SUBJECT = 'legacy-admin';

export type SessionMode = 'auth0' | 'disabled' | 'anonymous';

export interface SessionClaims {
    /** Which auth path this browser session is actually on. */
    mode: SessionMode;
    /** The `sub` the API will attribute requests to, when known. */
    subject: string | null;
    /** Auth0 RBAC permissions carried by the access token. */
    permissions: string[];
    /** OAuth scopes carried by the access token. */
    scopes: string[];
    /** Access token expiry, when the token could be read and decoded. */
    expiresAt: Date | null;
    /** True while the access token is being fetched. */
    isLoading: boolean;
    /** Set when the token could not be fetched or decoded. */
    error: string | null;
}

interface Options {
    /**
     * Gate the token fetch. The menu passes its open state so an access token is
     * only requested when someone actually looks at the session — not on every
     * page render.
     */
    enabled?: boolean;
}

/**
 * Describes the current browser session for display.
 *
 * The token is read via getAccessTokenSilently() rather than the api client's
 * resolveAuthToken(): that is a non-reactive module variable written
 * asynchronously by TokenSync, so reading it races the first render. The Auth0
 * SDK caches and dedupes this call.
 */
export function useSessionClaims({ enabled = true }: Options = {}): SessionClaims {
    const { user, isAuthenticated, getAccessTokenSilently } = useAuth0();
    const authEnabled = isAuthEnabled();

    const [token, setToken] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        if (!enabled || !authEnabled || !isAuthenticated) return;

        let cancelled = false;
        setIsLoading(true);

        getAccessTokenSilently()
            .then((accessToken) => {
                if (cancelled) return;
                setToken(accessToken);
                setError(null);
            })
            .catch((err: unknown) => {
                if (cancelled) return;
                setToken(null);
                setError(err instanceof Error ? err.message : 'Could not read the access token');
            })
            .finally(() => {
                if (!cancelled) setIsLoading(false);
            });

        return () => {
            cancelled = true;
        };
    }, [enabled, authEnabled, isAuthenticated, getAccessTokenSilently]);

    if (!authEnabled) {
        return {
            mode: 'disabled',
            subject: LEGACY_SUBJECT,
            permissions: [],
            scopes: [],
            expiresAt: null,
            isLoading: false,
            error: null,
        };
    }

    if (!isAuthenticated) {
        return {
            mode: 'anonymous',
            subject: null,
            permissions: [],
            scopes: [],
            expiresAt: null,
            isLoading: false,
            error: null,
        };
    }

    const payload = decodeJwtPayload(token);

    return {
        mode: 'auth0',
        subject: payload?.sub ?? user?.sub ?? null,
        permissions: payload?.permissions ?? [],
        scopes: scopesFromPayload(payload),
        expiresAt: expiryFromPayload(payload),
        // Only "loading" while there is nothing to show. Reopening the menu
        // refetches, and flipping back to a spinner over claims we already have
        // reads as a flicker.
        isLoading: isLoading && token === null,
        error,
    };
}
