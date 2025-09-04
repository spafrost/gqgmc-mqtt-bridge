FROM golang:1-alpine AS builder

# Build arguments for version information
ARG GIT_BRANCH=unknown
ARG GIT_COMMIT=unknown  
ARG BUILD_TIME=unknown
ARG VERSION=dev

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

# Build with version information using build args instead of git commands
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.GitBranch=${GIT_BRANCH} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME} -X main.Version=${VERSION}" \
    -a -installsuffix cgo -o gqgmc-mqtt-bridge .

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