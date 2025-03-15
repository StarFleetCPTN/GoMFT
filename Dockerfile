FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git build-base

# Install templ compiler
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Generate template files from .templ files
RUN templ generate

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o gomft

# Install rclone
RUN apk add --no-cache curl unzip && \
    curl -O https://downloads.rclone.org/rclone-current-linux-amd64.zip && \
    unzip rclone-current-linux-amd64.zip && \
    cd rclone-*-linux-amd64 && \
    cp rclone /usr/local/bin/ && \
    chmod 755 /usr/local/bin/rclone && \
    cd .. && \
    rm -rf rclone*

# Create a smaller runtime image
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite bash

# Copy the binary from the builder stage
COPY --from=builder /app/gomft /app/
COPY --from=builder /usr/local/bin/rclone /usr/local/bin/rclone

# Copy static files and configurations
COPY static/ /app/static/
COPY components/ /app/components/

# Create data and backup directories
RUN mkdir -p /app/data /app/backups

# Set executable permissions
RUN chmod +x /app/gomft

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["/app/gomft"] 
