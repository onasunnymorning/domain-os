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
