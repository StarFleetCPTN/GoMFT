---
sidebar_position: 3
title: Admin Tools
---

# Admin Tools

GoMFT provides a comprehensive set of administrative tools for system management, monitoring, and maintenance. These tools help administrators maintain the system, troubleshoot issues, and ensure optimal performance.

## Accessing Admin Tools

Admin tools are available to users with administrator privileges:

1. Log in with an administrator account
2. Navigate to **Admin Tools** in the sidebar menu

## Log Viewer

The Admin Tools panel includes an integrated log viewer with the following features:

### Features

- **Log File Browser**: View a list of all available log files in the system
- **Real-time Log Viewing**: View log file contents directly in the web interface
- **Refresh Function**: Update the log list and content with the latest information
- **User-friendly Interface**: Clean, readable presentation with custom scrolling
- **Dark Mode Support**: Consistent theming with the rest of the application
- **Navigation**: Easily switch between different log files

### Available Logs

- **Application Logs**: General application logs
- **Transfer Logs**: Detailed logs of file transfer operations
- **Authentication Logs**: Login attempts and authentication events
- **Scheduler Logs**: Information about scheduled job execution
- **API Logs**: API usage and requests
- **Webhook Logs**: Records of webhook delivery attempts
- **Email Logs**: Email sending attempts and errors

### Using the Log Viewer

1. Select a log category from the dropdown menu
2. Choose a specific log file from the list
3. View the log content in the main panel
4. Use the search function to find specific text
5. Click **Refresh** to update with the latest entries

## Database Management

The Admin Tools interface also includes database management capabilities:

### Backup Management

- **Create Backup**: Generate a backup of the GoMFT database
- **Schedule Backups**: Configure automatic backup schedules
- **View Backups**: List all available backups with their dates and sizes
- **Download Backup**: Download a backup file for safekeeping
- **Restore Backup**: Restore the system from a previous backup

### Database Operations

- **Optimize Database**: Run maintenance tasks to optimize performance
- **Check Integrity**: Verify database integrity and identify issues
- **Vacuum Database**: Reclaim unused space in the database
- **View Statistics**: Get database size and table statistics

## System Information

The System Information panel provides a comprehensive overview of your GoMFT installation:

### System Stats

- **Version Information**: Current GoMFT version and build details
- **System Resources**: CPU, memory, and disk usage
- **Uptime**: System uptime and start time
- **Active Transfers**: Currently running transfers
- **Queued Transfers**: Transfers waiting to be executed
- **Database Size**: Current size of the database

### Health Checks

- **Service Status**: Status of all system services
- **Storage Space**: Available space in data directories
- **Connection Tests**: Tests for external services like SMTP
- **Rclone Status**: Verify rclone availability and version

## User Management

Administrators can manage user accounts and permissions:

### User Operations

- **Create User**: Add new users to the system
- **Edit User**: Modify existing user details and permissions
- **Deactivate User**: Temporarily disable user accounts
- **Delete User**: Permanently remove a user account
- **Reset Password**: Force password reset for a user

### Role Management

- **View Roles**: List all available roles and their permissions
- **Create Role**: Define custom roles with specific permissions
- **Edit Role**: Modify permissions for existing roles
- **Assign Roles**: Change role assignments for users

## System Settings

The System Settings section allows customization of various system parameters:

### General Settings

- **System Name**: Customize the application name
- **Base URL**: Set the base URL for the application
- **Time Zone**: Configure the system time zone
- **Date Format**: Set the preferred date and time format
- **Default Language**: Set the default interface language

### Security Settings

- **Password Policy**: Configure password complexity requirements
- **Session Timeout**: Set the inactive session timeout period
- **Failed Login Limit**: Set thresholds for account lockouts
- **API Token Management**: Configure API token policies

### Email Settings

- **SMTP Configuration**: Set up the mail server for notifications
- **Email Templates**: Customize notification email templates
- **Notification Rules**: Configure default notification settings

### Transfer Settings

- **Concurrency Limits**: Set maximum simultaneous transfers
- **Bandwidth Limits**: Configure default bandwidth limitations
- **Temporary Storage**: Configure temp directory for transfers
- **Transfer Timeouts**: Set default timeouts for transfers

## Maintenance Mode

Administrators can put the system into maintenance mode when needed:

<!-- ### Maintenance Options

- **Enable Maintenance Mode**: Temporarily restrict access to admin users
- **Scheduled Maintenance**: Schedule maintenance windows
- **Maintenance Message**: Customize the message shown to users
- **Allow Specific IPs**: Allow specific IP addresses during maintenance -->

## Import/Export

The system provides facilities for importing and exporting configuration:

### Import/Export Features

- **Export Configurations**: Export transfer configurations as JSON
- **Import Configurations**: Import configurations from JSON files
- **Migrate Settings**: Move settings between GoMFT instances
- **Bulk Operations**: Perform operations on multiple items

## Audit Logs

For security and compliance, GoMFT maintains comprehensive audit logs:

### Audit Log Features

- **User Actions**: Records of all user-initiated actions
- **System Events**: Important system-level events
- **Authentication Events**: Login, logout, and access attempts
- **Configuration Changes**: Changes to system configuration
- **Filtering**: Filter logs by user, action type, and date range
- **Export**: Export audit logs for compliance reporting

<!-- ## Troubleshooting Tools

The Admin Tools includes several utilities for troubleshooting:

### Troubleshooting Features

- **Test Connections**: Verify connectivity to remote systems
- **Check File Permissions**: Test access to file systems
- **Debug Mode**: Enable additional logging for troubleshooting
- **Transfer Simulation**: Test transfers without moving data
- **System Check**: Run a comprehensive system check  -->