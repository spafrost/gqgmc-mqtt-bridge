FROM golang:1-alpine AS builder

RUN apk --no-cache --no-progress add git ca-certificates tzdata make \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

# Create non-root user during build
RUN adduser -D -s /bin/sh appuser

WORKDIR /go/gqgmc-mqtt-bridge

# Download go modules
COPY go.mod .
COPY go.sum .
RUN GO111MODULE=on GOPROXY=https://proxy.golang.org go mod download

COPY . .

RUN make build

# Use distroless image for better security
FROM gcr.io/distroless/static-debian11

# Add labels for security and maintenance
LABEL version="1.0" \
      description="HTTP to MQTT bridge for Wifi connected GMC GQ geiger counters"

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/gqgmc-mqtt-bridge/gqgmc-mqtt-bridge /gqgmc-mqtt-bridge

# Set security-focused user and permissions
USER appuser

# Health check for container orchestration
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/gqgmc-mqtt-bridge", "--health-check"]

ENTRYPOINT ["/gqgmc-mqtt-bridge"]
EXPOSE 80