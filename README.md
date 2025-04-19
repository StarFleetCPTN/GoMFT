<p align="center">
  <img src="static/img/logo.svg" alt="GoMFT Logo" width="200">
</p>

<h1 align="center">GoMFT - Go Managed File Transfer</h1>

GoMFT is a web-based managed file transfer application built with Go, leveraging rclone for robust file transfer capabilities. It provides a user-friendly interface for configuring, scheduling, and monitoring file transfers across various storage providers.

<p align="center">
  <a href="https://discord.gg/f9dwtM3j">
    <img src="https://img.shields.io/discord/1351354052654403675?color=7289da&logo=discord&logoColor=white&label=Discord" alt="Join our Discord server!" />
  </a>
  <a href="https://starfleetcptn.github.io/GoMFT/">
    <img src="https://img.shields.io/badge/docs-online-blue.svg" alt="Documentation" />
  </a>
</p>

> [!WARNING]  
> This application is actively under development. As such, any aspect of the application—including configurations, data structures, and database fields—may change rapidly and without prior notice. Please review all release notes thoroughly before updating.

---

## Screenshots

<table>
<tr>
  <td width="33%">
    <a href="screenshots/dashboard.gomft.png">
      <img src="screenshots/dashboard.gomft.png" alt="Dashboard Overview" width="100%">
    </a>
    <p align="center"><em>Dashboard showing active transfers</em></p>
  </td>
  <td width="33%">
    <a href="screenshots/dashboard.dark.gomft.png">
      <img src="screenshots/dashboard.dark.gomft.png" alt="Dashboard Dark Mode" width="100%">
    </a>
    <p align="center"><em>Dashboard dark mode</em></p>
  </td>
</tr>
</table>

---

## Features

- **Multiple Storage Support**: Leverage rclone's extensive support for cloud storage providers:
  - Google Drive
  - Google Photos
  - Amazon S3
  - MinIO
  - NextCloud
  - WebDAV
  - SFTP
  - FTP
  - SMB/CIFS shares
  - Hetzner Storage Box
  - Backblaze B2
  - Wasabi
  - Local filesystem
  - And more via rclone
- **Multiple Notification Services**: Get job status updates through various notification channels:
  - Email notifications with configurable SMTP settings
  - Webhooks with authentication for custom integrations
  - Pushbullet notifications with optional device targeting
  - Ntfy.sh (both public and self-hosted) for simple push notifications
  - Gotify server integration for self-hosted notifications
  - Pushover notifications with customizable sounds and priorities
  - Configurable message templates for all notification types
  - Event-based triggers (job start, completion, errors)
- **Scheduled Transfers**: Configure transfers using cron expressions with flexible scheduling options
- **Transfer Monitoring**: Real-time status updates and detailed transfer logs with bytes and files transferred statistics
- **File Metadata Tracking**: Complete history and status of all transferred files with detailed information:
  - Process status (processed, archived, deleted)
  - File size and hash information
  - Advanced search and filtering capabilities
  - Metadata retention for compliance and auditing
  - Detailed file view with processing timestamps and job association
  - Powerful filtering by status, filename, job, and date ranges
  - Advanced search interface with multiple criteria
  - Bulk management and record deletion capabilities
  - Responsive design with mobile-friendly interface
- **Multi-threaded File Transfers**: Significantly improve performance with concurrent file processing:
  - Configurable number of concurrent transfers (1-32) per job
  - Automatic queue management to prevent system overload
  - Independent configuration for each transfer job
  - Optimized for both high-volume small files and large file transfers
  - Maximizes bandwidth utilization for cloud storage providers
- **Web Interface**: User-friendly interface for managing transfers, built with Templ components
- **File Pattern Matching**: Support for file patterns to filter files during transfers
- **File Output Patterns**: Dynamic naming of destination files using patterns with date variables
- **Archive Function**: Option to archive transferred files for backup and compliance
- **Transfer Configurations**: Full control over source and destination connection parameters
- **Job Management**: Create, edit, and monitor transfer jobs with scheduling
- **Security**: Role-based access control with admin-managed user accounts and secure password management
- **Authentication Providers**: Flexible authentication options:
  - Built-in email/password authentication
  - Authentik integration for enterprise SSO
  - OpenID Connect (OIDC) support for standard identity providers
  - OAuth2 integration for popular providers (Google, GitHub, etc.)
  - Multiple provider support with fallback options
  - Automatic user provisioning from external providers
  - Role mapping from external identity providers
