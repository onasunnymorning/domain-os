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

# Version and commit are properties of this artifact, not of the environment:
# next.config.ts inlines them into the bundle (lib/build-info.ts), so they cannot
# be overridden at runtime. Defaults make a local `docker build` show "dev".
# Declared late so a changed version only invalidates the build layer.
ARG VERSION=dev
ARG GIT_SHA=unknown
ENV APP_BUILD_VERSION=${VERSION} \
    APP_BUILD_COMMIT=${GIT_SHA}

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
