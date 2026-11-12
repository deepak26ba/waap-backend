# syntax=docker/dockerfile:1
# ==============================================================================
# Multi-stage Dockerfile for Waap Backend E-Commerce Service
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Build binary
# ------------------------------------------------------------------------------
FROM golang:alpine AS builder

WORKDIR /app

# Enable automatic Go toolchain switching if needed
ENV GOTOOLCHAIN=auto \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Install git, SSL certs, and timezone definitions
RUN apk add --no-cache git ca-certificates tzdata

# Cache go modules layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile binary statically with stripped symbols for minimal image size
RUN go build \
    -ldflags="-w -s" \
    -o /app/server ./cmd/api

# ------------------------------------------------------------------------------
# Stage 2: Minimal runtime
# ------------------------------------------------------------------------------
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies (certificates for HTTPS/TLS, timezone data)
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy compiled binary and migration assets from builder
COPY --from=builder /app/server /app/server
COPY --from=builder /app/sql/migrations /app/sql/migrations

# Run as non-privileged user for security
USER appuser

# Expose HTTP port (Render injects PORT dynamically at runtime)
EXPOSE 8080

# Run server
ENTRYPOINT ["/app/server"]
