FROM golang:1.26.4-alpine AS builder
WORKDIR /app
# Leverage layer caching: download deps first
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
# Then copy the rest of the source
COPY . .
ENV GOTOOLCHAIN=auto
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILD_DATE=unknown
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -ldflags="-s -w \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.Version=${VERSION} \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.GitSHA=${GIT_SHA} \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.BuildDate=${BUILD_DATE}" \
      -o /out/unified-worker ./cmd/workers/unified

FROM alpine:3.21.4
RUN apk upgrade --no-cache
# Copy seed data (used by lifecycle worker for registrar sync)
COPY ./initdata /app/initdata
# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /out/unified-worker /usr/local/bin/unified-worker
RUN chown appuser:appgroup /usr/local/bin/unified-worker
USER appuser
ENTRYPOINT ["/usr/local/bin/unified-worker"]
