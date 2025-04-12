---
sidebar_position: 3
title: Gotify Notifications
---

# Gotify Notifications

Gotify is a simple server for sending and receiving push notifications. GoMFT integrates with Gotify to provide real-time notifications for transfer events and system alerts.

## Overview

Gotify integration allows GoMFT to:

- Send push notifications to your self-hosted Gotify server
- Customize notification priority based on event importance
- Include detailed transfer information in notifications
- Support private and secure notification delivery

## Prerequisites

Before setting up Gotify notifications in GoMFT, you need:

1. A running Gotify server (self-hosted)
2. An application token from your Gotify server
3. Network connectivity between GoMFT and the Gotify server

## Configuration

### Global Gotify Settings

To configure Gotify notifications:

1. Navigate to **Settings** > **Notification Services** > **Add New** > **Gotify**
2. Configure the following settings:
   - **Gotify Server URL**: The URL of your Gotify server (e.g., `https://gotify.example.com`)
   - **Application Token**: The token for your GoMFT application in Gotify
   - **Default Priority**: The default priority level for notifications (1-10)
   - **Verify SSL**: Whether to verify SSL certificates (recommended for production)

### Testing Gotify Connection

After configuring your Gotify settings:

1. Click **Test Connection** to verify connectivity with your Gotify server
2. Click **Send Test Notification** to send a test message

## Notification Content

### Priority Levels

Gotify uses numeric priority levels that GoMFT leverages for different event types:

| Priority | Usage in GoMFT |
|----------|----------------|
| 1-3 | Low priority: successful transfers, routine events |
| 4-7 | Medium priority: warnings, transfers with issues |
| 8-10 | High priority: failed transfers, critical system issues |

### Example Notifications

GoMFT sends structured notifications with helpful information:

#### Successful Transfer

```
Title: Transfer Completed: Daily Backup
Message: Successfully transferred 123 files (1.45 GB) in 2:15
Priority: 3
```

#### Failed Transfer

```
Title: Transfer Failed: Daily Backup
Message: Error: Connection refused to destination server
Files processed: 45/123
Size transferred: 0.5/1.45 GB
Priority: 8
```

## Troubleshooting

### Common Issues

- **Connection Refused**: Ensure the Gotify server URL is correct and accessible
- **Authentication Failed**: Verify the Application Token is correct
- **SSL Certificate Errors**: Check the Verify SSL setting and certificate validity

### Gotify Logs

To troubleshoot notification issues:

1. Check the GoMFT logs: **Admin Tools** > **Logs** > filter for "gotify"
2. Review the Gotify server logs for any errors
3. Verify network connectivity between GoMFT and the Gotify server

## Best Practices

- **Use HTTPS** for your Gotify server to ensure secure communication
- **Set Appropriate Priorities** to differentiate between routine and critical notifications
- **Use Client Applications** on your devices to receive Gotify notifications
- **Set Up Multiple Notification Methods** for critical events 