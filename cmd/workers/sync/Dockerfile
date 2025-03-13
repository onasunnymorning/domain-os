FROM golang:1.24.1-alpine3.21 AS build

WORKDIR /

# Install UPX for binary compression
RUN apk add upx

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./internal ./internal
COPY ./cmd/workers/sync/ ./cmd/workers/sync/

FROM build AS build-sync
ARG GIT_SHA
RUN go build -tags dynamic -ldflags="-s -w -X main.GitSHA=${GIT_SHA}" -o syncWorker //cmd/workers/sync/syncWorker.go

# Create API release image
FROM alpine:3.21.3 AS domain-sync

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy our executable
COPY --from=build-sync /syncWorker /syncWorker

# Ensure the binary is executable by the user
RUN chown appuser:appgroup /syncWorker

# Set the user
USER appuser

# Run the executable
CMD ["/syncWorker"]
