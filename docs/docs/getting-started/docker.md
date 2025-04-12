---
sidebar_position: 3
title: Docker Deployment
---

# Docker Deployment Guide

This guide provides detailed instructions for deploying GoMFT using Docker and Docker Compose, including advanced configuration options and best practices.

## Docker Image Information

GoMFT is available as a Docker image on Docker Hub:

- **Image Name**: `starfleetcptn/gomft`
- **Tags**:
  - `latest` - Latest stable release
  - `edge` - Latest development build
  - `v0.1.0`, `v0.2.0`, etc. - Specific version releases

## Basic Docker Run Command

The simplest way to run GoMFT with Docker:

```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/backups:/app/backups \
  starfleetcptn/gomft:latest
```

## Docker Compose Setup

For a more complete and production-ready setup, use Docker Compose:

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
    environment:
      - TZ=UTC
      - PUID=1000
      - PGID=1000
    restart: unless-stopped
```

Save this to a file named `docker-compose.yml` and run:

```bash
docker-compose up -d
```

## Persisting Data

GoMFT stores data in specific directories that should be mounted as volumes:

- **/app/data**: Contains the SQLite database, rclone configurations, and logs
- **/app/backups**: Contains database backups

Example with more specific volume mapping:

```yaml
volumes:
  - ./data/db:/app/data/db        # Database files
  - ./data/configs:/app/data/configs  # Rclone config files
  - ./data/logs:/app/data/logs    # Log files
  - ./backups:/app/backups        # Backup files
```

## File Transfer Volumes

In addition to the application data, you'll need to mount volumes for the files you want to transfer. These volumes provide GoMFT access to your source files and destination directories.

### Common File Volume Mounts

```yaml
volumes:
  # Application data volumes
  - ./data:/app/data
  - ./backups:/app/backups
  
  # File transfer volumes
  - /path/to/source/files:/sftp/files       # Source files for transfer
  - /path/to/destination:/mft/destination   # Destination for transferred files
  - /path/to/temp:/mft/temp                 # Temporary processing directory
```

### Docker Run Example with File Volumes

```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/backups:/app/backups \
  -v /path/to/source/files:/sftp/files \
  -v /path/to/destination:/mft/destination \
  -v /path/to/temp:/mft/temp \
  starfleetcptn/gomft:latest
```

### Docker Compose Example with File Volumes

```yaml
services:
  gomft:
    image: starfleetcptn/gomft:latest
    container_name: gomft
    ports:
      - "8080:8080"
    volumes:
      # Application data
      - ./data:/app/data
      - ./backups:/app/backups
      
      # File transfer directories
      - ./source_files:/sftp/files
      - ./destination:/mft/destination
      - ./temp:/mft/temp
    environment:
      - TZ=UTC
    restart: unless-stopped
```

### Volume Permissions

When mounting file volumes, ensure the container has appropriate permissions to access these directories:

1. If using `PUID` and `PGID` environment variables:
   ```bash
   # Set correct ownership on host directories
   chown -R 1000:1000 /path/to/source/files
   chown -R 1000:1000 /path/to/destination
   chown -R 1000:1000 /path/to/temp
   ```

2. Or set appropriate permissions:
   ```bash
   # Make directories accessible to the container
   chmod -R 755 /path/to/source/files
   chmod -R 755 /path/to/destination
   chmod -R 755 /path/to/temp
   ```

## Environment Variables

GoMFT can be configured using environment variables:

### Basic Configuration

```yaml
environment:
  - PORT=8080                   # Web UI port
  - BASE_URL=https://gomft.example.com  # Base URL for email links
  - TZ=America/New_York         # Timezone
```

### Data Directory Configuration

```yaml
environment:
  - DATA_DIR=/app/data          # Main data directory
  - LOGS_DIR=/app/data/logs     # Logs directory
  - BACKUP_DIR=/app/backups     # Backup directory
```

### Email Notification Configuration

```yaml
environment:
  - EMAIL_ENABLED=true          # Enable email notifications
  - EMAIL_HOST=smtp.example.com # SMTP server host
  - EMAIL_PORT=587              # SMTP server port
  - EMAIL_USER=user@example.com # SMTP username
  - EMAIL_PASSWORD=password     # SMTP password
  - EMAIL_FROM=gomft@example.com # From address for emails
```

### Security Configuration

```yaml
environment:
  - JWT_SECRET=your-secret-key  # Secret for JWT tokens
  - ENCRYPT_KEY=32-char-key     # Key for encrypting sensitive data
```

## Running as Non-Root User

For enhanced security, run GoMFT as a non-root user:

```yaml
environment:
  - PUID=1000                   # User ID to run as
  - PGID=1000                   # Group ID to run as
```

Make sure your mounted volumes have the appropriate permissions for this user.

## Exposing GoMFT Behind a Reverse Proxy

It's recommended to run GoMFT behind a reverse proxy like Nginx or Traefik for SSL termination and security.

### Nginx Example

```nginx
server {
    listen 80;
    server_name gomft.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name gomft.example.com;

    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;

    location / {
        proxy_pass http://gomft:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Docker Compose with Traefik Example

```yaml
version: '3'

services:
  traefik:
    image: traefik:v2.5
    command:
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.myresolver.acme.tlschallenge=true"
      - "--certificatesresolvers.myresolver.acme.email=your@email.com"
      - "--certificatesresolvers.myresolver.acme.storage=/acme.json"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./acme.json:/acme.json
    restart: unless-stopped

  gomft:
    image: starfleetcptn/gomft:latest
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
    environment:
      - PUID=1000
      - PGID=1000
      - BASE_URL=https://gomft.example.com
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.gomft.rule=Host(`gomft.example.com`)"
      - "traefik.http.routers.gomft.entrypoints=websecure"
      - "traefik.http.routers.gomft.tls.certresolver=myresolver"
    restart: unless-stopped
```

## Health Checks

You can configure a health check to monitor the container's health:

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 1m
  timeout: 10s
  retries: 3
  start_period: 30s
```

## Resource Limits

Set resource limits to prevent the container from consuming too many resources:

```yaml
deploy:
  resources:
    limits:
      cpus: '1'
      memory: 1G
    reservations:
      cpus: '0.25'
      memory: 512M
```

## Logging Configuration

Configure container logging:

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

## Troubleshooting Docker Deployment

If you encounter issues with your Docker deployment:

1. Check container logs:
   ```
   docker logs gomft
   ```

2. Check container status:
   ```
   docker ps -a | grep gomft
   ```

3. Verify volume permissions:
   ```
   ls -la ./data
   ```

4. Check container environment:
   ```
   docker exec gomft env
   ```

5. Inspect the container:
   ```
   docker inspect gomft
   ```

For more help, refer to the [GitHub repository](https://github.com/StarFleetCPTN/GoMFT) or open an issue. 