'use client';

import { useAuth0 } from '@auth0/auth0-react';
import { useRouter } from 'next/navigation';
import { useEffect, type ReactNode } from 'react';

interface ProtectedRouteProps {
    children: ReactNode;
}

/**
 * ProtectedRoute component ensures that only authenticated users can access its children.
 * If the user is not authenticated and no longer loading, it initiates the logout flow 
 * or redirects to the login.
 */
export function ProtectedRoute({ children }: ProtectedRouteProps) {
    const { isAuthenticated, isLoading, loginWithRedirect } = useAuth0();
    const router = useRouter();
    const authEnabled = process.env.NEXT_PUBLIC_AUTH0_ENABLED !== 'false';

    useEffect(() => {
        if (!authEnabled) return;

        // Don't redirect if we're mid-Auth0 callback (code + state in URL).
        // The Auth0Provider needs time to exchange the code for a session;
        // redirecting here would cause an infinite login loop.
        const params = new URLSearchParams(window.location.search);
        if (params.has('code') && params.has('state')) return;

        // If not authenticated and no longer loading, redirect to login
        if (!isLoading && !isAuthenticated) {
            loginWithRedirect();
        }
    }, [isAuthenticated, isLoading, loginWithRedirect, authEnabled]);

    if (!authEnabled) {
        return <>{children}</>;
    }

    if (isLoading) {
        return (
            <div className="flex h-screen w-screen items-center justify-center">
                <div className="h-32 w-32 animate-spin rounded-full border-b-2 border-t-2 border-primary"></div>
            </div>
        );
    }

    if (!isAuthenticated) {
        return null; // Will redirect in useEffect
    }

    return <>{children}</>;
}
