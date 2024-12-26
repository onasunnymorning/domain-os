# The main Build image to build all our binaries
FROM golang:1.23.4-alpine3.21 as build

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
RUN go build -ldflags="-s -w" -o eppClientApi /cmd/api/epp-client/eppClientApi.go
# RUN upx --brute /adminAPI # This takes a very long time to compress the binary we should only use if for official releases or when absolutley necessary. It does reduce the size of the binary from 30MB to less than 10MB


# Create API release image
FROM alpine:3.21 as epp-client-api
# Copy our static executable
COPY --from=build-epp-client-api /eppClientApi /eppClientApi

EXPOSE 8700
CMD [ "/eppClientApi" ]
