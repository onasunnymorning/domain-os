# The main Build image to build all our binaries.
# Pinned to the native build platform so Go cross-compiles to the target arch
# (GOARCH below) instead of running under QEMU emulation on the arm64 leg.
FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine AS build

WORKDIR /
ENV CGO_ENABLED=0

# Install build Dependencies for EPP
# RUN apk add libxml2
# RUN apk add libxml2-dev
# RUN apk add build-base
# RUN apk add pkgconfig

# Install swag
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go install github.com/swaggo/swag/cmd/swag@v1.16.3

# Install UPX for binary compression
RUN apk add upx

# Install Kafka dependencies
# RUN apk add --no-cache upx build-base pkgconfig git
# RUN apk add --no-cache bash \
#     && git clone https://github.com/edenhill/librdkafka.git /librdkafka \
#     && cd /librdkafka \
#     && git checkout v2.4.0 \
#     && ./configure --prefix=/usr --build=aarch64-alpine-linux-musl --host=aarch64-alpine-linux-musl \
#     && make \
#     && make install


# Set environment variables for CGO
# ENV CGO_ENABLED=1 \
#     CGO_CFLAGS="-I/usr/include" \
#     CGO_LDFLAGS="-L/usr/lib" \
#     LIBRDKAFKA=1

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY ./internal ./internal
COPY ./pkg ./pkg
COPY ./cmd/api/ry-admin ./cmd/api/ry-admin
COPY ./docs /docs


# Just build API
FROM build AS build-admin-api
# Generate swagger docs
WORKDIR /cmd/api/ry-admin
ARG SKIP_SWAG=false
RUN if [ "$SKIP_SWAG" = "true" ]; then \
        echo "Skipping swag init"; \
    else \
        swag init -g ryAdminAPI.go -o /docs --parseDependency -d ./,/pkg/domain/entities,/internal/application/commands,/internal/interface/rest; \
    fi
# build binary
WORKDIR /
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags dynamic -ldflags="-s -w \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.Version=${VERSION} \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.GitSHA=${GIT_SHA}" \
      -o ryAdminAPI ./cmd/api/ry-admin
# RUN upx --brute /ryAdminAPI # This takes a very long time to compress the binary we should only use if for official releases or when absolutley necessary. It does reduce the size of the binary from 30MB to less than 10MB


# Create API release image
FROM alpine:3.21.4 AS admin-api

## Install security patches and dnsviz dependencies
RUN apk upgrade --no-cache && \
    apk add --no-cache python3 py3-pip bind-tools graphviz py3-cryptography && \
    pip install dnsviz dnspython --break-system-packages --root-user-action=ignore

# Copy librdkafka from the build image
# COPY --from=build-admin-api /usr/lib/librdkafka* /usr/lib/

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy our static executable
COPY --from=build-admin-api /ryAdminAPI /ryAdminAPI

# Knowledge base for the AI agent's answer_system_question tool.
# NewKnowledgeService loads $KNOWLEDGE_BASE_DIR/docs/index.yaml; without these
# the API logs "KnowledgeService not available" and disables the tool.
# Copied from the build context (not the build stage, where swag overwrote /docs).
COPY ./docs /kb/docs
ENV KNOWLEDGE_BASE_DIR=/kb

# Ensure the user owns the binary
RUN chown -R appuser:appgroup /ryAdminAPI && chmod +x /ryAdminAPI

# Use an unprivileged user to run the binary
USER appuser

EXPOSE 8080
CMD [ "/ryAdminAPI" ]
