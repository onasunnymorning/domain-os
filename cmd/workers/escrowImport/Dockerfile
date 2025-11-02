FROM golang:1.24.1-alpine3.21 AS builder
WORKDIR /app
# Leverage layer caching: download deps first
COPY go.mod go.sum ./
RUN go mod download
# Then copy the rest of the source
COPY . .
ENV GOTOOLCHAIN=auto
RUN CGO_ENABLED=0 go build -o /out/escrow-import-worker ./cmd/workers/escrowImport

FROM alpine:3.21.3
WORKDIR /app
COPY --from=builder /out/escrow-import-worker /usr/local/bin/escrow-import-worker
ENTRYPOINT ["/usr/local/bin/escrow-import-worker"]
