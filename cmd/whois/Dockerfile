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
ARG GIT_SHA
RUN go build -tags dynamic -ldflags="-s -w -X main.GitSHA=${GIT_SHA}" -o whoisServer /cmd/whois/whois.go

# Create API release image
FROM alpine:3.21.3 AS whois-server

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy our executable
COPY --from=build-whois /whoisServer /whoisServer

# Ensure the binary is executable by the user
RUN chown appuser:appgroup /whoisServer

# Set the user
USER appuser

# Expose the port
EXPOSE 43

# Run the executable
CMD ["/whoisServer"]
