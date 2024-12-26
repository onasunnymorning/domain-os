# The main Build image to build all our binaries
FROM golang:1.23.4-alpine3.21 AS build

WORKDIR /

# Install build Dependencies for EPP
# RUN apk add libxml2
# RUN apk add libxml2-dev
# RUN apk add build-base
# RUN apk add pkgconfig

# Install swag
RUN go install github.com/swaggo/swag/cmd/swag@latest

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
RUN go mod download

# Copy source code
COPY ./internal ./internal
COPY ./cmd/api/ry-admin ./cmd/api/ry-admin


# Just build API
FROM build AS build-admin-api
# Generate swagger docs
WORKDIR /cmd/api/ry-admin
RUN swag init -g ryAdminAPI.go -o /docs --parseDependency -d ./,/internal/domain/entities,/internal/application/commands,/internal/interface/rest
# build binary
WORKDIR /
RUN go build -tags dynamic -ldflags="-s -w" -o ryAdminAPI /cmd/api/ry-admin/ryAdminAPI.go
# RUN upx --brute /ryAdminAPI # This takes a very long time to compress the binary we should only use if for official releases or when absolutley necessary. It does reduce the size of the binary from 30MB to less than 10MB


# Create API release image
FROM alpine:3.21 AS admin-api

# Copy librdkafka from the build image
# COPY --from=build-admin-api /usr/lib/librdkafka* /usr/lib/

# Copy our static executable
COPY --from=build-admin-api /ryAdminAPI /ryAdminAPI

EXPOSE 8080
CMD [ "/ryAdminAPI" ]
