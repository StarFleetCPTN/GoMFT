---
sidebar_position: 3
title: Schedules
---

# Schedule Management

GoMFT's scheduling system allows you to automate file transfers to run at specific times or on a recurring basis. This section explains how to create, manage, and monitor scheduled transfers.

## Schedule Types

GoMFT supports several types of schedules:

### One-Time Schedules

Run a transfer once at a specific date and time.

### Recurring Schedules

Run a transfer repeatedly according to a defined pattern:

- **Hourly**: Run every hour at a specific minute
- **Daily**: Run every day at a specific time
- **Weekly**: Run on specific days of the week
- **Monthly**: Run on specific days of the month
- **Custom**: Define a custom schedule using cron syntax

## Creating a Schedule

To create a new schedule:

1. Navigate to the **Schedules** section in the sidebar
2. Click **Create New Schedule**
3. Select the transfer to schedule
4. Choose the schedule type
5. Configure the schedule details
6. Set additional options
7. Click **Save Schedule**

## Schedule Configuration

### Basic Configuration

- **Name**: A descriptive name for the schedule
- **Transfer**: The transfer configuration to run
- **Enabled**: Toggle to enable or disable the schedule
- **Schedule Type**: One-time or recurring

### One-Time Schedule Options

- **Date**: The date to run the transfer
- **Time**: The time to run the transfer

### Recurring Schedule Options

#### Simple Options

- **Frequency**: Hourly, Daily, Weekly, Monthly, or Custom
- **Time**: The time to run (for Daily, Weekly, Monthly)
- **Days**: The days to run (for Weekly, Monthly)
- **Minutes**: The minute to run (for Hourly)

#### Advanced Options (Cron Syntax)

For more complex scheduling needs, you can use cron syntax:

```
┌─────────── minute (0 - 59)
│  ┌──────── hour (0 - 23)
│  │  ┌────── day of month (1 - 31)
│  │  │  ┌──── month (1 - 12)
│  │  │  │  ┌── day of week (0 - 6) (Sunday to Saturday)
│  │  │  │  │                  
│  │  │  │  │
│  │  │  │  │
*  *  *  *  *
```

Examples:
- `0 2 * * *`: Every day at 2:00 AM
- `0 9-17 * * 1-5`: Every hour from 9 AM to 5 PM, Monday to Friday
- `*/15 * * * *`: Every 15 minutes
- `0 0 1,15 * *`: 1st and 15th of every month at midnight

### Additional Options

- **Timeout**: Maximum duration for the transfer (after which it will be terminated)
- **Retry Count**: Number of times to retry on failure
- **Retry Delay**: Time to wait between retry attempts
- **Priority**: Schedule priority (higher priority schedules run first when multiple are due)
- **Description**: Additional notes about the schedule

## Schedule Groups

GoMFT allows you to organize schedules into logical groups:

1. Navigate to **Schedule Groups** in the Schedules section
2. Create a new group with a name and description
3. Assign schedules to the group
4. View and manage grouped schedules together

Benefits of groups:
- Organize related schedules
- Apply batch operations to multiple schedules
- Monitor group-level statistics

## Managing Schedules

### Viewing Schedules

The **Schedules** page displays all configured schedules with:
- Schedule name
- Associated transfer
- Next run time
- Last run status
- Enabled/disabled status

### Editing Scheduled Jobs

To edit an existing schedule:
1. Navigate to the **Scheduled Jobs** section
2. Find the schedule you want to edit
3. Click the **Edit** button
4. Modify the schedule details
5. Save your changes

### Enabling/Disabling Scheduled Jobs

To temporarily disable a schedule without deleting it:
1. Navigate to the **Scheduled Job** section
2. Find the schedule you want to disable
3. Click edit
3. Toggle the **Enabled** switch to Off and save
4. The schedule will remain configured but won't run until re-enabled

### Deleting Schedules

To delete a schedule:
1. Navigate to the **Scheduled Job** section
2. Find the schedule you want to delete
3. Click the **Delete** button
4. Confirm the deletion

## Schedule Execution

When a schedule runs, GoMFT performs these actions:

1. Identifies schedules due for execution
2. Prioritizes schedules based on priority setting
3. Creates execution jobs for the associated transfers
4. Monitors job execution
5. Records results in the history
6. Handles retries if configured and needed
7. Updates next run time for recurring schedules

## Monitoring Schedules

GoMFT provides several ways to monitor your scheduled transfers:

<!-- ### Schedule Calendar

View all scheduled transfers in a calendar view:
1. Navigate to **Schedule Calendar** in the Schedules section
2. See all upcoming scheduled transfers in a monthly, weekly, or daily view
3. Click on any scheduled transfer to see details or edit it -->

### Transfer History

View the execution history of your schedules:
1. Navigate to **Transfer History** on the sidebar
2. See when schedules ran, their status, and execution details
3. Filter by date range, status, or schedule name

## Schedule Notifications

Configure notifications for scheduled transfers:

1. Edit a schedule
2. Navigate to the **Notifications** tab
3. Configure email notifications or webhooks
4. Specify notification conditions (success, failure, or both)

## Best Practices

- **Use descriptive names** for schedules to easily identify them
- **Set appropriate timeouts** based on expected transfer duration
- **Configure retries** for critical transfers
- **Use schedule groups** to organize related schedules
- **Stagger schedules** to avoid resource contention
- **Set up notifications** for critical schedules
- **Review schedule history** regularly to identify issues
- **Disable schedules** instead of deleting them for temporary pauses 