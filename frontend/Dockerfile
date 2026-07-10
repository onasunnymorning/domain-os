# --- Stage 1: Install dependencies ---
FROM node:20-alpine AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

COPY package.json package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

# --- Stage 2: Build the application ---
FROM node:20-alpine AS builder
WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# No NEXT_PUBLIC_* build args: they resolve at runtime via next-runtime-env,
# so this image is environment-agnostic. See docs/adr/0001-runtime-env-configuration.md

# Build without --turbopack (unsupported in production for Next.js 15.5.x)
ENV __NEXT_CONTINUE_EXPERIMENTAL=1
RUN npx next build

# --- Stage 3: Production runner ---
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production

# Patch base-image OS packages, and drop the bundled npm CLI: the runtime only
# executes `node server.js`, and npm's bundled deps (tar, minimatch, cross-spawn,
# sigstore, …) are the sole source of HIGH CVEs in this image.
RUN apk upgrade --no-cache && \
    rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx

# App version is genuinely build-time metadata. Stamping it as a runtime ENV in
# this stage lets PublicEnvScript pick it up like any other NEXT_PUBLIC_* var,
# while still allowing an override with `docker run -e`.
ARG NEXT_PUBLIC_APP_VERSION="dev"
ENV NEXT_PUBLIC_APP_VERSION=$NEXT_PUBLIC_APP_VERSION

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# Copy static assets and standalone server
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
