---
sidebar_position: 2
title: Quick Start
---

# GoMFT Quick Start Guide

This guide will help you get up and running with GoMFT quickly. We'll cover logging in, creating your first connection configuration, and setting up a file transfer.

## Accessing the Web Interface

After installation, access the GoMFT web interface at `http://your-server:8080` (or the appropriate port if you've modified it).

1. Log in with the default credentials:
   - **Username**: admin@example.com
   - **Password**: admin

## Initial Dashboard

The dashboard provides an overview of:
- Recent transfer jobs
- Upcoming scheduled transfers
- System status
- Quick action buttons

![GoMFT Dashboard](../../img/dashboard.gomft.png)

## Creating Your First Transfer Configuration

1. Navigate to **Transfer Configurations** in the sidebar menu
2. Click **+ New Configuration**
3. Configure the transfer:
   - Select source and destination configurations
   - Specify source and destination paths
   - Choose the transfer type (Copy, Sync, Move, etc.)
   - Configure transfer options (file filtering, bandwidth limits, etc.)
4. Click **Save Transfer**

![Create Transfer](../../img/transfer.config.gomft.png)

## Create Your First Scheduled Job
1. Naviagate to **Scheduled Jobs** in the sidebar menu
2. Click **+ New Job**
3. Configure the job:
   - Specifiy schedule
   - Select Job(s) to run this can be 1 or more
   - Change job run order if needed

## Running a Transfer

Once you've created a transfer configuration, you can:

### Run On-Demand

1. Navigate to **Secheduled Jobs**
2. Find your transfer in the list
3. Click the **Run Now** button
4. The transfer will execute immediately

### Schedule a Transfer

1. Navigate to **Schedules**
2. Click **Create New Schedule**
3. Select your transfer configuration
4. Set the schedule using cron syntax or the schedule builder
5. Set additional options (timeout, max retries, etc.)
6. Click **Save Schedule**

## Monitoring Transfers

1. Navigate to **Transfer History** to view all past and ongoing transfers
2. Click on a specific transfer to view detailed information:
   - Transfer status
   - Start and end times
   - Files transferred
   - Bytes transferred
   - Errors (if any)
   - Transfer log

## Next Steps

Now that you've set up your first transfer, explore these additional features:

- [Docker Deployment](/docs/getting-started/docker) - For containerized deployment
- [Traditional Installation](/docs/getting-started/traditional) - For non-Docker environments
- [Transfer Concepts](/docs/core-concepts/transfers) - Learn more about transfer operations
- [Scheduling](/docs/core-concepts/schedules) - Advanced scheduling options
- [Monitoring](/docs/core-concepts/monitoring) - Advanced monitoring capabilities 