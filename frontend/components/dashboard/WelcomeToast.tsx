'use client';

import { useEffect } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { toast } from 'sonner';
import { isAuthEnabled } from '@/lib/env';

/**
 * WelcomeToast — fires a personalized greeting toast once per browser session.
 * Uses sessionStorage to avoid re-firing on page navigation.
 */
export function WelcomeToast() {
  const { user, isAuthenticated } = useAuth0();

  useEffect(() => {
    const authEnabled = isAuthEnabled();
    if (!authEnabled || !isAuthenticated || !user) return;

    const key = 'dashboard-welcomed';
    if (sessionStorage.getItem(key)) return;

    const name = user.given_name || user.name?.split(' ')[0] || 'there';

    // Small delay so the page renders first, then the toast slides in
    const timer = setTimeout(() => {
      toast(`Welcome back, ${name}`, {
        description: new Date().toLocaleDateString('en-US', {
          weekday: 'long',
          month: 'long',
          day: 'numeric',
        }),
        duration: 4000,
      });
      sessionStorage.setItem(key, '1');
    }, 600);

    return () => clearTimeout(timer);
  }, [isAuthenticated, user]);

  return null;
}
