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
        CURRENT_GID=$(getent group ${USERNAME} | cut -d: -f3)
        CURRENT_UID=$(id -u ${USERNAME})

        if [ "${CURRENT_GID}" != "${PGID}" ] || [ "${CURRENT_UID}" != "${PUID}" ]; then
            echo "Attempting to update UID/GID to ${PUID}:${PGID} using delete/recreate..."
            
            # Delete existing user and group, ignoring errors
            deluser ${USERNAME} > /dev/null 2>&1 || true
            delgroup ${USERNAME} > /dev/null 2>&1 || true
            
            # Check if a group with the target GID already exists
            EXISTING_GROUP=$(getent group ${PGID} | cut -d: -f1 || echo "")
            
            if [ -n "${EXISTING_GROUP}" ]; then
                echo "Group with GID ${PGID} already exists as '${EXISTING_GROUP}', will use this group"
                # Set USERNAME_GROUP to the existing group name
                USERNAME_GROUP="${EXISTING_GROUP}"
            else
                # Add group with the specified GID
                echo "Adding group ${USERNAME} with GID ${PGID}"
                if ! addgroup -g ${PGID} ${USERNAME}; then
                    echo "⚠️ Failed to add group ${USERNAME} with GID ${PGID}."
                    # Exiting because user creation will likely fail
                    exit 1
                fi
                USERNAME_GROUP="${USERNAME}"
            fi
            
            # Add user with the specified UID and GID
            # Use -G for primary group with adduser in BusyBox
            # Use -h /app for home directory (consistent with expectations)
            # Use -s /bin/sh for shell
            # Use -D for no password (system user)
            echo "Adding user ${USERNAME} with UID ${PUID} and group ${USERNAME_GROUP}"
            if ! adduser -u ${PUID} -G ${USERNAME_GROUP} -h /app -s /bin/sh -D ${USERNAME}; then
                 echo "⚠️ Failed to add user ${USERNAME} with UID ${PUID} and group ${USERNAME_GROUP}."
                 # Exiting because the application cannot run as the correct user
                 exit 1
            fi
            
            # Verify the change
            FINAL_UID=$(id -u ${USERNAME} 2>/dev/null || echo "error")
            FINAL_GID=$(id -g ${USERNAME} 2>/dev/null || echo "error")
            
            if [ "${FINAL_UID}" = "${PUID}" ] && [ "${FINAL_GID}" = "${PGID}" ]; then
                echo "✅ Successfully updated UID/GID to ${PUID}:${PGID}"
            else
                echo "⚠️ Verification failed after update. Target: ${PUID}:${PGID}, Actual: ${FINAL_UID}:${FINAL_GID}"
                # Exiting because the UID/GID is not correct
                exit 1
            fi
        else
            echo "UID/GID ${PUID}:${PGID} already set."
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
            
            # Check if a group with the target GID already exists
            EXISTING_GROUP=$(getent group ${PGID} | cut -d: -f1 || echo "")
            
            if [ -n "${EXISTING_GROUP}" ]; then
                echo "Group with GID ${PGID} already exists as '${EXISTING_GROUP}', will use this group"
                # Set USERNAME_GROUP to the existing group name
                USERNAME_GROUP="${EXISTING_GROUP}"
            else
                # Recreate group with the specified GID
                echo "Creating group ${USERNAME} with GID ${PGID}"
                groupadd -g ${PGID} ${USERNAME} 2>/dev/null || groupadd ${USERNAME} 2>/dev/null || true
                USERNAME_GROUP="${USERNAME}"
            fi
            
            # Check if a user with the target UID already exists
            EXISTING_USER=$(getent passwd ${PUID} | cut -d: -f1 || echo "")
            
            if [ -n "${EXISTING_USER}" ] && [ "${EXISTING_USER}" != "${USERNAME}" ]; then
                echo "⚠️ Warning: User with UID ${PUID} already exists as '${EXISTING_USER}'. Using a different username may cause issues."
            fi
            
            echo "Creating user ${USERNAME} with UID ${PUID} and group ${USERNAME_GROUP}"
            useradd -u ${PUID} -g ${USERNAME_GROUP} -s /bin/sh ${USERNAME} 2>/dev/null || 
            useradd -g ${USERNAME_GROUP} -s /bin/sh ${USERNAME} 2>/dev/null || true
            
            # Verify the change
            FINAL_UID=$(id -u ${USERNAME} 2>/dev/null || echo "error")
            FINAL_GID=$(id -g ${USERNAME} 2>/dev/null || echo "error")
            
            if [ "${FINAL_UID}" = "${PUID}" ] && [ "${FINAL_GID}" = "${PGID}" ]; then
                echo "✅ Successfully updated UID/GID to ${PUID}:${PGID}"
            else
                echo "⚠️ Warning: Verification failed. Target: ${PUID}:${PGID}, Actual: ${FINAL_UID}:${FINAL_GID}"
            fi
        } || {
            echo "⚠️ Warning: Failed to update UID/GID, continuing with built-in user"
        }
    fi
    
    # Fix ownership of app directories
    echo "Setting ownership of app directories"
    chown -R ${USERNAME}:${USERNAME_GROUP:-${USERNAME}} /app/data /app/backups || echo "⚠️ Warning: Failed to change ownership"
    
    # Ensure .env file exists and has correct permissions
    if [ -f /app/.env ]; then
        echo "Found .env file, setting permissions..."
        chown ${USERNAME}:${USERNAME_GROUP:-${USERNAME}} /app/.env || echo "⚠️ Warning: Failed to change .env ownership"
        chmod 644 /app/.env || echo "⚠️ Warning: Failed to change .env permissions"
    else
        echo "No .env file found, creating empty one..."
        touch /app/.env
        chown ${USERNAME}:${USERNAME_GROUP:-${USERNAME}} /app/.env || echo "⚠️ Warning: Failed to change .env ownership"
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