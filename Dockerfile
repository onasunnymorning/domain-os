# The main Build image to build all our binaries
FROM golang:1.21.3-alpine3.18 as build

WORKDIR /

# Install build Dependencies for EPP
# RUN apk add libxml2
# RUN apk add libxml2-dev
# RUN apk add build-base
# RUN apk add pkgconfig

# Install swag
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./internal ./internal
COPY ./cmd/registry ./cmd/registry


# Just build API
FROM build as build-admin-api
# Generate swagger docs
WORKDIR /cmd/registry
RUN swag init -g main.go -o /docs -d ./,/internal/domain/entities,/internal/application/commands,/internal/interface/rest
# build binary
WORKDIR /
RUN go build -o adminAPI /cmd/registry/main.go


# Create API release image
FROM alpine:3.18 as admin-api
# Copy our static executable
COPY --from=build-admin-api /adminAPI /adminAPI

EXPOSE 8080
CMD [ "/adminAPI" ]
