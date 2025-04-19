---
sidebar_position: 8
title: Command Line Tools
---

GoMFT provides a command line tool called `gomftctl` that allows administrators to perform various management tasks without using the web interface. This tool is particularly useful for automation, scripting, and performing administrative tasks in environments where the web UI is not accessible.

## Installation

The `gomftctl` tool is included with your GoMFT installation. You can find it in the root directory of your GoMFT installation.

If you need to build it manually, you can do so with:

```bash
cd /path/to/gomft
go build -o gomftctl ./cmd/gomftctl
```

### Using with Docker

If you're running GoMFT in a Docker container, the `gomftctl` tool is already included in the container. You can run it using the `docker exec` command:

```bash
# Replace gomft-container with your actual container name
docker exec -it gomft-container /app/gomftctl [command] [options]
```

For example, to view the version information:

```bash
docker exec -it gomft-container /app/gomftctl version
```

For commands that require stopping the application first (like key rotation), you'll need to:

1. Stop the container
2. Run the command in a new container using the same volumes
3. Restart the original container

```bash
# Stop the container
docker stop gomft-container

# Run a command using the same volumes
docker run --rm -v gomft_data:/app/data -v gomft_backups:/app/backups gomft/gomft:latest /app/gomftctl [command] [options]

# Restart the container
docker start gomft-container
```

## Available Commands

The `gomftctl` tool provides the following commands:

### Provider Data Migration

Migrate provider data from older versions of GoMFT to the new storage provider model:

```bash
./gomftctl migrate-providers [--dry-run] [--validate-only] [--force] [--backup-dir PATH] [--debug] [--auto-fill]
```

Options:
- `--dry-run`: Simulate migration without making changes
- `--validate-only`: Only validate if migration is possible without making changes
- `--force`: Force migration even if validation fails
- `--backup-dir`: Directory to store backup data (defaults to config backup_dir)
- `--debug`: Enable debug mode with more detailed error messages
- `--auto-fill`: Automatically fill missing required fields with placeholder values

### Security Key Rotation

Generate new security keys for the application:

```bash
./gomftctl rotate-key --type [jwt|totp|encryption] [--write]
```

Options:
- `--type`: Type of key to rotate (required)
  - `jwt`: JSON Web Token signing key
  - `totp`: TOTP encryption key
  - `encryption`: General encryption key used for sensitive data
- `--write`: Write the new key directly to .env file (otherwise just displays the key)

### Encryption Key Rotation

Rotate encryption keys for sensitive data stored in the database:

```bash
./gomftctl rotate-encryption-key [--dry-run] [--batch-size SIZE] [--max-errors NUM] [--backup-dir PATH] [--skip-backup] [--old-key-env VAR] [--models MODE]
```

This command will:
1. Create a backup of your database (unless `--skip-backup` is specified)
2. Re-encrypt all sensitive data with a new encryption key
3. Provide instructions for updating your configuration

**Important**: The application must be stopped before running this command to prevent data corruption.

Options:
- `--dry-run`: Simulate key rotation without making changes
- `--batch-size`: Number of records to process in each batch (default 100)
- `--max-errors`: Maximum number of errors before aborting (default 50)
- `--backup-dir`: Directory to store backup data (defaults to config backup_dir)
- `--skip-backup`: Skip database backup (not recommended)
- `--old-key-env`: Environment variable containing the old encryption key (defaults to GOMFT_ENCRYPTION_KEY)
- `--models`: Models to process (use 'auto' for automatic detection, default 'auto')

### Database Backup

Create a backup of the GoMFT database and configuration:

```bash
./gomftctl backup [--output-dir PATH]
```

Options:
- `--output-dir`: Directory to store backup files (defaults to config backup_dir)

### User Management

Commands for managing GoMFT users:

#### Create a new user

```bash
./gomftctl user create --email EMAIL --password PASSWORD [--admin]
```

Options:
- `--email`: User email address (required)
- `--password`: User password (required)
- `--admin`: Grant admin privileges to the user

#### Reset a user's password

```bash
./gomftctl user reset-password --email EMAIL --password PASSWORD
```

Options:
- `--email`: User email address (required)
- `--password`: New password (required)

#### List all users

```bash
./gomftctl user list
```

### Version Information

Display version information:

```bash
./gomftctl version
```

## Examples

### Migrating Provider Data

To migrate provider data with a dry run first:

```bash
# First do a dry run to see what would happen
./gomftctl migrate-providers --dry-run

# Then run the actual migration
./gomftctl migrate-providers
```

If you encounter errors due to missing required fields, you can use the auto-fill option:

```bash
# Migrate with auto-fill to handle missing required fields
./gomftctl migrate-providers --auto-fill

# For more detailed error information, add the debug flag
./gomftctl migrate-providers --auto-fill --debug
```

When using `--auto-fill`, the system will:
1. Automatically supply placeholder values for missing required fields
2. Mark providers with "[AUTO-FILLED]" in their names
3. Log warnings about which fields were auto-filled
4. Allow you to update the correct values after migration

### Rotating JWT Secret Key

To rotate the JWT secret key and update the .env file:

```bash
./gomftctl rotate-key --type jwt --write
```

### Rotating Encryption Key for Sensitive Data

To rotate the encryption key used for sensitive data in the database:

```bash
# First stop the GoMFT application
systemctl stop gomft

# Run a dry run to see what would be affected
./gomftctl rotate-encryption-key --dry-run

# Perform the actual key rotation
./gomftctl rotate-encryption-key

# Update your environment variable or .env file with the new key
# Then restart the application
systemctl start gomft
```

#### With Docker

To rotate encryption keys when running GoMFT in Docker:

```bash
# Stop the container
docker stop gomft-container

# Run a dry run to see what would be affected
docker run --rm -v gomft_data:/app/data -v gomft_backups:/app/backups gomft/gomft:latest /app/gomftctl rotate-encryption-key --dry-run

# Perform the actual key rotation
docker run --rm -v gomft_data:/app/data -v gomft_backups:/app/backups gomft/gomft:latest /app/gomftctl rotate-encryption-key

# Update your environment variables in your docker-compose.yml or run command
# Then restart the container
docker start gomft-container
```

### Creating an Admin User

To create a new admin user:

```bash
./gomftctl user create --email admin@example.com --password secure_password --admin
```

### Backing Up the Database

To create a backup of the database:

```bash
./gomftctl backup --output-dir /path/to/backup/directory
```

## Using in Scripts

The `gomftctl` tool is designed to be used in scripts and automation. For example, you could create a cron job to backup the database daily:

```bash
# Add to crontab
0 2 * * * /path/to/gomft/gomftctl backup --output-dir /path/to/backup/directory
```

Or you could create a script to rotate security keys periodically:

```bash
#!/bin/bash
# Stop the GoMFT service
systemctl stop gomft

# Rotate all security keys
/path/to/gomft/gomftctl rotate-key --type jwt --write
/path/to/gomft/gomftctl rotate-key --type totp --write
/path/to/gomft/gomftctl rotate-key --type encryption --write

# Rotate encryption key for sensitive data in the database
/path/to/gomft/gomftctl rotate-encryption-key

# Restart the GoMFT service to apply changes
systemctl restart gomft
```
