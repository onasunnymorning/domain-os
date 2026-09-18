'use client';

import { useAuth0 } from '@auth0/auth0-react';
import { formatDistanceToNow } from 'date-fns';
import { LogIn, LogOut, TriangleAlert, User as UserIcon, Wrench } from 'lucide-react';
import posthog from 'posthog-js';
import { useState } from 'react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { ScrollArea } from '@/components/ui/scroll-area';
import { setAuthToken } from '@/lib/api/client';
import { useSessionClaims } from '@/lib/hooks/useSessionClaims';

/**
 * The profile element in the top bar: who you are signed in as, what the API
 * will authorize you to do, and the way out.
 *
 * It renders in every auth state on purpose. It used to return null unless
 * Auth0 reported an authenticated user, which meant that with
 * NEXT_PUBLIC_AUTH0_ENABLED=false (the local dev default) the header carried no
 * profile element at all while ProtectedRoute still let you into the app.
 */
export function UserMenu() {
    const { user, logout, loginWithRedirect } = useAuth0();
    const [open, setOpen] = useState(false);
    // Only read the access token while the menu is actually open.
    const session = useSessionClaims({ enabled: open });

    const handleLogout = () => {
        posthog.capture('user_logged_out');
        posthog.reset();

        // Drop every token the api client can still resolve. resolveAuthToken()
        // falls back to localStorage['auth_token'], which the Auth0 logout
        // redirect would otherwise leave behind.
        setAuthToken(null);
        try {
            localStorage.removeItem('auth_token');
        } catch {
            // Storage can be unavailable (private mode, blocked cookies); the
            // in-memory token is already cleared, so this is not worth failing on.
        }

        logout({ logoutParams: { returnTo: window.location.origin } });
    };

    const displayName =
        session.mode === 'disabled' ? 'Local development' : user?.name || 'Not signed in';

    return (
        <DropdownMenu open={open} onOpenChange={setOpen}>
            <DropdownMenuTrigger asChild>
                <Button
                    variant="ghost"
                    className="relative h-8 w-8 rounded-full p-0"
                    aria-label="Profile and session"
                >
                    <Avatar className="h-8 w-8">
                        {session.mode === 'auth0' && user?.picture && (
                            <AvatarImage src={user.picture} alt={user.name ?? 'Signed-in user'} />
                        )}
                        <AvatarFallback>
                            {session.mode === 'disabled' ? (
                                <Wrench className="h-4 w-4" aria-hidden="true" />
                            ) : session.mode === 'anonymous' ? (
                                <UserIcon className="h-4 w-4" aria-hidden="true" />
                            ) : (
                                user?.name?.charAt(0) || 'U'
                            )}
                        </AvatarFallback>
                    </Avatar>
                </Button>
            </DropdownMenuTrigger>

            <DropdownMenuContent className="w-72" align="end" forceMount>
                <DropdownMenuLabel className="font-normal">
                    <div className="flex flex-col space-y-1">
                        <p className="text-sm font-medium leading-none">{displayName}</p>
                        {session.mode === 'auth0' && user?.email && (
                            <p className="text-xs leading-none text-muted-foreground">
                                {user.email}
                            </p>
                        )}
                        {session.mode === 'disabled' && (
                            <p className="text-xs leading-none text-muted-foreground">
                                Auth0 is disabled
                            </p>
                        )}
                    </div>
                </DropdownMenuLabel>

                <DropdownMenuSeparator />

                <div className="space-y-2 px-2 py-1.5">
                    {session.subject && (
                        <div className="space-y-0.5">
                            <p className="text-[10px] uppercase tracking-wide text-muted-foreground">
                                Subject
                            </p>
                            <p
                                className="truncate font-mono text-xs"
                                title={session.subject}
                            >
                                {session.subject}
                            </p>
                        </div>
                    )}

                    {session.mode === 'disabled' && (
                        <p className="text-xs leading-snug text-muted-foreground">
                            NEXT_PUBLIC_AUTH0_ENABLED=false — the API is accepting the static
                            token, and every request is attributed to this subject.
                        </p>
                    )}

                    {session.mode === 'auth0' && <SessionDetail session={session} />}
                </div>

                {session.mode !== 'disabled' && <DropdownMenuSeparator />}

                {session.mode === 'auth0' && (
                    <DropdownMenuItem
                        onClick={handleLogout}
                        className="text-red-600 focus:text-red-700"
                    >
                        <LogOut className="mr-2 h-4 w-4" />
                        <span>Log out</span>
                    </DropdownMenuItem>
                )}

                {session.mode === 'anonymous' && (
                    <DropdownMenuItem onClick={() => loginWithRedirect()}>
                        <LogIn className="mr-2 h-4 w-4" />
                        <span>Log in</span>
                    </DropdownMenuItem>
                )}
            </DropdownMenuContent>
        </DropdownMenu>
    );
}

/** Permissions and expiry, read from the access token. */
function SessionDetail({ session }: { session: ReturnType<typeof useSessionClaims> }) {
    if (session.isLoading) {
        return <p className="text-xs text-muted-foreground">Reading session…</p>;
    }

    if (session.error) {
        return (
            <p className="flex items-start gap-1.5 text-xs text-muted-foreground">
                <TriangleAlert className="mt-0.5 h-3 w-3 shrink-0" aria-hidden="true" />
                <span>Could not read the access token: {session.error}</span>
            </p>
        );
    }

    const grants = session.permissions.length > 0 ? session.permissions : session.scopes;

    return (
        <>
            <div className="space-y-1">
                <p className="text-[10px] uppercase tracking-wide text-muted-foreground">
                    Permissions
                </p>
                {grants.length === 0 ? (
                    <p className="text-xs text-muted-foreground">
                        None carried by this token.
                    </p>
                ) : (
                    <ScrollArea className="max-h-24">
                        <div className="flex flex-wrap gap-1 pr-2">
                            {grants.map((grant) => (
                                <Badge key={grant} variant="secondary" className="font-mono">
                                    {grant}
                                </Badge>
                            ))}
                        </div>
                    </ScrollArea>
                )}
            </div>

            {session.expiresAt && (
                <p className="text-xs text-muted-foreground">
                    Token expires {formatDistanceToNow(session.expiresAt, { addSuffix: true })}
                </p>
            )}
        </>
    );
}
