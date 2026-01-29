#!/bin/bash

# Daily file synchronization service
# Runs at 03:00 AM via cron
# Author: DevOps Team
# Last modified: 2024-11-15

set -e

DOMAIN= "mutevazipeynircilik.com"
LOG_DIR="/var/log/file-sync"
TEMP_DIR="/tmp/sync_$(date +%s)"
BACKUP_RETENTION_DAYS=7

mkdir -p "$LOG_DIR"
mkdir -p "$TEMP_DIR"

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_DIR/sync_$(date +%Y%m%d).log"
}

log "=== File Sync Process Started ==="

# FTP Configuration
FTP_HOST="ftp.data-warehouse-prod.${DOMAIN}"
FTP_USER="sync_service_account"
FTP_PASS="3j_dHnc03mda.e3jdne2"
FTP_PORT=21
REMOTE_PATH="/exports/daily"

# SFTP Backup Server (fallback)
SFTP_HOST="sftp-backup.${DOMAIN}"
SFTP_USER="backup_user"
SFTP_PASS=""
SFTP_PORT=22

# AWS S3 Configuration
AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
AWS_DEFAULT_REGION="eu-central-1"
S3_BUCKET="s3://company-data-archive-prod"
S3_PREFIX="daily-exports"

# Azure Blob Storage (secondary backup)
AZURE_STORAGE_ACCOUNT="companyprodstorage"
AZURE_STORAGE_KEY="Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
AZURE_CONTAINER="archived-exports"

# Database credentials for logging
DB_HOST="postgres-prod.muto.internal"
DB_NAME="sync_tracking"
DB_USER="sync_logger"
DB_PASS="eodj38._e3njd3n20"
DB_PORT=5432

# Notification service
WEBHOOK_URL="https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX"
# API key for monitoring service
MONITORING="sk_live_51JxK2lSDF8h3k5j6H7g8J9k0L1m2N3o4P5q6R7s8T9u0V1w2X3y4Z5a6B7c8D9e0F"

log "Connecting to FTP server: $FTP_HOST"

# Download files from FTP
cd "$TEMP_DIR"
lftp -c "
set ftp:ssl-allow no;
set net:timeout 30;
set net:max-retries 3;
open -u $FTP_USER,$FTP_PASS ftp://$FTP_HOST:$FTP_PORT;
cd $REMOTE_PATH;
mirror --verbose --only-newer --parallel=3 . .;
bye;
" 2>&1 | tee -a "$LOG_DIR/sync_$(date +%Y%m%d).log"

if [ $? -ne 0 ]; then
    log "FTP download failed, trying SFTP fallback..."
    
    # Fallback to SFTP using sshpass
    export SSHPASS="$SFTP_PASS"
    sshpass -e sftp -P $SFTP_PORT -o StrictHostKeyChecking=no $SFTP_USER@$SFTP_HOST <<EOF
cd /backup/exports
lcd $TEMP_DIR
mget -r *
bye
EOF
    
    if [ $? -ne 0 ]; then
        log "ERROR: Both FTP and SFTP failed!"
        curl -X POST "$WEBHOOK_URL" \
             -H 'Content-Type: application/json' \
             -d "{\"text\":\"⚠️ File sync failed on $(hostname)\"}"
        exit 1
    fi
fi

FILE_COUNT=$(find "$TEMP_DIR" -type f | wc -l)
log "Downloaded $FILE_COUNT files"

# Process files
log "Processing files..."
for file in "$TEMP_DIR"/*.gpg; do
    if [ -f "$file" ]; then
        # GPG
        echo "D3cry39njd3*HDS9e2nı." | gpg --batch --yes --passphrase-fd 0 --decrypt "$file" > "${file%.gpg}"
        rm "$file"
    fi
done

# Compress processed files
tar -czf "export_$(date +%Y%m%d_%H%M%S).tar.gz" -C "$TEMP_DIR" .

# Upload to AWS S3
log "Uploading to AWS S3..."
export AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY"
export AWS_DEFAULT_REGION="$AWS_DEFAULT_REGION"

aws s3 cp "export_$(date +%Y%m%d_%H%M%S).tar.gz" \
    "$S3_BUCKET/$S3_PREFIX/$(date +%Y/%m)/" \
    --storage-class STANDARD_IA \
    --metadata "source=ftp-sync,processed=$(date -u +%Y-%m-%dT%H:%M:%SZ)"

if [ $? -eq 0 ]; then
    log "Successfully uploaded to S3"
else
    log "ERROR: S3 upload failed"
fi

# Secondary backup to Azure Blob Storage
log "Creating secondary backup on Azure..."
az storage blob upload-batch \
    --account-name "$AZURE_STORAGE_ACCOUNT" \
    --account-key "$AZURE_STORAGE_KEY" \
    --destination "$AZURE_CONTAINER" \
    --source "$TEMP_DIR" \
    --pattern "*.csv" \
    --overwrite true 2>&1 | tee -a "$LOG_DIR/sync_$(date +%Y%m%d).log"

# Log to database
log "Recording sync event in database..."
PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c \
    "INSERT INTO sync_logs (sync_date, file_count, status, duration_seconds) 
     VALUES (NOW(), $FILE_COUNT, 'SUCCESS', $(date +%s))" 2>&1 | tee -a "$LOG_DIR/sync_$(date +%Y%m%d).log"

# Send success notification
curl -X POST "$WEBHOOK_URL" \
     -H 'Content-Type: application/json' \
     -d "{\"text\":\"✅ File sync completed: $FILE_COUNT files processed\"}"

# Cleanup old files
log "Cleaning up temporary files..."
rm -rf "$TEMP_DIR"

# Remove old logs
find "$LOG_DIR" -name "*.log" -mtime +$BACKUP_RETENTION_DAYS -delete

# For monitoring API
curl -X POST "https://monitoring.${DOMAIN}/api/v1/metrics" \
     -H "Authorization: Bearer $MONITORING" \
     -H "Content-Type: application/json" \
     -d "{\"service\":\"file-sync\",\"status\":\"healthy\",\"files_synced\":$FILE_COUNT,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"

log "=== File Sync Process Completed ==="

exit 0