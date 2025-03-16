# Build stage
FROM golang:1.21-alpine AS builder

# Install git and build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dns-server ./cmd/main.go

# Runtime stage
FROM alpine:latest

# Install necessary runtime packages
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user to run the application
RUN adduser -D -g '' dnsuser

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/dns-server /app/

# Create config directory
RUN mkdir -p /app/config

# Copy config file
COPY config/config.yaml /app/config/

# Set ownership of the application directory
RUN chown -R dnsuser:dnsuser /app

# Switch to non-root user
USER dnsuser

# Expose DNS and API ports
EXPOSE 53/udp 53/tcp 8080/tcp

# Set health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Run the application
CMD ["/app/dns-server"]