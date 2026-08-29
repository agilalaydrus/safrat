#!/usr/bin/env bash
#
# Put a passphrase on the backup private key.
#
# Why this is worth doing, and not just belt-and-braces: it changes what has to
# be kept secret. An unprotected PEM is the secret, so every copy of it is a
# liability and every place it lives has to be as safe as a vault. A
# passphrase-protected PEM is a locked box — the file can sit on a USB stick, a
# second laptop, a printout in a drawer, and none of those copies is dangerous
# on its own.
#
# That matters here because Bitwarden's free tier cannot attach files. It can
# hold a passphrase perfectly well, and the box can then live anywhere.
#
# The original is only removed once the protected copy has been proved to open.

set -euo pipefail

KEY="${1:-}"
[[ -n "$KEY" && -r "$KEY" ]] || { echo "Penggunaan: protect-key.sh <backup-key.pem>" >&2; exit 2; }

OUT="${KEY%.pem}.locked.pem"
[[ -e "$OUT" ]] && { echo "FATAL: $OUT sudah ada" >&2; exit 1; }

if head -1 "$KEY" | grep -q "ENCRYPTED"; then
  echo "Kunci ini sudah berkata sandi. Tidak ada yang perlu dikerjakan."
  exit 0
fi

cat <<'INTRO'
Anda akan diminta kata sandi dua kali.

Pilih yang panjang — empat sampai enam kata acak jauh lebih kuat dan lebih mudah
diingat daripada satu kata dengan simbol. Kata sandi inilah yang masuk Bitwarden.

Kehilangan kata sandi = kehilangan semua backup. Sama seperti kehilangan kuncinya.

INTRO

umask 077
openssl pkey -in "$KEY" -aes-256-cbc -out "$OUT" || { echo "FATAL: gagal mengunci kunci" >&2; exit 1; }
chmod 0400 "$OUT"

# Proved, not assumed. Removing the original because a command exited zero
# would be trusting the one thing that must not be trusted here — and the
# failure would surface only when a restore was already needed.
echo
echo "Verifikasi: membuka kembali kunci terkunci (masukkan kata sandi sekali lagi)"
openssl pkey -in "$OUT" -noout || { echo "FATAL: kunci terkunci tidak dapat dibuka; ASLINYA TIDAK DIHAPUS" >&2; exit 1; }

cat <<DONE

Berhasil. Kunci terkunci: ${OUT}

  1. Simpan kata sandinya di Bitwarden (field password biasa — muat di paket gratis).
  2. Salin ${OUT} ke minimal dua tempat: flash disk, dan satu lagi di luar rumah.
     Berkas ini sudah terkunci, jadi salinannya tidak berbahaya sendirian.
  3. Setelah keduanya beres, hapus aslinya yang belum terkunci:

       rm ${KEY}

  4. Saat memulihkan, pakai kunci terkunci ini — openssl akan menanyakan kata
     sandinya:

       restore-db.sh --key ${OUT} ...

Aslinya sengaja tidak dihapus otomatis. Menghapus satu-satunya salinan yang
pasti bisa dibuka, atas dasar perintah yang baru saja berjalan, bukan urutan
yang aman.
DONE
