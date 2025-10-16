# Build stage
FROM golang:1.23-alpine AS builder

# Install git and ca-certificates (needed for HTTPS requests)
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o s3-edgedelta-streamer ./cmd/streamer

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/s3-edgedelta-streamer .

# Copy config file
COPY --from=builder /app/config.yaml .

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose health check port
EXPOSE 8080

# Set default command
CMD ["./s3-edgedelta-streamer"]