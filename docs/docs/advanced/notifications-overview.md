---
sidebar_position: 1
title: Notifications Overview
---

# Notifications Overview

GoMFT provides a comprehensive notification system to keep you informed about important events in your file transfer workflows. This page provides an overview of the notification system and the different notification types available.

## Notification System

The notification system in GoMFT is designed to be:

- **Flexible**: Choose from multiple notification channels
- **Configurable**: Set up different notifications for different events
- **Reliable**: Ensure critical events are always reported
- **Secure**: Protect sensitive information in notifications

## Notification Triggers

Notifications can be triggered by various events in GoMFT:

### Transfer-Related Events

- **Transfer Completion**: When a file transfer is successfully completed
- **Transfer Failure**: When a file transfer fails for any reason
- **Transfer Start**: When a file transfer begins
- **Transfer Threshold**: When a transfer exceeds a defined duration threshold

### Schedule-Related Events

- **Schedule Execution**: When a scheduled task runs
- **Schedule Failure**: When a scheduled task fails to run
- **Schedule Creation/Modification**: When schedules are created or modified

<!-- ### System Events

- **System Warnings**: Alerts about system resource usage (disk space, memory, etc.)
- **Service Status Changes**: When system services change state
- **Authentication Events**: Failed login attempts or other security events
- **Database Events**: Database backup completion, migration, or issues -->

## Notification Types

GoMFT supports multiple notification types to ensure you can receive alerts through your preferred channels:


### Webhook Notifications

Send HTTP requests to external systems or services when events occur. Features include:
- Configurable HTTP methods (POST, PUT, PATCH)
- JSON or XML payload formats
- Support for authentication
- Customizable retry strategy for improved reliability
[Learn more about Webhook Notifications](./webhook-notifications)

### Mobile Push Notifications

Receive notifications directly on your mobile devices:

#### Ntfy Notifications
- Simple HTTP-based push notifications to phones and desktops
- Customizable priority levels and notification tags
- Support for self-hosted or cloud-based ntfy servers
[Learn more about Ntfy Notifications](./ntfy-notifications)

#### Pushover Notifications
- Real-time push notifications to all your devices
- Priority levels for urgent notifications
- Custom sounds and delivery options
[Learn more about Pushover Notifications](./pushover-notifications)

#### Pushbullet Notifications
- Cross-platform notifications across all your devices
- Optional end-to-end encryption
- Support for notification mirroring
[Learn more about Pushbullet Notifications](./pushbullet-notifications)

#### Gotify Notifications
- Self-hosted push notification service
- Customizable priority levels
- Private and secure notification delivery
[Learn more about Gotify Notifications](./gotify-notifications)

## Notification Templates

Each notification type uses customizable templates to format the notification content. Templates support variables that are replaced with actual values when the notification is sent.

Common template variables include:

- `{{transfer_name}}`: Name of the transfer
- `{{transfer_status}}`: Status of the transfer (success, failure, etc.)
- `{{start_time}}`: When the transfer started
- `{{end_time}}`: When the transfer completed
- `{{duration}}`: How long the transfer took
- `{{total_files}}`: Number of files transferred
- `{{total_size}}`: Total size of transferred data
- `{{error_message}}`: Detailed error information (for failures)

## Notification Management

### Configuration

Notifications are configured at multiple levels:

1. **Global Level**: Default notification settings for all transfers
2. **Transfer Level**: Specific notification settings for individual transfers
3. **Schedule Level**: Notification settings for scheduled transfers

### Notification History

GoMFT maintains a history of sent notifications, allowing you to:

- Review past notifications
- Verify notification delivery
- Resend notifications if needed
- Audit notification patterns

## Getting Started with Notifications

To start using GoMFT notifications:

1. Navigate to **Settings** > **Notifications**
2. Configure your preferred notification channels
3. Test each notification channel
4. Apply notifications to specific transfers or schedules

For specific notification types, refer to the corresponding documentation pages in this section. 