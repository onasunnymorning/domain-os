# --- Stage 1: Install dependencies ---
FROM node:18-alpine AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

# --- Stage 2: Build the application ---
FROM node:18-alpine AS builder
WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# NEXT_PUBLIC_* vars must be available at build time
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_AUTH0_ENABLED=false
ARG NEXT_PUBLIC_AUTH0_DOMAIN=""
ARG NEXT_PUBLIC_AUTH0_CLIENT_ID=""
ARG NEXT_PUBLIC_AUTH0_AUDIENCE=""
ARG NEXT_PUBLIC_APP_VERSION="0.1.0"
ARG NEXT_PUBLIC_API_TOKEN=""
ARG NEXT_PUBLIC_TEMPORAL_UI_URL=""
ARG NEXT_PUBLIC_STORAGE_UI_URL=""
ARG NEXT_PUBLIC_GRAFANA_URL=""

ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_AUTH0_ENABLED=$NEXT_PUBLIC_AUTH0_ENABLED
ENV NEXT_PUBLIC_AUTH0_DOMAIN=$NEXT_PUBLIC_AUTH0_DOMAIN
ENV NEXT_PUBLIC_AUTH0_CLIENT_ID=$NEXT_PUBLIC_AUTH0_CLIENT_ID
ENV NEXT_PUBLIC_AUTH0_AUDIENCE=$NEXT_PUBLIC_AUTH0_AUDIENCE
ENV NEXT_PUBLIC_APP_VERSION=$NEXT_PUBLIC_APP_VERSION
ENV NEXT_PUBLIC_API_TOKEN=$NEXT_PUBLIC_API_TOKEN
ENV NEXT_PUBLIC_TEMPORAL_UI_URL=$NEXT_PUBLIC_TEMPORAL_UI_URL
ENV NEXT_PUBLIC_STORAGE_UI_URL=$NEXT_PUBLIC_STORAGE_UI_URL
ENV NEXT_PUBLIC_GRAFANA_URL=$NEXT_PUBLIC_GRAFANA_URL

# Build without --turbopack (unsupported in production for Next.js 15.5.x)
ENV __NEXT_CONTINUE_EXPERIMENTAL=1
RUN npx next build

# --- Stage 3: Production runner ---
FROM node:18-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production

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
