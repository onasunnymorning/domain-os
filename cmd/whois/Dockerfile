FROM golang:1.23.4-alpine3.21 AS build

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
RUN go build -tags dynamic -ldflags="-s -w -X main.GitSHA=${GIT_SHA}" -o whoisServer /cmd/whois/whois.go

# Create API release image
FROM alpine:3.21 AS whois-server

# Copy our executable
COPY --from=build-whois /whoisServer /whoisServer

# Expose the port
EXPOSE 43

# Run the executable
CMD ["/whoisServer"]
