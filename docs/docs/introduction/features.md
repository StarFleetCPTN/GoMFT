---
sidebar_position: 2
title: Features
---

# GoMFT Features

GoMFT offers a comprehensive set of features that make it a powerful solution for managed file transfers. Here's a detailed breakdown of what GoMFT offers:

## Core Features

### Multi-Protocol Support

GoMFT leverages rclone to support a wide range of storage providers and protocols:

- **Cloud Storage**: Amazon S3, Google Cloud Storage
- **Object Storage**: MinIO, Backblaze B2, Wasabi
- **FTP/SFTP**: FTP, FTPS, SFTP servers
- **WebDAV**: WebDAV servers and services
- **Local Storage**: Local disk, SMB/CIFS shares
- **And many more**: Over 40 storage systems supported

### Intuitive Web Interface

- **Clean, Modern UI**: Easy-to-use web interface built with Tailwind CSS and HTMX
- **Dashboard**: Overview of recent transfers, scheduled jobs, and system status
- **Configuration Manager**: Visual interface for creating and editing transfer configurations
- **Job Scheduler**: Interface for creating and managing scheduled jobs
- **Transfer Logs**: Detailed logs of all transfer operations
- **Dark Mode**: Support for light and dark themes

### Powerful Scheduling

- **Cron-style Scheduling**: Set up transfers using familiar cron syntax
- **Recurring Transfers**: Schedule transfers to run on a regular basis
- **One-time Transfers**: Run transfers immediately or at a specific time
- **Schedule Grouping**: Organize schedules into logical groups
- **Priority Control**: Set priority levels for scheduled tasks

## Advanced Features

### Transfer Options

- **Bidirectional Sync**: Synchronize files in both directions
- **File Filtering**: Include or exclude files based on patterns
- **Bandwidth Limiting**: Restrict bandwidth usage for transfers
- **Parallel Transfers**: Configure the number of simultaneous transfers
- **Delta Transfers**: Transfer only changed parts of files
- **Checksumming**: Verify file integrity during transfers

### Notification System

- **Notifications**: Receive alerts when transfers complete or fail
- **Custom Templates**: Customize notification content and format
- **Notification Rules**: Configure which events trigger notifications
- **Notification Providers**: Webhooks, Ntfy, Gotify, Pushover, Pushbullet

### Admin Tools

- **User Management**: Create and manage users with different roles
- **Role-Based Access Control**: Control access to different parts of the application
- **Audit Logging**: Track user actions for security and compliance
- **System Monitoring**: Monitor system performance and resource usage
- **Database Backup/Restore**: Back up and restore the application database
- **Log Viewer**: Browse and search through application logs

### Security Features

- **Authentication**: Secure login with optional MFA support
- **Encryption**: Encrypt data in transit and at rest
- **Secure Credential Storage**: Safely store connection credentials
- **Non-Root Container Support**: Run containers as non-root users for enhanced security

## Integration Capabilities

- **Docker Support**: Easy deployment with Docker containers
- **Docker Compose**: Multi-container deployment using Docker Compose
- **Reverse Proxy Compatible**: Works behind reverse proxies like Nginx or Traefik 