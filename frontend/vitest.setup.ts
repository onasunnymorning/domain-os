import '@testing-library/jest-dom';
import { expect, afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';

// Cleanup after each test
afterEach(() => {
  cleanup();
});

// Extend Vitest's expect with jest-dom matchers
expect.extend({});

// Mock ResizeObserver
global.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};

// Polyfill localStorage for jsdom (ensures .clear() etc. work in all tests)
if (!global.localStorage || typeof global.localStorage.clear !== 'function') {
  const store: Record<string, string> = {};
  global.localStorage = {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => { store[key] = String(value); },
    removeItem: (key: string) => { delete store[key]; },
    clear: () => { Object.keys(store).forEach(k => delete store[k]); },
    get length() { return Object.keys(store).length; },
    key: (index: number) => Object.keys(store)[index] ?? null,
  } as Storage;
}

// Mock Auth0 globally so ProtectedRoute renders children instead of a spinner
vi.mock('@auth0/auth0-react', () => ({
  useAuth0: vi.fn(() => ({
    isAuthenticated: true,
    isLoading: false,
    user: { sub: 'test-user', name: 'Test User', email: 'test@example.com' },
    loginWithRedirect: vi.fn(),
    logout: vi.fn(),
    getAccessTokenSilently: vi.fn(),
  })),
  Auth0Provider: ({ children }: { children: React.ReactNode }) => children,
}));

// Mock Next.js app router hooks globally for component tests
vi.mock('next/navigation', () => {
  // Expose mock functions so tests can override return values if needed
  const useParams = vi.fn();
  const useRouter = vi.fn(() => ({
    push: vi.fn(),
    replace: vi.fn(),
    prefetch: vi.fn(),
    back: vi.fn(),
  }));
  const usePathname = vi.fn(() => '/');
  const useSearchParams = vi.fn(() => new URLSearchParams());

  return {
    useRouter,
    usePathname,
    useSearchParams,
    useParams,
  };
});
