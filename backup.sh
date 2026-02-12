#!/bin/bash
# Memory Cloud — Backup Script
# Backs up SQLite databases from Docker volume to /opt/mcp-hub/backups/

set -e

BACKUP_DIR="/opt/mcp-hub/backups"
VOLUME_NAME="mcp-hub_memory_data"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/memory-backup-$DATE.tar.gz"
KEEP_DAYS=30

mkdir -p "$BACKUP_DIR"

# Get volume mount point
MOUNT=$(docker volume inspect "$VOLUME_NAME" --format '{{ .Mountpoint }}')

if [ -z "$MOUNT" ] || [ ! -d "$MOUNT" ]; then
    echo "ERROR: Volume $VOLUME_NAME not found or empty"
    exit 1
fi

# Create compressed backup
tar -czf "$BACKUP_FILE" -C "$MOUNT" .

# Calculate size
SIZE=$(du -sh "$BACKUP_FILE" | cut -f1)
echo "$(date +%Y-%m-%d\ %H:%M:%S) Backup created: $BACKUP_FILE ($SIZE)"

# Cleanup old backups (keep last N days)
find "$BACKUP_DIR" -name "memory-backup-*.tar.gz" -mtime +$KEEP_DAYS -delete
REMAINING=$(ls -1 "$BACKUP_DIR"/memory-backup-*.tar.gz 2>/dev/null | wc -l)
echo "$(date +%Y-%m-%d\ %H:%M:%S) Cleanup done. $REMAINING backups retained."
