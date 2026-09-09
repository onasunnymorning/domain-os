FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine AS builder
WORKDIR /app
# Dependencies come from the committed vendor tree, not the module proxy: the
# build stays hermetic and cannot fail on a proxy outage. go.mod/go.sum are
# still needed — vendor mode checks them against vendor/modules.txt.
# Leverage layer caching: the vendor tree changes far less often than the source.
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
# Then copy the rest of the source
COPY . .
ENV GOTOOLCHAIN=auto
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -mod=vendor -ldflags="-s -w \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.Version=${VERSION} \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.GitSHA=${GIT_SHA}" \
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
