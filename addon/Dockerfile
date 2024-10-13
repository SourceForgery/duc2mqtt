ARG BUILD_VERSION=master

FROM golang:1.23.2-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
RUN git clone https://github.com/SourceForgery/duc2mqtt.git ${BUILD_VERSION} .
RUN CGO_ENABLED=0 GOOS=linux go build -o duc2mqtt

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/duc2mqtt /duc2mqtt
ENTRYPOINT ["/duc2mqtt", "-c", "/data/options.json"]