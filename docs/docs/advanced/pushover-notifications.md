---
sidebar_position: 6
title: Pushover Notifications
---

# Pushover Notifications

Pushover is a simple notification service that makes it easy to send real-time notifications to your Android and iOS devices, as well as desktop computers. GoMFT integrates with Pushover to deliver instant notifications about your file transfers and system events.

## Overview

Pushover integration in GoMFT provides:

- Real-time push notifications to mobile devices and desktops
- Prioritized notifications with different sounds and attention levels
- Customizable notification content with detailed transfer information
- Support for notification grouping by device or application

## Prerequisites

Before configuring Pushover notifications in GoMFT, you need:

1. A [Pushover account](https://pushover.net/)
2. Your Pushover user key
3. A registered Pushover application (API token)
4. Pushover app installed on your devices

## Configuration

### Registering a GoMFT Application in Pushover

1. Log in to your Pushover account at [pushover.net](https://pushover.net/)
2. Go to [Your Applications](https://pushover.net/apps/build)
3. Create a new application:
   - **Name**: GoMFT
   - **Type**: Application
   - **Description**: GoMFT File Transfer Notifications
   - **URL**: Your GoMFT instance URL (optional)
   - **Icon**: Upload a custom icon (optional)
4. After creation, you'll receive an **API Token/Key** for your application

### Global Pushover Settings

To configure Pushover notifications in GoMFT:

1. Navigate to **Settings** > **Notification Services** > **Add New** > **Pushover**
2. Configure the following settings:
   - **User Key**: Your Pushover user key
   - **API Token**: Your GoMFT application's API token
   - **Default Priority**: Default priority level (-2 to 2)
   - **Default Sound**: Sound for notifications
   - **Default Device**: Specific device or blank for all devices

### Testing Pushover Connection

After configuring your Pushover settings:

1. Click **Send Test Notification** to send a test notification to your devices

## Notification Content

### Priority Levels

Pushover supports different priority levels that GoMFT uses effectively:

| Priority | Level | Usage in GoMFT |
|----------|-------|----------------|
| -2 | Lowest | Silent transfer logs, debugging info |
| -1 | Low | Successful transfers, routine events |
| 0 | Normal | Standard notifications |
| 1 | High | Transfer failures, important alerts |
| 2 | Emergency | Critical system issues |

Note: Emergency priority (2) notifications will repeat until acknowledged by the user.

### Example Notifications

#### Successful Transfer

```
Title: Transfer Complete: Daily Backup
Message: Successfully transferred 123 files (1.45 GB) in 2:15
Priority: Normal (0)
Sound: pushover
```

#### Failed Transfer

```
Title: Transfer Failed: Daily Backup
Message: Error: Connection refused to destination server
Files: 45/123 processed
Size: 0.5/1.45 GB transferred
Error: Failed to connect to destination server
Priority: High (1)
Sound: siren
URL: https://gomft.example.com/transfers/123
URL Title: View Transfer Details
```

## Troubleshooting

### Common Issues

- **Incorrect API Token/User Key**: Verify your Pushover credentials
- **Message Rate Limiting**: Pushover has monthly message limits for free accounts
- **Device Not Receiving**: Check device registration and network connectivity
- **Emergency Notifications**: Verify retry/expire settings for emergency priority

### Pushover Logs

To troubleshoot notification issues:

1. Check the GoMFT logs: **Admin Tools** > **Logs** > filter for "pushover"
2. Review your Pushover account's message history
3. Check your device's Pushover app settings

## Best Practices

- **Use Appropriate Priority Levels** based on event importance
- **Reserve Emergency Priority** for truly critical issues
- **Group Related Notifications** when possible
- **Include Action URLs** for immediate access to relevant information
- **Consider Sound Selection** based on notification importance
- **Set Up Multiple Notification Methods** for critical systems

## Pushover vs. Other Notification Systems

| Feature | Pushover | Email | Gotify | Ntfy | Pushbullet |
|---------|----------|-------|--------|------|------------|
| Cost | Paid (one-time) | Free | Free | Free | Free/Paid |
| Self-hosting | No | Varies | Yes | Yes | No |
| Priority levels | Yes | No | Yes | Yes | No |
| Acknowledgment | Yes | No | No | No | No |
| Sound options | Yes | No | No | Limited | No |
| Delivery guarantee | High | Varies | Good | Good | Good |
| Device support | iOS, Android, Desktop | All | All | All | iOS, Android, Desktop | 