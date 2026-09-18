import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useAuth0 } from '@auth0/auth0-react';
import { UserMenu } from '../user-menu';

// ── posthog ───────────────────────────────────────────────────────────────────
vi.mock('posthog-js', () => ({
  default: { capture: vi.fn(), reset: vi.fn() },
}));

// ── Auth0 (the global mock from vitest.setup.ts, overridden per test) ─────────
const mockLogout = vi.fn();
const mockLoginWithRedirect = vi.fn();
const mockGetAccessTokenSilently = vi.fn();

const AUTH0_USER = {
  sub: 'auth0|abc123',
  name: 'Ada Lovelace',
  email: 'ada@example.com',
  picture: 'https://example.com/ada.png',
};

function setAuth0(overrides: Record<string, unknown> = {}) {
  vi.mocked(useAuth0).mockReturnValue({
    isAuthenticated: true,
    isLoading: false,
    user: AUTH0_USER,
    logout: mockLogout,
    loginWithRedirect: mockLoginWithRedirect,
    getAccessTokenSilently: mockGetAccessTokenSilently,
    ...overrides,
  } as never);
}

/** Builds an unsigned, structurally valid JWT around the given payload. */
function makeToken(payload: Record<string, unknown>): string {
  const b64url = (value: object) =>
    btoa(JSON.stringify(value)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `${b64url({ alg: 'RS256' })}.${b64url(payload)}.signature-not-verified`;
}

async function openMenu() {
  await userEvent.click(screen.getByRole('button', { name: /profile and session/i }));
}

describe('UserMenu', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetAccessTokenSilently.mockResolvedValue(makeToken({ sub: AUTH0_USER.sub }));
    setAuth0();
    localStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  // ── Authenticated ──────────────────────────────────────────────────────────
  describe('when signed in through Auth0', () => {
    it('shows the identity and a Log out item', async () => {
      render(<UserMenu />);
      await openMenu();

      expect(await screen.findByText('Ada Lovelace')).toBeInTheDocument();
      expect(screen.getByText('ada@example.com')).toBeInTheDocument();
      expect(screen.getByText('auth0|abc123')).toBeInTheDocument();
      expect(screen.getByRole('menuitem', { name: /log out/i })).toBeInTheDocument();
    });

    it('logs out via Auth0 and returns to the origin', async () => {
      render(<UserMenu />);
      await openMenu();

      await userEvent.click(await screen.findByRole('menuitem', { name: /log out/i }));

      expect(mockLogout).toHaveBeenCalledWith({
        logoutParams: { returnTo: window.location.origin },
      });
    });

    it('clears the localStorage token the api client would otherwise still resolve', async () => {
      localStorage.setItem('auth_token', 'stale-token');

      render(<UserMenu />);
      await openMenu();
      await userEvent.click(await screen.findByRole('menuitem', { name: /log out/i }));

      expect(localStorage.getItem('auth_token')).toBeNull();
    });

    it('renders permissions decoded from the access token', async () => {
      mockGetAccessTokenSilently.mockResolvedValue(
        makeToken({
          sub: AUTH0_USER.sub,
          permissions: ['escrow:keys:admin', 'escrow:platform-keys:admin'],
        })
      );

      render(<UserMenu />);
      await openMenu();

      expect(await screen.findByText('escrow:keys:admin')).toBeInTheDocument();
      expect(screen.getByText('escrow:platform-keys:admin')).toBeInTheDocument();
    });

    it('falls back to scopes when the token carries no RBAC permissions', async () => {
      mockGetAccessTokenSilently.mockResolvedValue(
        makeToken({ sub: AUTH0_USER.sub, scope: 'openid profile' })
      );

      render(<UserMenu />);
      await openMenu();

      expect(await screen.findByText('openid')).toBeInTheDocument();
      expect(screen.getByText('profile')).toBeInTheDocument();
    });

    it('reports a token that could not be read instead of rendering nothing', async () => {
      mockGetAccessTokenSilently.mockRejectedValue(new Error('login_required'));

      render(<UserMenu />);
      await openMenu();

      expect(await screen.findByText(/login_required/)).toBeInTheDocument();
      // The way out must survive a token failure.
      expect(screen.getByRole('menuitem', { name: /log out/i })).toBeInTheDocument();
    });

    it('does not request an access token until the menu is opened', async () => {
      render(<UserMenu />);

      expect(mockGetAccessTokenSilently).not.toHaveBeenCalled();

      await openMenu();
      await waitFor(() => expect(mockGetAccessTokenSilently).toHaveBeenCalled());
    });
  });

  // ── Auth disabled (the local dev default) ──────────────────────────────────
  describe('when NEXT_PUBLIC_AUTH0_ENABLED=false', () => {
    beforeEach(() => {
      vi.stubEnv('NEXT_PUBLIC_AUTH0_ENABLED', 'false');
      // Auth0 never authenticates in this mode.
      setAuth0({ isAuthenticated: false, user: undefined });
    });

    it('still renders a profile element, naming the static-token subject', async () => {
      render(<UserMenu />);
      await openMenu();

      expect(await screen.findByText('Local development')).toBeInTheDocument();
      expect(screen.getByText('legacy-admin')).toBeInTheDocument();
      expect(screen.getByText(/NEXT_PUBLIC_AUTH0_ENABLED=false/)).toBeInTheDocument();
    });

    it('offers neither log out nor log in — there is no session to end', async () => {
      render(<UserMenu />);
      await openMenu();

      await screen.findByText('Local development');
      expect(screen.queryByRole('menuitem', { name: /log out/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('menuitem', { name: /log in/i })).not.toBeInTheDocument();
    });

    it('does not request an access token', async () => {
      render(<UserMenu />);
      await openMenu();

      await screen.findByText('Local development');
      expect(mockGetAccessTokenSilently).not.toHaveBeenCalled();
    });
  });

  // ── Auth enabled, not signed in ────────────────────────────────────────────
  describe('when auth is enabled but nobody is signed in', () => {
    beforeEach(() => {
      setAuth0({ isAuthenticated: false, user: undefined });
    });

    it('offers a Log in item', async () => {
      render(<UserMenu />);
      await openMenu();

      expect(await screen.findByText('Not signed in')).toBeInTheDocument();

      await userEvent.click(screen.getByRole('menuitem', { name: /log in/i }));
      expect(mockLoginWithRedirect).toHaveBeenCalledTimes(1);
    });
  });
});
