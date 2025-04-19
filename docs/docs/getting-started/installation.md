---
sidebar_position: 1
title: Installation
---

# Installing GoMFT

GoMFT can be installed using Docker (recommended) or through a traditional installation. This guide covers both methods.

## System Requirements

- **CPU**: 1+ cores (2+ recommended for production)
- **RAM**: 512MB minimum (1GB+ recommended for production)
- **Disk Space**: 100MB for the application plus space for your transfer data and logs
- **Operating System**: Linux, macOS, or Windows with Docker support

## Docker Installation (Recommended)

The easiest way to deploy GoMFT is using Docker. This method handles all dependencies and provides an isolated environment.

### Using Docker Run

```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  starfleetcptn/gomft:latest
```

Replace `/path/to/data` and `/path/to/backups` with your desired local paths for persistent storage.

### Using Docker Compose

Create a `docker-compose.yaml` file:

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
    restart: unless-stopped
```

Then run:

```bash
docker-compose up -d
```

### Environment Variables

You can customize your GoMFT installation using environment variables:

```yaml
environment:
  - TZ=America/New_York
  - PORT=8080
  - DATA_DIR=/app/data
  - BACKUP_DIR=/app/backups
  - LOGS_DIR=/app/data/logs
  - EMAIL_ENABLED=false
  - BASE_URL=http://localhost:8080
```

See the [Configuration](/docs/getting-started/configuration) section for a complete list of environment variables.

### File Volume Mounts

When running GoMFT in Docker, you'll need to mount volumes to provide access to the files you want to transfer. Here are common volume mount scenarios:

#### For SFTP/FTP Source Files
```bash
-v /path/to/local/files:/sftp/files
```

#### For Destination Directories
```bash
-v /path/to/destination:/mft/destination
```

#### For Processing Temporary Files
```bash
-v /path/to/temp:/mft/temp
```

Example using Docker Run with file volumes:
```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  -v /path/to/local/files:/sftp/files \
  -v /path/to/destination:/mft/destination \
  starfleetcptn/gomft:latest
```

Example Docker Compose configuration with file volumes:
```yaml
services:
  gomft:
    image: starfleetcptn/gomft:latest
    container_name: gomft
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
      - ./source_files:/sftp/files
      - ./destination:/mft/destination
      - ./temp:/mft/temp
    environment:
      - TZ=UTC
    restart: unless-stopped
```

> **Note**: Ensure the container has appropriate permissions to access the mounted directories. You may need to adjust host-side permissions accordingly.

## Traditional Installation

For environments where Docker is not available or preferred, you can install GoMFT directly.

### Prerequisites

- Go 1.20 or later
- Node.js 18 or later
- gcc (for building SQLite dependencies)
- templ (for generating template code)

### Building from Source

1. Clone the repository:

```bash
git clone https://github.com/StarFleetCPTN/GoMFT.git
cd GoMFT
```

2. Install Node.js dependencies:

```bash
npm install
```

3. Build the frontend assets:

```bash
npm run build
```

4. Install templ if you haven't already:

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

5. Generate templ templates:

```bash
templ generate
```

6. Build the Go application:

```bash
go build -o gomft
```

7. Run the application:

```bash
./gomft
```

## Verifying the Installation

After installation, access the GoMFT web interface by navigating to:

```
http://localhost:8080
```

The default login credentials are:

- **Username**: admin
- **Password**: admin

**Important**: Change the default password immediately after the first login for security reasons.

## Next Steps

Once GoMFT is installed, proceed to the [Quick Start](/docs/getting-started/quick-start) guide to begin configuring your file transfers. 