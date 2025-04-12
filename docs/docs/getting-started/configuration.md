---
sidebar_position: 4
title: Configuration
---

# Configuration

GoMFT can be customized through environment variables. This document provides a complete list of configuration options available in GoMFT.

## Environment Variables

Environment variables are the primary way to configure GoMFT, especially when running in Docker. These variables can be set in your Docker Compose file, `.env` file, or directly in your system environment.

### Core Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| SERVER_ADDRESS | Server address and port | :8080 | `SERVER_ADDRESS=:9000` |
| DATA_DIR | Main data directory | ./data | `DATA_DIR=/app/data` |
| BACKUP_DIR | Directory for backups | ./backups | `BACKUP_DIR=/app/backups` |
| JWT_SECRET | Secret for JWT tokens | change_this_to_a_secure_random_string | `JWT_SECRET=your-secure-secret-key` |
| BASE_URL | Base URL for GoMFT (used in email links) | http://localhost:8080 | `BASE_URL=https://gomft.example.com` |
| SKIP_SSL_VERIFY | Skip SSL verification for outgoing webhooks/notifications | false | `SKIP_SSL_VERIFY=false` |

### Authentication Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| TOTP_ENCRYPTION_KEY | Encryption key for TOTP secrets | this-is-a-dev-key-not-for-production! | `TOTP_ENCRYPTION_KEY=your-secure-key` |
| PUID | User ID to run as (Docker only) | | `PUID=1000` |
| PGID | Group ID to run as (Docker only) | | `PGID=1000` |

### Email Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| EMAIL_ENABLED | Enable email functionality | false | `EMAIL_ENABLED=true` |
| EMAIL_HOST | SMTP server hostname | smtp.example.com | `EMAIL_HOST=smtp.gmail.com` |
| EMAIL_PORT | SMTP server port | 587 | `EMAIL_PORT=587` |
| EMAIL_USERNAME | SMTP username | user@example.com | `EMAIL_USERNAME=your-email@example.com` |
| EMAIL_PASSWORD | SMTP password | your-password | `EMAIL_PASSWORD=your-smtp-password` |
| EMAIL_FROM_EMAIL | From email address | gomft@example.com | `EMAIL_FROM_EMAIL=gomft@example.com` |
| EMAIL_FROM_NAME | From name | GoMFT | `EMAIL_FROM_NAME=GoMFT Notifications` |
| EMAIL_REPLY_TO | Reply-to email address | | `EMAIL_REPLY_TO=support@example.com` |
| EMAIL_ENABLE_TLS | Use TLS for SMTP connection | true | `EMAIL_ENABLE_TLS=true` |
| EMAIL_REQUIRE_AUTH | Require authentication for SMTP | true | `EMAIL_REQUIRE_AUTH=true` |

### OAuth Configuration (Optional)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| GOOGLE_CLIENT_ID | Google OAuth client ID | | `GOOGLE_CLIENT_ID=your-client-id` |
| GOOGLE_CLIENT_SECRET | Google OAuth client secret | | `GOOGLE_CLIENT_SECRET=your-client-secret` |

## Configuration File

In addition to setting environment variables directly, GoMFT can also be configured using a `.env` file. This file should be placed in the root directory of your GoMFT installation.

Example `.env` file:

```
# Server Configuration
SERVER_ADDRESS=:8080
DATA_DIR=./data
BACKUP_DIR=./backups
JWT_SECRET=change_this_to_a_secure_random_string
BASE_URL=http://localhost:8080
SKIP_SSL_VERIFY=false

# Two-Factor Authentication configuration
TOTP_ENCRYPTION_KEY=this-is-a-dev-key-not-for-production!

# OAuth Configuration (optional)
# GOOGLE_CLIENT_ID=your_google_client_id
# GOOGLE_CLIENT_SECRET=your_google_client_secret

# Email Configuration
EMAIL_ENABLED=true
EMAIL_HOST=smtp.example.com
EMAIL_PORT=587
EMAIL_FROM_EMAIL=gomft@example.com
EMAIL_FROM_NAME=GoMFT
EMAIL_REPLY_TO=
EMAIL_ENABLE_TLS=true
EMAIL_REQUIRE_AUTH=true
EMAIL_USERNAME=your-email@example.com
EMAIL_PASSWORD=your-smtp-password
```

## Priority Order

GoMFT uses the following priority order for configuration:

1. Environment variables set directly
2. Variables in the `.env` file
3. Default values

This means that environment variables set directly will override settings in the `.env` file, which in turn override the default values.

## Docker Configuration

When running GoMFT in Docker, you can configure the application in several ways:

### Using Environment Variables

```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  -e SERVER_ADDRESS=:8080 \
  -e JWT_SECRET=your-secure-secret \
  -e EMAIL_ENABLED=true \
  -e EMAIL_HOST=smtp.example.com \
  -e PUID=1000 \
  -e PGID=1000 \
  starfleetcptn/gomft:latest
```

### Using .env File

```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  -v /path/to/.env:/app/.env \
  starfleetcptn/gomft:latest
```

### Docker Compose Example

```yaml
version: '3'

services:
  gomft:
    image: starfleetcptn/gomft:latest
    container_name: gomft
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
      - ./.env:/app/.env  # Mount .env file (optional)
    environment:
      - PUID=1000
      - PGID=1000
    restart: unless-stopped
```

## Applying Configuration Changes

Most configuration changes require a restart of the GoMFT service to take effect. After modifying environment variables or the `.env` file, restart your container or service:

```bash
# For Docker
docker restart gomft

# For Docker Compose
docker-compose restart gomft
```

## Next Steps

- [Docker Deployment](/docs/getting-started/docker) - Advanced Docker deployment options
- [Non-Root Operation](/docs/security/non-root) - Running GoMFT as a non-root user
- [Best Practices](/docs/security/best-practices) - Security best practices 