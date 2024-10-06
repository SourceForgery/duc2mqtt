# Build Stage
FROM golang:1.17-alpine AS builder

# Install git and ca-certificates (for HTTPS requests)
RUN apk add --no-cache git ca-certificates

# Set the working directory
WORKDIR /app

# Clone the repository
RUN git clone https://github.com/SourceForgery/duc2mqtt.git .

# Build the Go binary statically
RUN CGO_ENABLED=0 GOOS=linux go build -a -o duc2mqtt ./src/...

# Runtime Stage
FROM scratch

# Copy CA certificates (needed for HTTPS connections)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from the builder stage
COPY --from=builder /app/duc2mqtt /duc2mqtt

# Set the entrypoint to run duc2mqtt with the configuration file
ENTRYPOINT ["/duc2mqtt", "-c", "/data/options.json"]