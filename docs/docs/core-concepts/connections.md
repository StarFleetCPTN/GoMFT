---
sidebar_position: 2
title: Connections
---

# Connection Management

Connections in GoMFT are configurations that define how to access different storage systems. Before you can transfer files, you need to set up connections for your source and destination systems.

## Supported Connection Types

GoMFT leverages rclone as its transfer engine, supporting a wide range of storage systems:

### Cloud Storage

- **Amazon S3**: Amazon's object storage service
- **Google Cloud Storage**: Google's object storage service
- **Backblaze B2**: Affordable cloud object storage
- **Wasabi**: Hot cloud storage

### File Transfer Protocols

- **FTP**: File Transfer Protocol
- **SFTP**: SSH File Transfer Protocol
- **WebDAV**: Web Distributed Authoring and Versioning

### Local Storage

- **Local Disk**: Files on the server running GoMFT
- **SMB/CIFS**: Windows file sharing

## Creating a Connection

To create a new connection:

1. Navigate to the **Transfer Configuration** section in the sidebar
2. Click **+ New Configuration**
3. Select the connection type from the dropdown menu
4. Fill in the required details for your selected type
5. Click **Test Source/Destination** to verify the connection works
6. Click **Create Configuration** to store the configuration

## Connection Configuration Fields

Different connection types require different configuration fields. Here are some common examples:

### Amazon S3 Connection

- **Name**: A descriptive name for the connection
- **Access Key ID**: AWS access key
- **Secret Access Key**: AWS secret key
- **Region**: AWS region (e.g., us-east-1)
- **Endpoint**: Optional custom endpoint for S3-compatible services
- **Bucket**: Default bucket to use (optional)
- **Path Prefix**: Default path prefix within the bucket (optional)

### SFTP Connection

- **Name**: A descriptive name for the connection
- **Host**: Server hostname or IP address
- **Port**: Server port (usually 22)
- **Username**: SFTP username
- **Authentication Method**: Password or SSH Key
- **Password**: User password (if using password authentication)
- **SSH Key**: Private SSH key (if using key authentication)
- **SSH Key Passphrase**: Passphrase for SSH key (if applicable)

### Local Storage Connection

- **Name**: A descriptive name for the connection
- **Path**: Base path on the local filesystem

## Connection Security

GoMFT follows best practices for handling connection credentials:

- **Encryption**: All sensitive credentials are encrypted at rest
- **Access Control**: Connections are protected by user permissions
- **Masked Values**: Passwords and secret keys are masked in the UI
- **Key Management**: SSH keys and other credentials are securely stored

## Managing Connections

### Viewing Connections

The **Transfer Configurations** page displays all configured connections with:
- Configuration name
- Configuration type
- Last updated date

### Editing Transfer Confirgurations

To edit an existing connection:
1. Navigate to the **Transfer Confiruations** section
2. Find the config you want to edit
3. Click the **Edit** button
4. Modify the config details
5. Test the updated configuration
6. Save your changes

### Deleting Transfer Configurations

To delete a connection:
1. Navigate to the **Transfer Configurations** section
2. Find the config you want to delete
3. Click the **Delete** button
4. Confirm the deletion

**Note**: You cannot delete connections that are in use by active transfers or schedules.

## Testing Transfer Configurations

GoMFT includes a configuration testing feature to verify connectivity:

1. After entering details for a configuration, click **Test Source/Destination**
2. GoMFT will attempt to authenticate with the remote system
3. For file storage, it will also verify read/write permissions
4. Results will display showing success or failure details

## Best Practices

- **Use descriptive names** for connections to easily identify them
- **Test connections regularly** to ensure they still work
- **Rotate credentials** periodically for enhanced security
- **Use service accounts** rather than personal accounts when possible
- **Document connection details** in the description field
- **Use the minimal required permissions** for enhanced security
- **Organize connections** using consistent naming conventions 