/**
 * Build-time identity of this image. next.config.ts inlines these from the
 * APP_BUILD_VERSION / APP_BUILD_COMMIT build env (set from the release build's
 * VERSION / GIT_SHA args in frontend/Dockerfile), so they are constants in the
 * bundle, not runtime configuration. This is the only file allowed to read
 * process.env besides NODE_ENV; see the override in eslint.config.mjs.
 */
export const buildVersion = () => process.env.APP_BUILD_VERSION || 'dev';

/** Full git SHA; '' or 'unknown' when the build was not stamped. */
export const buildCommit = () => process.env.APP_BUILD_COMMIT || '';
