# ADR 0001 — Runtime environment configuration for the frontend

- **Status:** Accepted
- **Date:** 2026-07-09
- **Deciders:** Frontend / Platform
- **Supersedes:** none

## Context

The frontend is a Next.js App Router application shipped as `geapex/domain-os-frontend`.
Next.js replaces every `process.env.NEXT_PUBLIC_*` reference with a string literal
at `next build` time. Any value read that way is therefore frozen into the JS
bundle when the image is built.

This had three consequences.

**The image was not portable.** `deploy/contract.json` advertised nine
`NEXT_PUBLIC_*` variables for the `frontend` service, but infra could not set any
of them at deploy time — by then the values were already compiled in. Promoting a
tested image from staging to production was impossible; each environment needed
its own build of the same commit.

**The published image was misconfigured.** Neither `.github/workflows/ci.yaml` nor
`release.yaml` ever passed `NEXT_PUBLIC_*` build args to the frontend image build.
Every published image was therefore built with those variables unset and fell back
to its local-development defaults — notably `NEXT_PUBLIC_API_URL` →
`http://localhost:8080`. The deployed frontend could only work because something
downstream overrode the origin.

**The contract could not express the difference.** `serviceMeta.BuildTime` existed
in `internal/config/contract.go` but was never read by `GenerateContract()`, so it
never reached `contract.json`. Infra consumers had no machine-readable signal that
the frontend's variables behaved differently from every other service's.

## Decision

Adopt [`next-runtime-env`](https://github.com/expatfile/next-runtime-env) and read
every public variable at runtime.

1. `<PublicEnvScript disableNextScript />` renders in `<head>` of `app/layout.tsx`.
   On each request it serialises every `NEXT_PUBLIC_*` variable found in the
   server's `process.env` into `window.__ENV`.
2. All reads go through `frontend/lib/env.ts`, the single sanctioned boundary to
   the environment. It exposes one **function** per variable — never a `const`,
   because a module-scope read would capture the value once per process and, in a
   server component, evaluate outside a request scope.
3. `process.env` is banned in the frontend by an ESLint `no-restricted-syntax`
   rule. `process.env.NODE_ENV` is the sole exception: it is a build-time
   constant, not runtime configuration.
4. `frontend/Dockerfile` declares no `NEXT_PUBLIC_*` build args. Infra sets plain
   container environment variables.
5. `ServiceContract.EnvInjection` (`"runtime"` | `"build"`) is emitted into
   `contract.json` for every service, making the distinction machine-readable.

### Why `disableNextScript`

`next-runtime-env` defaults to `next/script` with `strategy="beforeInteractive"`.
That does **not** guarantee execution before `instrumentation-client.ts`, which
initialises PostHog at module scope — the library's own source documents this
failure mode for Sentry. `disableNextScript` emits a plain inline `<script>`,
which the browser executes in document order, before any bundle chunk. Without it,
`window.__ENV` is `undefined` when PostHog initialises.

### Why `NEXT_PUBLIC_APP_VERSION` is still stamped at build

The app version is genuinely build-time metadata; a runtime-overridable version
string would be a lie waiting to happen. It is set as an `ENV` in the Dockerfile's
**runner** stage (not the builder), so `PublicEnvScript` picks it up like any other
runtime variable while still being overridable with `docker run -e` for debugging.

### Why the `NEXT_PUBLIC_` prefix is retained

`PublicEnvScript` only exposes variables carrying that prefix. Keeping it means no
renames in Doppler, the Tiltfile, or any infra repo consuming `contract.json`.

## Consequences

### Positive

- One image runs in every environment. Promote the artifact, not the commit.
- Changing an environment variable is a container restart, not a rebuild.
- `contract.json` now tells infra *how* to inject each service's variables.
- Adding a variable is one entry in `lib/env.ts` plus one in `env_registry.go`.

### Negative

- **Every route is now dynamic.** `PublicEnvScript` calls `unstable_noStore()`,
  which opts the root layout — and therefore the whole app — out of static
  prerendering. The build output reports every route as `ƒ (Dynamic)`. This is
  acceptable here: the app is an authenticated internal dashboard whose pages are
  already client components fetching live data. It would not be acceptable for a
  public, cacheable, statically-generated site.
- A small inline `<script>` is added to every HTML response.
- Tests must mock `next-runtime-env`, since `window.__ENV` is only populated by a
  real page render. `vitest.setup.ts` maps `env()` back onto `process.env` so
  `vi.stubEnv` keeps working.
- `next-runtime-env@3.3.0` declares `next` and `react` as *both* `dependencies`
  and `peerDependencies`, so npm installs a nested `next@14` (and `react@18`)
  tree. That nested copy is never bundled, but it carries CVEs that `npm audit`
  and the Trivy image scan in `make ci-local` will flag. An `overrides` entry in
  `frontend/package.json` pins the nested `next`/`react` to our top-level
  versions, which removes the nested tree entirely. **Do not remove that
  override** — `npm audit` goes from 0 vulnerabilities to 2 high without it.

### Security

`NEXT_PUBLIC_*` values are readable by anyone with a browser — before this change
by reading the JS bundle, after it by reading `window.__ENV`. The exposure is
unchanged. Accordingly `isSecret()` in `contract.go` classifies every
`NEXT_PUBLIC_*` variable as non-secret, and the Cloud Infrastructure page no
longer displays a 🔒 badge next to `NEXT_PUBLIC_API_TOKEN`: marking it "secret"
implied a protection that never existed. `NEXT_PUBLIC_API_TOKEN` must be a
low-privilege token, and nothing genuinely secret may ever carry the prefix.

## Alternatives considered

**Hand-rolled `window.__ENV` injection.** The mechanism is perhaps thirty lines: a
server component that serialises the public env into an inline script, plus a
reader. This avoids the dependency and its packaging bug. Rejected because
`next-runtime-env` also handles the nonce/CSP path and the `beforeInteractive`
ordering trap, both of which we would have had to rediscover. Revisit if the
upstream `dependencies`/`peerDependencies` bug is not fixed.

**Runtime config endpoint.** Fetch `/api/config` from the client on boot. Rejected:
it adds a render-blocking round trip and leaves the server render without values.

**Keep build-time injection, build once per environment.** Rejected: it defeats
artifact promotion and means the tested image is never the deployed image.

## References

- Accessors: `frontend/lib/env.ts`
- Injection point: `frontend/app/layout.tsx`
- Lint rule: `frontend/eslint.config.mjs`
- Contract source: `internal/config/contract.go`, `internal/config/env_registry.go`
- Generated contract: `deploy/contract.json`
- In-app guide: `/docs/runtime-env`
