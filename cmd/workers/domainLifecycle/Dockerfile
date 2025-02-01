FROM golang:1.23.4-alpine3.21 AS build

WORKDIR /

# Install UPX for binary compression
RUN apk add upx

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./internal ./internal
COPY ./cmd/workers/domainLifecycle/ ./cmd/workers/domainLifecycle/

FROM build AS build-domain-lifecycle
ARG GIT_SHA
RUN go build -tags dynamic -ldflags="-s -w -X main.GitSHA=${GIT_SHA}" -o domainLifeCycleWorker /cmd/workers/domainLifecycle/domainLifecycleWorker.go

# Create API release image
FROM alpine:3.21 AS domain-lifecycle-worker

# Copy our init data
COPY ./initdata /initdata

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy our executable
COPY --from=build-domain-lifecycle /domainLifeCycleWorker /domainLifeCycleWorker

# Ensure the user owns the binary
RUN chown appuser:appgroup /domainLifeCycleWorker

# Use an unprivileged user.
USER appuser

# Run the executable
CMD ["/domainLifeCycleWorker"]
