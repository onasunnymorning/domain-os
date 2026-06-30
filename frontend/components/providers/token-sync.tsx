'use client';

import { useAuth0 } from '@auth0/auth0-react';
import { useEffect } from 'react';
import { setAuthToken } from '@/lib/api/client';
import posthog from 'posthog-js';

/**
 * TokenSync component handles fetching the Auth0 access token and 
 * syncing it with the API client.
 * 
 * It must be placed inside the Auth0Provider.
 */
export function TokenSync() {
    const { getAccessTokenSilently, isAuthenticated, isLoading, user } = useAuth0();

    useEffect(() => {
        const authEnabled = process.env.NEXT_PUBLIC_AUTH0_ENABLED !== 'false';

        const updateToken = async () => {
            if (!authEnabled) {
                setAuthToken(null);
                return;
            }

            if (isAuthenticated) {
                try {
                    const token = await getAccessTokenSilently();
                    setAuthToken(token);
                } catch (error) {
                    console.error('Error fetching Auth0 token:', error);
                    setAuthToken(null);
                }
            } else if (!isLoading) {
                setAuthToken(null);
            }
        };

        updateToken();
    }, [getAccessTokenSilently, isAuthenticated, isLoading]);

    useEffect(() => {
        if (isAuthenticated && user?.sub) {
            posthog.identify(user.sub, {
                email: user.email,
                name: user.name,
            });
        }
    }, [isAuthenticated, user]);

    return null;
}
