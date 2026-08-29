#!/usr/bin/env bash
#
# Restore an encrypted backup — and, more often, rehearse restoring one.
#
# A backup nobody has restored is an assumption. This is deliberately easy to
# run against a scratch database so the rehearsal actually happens, and
# deliberately awkward to run against the live one.
#
# The private key lives wherever you keep it — a laptop, a hardware token, a
# sealed envelope — and is passed in for the length of this command only. It
# must never end up on the server this restores onto.

set -euo pipefail

usage() {
  cat <<'USAGE'
Penggunaan:
  restore-db.sh --archive <file.sql.gz.enc> --key <private.pem> --into <nama_db>
                [--manifest <file.manifest>] [--force-into-live]

  --into            nama database tujuan. Gunakan nama uji, mis. safrat_restore_test.
  --manifest        kalau diberikan, ukuran dan checksum plaintext dicocokkan.
  --force-into-live diperlukan untuk menimpa database produksi. Sengaja panjang.
USAGE
}

ARCHIVE="" ; KEY="" ; TARGET="" ; MANIFEST="" ; FORCE_LIVE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --archive) ARCHIVE="$2"; shift 2 ;;
    --key) KEY="$2"; shift 2 ;;
    --into) TARGET="$2"; shift 2 ;;
    --manifest) MANIFEST="$2"; shift 2 ;;
    --force-into-live) FORCE_LIVE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "argumen tidak dikenal: $1" >&2; usage; exit 2 ;;
  esac
done

COMPOSE_FILE="${COMPOSE_FILE:-/home/deploy/safrat/docker-compose.prod.yml}"
DB_USER="${DB_USER:-safrat}"
LIVE_DB="${LIVE_DB:-safrat}"

log() { printf '[%s] %s\n' "$(date -Iseconds)" "$*"; }
die() { log "FATAL: $*"; exit 1; }

[[ -r "${ARCHIVE:-}" ]] || die "arsip tidak terbaca"
[[ -r "${KEY:-}" ]] || die "kunci privat tidak terbaca"
[[ -n "${TARGET:-}" ]] || die "--into wajib diisi"

# The guard that matters. Restoring over the live database is occasionally the
# right thing to do and is never something to do by accident, so it takes a flag
# nobody types without meaning it.
if [[ "$TARGET" == "$LIVE_DB" && "$FORCE_LIVE" -ne 1 ]]; then
  die "menolak menimpa database live '$LIVE_DB' tanpa --force-into-live"
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log "dekripsi"
openssl cms -decrypt -inform DER -binary -in "$ARCHIVE" -inkey "$KEY" \
  -out "${WORK}/dump.sql.gz" || die "dekripsi gagal — kunci salah atau arsip rusak"

log "buka kompresi"
gunzip -c "${WORK}/dump.sql.gz" > "${WORK}/dump.sql" || die "gunzip gagal"

# The check that turns "the file decrypted" into "the file is the backup we
# took". Without the manifest a corrupted-but-decryptable archive restores
# quietly and wrongly.
if [[ -n "$MANIFEST" ]]; then
  [[ -r "$MANIFEST" ]] || die "manifest tidak terbaca"
  WANT_BYTES="$(grep '^plaintext_bytes=' "$MANIFEST" | cut -d= -f2)"
  WANT_SHA="$(grep '^plaintext_sha256=' "$MANIFEST" | cut -d= -f2)"
  GOT_BYTES="$(wc -c < "${WORK}/dump.sql" | tr -d ' ')"
  GOT_SHA="$(openssl dgst -sha256 -r "${WORK}/dump.sql" | cut -d' ' -f1)"
  [[ "$GOT_BYTES" == "$WANT_BYTES" ]] || die "ukuran plaintext $GOT_BYTES != manifest $WANT_BYTES"
  [[ "$GOT_SHA" == "$WANT_SHA" ]] || die "checksum plaintext tidak cocok dengan manifest"
  log "manifest cocok: ${GOT_BYTES} byte, sha256 ${GOT_SHA:0:16}…"
fi

tail -c 200 "${WORK}/dump.sql" | grep -q "PostgreSQL database dump complete" \
  || die "dump terpotong; penanda akhir tidak ditemukan"

log "buat ulang database ${TARGET}"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS \"${TARGET}\"" -c "CREATE DATABASE \"${TARGET}\"" \
  || die "gagal menyiapkan database tujuan"

log "pulihkan"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql -U "$DB_USER" -d "$TARGET" -v ON_ERROR_STOP=1 -q -o /dev/null < "${WORK}/dump.sql" \
  || die "restore gagal"

# Proof it is a working database and not just a file that loaded. A restore
# that ends without counting anything has told you nothing.
log "verifikasi isi"
docker compose -f "$COMPOSE_FILE" exec -T postgres \
  psql -U "$DB_USER" -d "$TARGET" -tAc "
    SELECT 'operators='||(SELECT count(*) FROM operators)
        || ' pilgrims='||(SELECT count(*) FROM pilgrims)
        || ' orders='||(SELECT count(*) FROM orders)
        || ' migrasi_terakhir='||(SELECT max(version_id) FROM goose_db_version)"

log "selesai: ${TARGET}"
