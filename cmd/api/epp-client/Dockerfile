# The main Build image to build all our binaries
FROM golang:1.24.1-alpine3.21 AS build

WORKDIR /

# Install swag
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Install UPX for binary compression
RUN apk add upx

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./cmd/api/epp-client ./cmd/api/epp-client


# Just build API
FROM build as build-epp-client-api
# Generate swagger docs
WORKDIR /cmd/api/epp-client
RUN swag init -g eppClientApi.go -o /docs --parseDependency
# build binary
WORKDIR /
ARG GIT_SHA
RUN go build -ldflags="-s -w -X main.GitSHA=${GIT_SHA}" -o eppClientApi /cmd/api/epp-client/eppClientApi.go
# RUN upx --brute /adminAPI # This takes a very long time to compress the binary we should only use if for official releases or when absolutley necessary. It does reduce the size of the binary from 30MB to less than 10MB


# Create API release image
FROM alpine:3.21.3 as epp-client-api

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy our static executable
COPY --from=build-epp-client-api /eppClientApi /eppClientApi

# Ensure the binary is executable by the user
RUN chown appuser:appgroup /eppClientApi

# Set the user
USER appuser

EXPOSE 8700
CMD [ "/eppClientApi" ]
