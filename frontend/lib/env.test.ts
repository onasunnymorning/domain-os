import { afterEach, describe, expect, it, vi } from 'vitest';
import { formatAppVersion } from './env';

describe('formatAppVersion', () => {
  afterEach(() => vi.unstubAllEnvs());

  it('shows version and short SHA for a stamped release build', () => {
    vi.stubEnv('APP_BUILD_VERSION', '0.10.0');
    vi.stubEnv('APP_BUILD_COMMIT', 'a1b2c3d4e5f6a7b8c9d0');
    expect(formatAppVersion()).toBe('v0.10.0 · a1b2c3d');
  });

  it('shows "dev" when unstamped', () => {
    vi.stubEnv('APP_BUILD_VERSION', '');
    vi.stubEnv('APP_BUILD_COMMIT', '');
    expect(formatAppVersion()).toBe('dev');
  });

  it('ignores the Dockerfile "unknown" SHA default', () => {
    vi.stubEnv('APP_BUILD_VERSION', '0.10.0');
    vi.stubEnv('APP_BUILD_COMMIT', 'unknown');
    expect(formatAppVersion()).toBe('v0.10.0');
  });
});
