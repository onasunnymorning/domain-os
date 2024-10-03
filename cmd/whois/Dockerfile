FROM golang:1.22.5-alpine3.20 AS build

WORKDIR /

# Install UPX for binary compression
RUN apk add upx

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./internal ./internal
COPY ./cmd/whois/ ./cmd/whois/

FROM build AS build-whois
RUN go build -tags dynamic -ldflags="-s -w" -o whoisServer /cmd/whois/whois.go

# Create API release image
FROM alpine:3.20 AS admin-api

# Copy our executable
COPY --from=build-whois /whoisServer /whoisServer

# Expose the port
EXPOSE 43

# Run the executable
CMD ["/whoisServer"]