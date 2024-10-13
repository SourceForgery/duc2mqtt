# Build Stage
FROM golang:1.23.2-alpine AS builder

# Install git and ca-certificates (for HTTPS requests)
RUN apk add --no-cache git ca-certificates

# Set the working directory
WORKDIR /app

# Clone the repository
RUN git clone https://github.com/SourceForgery/duc2mqtt.git version

# Build the Go binary statically
RUN CGO_ENABLED=0 GOOS=linux go build -o duc2mqtt

# Runtime Stage
FROM scratch

# Copy CA certificates (needed for HTTPS connections)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from the builder stage
COPY --from=builder /app/duc2mqtt /duc2mqtt

# Set the entrypoint to run duc2mqtt with the configuration file
ENTRYPOINT ["/duc2mqtt", "-c", "/data/options.json"]