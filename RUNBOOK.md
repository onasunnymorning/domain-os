# RUNBOOK — AlpacaNames TEST Deploy (Render + Neon)

Manual deploy steps for the TEST environment. Follow in order.

> **Region**: Both Neon and Render must be in **Oregon (us-west-2)** to minimize
> network latency. Neon's free tier defaults to `us-west-2`.

---

## Prerequisites

- A [Neon](https://neon.tech) account
- A [Render](https://render.com) account with this repo connected
- Git — the `deploy/test-render-neon` branch pushed to the remote

---

## Step 1 — Create the Neon project and TEST branch

1. Go to [Neon Console](https://console.neon.tech/) → **New Project**
2. **Project name**: `alpaca`
3. **Region**: `US West (Oregon)` — must match Render's `oregon` region
4. **Database name**: `domainos`
5. **Role**: leave the default (usually `neondb_owner` or your email)
6. Click **Create Project**
7. Go to **Branches** → **New Branch**
8. **Branch name**: `test`
9. **Parent branch**: `main` (the default branch created with the project)
10. Click **Create Branch**
11. Go to the `test` branch → **Connection Details** → copy the **connection string**. It looks like:
    ```
    postgresql://neondb_owner:abc123@ep-cool-name-12345.us-west-2.aws.neon.tech/domainos?sslmode=require
    ```
    Each branch has its own compute endpoint, so the connection string is branch-specific.
    You'll use this as `DATABASE_URL` in the next step.

---

## Step 2 — Deploy to Render via Blueprint

1. Push the branch to your remote:
   ```bash
   git push origin deploy/test-render-neon
   ```
2. Go to [Render Dashboard](https://dashboard.render.com/) → click **+ New** (top-right) → **Blueprint**
3. Connect your GitHub repo if not already connected
4. Select the repo and set the branch to `deploy/test-render-neon`
5. Render reads `render.yaml` and shows two services to create:
   - `alp-api-test` (Web Service — Go API)
   - `alp-ui-test` (Web Service — Next.js Frontend)
6. Click **Apply** to create both services

---

## Step 3 — Set environment variables in Render

For each service, go to **Environment** → **Edit** and paste the block below.

### API service (`alp-api-test`)

```env
DATABASE_URL=<paste Neon connection string from Step 1>
ADMIN_TOKEN=<generate with: openssl rand -hex 32>
API_PORT=8080
API_HOST=0.0.0.0
API_NAME=AlpacaNames TEST API
API_VERSION=0.1.0-test
GIN_MODE=release
AUTO_MIGRATE=true
CORS_ALLOWED_ORIGINS=https://alp-ui-test.onrender.com
AUTH0_ENABLED=false
NEW_RELIC_ENABLED=false
PROMETHEUS_ENABLED=false
```

### Frontend service (`alp-ui-test`)

```env
NEXT_PUBLIC_API_URL=https://alp-api-test.onrender.com
NEXT_PUBLIC_AUTH0_ENABLED=false
NEXT_PUBLIC_APP_VERSION=0.1.0-test
```

> **Note**: The `render.yaml` Blueprint also sets some of these env vars as
> defaults. Values you paste here will override them.

---

## Step 4 — Trigger builds

After setting the secrets:

1. Go to `alp-api-test` → **Manual Deploy** → **Deploy latest commit**
2. Wait for the API build to complete and become healthy (check the `/ping` health check in the Render logs)
3. Go to `alp-ui-test` → **Manual Deploy** → **Deploy latest commit**
4. Wait for the frontend build to complete

> The API must be deployed first so it's reachable when the frontend build
> inlines `NEXT_PUBLIC_API_URL`. However, since the URL is deterministic
> (`https://alp-api-test.onrender.com`), both can technically build in parallel.

---

## Step 5 — Migrations

Migrations run automatically on API startup via GORM AutoMigrate when
`AUTO_MIGRATE=true` (set in the Blueprint). No manual migration step needed.

Check the Render logs for `alp-api-test` — you should see:
```
Auto migrating database
```

If you see errors, check:
- `DATABASE_URL` is correct and the Neon project is accessible
- The Neon database name matches (should be `domainos`)

---

## Smoke Checks

### 1. API health

```bash
curl https://alp-api-test.onrender.com/ping
```

Expected:
```json
{"message":"pong"}
```

### 2. UI loads

Open in a browser:
```
https://alp-ui-test.onrender.com
```

The page should load without errors.

### 3. End-to-end: UI calls API

1. Open `https://alp-ui-test.onrender.com`
2. Navigate to the **Registry Operators** page (or any data page)
3. The page should show data fetched from the API (or an empty list if the DB is fresh)
4. Open browser DevTools → Network tab → verify requests go to
   `https://alp-api-test.onrender.com` and return 200 (or 401 if auth is required)

If you see CORS errors:
- Verify `CORS_ALLOWED_ORIGINS` on `alp-api-test` is set to `https://alp-ui-test.onrender.com`
- Redeploy the API after changing the env var

### 4. API with auth token

```bash
# Use the ADMIN_TOKEN you set in Render
curl -H "Authorization: Bearer <token>" \
  https://alp-api-test.onrender.com/registry-operators
```

Expected: `200` with a JSON response (empty array `[]` is fine on a fresh DB).

---

## Troubleshooting

| Symptom | Check |
|---------|-------|
| API won't start | Render logs → look for DB connection errors. Verify `DATABASE_URL`. |
| Frontend shows blank page | Browser DevTools → Console. Usually a build-time env var issue. |
| CORS errors | `CORS_ALLOWED_ORIGINS` must exactly match the frontend URL (including `https://`). |
| 401 on API calls | Set `ADMIN_TOKEN` and use it in the `Authorization: Bearer` header. With `AUTH0_ENABLED=false`, the legacy token auth is active. |
| Slow cold starts | Render Starter plan has cold starts (~30s). First request after idle will be slow. This is expected for TEST. |

---

## Tear Down

To remove the TEST environment:

1. **Render**: Dashboard → each service → Settings → Delete Service
2. **Neon**: Console → Project `alpaca` → Branches → `test` → Delete Branch
   (Keep the project and `main` branch for future environments)
