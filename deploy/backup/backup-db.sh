#!/usr/bin/env bash
#
# Nightly encrypted, off-site database backup.
#
# Two properties this exists to hold, and everything below serves one of them:
#
#   1. A backup the server itself cannot read. Encryption is public-key, and the
#      private key never touches this machine. Whoever compromises the VPS gets
#      the live database — that is unavoidable — but must not also get every
#      historical copy of it.
#
#   2. A backup that is somewhere else. A dump sitting next to the database it
#      came from survives a dropped table and nothing else. Disk failure, a
#      wrong `rm`, a provider losing the instance: all of those take both.
#
# It writes a manifest beside each archive recording the plaintext size and
# checksum, because the one thing that cannot be checked here is whether the
# archive decrypts — that needs the private key, which is deliberately absent.
# The manifest is what a restore compares against.

set -euo pipefail

# --- configuration -----------------------------------------------------------
COMPOSE_FILE="${COMPOSE_FILE:-/home/deploy/safrat/docker-compose.prod.yml}"
DB_NAME="${DB_NAME:-safrat}"
DB_USER="${DB_USER:-safrat}"
LOCAL_DIR="${LOCAL_DIR:-/home/deploy/backups}"
LOCAL_KEEP_DAYS="${LOCAL_KEEP_DAYS:-7}"

# The public half of the backup key, carried in a self-signed certificate —
# `openssl cms` takes a certificate rather than a bare public key. Generated
# once, off this machine; see deploy/backup/README.md. The private half must
# never appear on this server, and nothing here needs it.
BACKUP_CERT="${BACKUP_CERT:-/home/deploy/backup-cert.pem}"

# Off-site target. R2 is already in use for uploads, so the credentials and the
# endpoint exist; this uses a separate bucket so a leaked uploads key cannot
# reach the backups.
R2_BUCKET="${BACKUP_R2_BUCKET:-safrat-backups}"
R2_ENDPOINT="${BACKUP_R2_ENDPOINT:-}"

log() { printf '[%s] %s\n' "$(date -Iseconds)" "$*"; }
die() { log "FATAL: $*"; exit 1; }

# --- preflight ---------------------------------------------------------------
# Every one of these is a reason the backup would otherwise fail quietly, at
# night, and be discovered on the day it was needed.
[[ -r "$BACKUP_CERT" ]] || die "sertifikat backup tidak terbaca di $BACKUP_CERT"
command -v openssl >/dev/null || die "openssl tidak ada"
command -v aws >/dev/null || die "aws cli tidak ada; instal atau setel BACKUP_R2_ENDPOINT kosong untuk melewati unggahan"
[[ -n "$R2_ENDPOINT" ]] || die "BACKUP_R2_ENDPOINT belum diset — backup lokal saja bukan pemulihan bencana"

mkdir -p "$LOCAL_DIR"
STAMP="$(date +%Y%m%d_%H%M%S)"
BASE="safrat_${STAMP}"
ARCHIVE="${LOCAL_DIR}/${BASE}.sql.gz.enc"
MANIFEST="${LOCAL_DIR}/${BASE}.manifest"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# --- dump --------------------------------------------------------------------
# Dumped to a file rather than straight down a pipe. A pipeline writes a
# plausible-looking archive even when pg_dump dies halfway: the bytes are there,
# the gzip trailer is there, and the truncation is invisible until a restore
# needs the rows that were never written. Landing it first means the exit code
# is checked before anything is encrypted or uploaded.
log "dump ${DB_NAME}"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  pg_dump -U "$DB_USER" --format=plain --no-owner --no-privileges "$DB_NAME" \
  > "${WORK}/dump.sql" || die "pg_dump gagal"

[[ -s "${WORK}/dump.sql" ]] || die "dump kosong"

# A dump that did not finish has no terminator. Cheap to check, and it catches
# the exact failure a size check misses.
tail -c 200 "${WORK}/dump.sql" | grep -q "PostgreSQL database dump complete" \
  || die "dump terpotong; penanda akhir tidak ditemukan"

