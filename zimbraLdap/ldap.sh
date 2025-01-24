#!/usr/bin/env bash

# Set the backup directory
backup_dir="/srv/zimbra-ldap"

# Create the backup directory if it doesn't exist
if [ ! -d "$backup_dir" ]; then
  mkdir "$backup_dir"
fi

chown zimbra:zimbra "$backup_dir"

# Backup the LDAP database as the Zimbra user
su - zimbra -c "/opt/zimbra/libexec/zmslapcat -c \"$backup_dir\""
su - zimbra -c "/opt/zimbra/libexec/zmslapcat \"$backup_dir\""

# Remove old backups (older than 7 days)
find "$backup_dir" -type f -name "ldap-*" -mtime +7 -exec rm {} \;
