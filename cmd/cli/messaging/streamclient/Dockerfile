# The main Build image to build all our binaries
FROM golang:1.22.5-alpine3.20 AS build

WORKDIR /

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./cmd/cli/messaging/receive ./cmd/cli/messaging/receive


# Build the CONSUMER
FROM build AS build-receiver
WORKDIR /
RUN go build -tags dynamic -ldflags="-s -w" -o streamClient /cmd/cli/messaging/receive


# Create CONSUMER release image
FROM alpine:3.20 AS release-consumer
# Copy our static executable
COPY --from=build-receiver /streamClient /streamClient

CMD [ "/streamClient" ]