PLAIN_BYTES="$(wc -c < "${WORK}/dump.sql" | tr -d ' ')"
PLAIN_SHA="$(openssl dgst -sha256 -r "${WORK}/dump.sql" | cut -d' ' -f1)"

# --- compress and encrypt ----------------------------------------------------
# CMS rather than raw RSA: the archive is far larger than any RSA key can take,
# so this generates a random AES-256 key per run and wraps that. Streaming, so
# memory does not scale with the database.
log "kompres dan enkripsi"
gzip -9 -c "${WORK}/dump.sql" > "${WORK}/dump.sql.gz" || die "gzip gagal"
openssl cms -encrypt -binary -aes-256-cbc -outform DER \
  -in "${WORK}/dump.sql.gz" -out "${ARCHIVE}" "$BACKUP_CERT" \
  || die "enkripsi gagal"

[[ -s "$ARCHIVE" ]] || die "arsip terenkripsi kosong"

# Which key this archive was sealed to. A restore that will not decrypt is
# almost always the wrong key, and this says which one was used without
# revealing anything about it. Computed before the heredoc so no comment can
# leak into the manifest itself.
CERT_FINGERPRINT="$(openssl x509 -noout -fingerprint -sha256 -in "$BACKUP_CERT" | cut -d= -f2)"

cat > "$MANIFEST" <<MANIFEST_EOF
database=${DB_NAME}
taken_at=$(date -Iseconds)
plaintext_bytes=${PLAIN_BYTES}
plaintext_sha256=${PLAIN_SHA}
encrypted_bytes=$(wc -c < "$ARCHIVE" | tr -d ' ')
encrypted_sha256=$(openssl dgst -sha256 -r "$ARCHIVE" | cut -d' ' -f1)
certificate_fingerprint=${CERT_FINGERPRINT}
MANIFEST_EOF

# --- off-site ----------------------------------------------------------------
# Uploaded before local pruning, and the run fails if it does not land. A
# "successful" backup that only exists on the machine being backed up is the
# failure this script was written to remove.
log "unggah ke ${R2_BUCKET}"
aws s3 cp "$ARCHIVE" "s3://${R2_BUCKET}/${BASE}.sql.gz.enc" \
  --endpoint-url "$R2_ENDPOINT" --only-show-errors || die "unggah arsip gagal"
aws s3 cp "$MANIFEST" "s3://${R2_BUCKET}/${BASE}.manifest" \
  --endpoint-url "$R2_ENDPOINT" --only-show-errors || die "unggah manifest gagal"

# Read back the size the bucket reports. A truncated upload otherwise looks
# exactly like a complete one from here.
REMOTE_BYTES="$(aws s3api head-object --bucket "$R2_BUCKET" --key "${BASE}.sql.gz.enc" \
  --endpoint-url "$R2_ENDPOINT" --query ContentLength --output text)" || die "verifikasi unggahan gagal"
LOCAL_BYTES="$(wc -c < "$ARCHIVE" | tr -d ' ')"
[[ "$REMOTE_BYTES" == "$LOCAL_BYTES" ]] \
  || die "ukuran di bucket ($REMOTE_BYTES) tidak sama dengan lokal ($LOCAL_BYTES)"

# --- prune -------------------------------------------------------------------
# Local only. Remote retention is a bucket lifecycle rule, so it keeps working
# when this machine is the thing that failed — see README.
find "$LOCAL_DIR" -name 'safrat_*.sql.gz.enc' -mtime "+${LOCAL_KEEP_DAYS}" -delete
find "$LOCAL_DIR" -name 'safrat_*.manifest' -mtime "+${LOCAL_KEEP_DAYS}" -delete

log "selesai: ${BASE} (${PLAIN_BYTES} byte plaintext, ${LOCAL_BYTES} byte terenkripsi)"
