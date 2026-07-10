export const RUNTIME_ENV_DOC_MARKDOWN = `# Runtime Environment Variables

The frontend reads every \`NEXT_PUBLIC_*\` variable **at container start**, not at
build time. One image runs in every environment: promote the artifact, don't
rebuild the commit.

> **Never read \`process.env\` in frontend code.** An ESLint rule fails the build if
> you do. Add an accessor to \`lib/env.ts\` instead. The only exception is
> \`process.env.NODE_ENV\`, which is a build-time constant rather than configuration.

## How it works

\`\`\`mermaid
flowchart TD
    A[Container starts with env vars] --> B[Next.js server process.env]
    B --> C["PublicEnvScript renders in &lt;head&gt;"]
    C --> D["Inline script sets window.__ENV"]
    D --> E["env('NEXT_PUBLIC_X') in the browser"]
    B --> F["env('NEXT_PUBLIC_X') on the server"]
    E --> G[lib/env.ts accessor]
    F --> G
    G --> H[Component]
\`\`\`

On every request, \`<PublicEnvScript />\` in \`app/layout.tsx\` collects each
\`NEXT_PUBLIC_*\` variable from the server's environment and serialises it into an
inline \`<script>\` as \`window.__ENV\`. The \`env()\` helper reads \`window.__ENV\` in
the browser and \`process.env\` on the server.

## Adding a new variable

Four steps, all required:

| # | File | What to add |
|---|------|-------------|
| 1 | \`frontend/lib/env.ts\` | An accessor function, e.g. \`export const getFooUrl = () => env('NEXT_PUBLIC_FOO_URL') \\|\\| '';\` |
| 2 | \`internal/config/env_registry.go\` | A \`ServiceFrontend\` entry with a description and default |
| 3 | \`deploy/contract.json\` | Regenerate with \`make generate-contract\` — never edit by hand |
| 4 | \`frontend/app/cloud/page.tsx\` | A row in \`envVarsByCategory\` so operators can find it |

Also add it to \`example.env\`, and to \`frontend/.env.local\` if local dev needs a value.

The CI check \`make ci-envcheck\` fails if \`contract.json\` drifts from the registry.

## Rules

**Accessors are functions, never constants.**

\`\`\`ts
// Wrong — captured once per process; in a server component this evaluates
// outside a request scope.
export const API_URL = env('NEXT_PUBLIC_API_URL');

// Right
export const getApiUrl = () => env('NEXT_PUBLIC_API_URL') || 'http://localhost:8080';
\`\`\`

The same applies to callers: read the value **inside** a component or handler, not
at module scope.

\`\`\`tsx
// Wrong — module scope
const links = [{ href: getTemporalUiUrl() }];

// Right — per render
export function Header() {
  const temporalUiUrl = getTemporalUiUrl();
  // ...
}
\`\`\`

**Nothing secret may carry the \`NEXT_PUBLIC_\` prefix.** Every such value is
serialised into the HTML of every response and is readable by anyone with a
browser. \`NEXT_PUBLIC_API_TOKEN\` is a low-privilege fallback used only when Auth0
is disabled — it is not a secret, and the deployment contract classifies it as
non-secret deliberately.

## Deployment

Infra sets plain container environment variables. The frontend image declares no
\`NEXT_PUBLIC_*\` build args.

\`\`\`bash
docker run -e NEXT_PUBLIC_API_URL=https://api.example.com geapex/domain-os-frontend
\`\`\`

\`deploy/contract.json\` marks each service with \`env_injection\`. Every service is
currently \`"runtime"\`. A service marked \`"build"\` would require its variables as
\`docker build --build-arg\` instead.

\`NEXT_PUBLIC_APP_VERSION\` is the one variable stamped during the image build (in
the runner stage, as an \`ENV\`), because the app version is genuinely build-time
metadata. It can still be overridden at runtime.

## Trade-off: every route is dynamic

\`PublicEnvScript\` calls \`unstable_noStore()\`, which opts the root layout — and so
the entire app — out of static prerendering. \`next build\` reports every route as
\`ƒ (Dynamic)\`.

This is deliberate and acceptable: this is an authenticated internal dashboard
whose pages are already client components fetching live data. Do not copy this
pattern into a public, cacheable, statically-generated site.

## Testing

\`vitest.setup.ts\` mocks \`next-runtime-env\` so \`env()\` reads \`process.env\`. This
keeps \`vi.stubEnv('NEXT_PUBLIC_AUTH0_ENABLED', 'false')\` working. Without the mock,
\`env()\` would look for \`window.__ENV\` in jsdom and throw.

## Reference

- Decision record: \`docs/adr/0001-runtime-env-configuration.md\`
- Accessors: \`frontend/lib/env.ts\`
- Injection: \`frontend/app/layout.tsx\`
- Lint rule: \`frontend/eslint.config.mjs\`
`;
