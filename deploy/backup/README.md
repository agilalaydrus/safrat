# Backup: terenkripsi, di luar VPS, dan terbukti bisa dipulihkan

Prosedur lama menulis `pg_dump` ke `/home/deploy/backups` di VPS yang sama,
tanpa enkripsi, dan tidak pernah sekali pun diuji restore. Tiga masalah
sekaligus, dan hanya satu yang terlihat sebelum dibutuhkan.

## Prinsip yang menentukan rancangannya

**VPS tidak boleh bisa membuka backup-nya sendiri.** Kalau server dibobol,
penyerang memang mendapat database yang hidup — itu tidak terhindarkan. Yang
tidak boleh ikut jatuh adalah *seluruh riwayat*. Jadi enkripsinya kunci publik:
server hanya memegang sertifikat, kunci privatnya tidak pernah ada di sana.

**Backup di sebelah database bukan backup.** Dump yang duduk di mesin yang sama
selamat dari `DROP TABLE` dan tidak selamat dari apa pun yang lain — disk mati,
`rm` yang salah, penyedia kehilangan instance.

## 1. Buat kunci — sekali saja, dan BUKAN di VPS

```bash
./deploy/backup/new-key.sh ~/safrat-backup-key
```

Skrip itu menolak berjalan kalau mendeteksi jalur deploy produksi, memasang izin
0400 pada kunci privat, dan **hanya mencetak sidik jarinya** — isinya tidak
pernah muncul di layar, karena layar bisa saja sedang direkam, dibagikan, atau
dibaca sesi agen.

Jangan menjalankannya di dalam sesi agen atau screen share. Kunci ini membuka
seluruh riwayat database.

`openssl cms` meminta **sertifikat**, bukan kunci publik telanjang — itulah
kenapa skripnya juga membuat sertifikat self-signed. Sertifikat inilah "bagian
publik"-nya.

- **`backup-key.pem`** → **jangan pernah** taruh di VPS.
- **`backup-cert.pem`** → salin ke VPS di `/home/deploy/backup-cert.pem`.

### Menyimpan kunci privat tanpa Bitwarden berbayar

Paket gratis tidak bisa melampirkan berkas. Beri kata sandi pada kuncinya, dan
yang perlu disimpan rahasia berubah: bukan lagi berkasnya, tapi kata sandinya —
dan itu muat di field password biasa.

```bash
./deploy/backup/protect-key.sh ~/safrat-backup-key/backup-key.pem
```

Setelah terkunci, berkas kuncinya **tidak berbahaya sendirian**. Ia boleh ada di
flash disk, di laptop kedua, di Drive, di email ke diri sendiri. Yang tidak boleh
ada di tempat yang sama hanyalah kata sandinya.

**Satu salinan saja tidak cukup, di mana pun ia disimpan.** Laptop hilang, disk
mati, atau `rm` yang salah membuat setiap backup tidak terbaca selamanya — persis
kegagalan yang backup ini ada untuk mencegah. Minimal dua tempat, dan salah
satunya di luar rumah.

Kehilangan kunci privat berarti kehilangan seluruh backup. Itu memang harganya
dari server yang tidak bisa membaca miliknya sendiri.

Catat sidik jari sertifikatnya, karena manifest mencantumkannya:

```bash
openssl x509 -noout -fingerprint -sha256 -in backup-cert.pem
```

## 2. Pasang di VPS

```bash
install -m 0700 deploy/backup/backup-db.sh  /home/deploy/backup-db.sh
install -m 0700 deploy/backup/restore-db.sh /home/deploy/restore-db.sh
install -m 0400 backup-cert.pem /home/deploy/backup-cert.pem

# Bucket terpisah dari bucket unggahan: kunci R2 yang bocor untuk aset
# tidak boleh bisa menjangkau backup.
cat >> /home/deploy/.backup-env <<'ENV'
export BACKUP_R2_BUCKET=safrat-backups
export BACKUP_R2_ENDPOINT=https://<account>.r2.cloudflarestorage.com
export AWS_ACCESS_KEY_ID=<kunci khusus backup>
export AWS_SECRET_ACCESS_KEY=<rahasianya>
ENV
chmod 0600 /home/deploy/.backup-env

crontab -e
# 0 2 * * * . /home/deploy/.backup-env && /home/deploy/backup-db.sh >> /home/deploy/backup.log 2>&1
```

