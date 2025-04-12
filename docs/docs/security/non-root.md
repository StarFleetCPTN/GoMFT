---
sidebar_position: 3
title: Running as Non-Root
---

# Running GoMFT as a Non-Root User

By default, Docker containers run as the root user, which can pose security risks. GoMFT fully supports running as a non-root user, which is recommended for production environments.

## Benefits of Running as Non-Root

- **Improved Security**: Limits the potential damage if the container is compromised
- **Better File Permissions**: Files created by the container will match your host user permissions
- **Compliance**: Many security policies and best practices require containers to run as non-root

## Methods to Run GoMFT as Non-Root

GoMFT supports several methods for running as a non-root user, each with its own advantages.

### Method 1: Using PUID/PGID Environment Variables (Recommended)

This method allows changing the user at runtime without rebuilding the image:

```bash
# Using current user's ID
docker run -e PUID=$(id -u) -e PGID=$(id -g) starfleetcptn/gomft:latest
```

Or in docker-compose.yml:

```yaml
services:
  gomft:
    image: starfleetcptn/gomft:latest
    environment:
      - PUID=1000  # Your user ID
      - PGID=1000  # Your group ID
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
```

### Method 2: Using the `--user` Flag with Docker Run

This method is simple but doesn't support some advanced features like permission fixing:

```bash
docker run --user $(id -u):$(id -g) starfleetcptn/gomft:latest
```

### Method 3: Using Docker Compose with Environment Variables

This approach uses environment variables from the host for the user directive:

```yaml
services:
  gomft:
    image: starfleetcptn/gomft:latest
    user: "${UID:-1000}:${GID:-1000}"
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
```

Run with:

```bash
UID=$(id -u) GID=$(id -g) docker-compose up -d
```

### Method 4: Building a Custom Image with Specified UID/GID

This method builds a custom image with your specified user ID:

```dockerfile
FROM starfleetcptn/gomft:latest

ARG UID=1000
ARG GID=1000

RUN usermod -u $UID gomft && groupmod -g $GID gomft
```

In docker-compose.yml:

```yaml
services:
  gomft:
    build:
      context: .
      args:
        UID: ${UID:-1000}
        GID: ${GID:-1000}
```

## Environment Variables for User Management

| Variable | Description        | Default                  |
| -------- | ------------------ | ------------------------ |
| PUID     | User ID to run as  | Built-in user ID (1000)  |
| PGID     | Group ID to run as | Built-in group ID (1000) |
| USERNAME | Username to use    | gomft                    |

## Volume Permissions

When running as a non-root user, ensure that the directories on the host have appropriate permissions for the container user:

### Option 1: Create Directories with Correct Ownership (Recommended)

```bash
# Create directories
mkdir -p data backups

# Set ownership to match the PUID/PGID you'll use
chown -R 1000:1000 data backups
```

### Option 2: Adjust Permissions (Less Secure, but Easier for Testing)

```bash
mkdir -p data backups
chmod -R 777 data backups
```

## Verifying Non-Root Operation

To verify that GoMFT is running as a non-root user:

```bash
docker exec gomft id
```

You should see output showing the UID and GID you specified.

## Troubleshooting Permission Issues

### Common Issues

1. **Volume Mount Permission Denied**: The container user doesn't have permission to access mounted volumes

   **Solution**:
   ```bash
   chown -R <PUID>:<PGID> ./data ./backups
   ```

2. **Cannot Write to Log Files**: Permission issues with log files

   **Solution**:
   ```bash
   # Ensure log directory exists and has correct permissions
   mkdir -p ./data/logs
   chown -R <PUID>:<PGID> ./data/logs
   ```

3. **Database Permission Errors**: SQLite database permissions

   **Solution**:
   ```bash
   # Check and fix database file permissions
   chown <PUID>:<PGID> ./data/gomft.db
   chmod 644 ./data/gomft.db
   ```

### Checking Container Logs for Permission Issues

```bash
docker logs gomft | grep -i "permission denied"
```

### Volume Permission Script

You can use this script to fix permissions on your data volumes:

```bash
#!/bin/bash
# Fix permissions for GoMFT volumes

# Set your PUID and PGID here
PUID=1000
PGID=1000

# Create directories if they don't exist
mkdir -p ./data ./backups

# Fix ownership
chown -R $PUID:$PGID ./data ./backups

echo "Permissions fixed for GoMFT volumes"
```

## Security Considerations

When running as non-root, there are still some security considerations:

- Avoid using `chmod 777` in production environments
- Use volume binding with caution, especially for sensitive data
- Consider using Docker secrets for sensitive credentials
- Regularly update your GoMFT image to get the latest security fixes
- Implement network segmentation to limit the container's access

## Example: Complete Docker Compose Setup with Non-Root User

```yaml
version: '3.8'

services:
  gomft:
    image: starfleetcptn/gomft:latest
    container_name: gomft
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=UTC
      - BASE_URL=http://localhost:8080
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 1m
      timeout: 10s
      retries: 3
      start_period: 30s
```

## Best Practices Summary

1. **Always run GoMFT as a non-root user in production**
2. **Use PUID/PGID environment variables for flexible user mapping**
3. **Set appropriate permissions on volume mounts**
4. **Verify the container is running as the expected user**
5. **Follow least privilege principles for the container user** 