#!/bin/sh
set -eu

BACKUP_DIR="/backups"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${BACKUP_DIR}/jukespotify-${STAMP}.sql.gz"

mkdir -p "${BACKUP_DIR}"

mysqldump \
  -h "${DB_HOST:-mysql}" \
  -P "${DB_PORT:-3306}" \
  -u "${DB_USER:-root}" \
  -p"${DB_PASSWORD}" \
  --protocol=TCP \
  --single-transaction \
  --routines \
  --triggers \
  "${DB_NAME:-jukespotify}" | gzip -c > "${OUT}"

echo "mysql backup wrote ${OUT}"

ls -1t "${BACKUP_DIR}"/jukespotify-*.sql.gz 2>/dev/null | tail -n +15 | xargs -r rm -f
