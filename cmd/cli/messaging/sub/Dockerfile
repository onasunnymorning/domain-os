# The main Build image to build all our binaries
FROM golang:1.23.4-alpine3.21 AS build

WORKDIR /

# Install necessary tools
RUN apk add --no-cache upx build-base pkgconfig git

# Install dependencies for librdkafka
RUN apk add --no-cache bash libsasl openssl-dev

# Install librdkafka v2.4.0 from source
RUN git clone https://github.com/edenhill/librdkafka.git /librdkafka \
    && cd /librdkafka \
    && git checkout v2.4.0 \
    && ./configure --prefix=/usr --build=aarch64-alpine-linux-musl --host=aarch64-alpine-linux-musl \
    && make \
    && make install

# # Set environment variables for CGO
ENV CGO_ENABLED=1 \
    CGO_CFLAGS="-I/usr/include" \
    CGO_LDFLAGS="-L/usr/lib" \
    LIBRDKAFKA=1

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./cmd/cli/messaging/sub ./cmd/cli/messaging/sub


# Just build the consumer binary
FROM build AS build-consumer
RUN go build -tags dynamic -ldflags="-s -w" -o consumerApp /cmd/cli/messaging/sub/consumer.go
# RUN upx --brute /consumerApp


# Create Consumer release image
FROM alpine:3.20 AS consumerApp

# Copy librdkafka from the build image
COPY --from=build /usr/lib/librdkafka* /usr/lib/

# Copy our static executable
COPY --from=build-consumer /consumerApp /consumerApp

CMD [ "/consumerApp", "domain-os-kafka-1", "mygroup", "DOS-AdminAPI-Events" ]
