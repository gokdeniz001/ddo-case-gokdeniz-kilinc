# Multi-stage Dockerfile for SiteYonet Go API
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Disable sum verification to avoid go.sum checksum mismatch with upstream
ENV GONOSUMDB=*
ENV GOFLAGS=-mod=mod

# Copy dependency files and download (go.sum excluded via .dockerignore, will be regenerated)
COPY go.mod ./
RUN go mod download && go mod tidy

# Copy source code
COPY . .

# Build the binary with optimizations (stripped symbols for minimal size)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/simulator ./cmd/simulator

# Final minimal runtime stage
FROM alpine:3.19

# Security: create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy certificates and compiled binaries
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/simulator /app/simulator
COPY --from=builder /app/migrations /app/migrations

# Assign ownership to non-root user
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=5 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/api"]
