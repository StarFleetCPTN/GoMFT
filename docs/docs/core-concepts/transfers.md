---
sidebar_position: 1
title: Transfers
---

# Transfer Operations

GoMFT's primary function is to manage file transfers between different storage systems. This page explains the transfer operations available in GoMFT and how to configure them.

> **Note**: GoMFT now supports the Storage Provider feature, which allows you to create reusable connection profiles for your transfers. For detailed information, see the [Storage Providers](/docs/user-guides/storage-provider-guide) guide.

## Transfer Types

GoMFT supports several types of transfer operations, each with different behaviors:

### Copy

The **Copy** operation copies files from the source to the destination. Files are only copied if they don't exist at the destination or if they've been modified at the source.

```
Source → Destination
```

### Sync

The **Sync** operation makes the destination identical to the source, adding, removing, and updating files as necessary.

```
Source → Destination (with deletions)
```

### Move

The **Move** operation copies files from the source to the destination and then deletes the source files after a successful transfer.

```
Source → Destination → Delete Source
```

### Bidirectional Sync

The **Bidirectional Sync** operation synchronizes files in both directions, ensuring that the newest version of each file is present in both locations.

```
Source ⟷ Destination
```

## Transfer Configuration

When creating a transfer in GoMFT, you need to configure the following elements:

### Basic Configuration

- **Name**: A descriptive name for the transfer
- **Description**: Optional details about the transfer's purpose
- **Source**: Either a direct connection configuration or a Storage Provider
- **Destination**: Either a direct connection configuration or a Storage Provider
- **Transfer Type**: Copy, Sync, Move, or Bidirectional Sync

#### Using Storage Providers

When creating a transfer, you can now select a Storage Provider instead of entering connection details directly:

1. In the Source or Destination section, select **Provider** from the dropdown
2. Choose from your available Storage Providers
3. Enter the path within the selected provider

This approach offers several benefits:
- Reuse the same provider across multiple transfers
- Update credentials in one place
- Enhanced security with AES-256 encryption for credentials

### Advanced Options

#### File Selection

- **Include Patterns**: Patterns for files to include (e.g., `*.txt`, `data/**/*.csv`)
- **Exclude Patterns**: Patterns for files to exclude (e.g., `*.tmp`, `**/._*`)
- **Min Size**: Minimum file size to transfer
- **Max Size**: Maximum file size to transfer
- **Min Age**: Only transfer files older than this
- **Max Age**: Only transfer files newer than this

#### Transfer Behavior

- **Checksum**: Compare files using checksums instead of size/date
- **Delete Before**: Delete destination files before transferring
- **Delete During**: Delete destination files during transfer
- **Delete After**: Delete destination files not in source after transfer
- **Update Existing**: Update existing files at destination
- **Skip New**: Skip new files not present at destination
- **Skip Newer**: Skip files that are newer at the destination

#### Performance Options

- **Transfers**: Number of concurrent file transfers
- **Checkers**: Number of concurrent file checkers
- **Bandwidth Limit**: Maximum bandwidth to use in bytes/s
- **Buffer Size**: Size of transfer buffer (default: 16MB)
- **Chunk Size**: Upload chunk size for chunked uploads

## Transfer Execution

### Manual Execution

Transfers can be run on-demand:

1. Navigate to the **Scheduled Jobs** section
2. Find your job in the list
3. Click **Run Now**
4. Monitor the job progress in real-time

### Scheduled Execution

Transfers can be scheduled to run automatically:

1. Navigate to the **Scheduled Jobs** section
2. Create a new schedule linked to your transfer configuration
3. Set up the schedule using cron syntax or the schedule builder
4. The transfer will run automatically according to the schedule

## Transfer Monitoring

### Status Indicators

- **Pending**: Transfer is waiting to start
- **Running**: Transfer is in progress
- **Completed**: Transfer finished successfully
- **Failed**: Transfer encountered an error
- **Canceled**: Transfer was manually canceled

### Transfer Details

For each transfer execution, GoMFT records:

- Start and end times
- Duration
- Number of files transferred
- Total bytes transferred
- Files skipped
- Errors encountered
- Detailed logs

## Transfer Logs

GoMFT provides detailed logs for each transfer:

1. Navigate to **Transfer History**
2. Click on a **View Details** button on the transfer entry

The logs include information about:

- Each file transferred
- Skipped files
- Errors
- Performance metrics
- Overall transfer summary

## Troubleshooting Failed Transfers

When a transfer fails, GoMFT provides information to help identify the cause:

1. Check the error message in the transfer history
2. Review the detailed logs for the specific error
3. For transfers using Storage Providers, you can test the provider connection directly from the Storage Providers section
4. Common issues include:
   - Permission problems
   - Network connectivity
   - Invalid credentials
   - Path not found
   - Disk space issues
   - Expired tokens (for OAuth providers like OneDrive or Google Drive)

## Best Practices

- **Use meaningful names** for your transfers to easily identify them
- **Start small** when testing new configurations
- **Use include/exclude patterns** to limit scope when working with large directories
- **Set appropriate concurrency** based on network conditions and system resources
- **Use checksumming** for critical data to ensure integrity
- **Set bandwidth limits** to avoid network congestion during peak hours
- **Schedule large transfers** during off-peak times
- **Use notifications** to stay informed about transfer results
- **Regularly review logs** to identify potential issues
- **Use Storage Providers** for reusable connections across multiple transfers
- **Convert existing transfers** to use Storage Providers for easier credential management 