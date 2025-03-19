#!/bin/sh
set -e

# Default username
USERNAME=${USERNAME:-gomft}

# If PUID/PGID env vars are set, update the user's UID/GID
if [ -n "${PUID}" ] && [ -n "${PGID}" ]; then
    echo "🔒 Updating user ${USERNAME} with UID:GID = ${PUID}:${PGID}"
    
    # Make sure we have directories to work with
    mkdir -p /app/data /app/backups
    
    # Check if we're on Alpine (busybox)
    if grep -q "Alpine" /etc/os-release 2>/dev/null; then
        echo "Detected Alpine Linux, using busybox usermod/groupmod..."
        
        # Update group ID first
        if [ "$(getent group ${USERNAME} | cut -d: -f3)" != "${PGID}" ]; then
            echo "Updating GID to ${PGID}..."
            groupmod -g ${PGID} ${USERNAME} || echo "⚠️ Failed to change GID"
        fi
        
        # Update user ID
        if [ "$(id -u ${USERNAME})" != "${PUID}" ]; then
            echo "Updating UID to ${PUID}..."
            usermod -u ${PUID} ${USERNAME} || echo "⚠️ Failed to change UID"
        fi
    else
        echo "Non-Alpine system, using standard user management..."
        # Handle user/group changes with error recovery
        {
            # First remove the user (since user has the group as primary group)
            if getent passwd ${USERNAME} > /dev/null; then
                echo "Removing existing user ${USERNAME}"
                userdel ${USERNAME} 2>/dev/null || true
            fi
            
            # Wait a moment for system to clean up user
            sleep 1
            
            # Then remove the group
            if getent group ${USERNAME} > /dev/null; then
                echo "Removing existing group ${USERNAME}"
                groupdel ${USERNAME} 2>/dev/null || true
            fi
            
            # Recreate group and user in the correct order
            echo "Creating group ${USERNAME} with GID ${PGID}"
            groupadd -g ${PGID} ${USERNAME} 2>/dev/null || groupadd ${USERNAME} 2>/dev/null || true
            
            echo "Creating user ${USERNAME} with UID ${PUID}"
            useradd -u ${PUID} -g ${USERNAME} -s /bin/sh ${USERNAME} 2>/dev/null || 
            useradd -g ${USERNAME} -s /bin/sh ${USERNAME} 2>/dev/null || true
        } || {
            echo "⚠️ Warning: Failed to update UID/GID, continuing with built-in user"
        }
    fi
    
    # Fix ownership of app directories
    echo "Setting ownership of app directories"
    chown -R ${USERNAME}:${USERNAME} /app/data /app/backups || echo "⚠️ Warning: Failed to change ownership"
    
    # Ensure .env file exists and has correct permissions
    if [ -f /app/.env ]; then
        echo "Found .env file, setting permissions..."
        chown ${USERNAME}:${USERNAME} /app/.env || echo "⚠️ Warning: Failed to change .env ownership"
        chmod 644 /app/.env || echo "⚠️ Warning: Failed to change .env permissions"
    else
        echo "No .env file found, creating empty one..."
        touch /app/.env
        chown ${USERNAME}:${USERNAME} /app/.env || echo "⚠️ Warning: Failed to change .env ownership"
        chmod 644 /app/.env || echo "⚠️ Warning: Failed to change .env permissions"
    fi
    
    # Run the application as the specified user
    echo "Starting application as user ${USERNAME}"
    if command -v su-exec >/dev/null 2>&1; then
        exec su-exec ${USERNAME} "$@"
    elif command -v gosu >/dev/null 2>&1; then
        exec gosu ${USERNAME} "$@"
    else
        exec su -m ${USERNAME} -c "$*"
    fi
else
    # Run as the predefined user (set during build)
    echo "Starting application with predefined user"
    exec "$@"
fi 