- **Password Recovery**: Self-service password reset via email with secure token-based authentication
- **User Profile Management**: Personal settings including theme preferences
- **Modern UI**: Built with Templ, HTMX and Tailwind CSS for a responsive experience
- **Docker Support**: Easy deployment with Docker images and Docker Compose support
- **Portable Deployment**: Run on any platform that supports Docker or Go

---

## Prerequisites

- Go 1.21 or later
- rclone installed and configured
- SQLite 3

---

## Installation

### Standard Installation

1. Clone the repository:
```bash
git clone https://github.com/starfleetcptn/gomft.git
cd gomft
```

2. Install dependencies:
```bash
go mod download
go install github.com/a-h/templ/cmd/templ@latest
```

3. Generate template code:
```bash
templ generate
```

4. Build the application:
```bash
go build -o gomft
```

### Docker Installation

GoMFT is available as a Docker image for quick and easy deployment.

1. Pull the latest image from Docker Hub:
```bash
docker pull starfleetcptn/gomft:latest
```

2. Run the container:

#### Basic run
```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  starfleetcptn/gomft:latest
```

#### Run with specific user ID and group ID (using environment variables)
```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  -e PUID=$(id -u) \
  -e PGID=$(id -g) \
  starfleetcptn/gomft:latest
```

#### Or specify user IDs directly
```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  -e PUID=1001 \
  -e PGID=1001 \
  starfleetcptn/gomft:latest
```

#### Using a .env file for configuration
```bash
docker run -d \
  --name gomft \
  -p 8080:8080 \
  -v /path/to/data:/app/data \
  -v /path/to/backups:/app/backups \
  -v /path/to/.env:/app/.env \
  -e PUID=$(id -u) \
  -e PGID=$(id -g) \
  starfleetcptn/gomft:latest
```
3. Access the web interface at `http://localhost:8080`

#### Docker Compose Example

For production deployments, you can use Docker Compose with environment variables:

```yaml
version: '3'
services:
  gomft:
    image: starfleetcptn/gomft:latest
    container_name: gomft
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
      - ./.env:/app/.env
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=UTC
      - SERVER_ADDRESS=:8080
      - DATA_DIR=/app/data
      - BACKUP_DIR=/app/backups
      - JWT_SECRET=change_this_to_a_secure_random_string
      - BASE_URL=http://localhost:8080
      - GOOGLE_CLIENT_ID=your_google_client_id
      - GOOGLE_CLIENT_SECRET=your_google_client_secret
      - TOTP_ENCRYPTION_KEY=your_32_byte_encryption_key_here
      - GOMFT_ENCRYPTION_KEY=your_32_byte_encryption_key_here
      - EMAIL_ENABLED=true
      - EMAIL_HOST=smtp.example.com
      - EMAIL_PORT=587
      - EMAIL_FROM_EMAIL=gomft@example.com
      - EMAIL_FROM_NAME=GoMFT
      - EMAIL_ENABLE_TLS=true
      - EMAIL_REQUIRE_AUTH=true
      - EMAIL_USERNAME=smtp_username
      - EMAIL_PASSWORD=smtp_password
      - LOGS_DIR=/app/data/logs
      - LOG_MAX_SIZE=10
      - LOG_MAX_BACKUPS=5
      - LOG_MAX_AGE=30
      - LOG_COMPRESS=true
      - LOG_LEVEL=info
    # The user directive is no longer needed when using PUID/PGID environment variables
```
Save this as `docker-compose.yml` and run:

```bash
docker-compose up -d
```

For more information and available tags, visit the [GoMFT Docker Hub page](https://hub.docker.com/r/starfleetcptn/gomft).

Full documentation is available at [https://starfleetcptn.github.io/GoMFT/](https://starfleetcptn.github.io/GoMFT/).

## License

[MIT License](LICENSE) - see the full license terms

The GoMFT logo is licensed under the Creative Commons Attribution 4.0 International Public License.

The gopher design is from https://github.com/egonelbre/gophers.

The original Go gopher was designed by Renee French (http://reneefrench.blogspot.com/).