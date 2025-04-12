# Build frontend assets
FROM node:20-alpine AS frontend-builder

WORKDIR /app

# Copy all files needed for the build first
COPY package.json package-lock.json ./
COPY build.js ./
COPY static/ ./static/
COPY tailwind.config.js ./

# Debug: Show the contents of build.js
RUN echo "Contents of build.js:" && cat build.js

# Remove the build script from postinstall
RUN sed -i 's/"postinstall": "npm run build",//' package.json

# Install dependencies and build with verbose output
RUN npm ci && \
    echo "Building frontend assets..." && \
    node build.js && \
    echo "Build complete. Contents of dist:" && \
    ls -la static/dist/ && \
    echo "Sample of app.js:" && \
    head -n 10 static/dist/app.js && \
    echo "Sample of app.css:" && \
    head -n 10 static/dist/app.css

# Go build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Accept build arguments for version information
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG COMMIT=unknown
# Architecture-related build arguments
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=""

# Install build dependencies
RUN apk add --no-cache git build-base

# Install templ compiler
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Create static directory structure
RUN mkdir -p /app/static/dist

# Copy built frontend assets from frontend-builder BEFORE copying Go source
COPY --from=frontend-builder /app/static/dist/ /app/static/dist/

# Copy the rest of the static files
COPY static/ /app/static/

# Verify static files are in place before Go build
RUN echo "Verifying static files before Go build:" && \
    ls -la /app/static/dist/

# Now copy the rest of the source code
COPY . .

# Generate template files from .templ files
RUN templ generate

# Compile the application with version information
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags "-X github.com/starfleetcptn/gomft/components.AppVersion=${VERSION} -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.Commit=${COMMIT} -X github.com/starfleetcptn/gomft/components.BuildTime=${BUILD_TIME} -X github.com/starfleetcptn/gomft/components.Commit=${COMMIT}" \
    -o /app/gomft

# Install rclone with appropriate architecture
RUN apk add --no-cache curl unzip && \
    if [ "$TARGETARCH" = "arm64" ]; then \
        RCLONE_ARCH="arm64"; \
    elif [ "$TARGETARCH" = "arm" ]; then \
        RCLONE_ARCH="arm-v7"; \
    else \
        RCLONE_ARCH="amd64"; \
    fi && \
    curl -O https://downloads.rclone.org/rclone-current-linux-${RCLONE_ARCH}.zip && \
    unzip rclone-current-linux-${RCLONE_ARCH}.zip && \
    cd rclone-*-linux-${RCLONE_ARCH} && \
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
ARG TARGETOS
ARG TARGETARCH

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

# Copy components
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
