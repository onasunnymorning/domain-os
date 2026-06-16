FROM golang:1.25-alpine AS builder
WORKDIR /app
# Leverage layer caching: download deps first
COPY go.mod go.sum ./
RUN go mod download
# Then copy the rest of the source
COPY . .
ENV GOTOOLCHAIN=auto
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/unified-worker ./cmd/workers/unified

FROM alpine:3.21.3
# Copy seed data (used by lifecycle worker for registrar sync)
COPY ./initdata /initdata
# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /out/unified-worker /usr/local/bin/unified-worker
RUN chown appuser:appgroup /usr/local/bin/unified-worker
USER appuser
ENTRYPOINT ["/usr/local/bin/unified-worker"]
