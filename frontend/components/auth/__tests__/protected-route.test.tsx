import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { ProtectedRoute } from '../protected-route';

// ── Auth0 mock ────────────────────────────────────────────────────────────────
const mockLoginWithRedirect = vi.fn();
const mockAuth0State = {
  isAuthenticated: false,
  isLoading: false,
  loginWithRedirect: mockLoginWithRedirect,
};

vi.mock('@auth0/auth0-react', () => ({
  useAuth0: () => mockAuth0State,
}));

// ── Helpers ───────────────────────────────────────────────────────────────────
function setAuth(overrides: Partial<typeof mockAuth0State>) {
  Object.assign(mockAuth0State, overrides);
}

function setUrl(search: string) {
  Object.defineProperty(window, 'location', {
    writable: true,
    value: { ...window.location, search },
  });
}

// ── Tests ─────────────────────────────────────────────────────────────────────
describe('ProtectedRoute', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: auth enabled, no callback params in URL
    setUrl('');
    setAuth({ isAuthenticated: false, isLoading: false });
  });

  afterEach(() => {
    setUrl('');
  });

  it('renders children when authenticated', () => {
    setAuth({ isAuthenticated: true, isLoading: false });

    render(
      <ProtectedRoute>
        <div data-testid="child">Protected Content</div>
      </ProtectedRoute>
    );

    expect(screen.getByTestId('child')).toBeInTheDocument();
    expect(mockLoginWithRedirect).not.toHaveBeenCalled();
  });

  it('shows a loading spinner while Auth0 is initialising', () => {
    setAuth({ isAuthenticated: false, isLoading: true });

    render(
      <ProtectedRoute>
        <div data-testid="child">Protected Content</div>
      </ProtectedRoute>
    );

    // Children must not be visible during loading
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
    // Spinner element is rendered
    expect(document.querySelector('.animate-spin')).toBeInTheDocument();
  });

  it('calls loginWithRedirect when unauthenticated and not loading', async () => {
    setAuth({ isAuthenticated: false, isLoading: false });

    render(
      <ProtectedRoute>
        <div data-testid="child">Protected Content</div>
      </ProtectedRoute>
    );

    await waitFor(() => expect(mockLoginWithRedirect).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
  });

  // ── Login loop regression ──────────────────────────────────────────────────
  describe('login loop prevention (Auth0 callback URL)', () => {
    it('does NOT call loginWithRedirect when code + state params are present', async () => {
      setAuth({ isAuthenticated: false, isLoading: false });
      setUrl('?code=abc123&state=xyz');

      render(
        <ProtectedRoute>
          <div data-testid="child">Protected Content</div>
        </ProtectedRoute>
      );

      // Give effects a chance to fire
      await new Promise((r) => setTimeout(r, 50));

      expect(mockLoginWithRedirect).not.toHaveBeenCalled();
    });

    it('calls loginWithRedirect when only "code" is present (not a valid callback)', async () => {
      setAuth({ isAuthenticated: false, isLoading: false });
      setUrl('?code=abc123'); // no "state" → not a real Auth0 callback

      render(
        <ProtectedRoute>
          <div data-testid="child">Protected Content</div>
        </ProtectedRoute>
      );

      await waitFor(() => expect(mockLoginWithRedirect).toHaveBeenCalledTimes(1));
    });

    it('calls loginWithRedirect when only "state" is present (not a valid callback)', async () => {
      setAuth({ isAuthenticated: false, isLoading: false });
      setUrl('?state=xyz'); // no "code" → not a real Auth0 callback

      render(
        <ProtectedRoute>
          <div data-testid="child">Protected Content</div>
        </ProtectedRoute>
      );

      await waitFor(() => expect(mockLoginWithRedirect).toHaveBeenCalledTimes(1));
    });

    it('renders children (not redirect) once authenticated after callback completes', async () => {
      // Simulate Auth0 having processed the callback and now authenticated
      setAuth({ isAuthenticated: true, isLoading: false });
      setUrl(''); // callback params would already be stripped by Auth0 SDK

      render(
        <ProtectedRoute>
          <div data-testid="child">Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.getByTestId('child')).toBeInTheDocument();
      expect(mockLoginWithRedirect).not.toHaveBeenCalled();
    });
  });

  // ── Auth disabled ──────────────────────────────────────────────────────────
  describe('when NEXT_PUBLIC_AUTH0_ENABLED=false', () => {
    beforeEach(() => {
      vi.stubEnv('NEXT_PUBLIC_AUTH0_ENABLED', 'false');
    });

    afterEach(() => {
      vi.unstubAllEnvs();
    });

    it('renders children without any auth check', () => {
      setAuth({ isAuthenticated: false, isLoading: false });

      render(
        <ProtectedRoute>
          <div data-testid="child">Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.getByTestId('child')).toBeInTheDocument();
      expect(mockLoginWithRedirect).not.toHaveBeenCalled();
    });
  });
});
