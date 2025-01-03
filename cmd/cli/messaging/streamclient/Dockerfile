# The main Build image to build all our binaries
FROM golang:1.23.4-alpine3.21 AS build

WORKDIR /

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./cmd/cli/messaging/streamclient ./cmd/cli/messaging/streamclient


# Build the CONSUMER
FROM build AS build-receiver
WORKDIR /
RUN go build -tags dynamic -ldflags="-s -w -X main.GitSHA=${GIT_SHA}" -o streamClient /cmd/cli/messaging/streamclient


# Create CONSUMER release image
FROM alpine:3.21 AS release-consumer
# Copy our static executable
COPY --from=build-receiver /streamClient /streamClient

CMD [ "/streamClient" ]
