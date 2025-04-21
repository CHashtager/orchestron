# Build stage
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o orchestron ./cmd/orchestron

# Final stage
FROM alpine:3.17

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from build stage
COPY --from=builder /app/orchestron .

# Copy configuration
COPY config/config.yaml.example /app/config/config.yaml
COPY config/prometheus.yml /app/config/

# Expose API port
EXPOSE 8080

# Run the service
CMD ["./orchestron"]