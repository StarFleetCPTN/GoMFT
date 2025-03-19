FROM golang:1.24-alpine AS builder

WORKDIR /app

# Accept build arguments for version information
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG COMMIT=unknown

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

# Compile the application with version information
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X github.com/starfleetcptn/gomft/components.AppVersion=${VERSION} -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.Commit=${COMMIT} -X github.com/starfleetcptn/gomft/components.BuildTime=${BUILD_TIME} -X github.com/starfleetcptn/gomft/components.Commit=${COMMIT}" \
    -o /app/gomft

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

# Add arguments for UID and GID with defaults
ARG UID=1000
ARG GID=1000
ARG USERNAME=gomft

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite bash shadow su-exec \
    && apk add --no-cache --virtual .user-deps \
    shadow curl xz

# Create user and group with specified IDs
RUN addgroup -g ${GID} ${USERNAME} && \
    adduser -D -u ${UID} -G ${USERNAME} -s /bin/sh ${USERNAME}

# Copy the binary from the builder stage
COPY --from=builder /app/gomft /app/
COPY --from=builder /usr/local/bin/rclone /usr/local/bin/rclone

# Copy static files and configurations
COPY static/ /app/static/
COPY components/ /app/components/

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create data and backup directories
RUN mkdir -p /app/data /app/backups

# Create a placeholder .env file with proper permissions
RUN touch /app/.env && chmod 644 /app/.env && chown ${USERNAME}:${USERNAME} /app/.env

# Set executable permissions
RUN chmod +x /app/gomft

# Set ownership of application files
RUN chown -R ${USERNAME}:${USERNAME} /app

# Expose the application port
EXPOSE 8080

# Use our entrypoint script
ENTRYPOINT ["/entrypoint.sh"]

# Run the application
CMD ["/app/gomft"] 
