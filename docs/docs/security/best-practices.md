---
sidebar_position: 1
title: Security Best Practices
---

# Security Best Practices

This guide provides recommendations for securing your GoMFT installation and maintaining a secure file transfer environment.

## Installation Security

### Use Docker Security Features

When deploying GoMFT with Docker:

- **Run as Non-Root**: Always run the container as a non-root user (see [Running as Non-Root](/docs/security/non-root))
- **Use Read-Only Filesystem**: Mount the filesystem as read-only except for specific data directories
- **Limit Capabilities**: Use Docker's `--cap-drop` to limit container capabilities
- **Set Resource Limits**: Prevent resource exhaustion with memory and CPU limits
- **Use Docker Secrets**: Store sensitive configuration in Docker secrets instead of environment variables

Example secure docker-compose configuration:

```yaml
services:
  gomft:
    image: starfleetcptn/gomft:latest
    user: "1000:1000"
    read_only: true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    security_opt:
      - no-new-privileges:true
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
    environment:
      - TZ=UTC
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 1G
```

### Traditional Installation Security

For traditional installations:

- **Dedicated User**: Create a dedicated system user for running GoMFT
- **Minimal Permissions**: Give the user only the permissions it needs
- **Firewall Rules**: Restrict access to only necessary ports
- **SELinux/AppArmor**: Use system security modules to limit application scope

## Network Security

### Use HTTPS

Always use HTTPS for the web interface:

- **Configure TLS**: Use a reverse proxy like Nginx or Traefik for TLS termination
- **Strong Ciphers**: Use modern, secure cipher suites
- **HSTS**: Enable HTTP Strict Transport Security
- **Valid Certificates**: Use trusted certificates from Let's Encrypt or other providers

### Access Control

- **IP Restrictions**: Limit access to trusted IP addresses where possible
- **VPN Access**: Consider placing GoMFT behind a VPN for additional security
- **Firewall Rules**: Configure firewall rules to restrict access to essential ports only

## Authentication and Authorization

### Strong Authentication

- **Password Policy**: Enforce strong password requirements
- **MFA**: Enable Multi-Factor Authentication for all users
- **Session Management**: Set appropriate session timeouts
- **Failed Login Limits**: Implement account lockouts after several failed attempts

### Role-Based Access Control

- **Principle of Least Privilege**: Grant users only the permissions they need
- **Separation of Duties**: Use roles to separate administrative functions
- **Regular Review**: Periodically review user roles and permissions

## Credential Management

### Secure Storage

- **Encrypted Credentials**: Ensure all credentials are encrypted at rest
- **Isolated Storage**: Store sensitive credentials in a separate database or secure storage
- **Key Rotation**: Regularly rotate encryption keys

### Credential Practices

- **Service Accounts**: Use service accounts instead of personal accounts for connections
- **Temporary Credentials**: Use temporary credentials where supported (e.g., AWS STS)
- **API Keys**: Regularly rotate API keys and access tokens
- **Minimal Scope**: Grant credentials the minimum required permissions

## Transfer Security

### Secure Protocols

- **Choose Secure Protocols**: Prefer SFTP, FTPS, or HTTPS over unencrypted protocols
- **Disable Legacy Protocols**: Disable insecure protocols like FTP where possible
- **Strong Ciphers**: Configure secure cipher suites for encrypted protocols

### Data Handling

- **Data Classification**: Classify data by sensitivity and apply appropriate controls
- **Data Validation**: Validate files before processing them
- **Virus Scanning**: Implement virus scanning for transferred files
- **Data Loss Prevention**: Consider DLP measures for sensitive data

## Auditing and Monitoring

### Comprehensive Logging

- **Detailed Logs**: Enable detailed logging for all operations
- **Secure Log Storage**: Store logs securely with access controls
- **Log Rotation**: Implement log rotation to manage disk space
- **Tamper Protection**: Ensure logs cannot be modified or deleted

### Monitoring and Alerting

- **Real-time Monitoring**: Monitor for suspicious activities
- **Security Alerts**: Configure alerts for security-related events
- **Performance Monitoring**: Watch for performance issues that might indicate attacks
- **Regular Review**: Establish a process for regular log review

## System Security

### Regular Updates

- **Update GoMFT**: Keep GoMFT updated to the latest version
- **Patch Host System**: Keep the host operating system patched
- **Update Dependencies**: Keep all dependencies (Docker, etc.) updated

### Backup and Recovery

- **Regular Backups**: Back up the GoMFT database and configurations regularly
- **Secure Backups**: Encrypt backups and store them securely
- **Test Restoration**: Regularly test backup restoration
- **Disaster Recovery Plan**: Create and maintain a disaster recovery plan

## Periodic Security Review

### Security Assessments

- **Vulnerability Scanning**: Regularly scan for vulnerabilities
- **Penetration Testing**: Conduct periodic penetration tests
- **Configuration Review**: Review security configurations regularly
- **Compliance Checks**: Ensure ongoing compliance with relevant standards

### Documentation

- **Security Policies**: Document security policies and procedures
- **Configuration Documentation**: Maintain documentation of secure configurations
- **Incident Response Plan**: Create and maintain an incident response plan

## Integrating with Security Tools

GoMFT can be integrated with external security tools:

- **SIEM Integration**: Forward logs to Security Information and Event Management tools
- **Vulnerability Scanners**: Include GoMFT in vulnerability scanning
- **Compliance Tools**: Integrate with compliance monitoring tools

## Best Practices for Specific Environments

### Cloud Deployment

- **Cloud Security Services**: Utilize cloud provider security services
- **Network Security Groups**: Configure appropriate network security groups
- **Private Endpoints**: Use private endpoints where possible
- **Cloud IAM**: Leverage cloud Identity and Access Management

### On-Premises Deployment

- **Network Segmentation**: Place GoMFT in an appropriate network segment
- **Physical Security**: Ensure physical security of the servers
- **Environmental Controls**: Implement appropriate environmental controls
- **Backup Power**: Ensure backup power for critical systems 