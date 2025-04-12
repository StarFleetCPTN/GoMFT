---
sidebar_position: 2
title: Authentication
---

# Authentication and Authorization

GoMFT provides robust authentication and authorization mechanisms to ensure secure access to the application and its features. This page explains how to configure and manage authentication in GoMFT.

## Authentication Methods

GoMFT supports multiple authentication methods to secure access to the application:

### Local Authentication

The default authentication method using GoMFT's built-in user database:

- **Username/Password**: Traditional username and password authentication
- **Password Requirements**: Configurable password complexity rules
- **Password Expiration**: Force password changes after a configurable period
- **Account Lockout**: Temporarily lock accounts after failed login attempts

### OAuth/OpenID Connect

Support for modern identity providers:

- **Single Sign-On**: Integrate with SSO solutions
- **Identity Providers**: Support for popular providers (Google, Microsoft, Okta, etc.)
- **JWT Tokens**: Secure token-based authentication
- **Automatic Account Provisioning (COMING SOON)**: Create GoMFT accounts based on SSO information

## Setting Up Authentication

### Configuring Local Authentication

Local authentication is enabled by default and requires minimal setup:

### Configuring OAuth/OpenID Connect

To set up OAuth or OpenID Connect:

1. Navigate to **Settingss** > **Authentication Providers**
2. Select **OAuth/OIDC** as an authentication method
3. Configure provider settings:
   - Provider URL
   - Client ID
   - Client Secret
   - Scope (e.g., `openid profile email`)
   - Callback URL
4. Set up attribute mappings:
   - Map provider attributes to GoMFT user properties
   - Configure role attribute or claim
5. Test the configuration

## Multi-Factor Authentication (MFA)

GoMFT supports multi-factor authentication for enhanced security:

### MFA Options

- **Time-based One-Time Password (TOTP)**: Compatible with apps like Google Authenticator
- **Email Verification Codes**: One-time codes sent via email
- **Recovery Codes**: Backup codes for emergency access

### Enabling MFA

For users to set up MFA:

1. Log in to GoMFT
2. Navigate to **Profile** > **Security Settings**
3. Select **Enable Multi-Factor Authentication**
4. Choose the MFA method (e.g., TOTP)
5. Follow the setup instructions:
   - For TOTP: Scan QR code with authenticator app
   - For Email: Verify email address
6. Generate and save recovery codes

## User Management

### Creating Users

To create new users:

1. Navigate to **Administration** > **Users**
2. Click **Create New User**
3. Fill in the user details:
   - Username
   - Email address
   - Full name
   - Initial password or send password reset link
   - Role assignment
4. Click **Create Users**

### Managing User Accounts

To manage existing users:

1. Navigate to **Administration** > **Users**
2. Find the user in the list
3. Available actions:
   - Edit user details
   - Change role assignment
   - Reset password
   - Enable/disable account
   - Force MFA enrollment
   - Delete user

### User Self-Service

GoMFT provides self-service features for users:

- **Profile Management**: Users can update their profile information
- **Password Change**: Users can change their password
- **MFA Setup**: Users can configure their MFA preferences

## Role-Based Access Control

GoMFT implements role-based access control (RBAC) to manage permissions:

### Default Roles

- **Administrator**: Full access to all system features
- **System**: Can manage transfers and connections but not admin settings
- **User**: Basic access to create and manage personal transfers

### Creating Custom Roles

To create a custom role:

1. Navigate to **Administration** > **Roles**
2. Click **Create New Role**
3. Define the role:
   - Role name
   - Description
   - Permission assignments
4. Save the role

### Permission Categories

GoMFT organizes permissions into categories:

- **System Administration**: System-wide settings and maintenance
- **User Management**: User and role administration
- **Transfer Management**: Creating and managing transfers
- **Connection Management**: Creating and managing connections
- **Schedule Management**: Managing transfer schedules
- **Execution Control**: Running and controlling transfers
- **Monitoring**: Viewing logs and reports

## Security Best Practices

- **Enforce Strong Passwords**: Configure strong password requirements
- **Enable MFA**: Require MFA for all users, especially administrators
- **Regular Review**: Periodically review user accounts and permissions
- **Principle of Least Privilege**: Assign the minimum necessary permissions
- **Audit Authentication**: Monitor and audit authentication events
- **Secure Configuration**: Properly secure authentication configuration files
- **Account Lifecycle**: Implement processes for account creation and termination 