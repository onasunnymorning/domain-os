export const POSTHOG_ANALYTICS_DOC_MARKDOWN = `# PostHog Analytics

PostHog provides **product analytics**, **session recordings**, **error tracking**, and **autocapture** for the registry administration UI. It answers the question: *how are operators actually using this tool?*

---

## Architecture

\`\`\`
┌─────────────────────────┐       ┌──────────────────────────┐
│   Next.js Frontend      │       │     PostHog Cloud (US)   │
│                         │       │                          │
│  instrumentation-client │──────▶│  Events & Sessions       │
│        /ingest/*        │ proxy │  Dashboards & Insights   │
│                         │       │  Error Tracking          │
└─────────────────────────┘       └──────────────────────────┘
\`\`\`

### How It Works

1. **Client init** — \`instrumentation-client.ts\` initializes the PostHog SDK using the Next.js 15.3+ client instrumentation hook. This runs once on app load.
2. **Reverse proxy** — All PostHog requests are routed through \`/ingest\` via Next.js rewrites in \`next.config.ts\`. This avoids ad-blockers and keeps analytics reliable.
3. **User identification** — When a user authenticates via Auth0, \`posthog.identify()\` is called with their Auth0 \`sub\`, email, and name (in \`token-sync.tsx\`).
4. **Autocapture** — Clicks, form submissions, and input changes are automatically captured with element metadata. No manual instrumentation needed.
5. **Session recordings** — Full session replays are captured, enabling visual analysis of user journeys.

---

## Event Inventory

### Registrar Events

| Event | Trigger | Properties |
|---|---|---|
| \`registrar_created\` | New registrar successfully created | \`clid\`, \`name\` |
| \`registrar_updated\` | Registrar details saved | \`clid\`, \`name\` |
| \`tld_accredited_to_registrar\` | TLD accreditation added to a registrar | \`clid\`, \`tld\` |
| \`tld_deaccredited_from_registrar\` | TLD accreditation removed from a registrar | \`clid\`, \`tld\` |

### TLD Events

| Event | Trigger | Properties |
|---|---|---|
| \`tld_updated\` | TLD settings saved | \`tld\`, fields changed |
| \`tld_deleted\` | TLD deletion workflow triggered | \`tld\` |
| \`registrar_accredited_to_tld\` | Registrar accredited to a TLD | \`tld\`, \`clid\` |
| \`registrar_deaccredited_from_tld\` | Registrar deaccredited from a TLD | \`tld\`, \`clid\` |

### Domain Events

| Event | Trigger | Properties |
|---|---|---|
| \`domain_created\` | New domain created under a TLD | \`domain_name\`, \`tld\` |

### Workflow Events

| Event | Trigger | Properties |
|---|---|---|
| \`workflow_launched\` | Workflow launched from the launchpad | \`workflow_key\`, \`workflow_name\`, \`workflow_id\` |
| \`workflow_signal_sent\` | Approve/reject signal sent to a running workflow | \`workflow_id\`, \`signal_type\` |
| \`workflow_artifact_downloaded\` | Workflow artifact (QA report, database, etc.) downloaded | \`workflow_id\`, \`artifact_name\` |

### Session Events

| Event | Trigger | Properties |
|---|---|---|
| \`user_logged_out\` | User clicked logout in the user menu | — |

---

## Error Tracking

All create, update, and launch operations include \`posthog.captureException()\` in their error paths. This means failed API calls, validation errors, and unexpected exceptions are automatically reported to PostHog's error tracking.

Errors appear in the **Error Tracking** section of the PostHog dashboard with full stack traces (when source maps are uploaded).

---

## Dashboards & Insights

Pre-built dashboards created during setup:

- **Analytics basics** — Overview of event volume, active users, and page views
- **Registry actions over time** — Trend chart of create/update/delete operations
- **Workflow launch to approval funnel** — Conversion funnel from launch → signal
- **Accreditation activity** — TLD ↔ Registrar accreditation changes over time
- **Workflow launches by type** — Breakdown of which workflows are used most
- **Registrar & TLD edit activity** — Edit frequency across entities

---

## Configuration

### Environment Variables

| Variable | Purpose | Example |
|---|---|---|
| \`NEXT_PUBLIC_POSTHOG_PROJECT_TOKEN\` | PostHog project API key (write-only, safe for client) | \`phc_xxxxx\` |
| \`NEXT_PUBLIC_POSTHOG_HOST\` | PostHog ingest endpoint | \`https://us.i.posthog.com\` |

These are set in:
- \`.env.local\` for local development
- **Container environment** in every deployed environment

They are read at container start via \`getPostHogToken()\` / \`getPostHogHost()\` in
\`lib/env.ts\`, not baked into the bundle.
When the token is absent at runtime, \`instrumentation-client.ts\` skips \`posthog.init()\` entirely.

> **Note:** The API key is a write-only ingest key. It is intentionally readable by the browser — this is the standard PostHog pattern.

### Reverse Proxy (Ad-blocker Bypass)

PostHog requests are proxied through the app's own domain via Next.js rewrites:

| Path | Destination |
|---|---|
| \`/ingest/static/*\` | \`https://us-assets.i.posthog.com/static/*\` |
| \`/ingest/array/*\` | \`https://us-assets.i.posthog.com/array/*\` |
| \`/ingest/*\` | \`https://us.i.posthog.com/*\` |

This ensures analytics data is collected even when users have ad-blockers enabled.

---

## Key Files

| File | Purpose |
|---|---|
| \`instrumentation-client.ts\` | PostHog SDK initialization (Next.js 15.3+ pattern) |
| \`next.config.ts\` | Reverse proxy rewrites for \`/ingest\` |
| \`components/providers/token-sync.tsx\` | \`posthog.identify()\` on Auth0 authentication |
| \`components/auth/user-menu.tsx\` | \`posthog.reset()\` + logout event on sign-out |

---

## Adding New Events

To instrument a new user action:

\`\`\`typescript
import posthog from 'posthog-js';

// In your success handler:
posthog.capture('event_name', {
  property_key: 'value',
});

// In your error handler:
posthog.captureException(error);
\`\`\`

**Naming convention:** Use \`snake_case\` for event names. Prefix with the entity type when applicable (e.g., \`domain_created\`, \`registrar_updated\`, \`workflow_launched\`).
`;
