---
id: storage-provider-guide
title: Storage Provider Guide
sidebar_label: Storage Providers
description: Detailed instructions for using the Storage Provider feature in GoMFT
---

# Storage Provider User Guide

This guide provides detailed instructions for using the new Storage Provider feature in GoMFT.

## Table of Contents

1. [Introduction](#introduction)
2. [Managing Storage Providers](#managing-storage-providers)
   - [Viewing Your Storage Providers](#viewing-your-storage-providers)
   - [Creating a New Storage Provider](#creating-a-new-storage-provider)
   - [Editing Storage Providers](#editing-storage-providers)
   - [Testing Connections](#testing-connections)
   - [Deleting Storage Providers](#deleting-storage-providers)
3. [Using Storage Providers in Transfers](#using-storage-providers-in-transfers)
   - [Creating Transfers with Storage Providers](#creating-transfers-with-storage-providers)
   - [Converting Existing Transfers](#converting-existing-transfers)
4. [Provider Type Reference](#provider-type-reference)
   - [SFTP Configuration](#sftp-configuration)
   - [S3 Configuration](#s3-configuration)
   - [OneDrive Configuration](#onedrive-configuration)
   - [Google Drive Configuration](#google-drive-configuration)
   - [FTP Configuration](#ftp-configuration)
   - [SMB Configuration](#smb-configuration)
5. [Troubleshooting](#troubleshooting)
   - [Common Connection Issues](#common-connection-issues)
   - [Error Messages](#error-messages)
6. [FAQ](#faq)

## Introduction

The Storage Provider feature allows you to securely store and manage credentials for various storage systems. Instead of entering connection details each time you create a transfer, you can now create reusable storage provider profiles. This approach offers several benefits:

- **Improved Security**: Credentials are stored securely using AES-256 encryption
- **Simplified Management**: Update credentials in one place instead of in each transfer
- **Easier Testing**: Test connections before creating transfers
- **Reusability**: Use the same provider for multiple transfers

## Managing Storage Providers

### Viewing Your Storage Providers

To view your storage providers:

1. Navigate to the **Storage Providers** section in the left sidebar
2. You'll see a list of all storage providers you have created
3. The list shows the provider name, type, and creation date

### Creating a New Storage Provider

To create a new storage provider:

1. From the Storage Providers page, click the **Add Provider** button
2. Enter a descriptive name for the provider
3. Select the provider type from the dropdown (SFTP, S3, OneDrive, etc.)
4. Fill in the required fields for the selected provider type
5. Click **Save** to create the provider or **Save & Test** to create and test the connection

#### Example: Creating an S3 Provider

1. Name: "Company AWS S3 Bucket"
2. Type: S3
3. Fill in the required fields:
   - Access Key: Your AWS access key
   - Secret Key: Your AWS secret key
   - Region: e.g., us-west-2
   - Bucket: Your bucket name
   - Endpoint: Leave blank for AWS S3 or specify for S3-compatible services
4. Click **Save & Test**

### Editing Storage Providers

To edit an existing storage provider:

1. From the Storage Providers list, click the **Edit** button next to the provider
2. Update the fields as needed
3. For security reasons, sensitive fields (passwords, secret keys) appear empty
   - Leave these fields empty to keep the existing values
   - Enter new values only if you want to change them
4. Click **Save** to update the provider

### Testing Connections

Testing your storage provider connections ensures they're properly configured:

1. From the Storage Providers list, click the **Test** button next to the provider
2. Or when creating/editing a provider, use the **Save & Test** button
3. The system will attempt to connect using the provided credentials
4. You'll see a success message or an error with details about what went wrong

### Deleting Storage Providers

To delete a storage provider:

1. From the Storage Providers list, click the **Delete** button next to the provider
2. A confirmation dialog will appear
   - If the provider is used in any transfers, you'll see a warning listing those transfers
   - You cannot delete a provider that's in use without first updating those transfers
3. Confirm deletion if the provider is not in use

## Using Storage Providers in Transfers

### Creating Transfers with Storage Providers

To create a new transfer using storage providers:

1. Navigate to the **Transfers** section and click **New Transfer**
2. Fill in the transfer name and schedule as usual
3. In the Source section, select **Provider** and choose from the dropdown
   - Only providers of appropriate types will be shown
   - You'll see only providers you've created (unless you're an admin)
4. In the Destination section, also select a provider
5. Configure other transfer settings as needed (paths, file patterns, etc.)
6. Click **Save** to create the transfer

### Converting Existing Transfers

Existing transfers with embedded credentials can be converted to use storage providers:

1. Edit an existing transfer
2. In the Source section, click **Convert to Provider**
   - This will create a new storage provider using the embedded credentials
   - The provider will be named based on the transfer name
3. Do the same for the Destination section if needed
4. Click **Save** to update the transfer

## Provider Type Reference

### SFTP Configuration

Required fields:
- **Host**: The hostname or IP address of the SFTP server
- **Port**: Server port (usually 22)
- **Username**: Your SFTP username
- **Authentication Method**: Password or Key File
  - **Password**: Your SFTP password (if using password authentication)
  - **Key File**: Path to SSH private key file (if using key authentication)

Optional fields:
- **Key File Password**: Password for the key file (if the key is password-protected)

Example configuration:
```
Name: Company SFTP Server
Type: SFTP
Host: sftp.example.com
Port: 22
Username: user123
Authentication: Password
Password: ********
```

### S3 Configuration

Required fields:
- **Access Key**: Your S3 access key ID
- **Secret Key**: Your S3 secret access key
- **Bucket**: The S3 bucket name

Optional fields:
- **Region**: The AWS region (e.g., us-east-1)
- **Endpoint**: Server URL for S3-compatible services (leave blank for AWS S3)

Example configuration:
```
Name: Analytics Data Bucket
Type: S3
Access Key: AKIAIOSFODNN7EXAMPLE
Secret Key: ********
Region: us-west-2
Bucket: data-analytics-bucket
```

### OneDrive Configuration

Required fields:
- **Client ID**: Your Microsoft application client ID
- **Client Secret**: Your Microsoft application client secret
- **Refresh Token**: OAuth refresh token for authentication

Optional fields:
- **Drive ID**: Specific drive ID (for accessing shared or team drives)

Example configuration:
```
Name: Marketing OneDrive
Type: OneDrive
Client ID: 12345678-1234-1234-1234-123456789012
Client Secret: ********
Refresh Token: ********
```

### Google Drive Configuration

Required fields:
- **Client ID**: Your Google API client ID
- **Client Secret**: Your Google API client secret
- **Refresh Token**: OAuth refresh token for authentication

Optional fields:
- **Team Drive**: Team drive ID (for accessing shared drives)

Example configuration:
```
Name: Sales Team Drive
Type: Google Drive
Client ID: 123456789012-abcdefghijklmnopqrstuvwxyz.apps.googleusercontent.com
Client Secret: ********
Refresh Token: ********
Team Drive: 0ABCDEFGhijklMNOPQrstuvwxyz
```

### FTP Configuration

Required fields:
- **Host**: The hostname or IP address of the FTP server
- **Port**: Server port (usually 21)
- **Username**: Your FTP username
- **Password**: Your FTP password

Optional fields:
- **Passive Mode**: Enable/disable passive mode (default: enabled)

Example configuration:
```
Name: Legacy FTP Server
Type: FTP
Host: ftp.example.com
Port: 21
Username: ftpuser
Password: ********
Passive Mode: Enabled
```

### SMB Configuration

Required fields:
- **Host**: The hostname or IP address of the SMB/CIFS server
- **Share**: The share name
- **Username**: Your username
- **Password**: Your password

Optional fields:
- **Domain**: Windows domain (if applicable)
- **Port**: Server port (default: 445)

Example configuration:
```
Name: Finance Share
Type: SMB
Host: fileserver.example.com
Share: finance
Username: jsmith
Password: ********
Domain: EXAMPLE
```

## Troubleshooting

### Common Connection Issues

#### SFTP Connection Problems

- **Authentication Failed**: Verify username and password/key file
- **Host Not Found**: Check hostname and network connectivity
- **Permission Denied**: Ensure the user has proper permissions on the server
- **Connection Timeout**: Check firewall settings and server availability

#### S3 Connection Problems

- **Access Denied**: Verify access key, secret key, and bucket permissions
- **Invalid Region**: Ensure the region matches the bucket's region
- **No Such Bucket**: Verify the bucket name and existence
- **Endpoint Error**: For S3-compatible services, verify the endpoint URL

#### OAuth Provider Issues (OneDrive/Google Drive)

- **Invalid Client**: Verify client ID and secret
- **Token Expired**: Refresh tokens may need to be regenerated
- **Permission Scope**: Ensure the token has appropriate scopes for file access
- **Rate Limiting**: You may be making too many requests in a short period

### Error Messages

Common error messages and their solutions:

| Error Message | Possible Cause | Solution |
|---------------|----------------|----------|
| "Connection refused" | Server is not running or blocked by firewall | Check server status and firewall settings |
| "Authentication failed" | Incorrect credentials | Verify username/password or key file |
| "Invalid access key" | Incorrect or expired AWS credentials | Check your access key ID and regenerate if needed |
| "Permission denied" | Insufficient permissions | Check file/folder permissions on the server |
| "Connection timed out" | Network issue or server unavailable | Check network connectivity and server status |
| "No such file or directory" | Path does not exist | Verify the path exists on the server |

## FAQ

**Q: Can I use the same storage provider for multiple transfers?**
A: Yes, that's one of the main benefits. Create the provider once and use it in as many transfers as needed.

**Q: Can I see the passwords or secret keys I've stored?**
A: No, for security reasons, passwords and secret keys are never displayed after they're saved. You can update them, but you cannot view the existing values.

**Q: What happens if I need to update credentials?**
A: Edit the storage provider and enter the new credentials. All transfers using that provider will automatically use the updated credentials.

**Q: Are my credentials secure?**
A: Yes, all sensitive information is encrypted using AES-256 encryption before being stored in the database.

**Q: Can other users see my storage providers?**
A: No, each user can only see and use their own storage providers unless they have administrator privileges.

**Q: Can I export or import storage providers?**
A: Not currently. For security reasons, credential export is not supported.

**Q: What if I'm not sure if a provider is in use?**
A: When attempting to delete a provider, the system will show you all transfers that use it. You can also see usage information in the provider details.

**Q: Can I test a provider without creating a transfer?**
A: Yes, use the "Test" button on the provider list.