Butuh `aws` cli. Skripnya berhenti dengan pesan jelas kalau tidak ada, bukan
melewati unggahan diam-diam — backup yang "berhasil" tapi hanya ada di mesin
yang sedang di-backup persis masalah yang mau dihilangkan.

## 3. Retensi

Lokal 7 hari (dipangkas skrip). Jarak jauh diatur **lifecycle rule di bucket**,
bukan oleh skrip — supaya tetap berjalan justru ketika VPS-nya yang mati:

```json
{"Rules":[{"ID":"safrat-backups-30d","Status":"Enabled",
  "Filter":{"Prefix":"safrat_"},"Expiration":{"Days":30}}]}
```

Kunci R2 untuk backup harus **write-only**. Kebijakannya siap pakai di
`r2-backup-policy.json`: `PutObject` + `HeadObject`, tanpa `DeleteObject` dan
tanpa `GetObject`.

- **Tanpa `DeleteObject`** — server yang dibobol lalu bisa menghapus backup lama
  kehilangan justru hal yang dilindungi.
- **Tanpa `GetObject`** — server tidak perlu membaca backup-nya sendiri. Yang
  memulihkan adalah mesin yang memegang kunci privat.
- **`HeadObject` tetap ada** karena skrip mencocokkan ukuran setelah unggah;
  unggahan terpotong dari sisi pengirim terlihat identik dengan yang utuh.

Aturan lifecycle-nya ada di `r2-lifecycle.json` — 30 hari, bukan 7, karena
prosedur rotasi kunci (`DEPLOY.md` 12c) mensyaratkan kunci lama disimpan sampai
backup yang mendahului rotasi habis retensinya.

## 4. Latihan restore — ini bagian yang tidak boleh dilewati

Backup yang belum pernah dipulihkan adalah asumsi. Jalankan ini **setiap bulan**
dan setelah setiap perubahan skema besar:

```bash
aws s3 cp s3://safrat-backups/safrat_<stamp>.sql.gz.enc . --endpoint-url "$BACKUP_R2_ENDPOINT"
aws s3 cp s3://safrat-backups/safrat_<stamp>.manifest    . --endpoint-url "$BACKUP_R2_ENDPOINT"

./restore-db.sh \
  --archive safrat_<stamp>.sql.gz.enc \
  --manifest safrat_<stamp>.manifest \
  --key /path/ke/backup-key.pem \
  --into safrat_restore_test
```

Hasilnya harus mencetak jumlah baris dan versi migrasi. **Kalau tidak mencetak
angka, restore-nya tidak terbukti** — file yang termuat tanpa dihitung tidak
memberi tahu apa pun.

Menimpa database live butuh `--force-into-live`. Sengaja panjang.

Setelah latihan: `DROP DATABASE safrat_restore_test;`

## Apa yang diperiksa dan kenapa

| Pemeriksaan | Kegagalan yang ditangkapnya |
|---|---|
| exit code `pg_dump`, dump ditulis ke file dulu | pipeline menulis arsip yang tampak wajar padahal `pg_dump` mati di tengah — byte-nya ada, trailer gzip ada, potongannya tak terlihat sampai restore |
| penanda `dump complete` di ekor | dump terpotong yang ukurannya masih besar |
| checksum plaintext di manifest | arsip yang bisa didekripsi tapi isinya sudah rusak |
| `head-object` setelah unggah | unggahan terpotong, yang dari sisi VPS terlihat sama persis dengan yang utuh |
| unggah sebelum pemangkasan lokal | menghapus salinan lokal saat salinan jauh belum benar-benar mendarat |
| penolakan `--into safrat` | menimpa produksi saat yang dimaksud adalah latihan |

Semuanya sudah dijalankan sungguhan terhadap Postgres, bukan hanya ditulis:
roundtrip enkripsi cocok byte-per-byte, kunci salah ditolak, manifest yang
dipalsukan ditolak, dan restore menghasilkan database berisi.
