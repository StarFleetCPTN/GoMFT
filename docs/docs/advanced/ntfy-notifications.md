---
sidebar_position: 4
title: Ntfy Notifications
---

# Ntfy Notifications

Ntfy is a simple HTTP-based pub-sub notification service that allows you to send push notifications to your phone or desktop. GoMFT seamlessly integrates with Ntfy to deliver notifications about your file transfers and system events.

## Overview

Ntfy integration in GoMFT enables:

- Push notifications to mobile devices and desktops
- Choice between public ntfy.sh service or self-hosted Ntfy server
- Customizable notification topics, priorities, and tags
- Support for notification actions and attachments

## Prerequisites

Before configuring Ntfy notifications in GoMFT, you should:

1. Install the Ntfy app on your devices (available for Android, iOS, and desktop)
2. Subscribe to your chosen topic in the Ntfy app
3. Optionally set up your own Ntfy server for increased privacy

## Configuration

### Global Ntfy Settings

To configure Ntfy notifications in GoMFT:

1. Navigate to **Settings** > **Notification Services** > **Add New** > **Ntfy**
2. Configure the following settings:
   - **Ntfy Server URL**: The URL of the Ntfy server (default: `https://ntfy.sh`)
   - **Default Topic**: The notification topic your devices are subscribed to
   - **Default Priority**: Priority level for notifications (1-5)
   - **Authentication**: Access token or username/password if required
   - **Default Tags**: Icon tags for different notification types

### Testing Ntfy Connection

After configuring your Ntfy settings:

1. Click **Send Test Notification** to send a test notification to your devices

## Notification Content

### Priority Levels

Ntfy supports five priority levels that GoMFT uses effectively:

| Priority | Level | Usage in GoMFT |
|----------|-------|----------------|
| 1 | Min | Background information, debug notifications |
| 2 | Low | Successful transfers, routine events |
| 3 | Default | Standard notifications, warnings |
| 4 | High | Transfer failures, important alerts |
| 5 | Max | Critical system issues, emergency alerts |

### Notification Tags

GoMFT uses meaningful tags in Ntfy notifications to provide visual cues:

| Tag | Usage |
|-----|-------|
| `✅` | Successful transfers |
| `❌` | Failed transfers |
| `⚠️` | Warnings or transfers with issues |
| `🔄` | Transfer in progress |
| `🔍` | Monitoring events |
| `⚙️` | System events |

### Example Notifications

GoMFT sends structured notifications with helpful information:

#### Successful Transfer

```
Title: Transfer Completed: Daily Backup
Message: Successfully transferred 123 files (1.45 GB) in 2:15
Priority: 2 (Low)
Tags: ✅,📁
```

#### Failed Transfer

```
Title: Transfer Failed: Daily Backup
Message: Error: Connection refused to destination server
Files processed: 45/123
Size transferred: 0.5/1.45 GB
Priority: 4 (High)
Tags: ❌,📁
Click action: Open GoMFT
```

## Advanced Features

### Custom Templates

Customize notification content with templates:

```
Title: {{event_type}}: {{transfer_name}}
Message: {{status}} - {{files_transferred}} files ({{total_size}}) in {{duration}}
Priority: {% if status == "failed" %}4{% else %}2{% endif %}
Tags: {% if status == "success" %}✅{% else %}❌{% endif %},📁
```

## Self-Hosting Ntfy

For enhanced privacy and control, you can self-host your own Ntfy server:

1. Follow the [Ntfy self-hosting guide](https://docs.ntfy.sh/install/)
2. Update your GoMFT configuration to point to your self-hosted server
3. Configure authentication as needed

Example configuration for self-hosted Ntfy:

```yaml
ntfy:
  server_url: https://ntfy.example.com
  default_topic: gomft
  authentication:
    type: basic
    username: ${NTFY_USERNAME}
    password: ${NTFY_PASSWORD}
```

## Troubleshooting

### Common Issues

- **Notifications Not Arriving**: Verify you've subscribed to the correct topic
- **Authentication Errors**: Check credentials and authentication method
- **Connection Issues**: Ensure the Ntfy server is accessible from GoMFT
- **App Configuration**: Verify notification settings in your Ntfy app

### Ntfy Logs

To troubleshoot notification issues:

1. Check the GoMFT logs: **Administration** > **Log Viewer** > filter for "ntfy"
2. If self-hosting, check your Ntfy server logs
3. Verify your device has properly functioning notifications

## Best Practices

- **Use Unique Topics** to prevent unauthorized notifications
- **Set Appropriate Priorities** based on event importance
- **Consider Self-Hosting** for sensitive environments
- **Keep Topic Names Secret** as they act like passwords
- **Set Up Multiple Notification Methods** for critical systems 