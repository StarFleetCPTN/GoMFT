---
sidebar_position: 4
title: Traditional Installation
---

# Traditional Installation Guide

This guide covers installing GoMFT directly on your system without using Docker. This approach is useful for environments where containers aren't available or when you need more direct control over the installation.

## System Requirements

- **Operating System**: Linux, macOS, or Windows
- **Go**: Version 1.20 or later
- **Node.js**: Version 18 or later
- **Build Tools**: gcc and related build tools (for SQLite compilation)

## Prerequisites Installation

### On Debian/Ubuntu Linux

```bash
# Install Go
wget https://go.dev/dl/go1.20.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile

# Install Node.js
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs

# Install build tools
sudo apt-get install -y build-essential
```

### On macOS

```bash
# Using Homebrew
brew install go
brew install node
brew install gcc
```

### On Windows

1. Install Go from [https://golang.org/dl/](https://golang.org/dl/)
2. Install Node.js from [https://nodejs.org/](https://nodejs.org/)
3. Install Build Tools for Visual Studio

## Building GoMFT from Source

1. Clone the repository:

```bash
git clone https://github.com/StarFleetCPTN/GoMFT.git
cd GoMFT
```

2. Install Node.js dependencies and build the frontend:

```bash
npm install
npm run build
```

3. Compile the Go application:

```bash
go build -o gomft main.go
```

## Installation Options

### Option 1: Run Directly

After building, you can run the application directly:

```bash
./gomft
```

### Option 2: Install as a System Service

#### On Linux (systemd)

Create a systemd service file:

```bash
sudo nano /etc/systemd/system/gomft.service
```

Add the following content:

```ini
[Unit]
Description=GoMFT - Go Managed File Transfer
After=network.target

[Service]
Type=simple
User=gomft
Group=gomft
WorkingDirectory=/opt/gomft
ExecStart=/opt/gomft/gomft
Restart=on-failure
RestartSec=5s
Environment="PORT=8080"
Environment="DATA_DIR=/var/lib/gomft/data"
Environment="BACKUP_DIR=/var/lib/gomft/backups"
Environment="LOGS_DIR=/var/log/gomft"

[Install]
WantedBy=multi-user.target
```

Create a dedicated user and set up directories:

```bash
# Create user
sudo useradd -r -s /bin/false gomft

# Create directories
sudo mkdir -p /opt/gomft /var/lib/gomft/data /var/lib/gomft/backups /var/log/gomft

# Copy application
sudo cp -r * /opt/gomft/

# Set permissions
sudo chown -R gomft:gomft /opt/gomft /var/lib/gomft /var/log/gomft
```

Enable and start the service:

```bash
sudo systemctl enable gomft
sudo systemctl start gomft
```

#### On macOS (launchd)

Create a launchd plist file:

```bash
sudo nano /Library/LaunchDaemons/com.gomft.plist
```

Add the following content:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gomft</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/gomft/gomft</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>/usr/local/gomft</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PORT</key>
        <string>8080</string>
        <key>DATA_DIR</key>
        <string>/var/lib/gomft/data</string>
        <key>BACKUP_DIR</key>
        <string>/var/lib/gomft/backups</string>
        <key>LOGS_DIR</key>
        <string>/var/log/gomft</string>
    </dict>
</dict>
</plist>
```

Set up directories and install:

```bash
# Create directories
sudo mkdir -p /usr/local/gomft /var/lib/gomft/data /var/lib/gomft/backups /var/log/gomft

# Copy application
sudo cp -r * /usr/local/gomft/

# Set permissions
sudo chown -R $(whoami):staff /usr/local/gomft /var/lib/gomft /var/log/gomft

# Load service
sudo launchctl load /Library/LaunchDaemons/com.gomft.plist
```

#### On Windows (Windows Service)

1. Install [NSSM (Non-Sucking Service Manager)](https://nssm.cc/download)
2. Open Command Prompt as Administrator
3. Create the service:

```bat
nssm install GoMFT C:\path\to\gomft.exe
nssm set GoMFT AppDirectory C:\path\to\gomft\directory
nssm set GoMFT AppEnvironmentExtra PORT=8080 DATA_DIR=C:\ProgramData\GoMFT\data BACKUP_DIR=C:\ProgramData\GoMFT\backups LOGS_DIR=C:\ProgramData\GoMFT\logs
nssm start GoMFT
```

## Configuration

### Environment Variables

Create a `.env` file in the application directory or set system environment variables:

```
PORT=8080
DATA_DIR=/var/lib/gomft/data
BACKUP_DIR=/var/lib/gomft/backups
LOGS_DIR=/var/log/gomft
BASE_URL=http://localhost:8080
EMAIL_ENABLED=false
JWT_SECRET=your-secret-key
ENCRYPT_KEY=32-character-encryption-key
```

### Web Server Setup

For production use, it's recommended to run GoMFT behind a web server like Nginx:

#### Nginx Configuration

```nginx
server {
    listen 80;
    server_name your-gomft-server.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Updating GoMFT

To update a traditionally installed GoMFT:

1. Stop the service:
   ```bash
   sudo systemctl stop gomft  # For Linux
   sudo launchctl unload /Library/LaunchDaemons/com.gomft.plist  # For macOS
   nssm stop GoMFT  # For Windows
   ```

2. Back up your data:
   ```bash
   cp -r /var/lib/gomft/data /var/lib/gomft/data.backup
   ```

3. Get the latest code:
   ```bash
   cd /path/to/gomft/source
   git pull
   ```

4. Rebuild:
   ```bash
   npm install
   npm run build
   go build -o gomft main.go
   ```

5. Update the installation:
   ```bash
   sudo cp gomft /opt/gomft/  # For Linux
   sudo cp gomft /usr/local/gomft/  # For macOS
   copy gomft.exe C:\path\to\gomft.exe  # For Windows
   ```

6. Restart the service:
   ```bash
   sudo systemctl start gomft  # For Linux
   sudo launchctl load /Library/LaunchDaemons/com.gomft.plist  # For macOS
   nssm start GoMFT  # For Windows
   ```

## Troubleshooting

### Common Issues

1. **Permission Errors**:
   - Check that the user running GoMFT has write permissions to the data, backup, and logs directories.

2. **Database Errors**:
   - Ensure the SQLite database path is writeable.
   - Check database integrity: `sqlite3 /var/lib/gomft/data/gomft.db "PRAGMA integrity_check;"`

3. **Port Already in Use**:
   - Change the port in the configuration.
   - Check what's using port 8080: `sudo lsof -i :8080`

4. **Missing Dependencies**:
   - Make sure all required Go and Node.js dependencies are installed.

### Viewing Logs

- **Application Logs**: Check `/var/log/gomft/` or your configured logs directory
- **System Service Logs**:
  ```bash
  # For Linux
  journalctl -u gomft
  
  # For macOS
  log show --predicate 'senderImagePath contains "gomft"'
  
  # For Windows
  Get-EventLog -LogName Application -Source GoMFT
  ```

For more help, refer to the [GitHub repository](https://github.com/StarFleetCPTN/GoMFT) or open an issue. 