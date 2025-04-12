---
sidebar_position: 3
title: Webhook Notifications
---

# Webhook Notifications

Webhook notifications in GoMFT provide a powerful way to integrate with external systems by sending HTTP requests when events occur. This enables automation workflows and integration with your existing tools and services.

## Overview

Webhooks allow GoMFT to:

- Send real-time notifications to external systems
- Trigger automated workflows in third-party applications
- Integrate with custom applications or services
- Provide machine-readable event data (JSON or XML)

## Configuration

### Global Webhook Settings

To configure webhook notifications:

1. Navigate to **Settings** > **Notification Services** > **Webhooks**
2. Configure the following settings:
   - **Default Webhook URL**: The base URL for webhook requests
   - **HTTP Method**: POST (default), PUT, or PATCH
   - **Content Type**: application/json (default), application/xml, or custom
   - **Authentication**: None, Basic Auth, API Key, or Bearer Token
   - **Retry Strategy**: Number of retries and delay between retries
   - **Timeout**: Maximum wait time for responses

### Testing Webhook Connectivity

After configuring your webhook settings:

1. Click **Test Connection** to send a test webhook request
2. Review the response status and body from the server

## Webhook Payload

### Default JSON Payload

By default, GoMFT sends a JSON payload with information about the event:

```json
{
  "event_type": "transfer_completed",
  "timestamp": "2023-09-15T14:22:33Z",
  "transfer": {
    "id": "transfer-123",
    "name": "Daily Backup",
    "source": "local-server",
    "destination": "cloud-storage",
    "status": "success",
    "start_time": "2023-09-15T14:20:01Z",
    "end_time": "2023-09-15T14:22:33Z",
    "duration_seconds": 152,
    "files_transferred": 258,
    "total_size_bytes": 1073741824,
    "transfer_rate_bytes_per_second": 7064091
  },
  "schedule": {
    "id": "schedule-456",
    "name": "Daily Backup Schedule"
  }
}
```

For failure events, additional error information is included:

```json
{
  "event_type": "transfer_failed",
  "timestamp": "2023-09-15T14:22:33Z",
  "transfer": {
    "id": "transfer-123",
    "name": "Daily Backup",
    "source": "local-server",
    "destination": "cloud-storage",
    "status": "failed",
    "start_time": "2023-09-15T14:20:01Z",
    "end_time": "2023-09-15T14:22:33Z",
    "duration_seconds": 152,
    "error": {
      "code": "CONNECTION_ERROR",
      "message": "Failed to connect to destination: Connection timed out",
      "details": "TCP connection to cloud-storage:22 timed out after 60 seconds"
    }
  }
}
```

### Custom Payload Templates

You can customize the webhook payload using templates:

1. Navigate to the webhook configuration
2. Switch from **Default Payload** to **Custom Payload**
3. Edit the JSON or XML template

Example custom template:

```json
{
  "alert": {
    "type": "{{event_type}}",
    "system": "GoMFT",
    "environment": "{{environment}}",
    "details": {
      "transfer_name": "{{transfer_name}}",
      "status": "{{status}}",
      "time": "{{timestamp}}",
      "size_mb": "{{total_size_mb}}"
    }{% if status == "failed" %},
    "error": "{{error_message}}"{% endif %}
  }
}
```

## Troubleshooting

### Common Issues

- **Connection Refused**: Check network connectivity and firewall rules
- **Authentication Failed**: Verify credentials and authentication method
- **Timeout Issues**: Increase timeout settings for slow endpoints
- **Invalid Payload**: Validate your custom template syntax
- **HTTP Error Codes**: Check destination service logs for details

### Webhook Logs

GoMFT logs all webhook attempts:

1. Navigate to **Administartion** > **Log Viewer**
2. Filter for "webhook" to see relevant log entries
3. Review request and response details for troubleshooting

## Best Practices

- **Use HTTPS** for all webhook endpoints
- **Implement Retries** for important notifications
- **Monitor Webhook Deliveries** to ensure reliability
- **Set Up Fallback Notification Methods** for critical transfers
- **Validate Webhook Payloads** on the receiving end
- **Keep Webhook Processing Fast** to avoid timeouts 