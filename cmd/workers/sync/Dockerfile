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
COPY ./cmd/workers/sync/ ./cmd/workers/sync/

FROM build AS build-sync
RUN go build -tags dynamic -ldflags="-s -w" -o syncWorker //cmd/workers/sync/syncWorker.go

# Create API release image
FROM alpine:3.21 AS domain-sync
# Copy our executable
COPY --from=build-sync /syncWorker /syncWorker

# Run the executable
CMD ["/syncWorker"]
