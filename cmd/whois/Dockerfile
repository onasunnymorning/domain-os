FROM golang:1.26.5-alpine AS build

WORKDIR /
ENV CGO_ENABLED=0

# Install UPX for binary compression
RUN apk add upx

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY ./internal ./internal
COPY ./pkg ./pkg
COPY ./cmd/whois/ ./cmd/whois/

FROM build AS build-whois
ARG VERSION=dev
ARG GIT_SHA=unknown
ARG BUILD_DATE=unknown
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -tags dynamic -ldflags="-s -w \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.Version=${VERSION} \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.GitSHA=${GIT_SHA} \
      -X github.com/onasunnymorning/domain-os/internal/buildinfo.BuildDate=${BUILD_DATE}" \
      -o whoisServer /cmd/whois/whois.go

# Create API release image
FROM alpine:3.21.3 AS whois-server

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy our executable
COPY --from=build-whois /whoisServer /whoisServer

# Ensure the binary is executable by the user
RUN chown appuser:appgroup /whoisServer

# Set the user
USER appuser

# Expose the port
EXPOSE 43

# Run the executable
CMD ["/whoisServer"]
