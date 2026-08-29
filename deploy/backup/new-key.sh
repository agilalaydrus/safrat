#!/usr/bin/env bash
#
# Generate the backup key pair — on your own machine, once.
#
# The private half decrypts every backup ever taken. It must never reach the
# server, and it must never be pasted into a terminal that something else is
# reading: an agent session, a screen share, a CI log. This script prints the
# fingerprint and nothing else for exactly that reason.

set -euo pipefail

OUT_DIR="${1:-.}"
KEY="${OUT_DIR}/backup-key.pem"
CERT="${OUT_DIR}/backup-cert.pem"

die() { printf 'FATAL: %s\n' "$*" >&2; exit 1; }

# A guard against the mistake that matters most: creating the private key on the
# machine it is supposed to protect.
#
# Matched on the deployed path only. Checking for ./docker-compose.prod.yml
# refuses every laptop too, because that file is tracked in the repository —
# which is what the first version of this did, and it made the script unusable
# from a normal checkout.
if [[ -f "/home/deploy/safrat/docker-compose.prod.yml" ]]; then
  die "ini tampaknya server produksi. Kunci privat backup tidak boleh dibuat di sini — jalankan di laptop Anda."
fi

mkdir -p "$OUT_DIR" || die "tidak bisa membuat direktori $OUT_DIR"
[[ -e "$KEY" ]] && die "$KEY sudah ada; menimpanya berarti membuang akses ke semua backup lama"

# Errors are captured rather than discarded. Hiding stderr keeps the progress
# dots out of the way, but it also hid the real message the one time this went
# wrong — leaving a failure nobody could diagnose. They are shown on failure and
# only then.
ERRLOG="$(mktemp)"
trap 'rm -f "$ERRLOG"' EXIT
# openssl writes key-generation progress to stderr as long runs of dots and
# plus signs. Kept out of the way, or the one line that explains the failure is
# buried in a screen of punctuation.
fail() {
  printf 'FATAL: %s\n' "$1" >&2
  grep -vE '^[.+*[:space:]]*$' "$ERRLOG" | sed 's/^/  openssl: /' >&2 || true
  exit 1
}

umask 077
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out "$KEY" 2>"$ERRLOG" \
  || fail "gagal membuat kunci"

# The exit code alone is not enough: openssl genpkey returns 0 even when it
# could not write the file — a missing directory produces a clean success and no
# key. Everything after would then fail one step later, blaming the wrong thing,
# which is exactly what happened the first time this was run for real.
[[ -s "$KEY" ]] || fail "kunci tidak tertulis di $KEY meski openssl melaporkan sukses"

# openssl cms encrypts to a certificate, not a bare public key. Long expiry
# because this is not a TLS identity — it is a lockbox, and an expired
# certificate here would stop backups for no security benefit.
openssl req -x509 -new -key "$KEY" -out "$CERT" -days 7300 -subj "/CN=safrat-backup" 2>"$ERRLOG" \
  || fail "gagal membuat sertifikat"
[[ -s "$CERT" ]] || fail "sertifikat tidak tertulis di $CERT"

chmod 0400 "$KEY"
chmod 0444 "$CERT"

FINGERPRINT="$(openssl x509 -noout -fingerprint -sha256 -in "$CERT" | cut -d= -f2)"

cat <<DONE

Selesai. Dua berkas dibuat di ${OUT_DIR}:

  backup-key.pem    RAHASIA — jangan pernah ke server, jangan pernah ke chat
  backup-cert.pem   boleh disalin ke server

Sidik jari sertifikat:
  ${FINGERPRINT}

Isinya sengaja tidak dicetak. Langkah berikutnya:

  1. Simpan backup-key.pem di Bitwarden (lampirkan berkasnya, bukan tempel isinya),
     bersama sidik jari di atas.
  2. Buat satu salinan luring — flash disk di tempat terkunci, atau cetak
     dengan  paperkey/qrencode. Bitwarden pun tidak bisa memulihkan master
     password yang lupa.
  3. Salin HANYA sertifikatnya ke server:

       scp ${CERT} deploy@<vps>:/home/deploy/backup-cert.pem

  4. Hapus salinan kerja kunci privat dari folder ini setelah tersimpan aman.

Kehilangan kunci privat = kehilangan semua backup. Itu harga dari server yang
tidak bisa membaca miliknya sendiri.
DONE
