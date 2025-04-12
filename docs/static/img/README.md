# Images directory

This directory contains various images used throughout the documentation, including:

1. Logo files
2. Screenshots
3. Icons and other assets

Some screenshots are automatically copied from the main repository's `/screenshots` directory during the build process.

The automated copy is handled by the `prepare-screenshots` script in package.json.

## Required Images

The following images are used in the documentation:

- `logo.svg` - The GoMFT logo for the header
- `favicon.ico` - The website favicon
- `docusaurus-social-card.jpg` - Social media preview image
- `dashboard.gomft.png` - Screenshot of the GoMFT dashboard
- `dashboard.dark.gomft.png` - Screenshot of the GoMFT dashboard in dark mode
- `transfer.config.gomft.png` - Screenshot of the transfer configuration
- `scheduled.jobs.gomft.png` - Screenshot of the scheduled jobs
- `transfer.history.gomft.png` - Screenshot of the transfer history
- `user.management.gomft.png` - Screenshot of the user management
- `role.management.gomft.png` - Screenshot of the role management
- `authentication.providers.gomft.png` - Screenshot of the authentication providers
- `notifications.gomft.png` - Screenshot of the notifications
- `notification.service.gomft.png` - Screenshot of the notification service
- `audit.logs.gomft.png` - Screenshot of the audit logs
- `file.details.gomft.png` - Screenshot of the file details
- `file.metadata.gomft.png` - Screenshot of the file metadata
- `database.tools.gomft.png` - Screenshot of the database tools
- `job.run.details.gomft.png` - Screenshot of the job run details

## Symlinks for Backward Compatibility

- `dashboard.png` → `dashboard.gomft.png`
- `create-transfer.png` → `transfer.config.gomft.png`
- `docker.png` - Placeholder for Docker-related screenshots

## Adding New Images

When adding images to this directory:

1. Use descriptive filenames
2. Optimize image size when possible
3. Use PNG for screenshots and SVG for vector graphics
4. Include alt text when referencing images in documentation 