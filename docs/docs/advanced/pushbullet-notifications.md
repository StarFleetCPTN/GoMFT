---
sidebar_position: 5
title: Pushbullet Notifications
---

# Pushbullet Notifications

Pushbullet is a cross-platform notification service that allows you to receive notifications on multiple devices. GoMFT integrates with Pushbullet to deliver timely notifications about your file transfers and system events.

## Overview

Pushbullet integration in GoMFT offers:

- Cross-platform notifications across your devices (Android, iOS, Chrome, Firefox, etc.)
- Option to send to all devices or specific devices
- Rich notification content with transfer details
- Support for notification mirroring between devices

## Prerequisites

Before configuring Pushbullet notifications in GoMFT, you need:

1. A Pushbullet account
2. Pushbullet API access token
3. Pushbullet app installed on your devices

## Configuration

### Global Pushbullet Settings

To configure Pushbullet notifications:

1. Navigate to **Settings** > **Notification Services** > **Add New** > **Pushbullet**
2. Configure the following settings:
   - **API Token**: Your Pushbullet access token
   - **Default Device**: The device identifier to send notifications to (optional)
   - **Default Type**: "Note" (default) or "Link"

### Getting Your Pushbullet API Token

1. Log in to your Pushbullet account at [pushbullet.com](https://www.pushbullet.com/)
2. Go to **Settings** > **Account**
3. In the **Access Tokens** section, click **Create Access Token**
4. Copy the generated token and paste it into GoMFT

### Testing Pushbullet Connection

After configuring your Pushbullet settings:

1. Click **Send Test Notification** to send a test notification to your devices

## Notification Content

### Notification Types

GoMFT supports two types of Pushbullet notifications:

#### Note Type

Simple notifications with a title and body:

```
Title: Transfer Complete: Daily Backup
Body: Successfully transferred 123 files (1.45 GB) in 2:15
```

#### Link Type

Notifications that include a link to the GoMFT interface:

```
Title: Transfer Failed: Daily Backup
Body: Error: Connection refused to destination server
URL: https://gomft.example.com/transfers/123
```

### Example Notifications

#### Successful Transfer

```
Title: Transfer Complete: Daily Backup
Body: Transfer completed successfully at 2023-09-15 14:22:33
      Files: 123
      Size: 1.45 GB
      Duration: 00:02:15
```

#### Failed Transfer

```
Title: Transfer Failed: Daily Backup
Body: Transfer failed with error: Connection refused
      Files Processed: 45/123
      Size Transferred: 0.5/1.45 GB
      Duration: 00:01:05
      Error: Failed to connect to destination server
URL: https://gomft.example.com/transfers/123
```

## Troubleshooting

### Common Issues

- **API Token Errors**: Verify your Pushbullet API token is correct
- **No Notifications Arriving**: Check device connectivity and Pushbullet app settings
- **Rate Limiting**: Pushbullet has API rate limits; spread out notification frequency
- **Device Selection Issues**: Verify device identifiers if targeting specific devices

### Pushbullet Logs

To troubleshoot notification issues:

1. Check the GoMFT logs: **Administration** > **Log Viewer** > filter for "pushbullet"
2. Review the Pushbullet account activity in your Pushbullet account

## Best Practices

- **Secure Your API Token**: Treat your Pushbullet API token as sensitive information
- **Group Related Notifications** to avoid notification fatigue
- **Include Action Links** for quick access to relevant GoMFT pages
- **Set Up Multiple Notification Methods** for critical systems

## Pushbullet Alternatives

If you encounter limitations with Pushbullet, GoMFT also supports:

- [Email Notifications](./email-notifications)
- [Gotify](./gotify-notifications) 
- [Ntfy](./ntfy-notifications)
- [Pushover](./pushover-notifications) 