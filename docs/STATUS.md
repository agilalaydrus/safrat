# Di mana kita sekarang

Satu halaman, selalu diperbarui. **Titik masuk pertama untuk agen mana pun** —
Claude, Codex, atau siapa pun berikutnya. Kalau hanya sempat membaca satu
berkas, baca ini.

Diperbarui: **2 September 2026**

---

## Dua jalur berjalan paralel

| Jalur | Rute | Berkas tugas | Posisi |
|---|---|---|---|
| **Dashboard Travel** | `/dashboard` | [TUGAS-DASHBOARD-TRAVEL.md](TUGAS-DASHBOARD-TRAVEL.md) | Tahap 0–2 selesai · **54/74 butir** · sisa Tahap 3–4 |
| **Panel SaaS** | `/admin` | [TUGAS-PANEL-SAAS.md](TUGAS-PANEL-SAAS.md) | **A1, A2, B1 selesai** · 21/68 butir · berikutnya B2 |
| **Corong Pengunjung** | `/dashboard` + `/admin` | [TUGAS-CORONG.md](TUGAS-CORONG.md) | dirancang, **antre** · 0/33 butir |
| **SEO & Konten** | storefront | [TUGAS-SEO-KONTEN.md](TUGAS-SEO-KONTEN.md) | dirancang, **antre** · 0/17 butir |

Keduanya tidak beririsan berkas kecuali `globals.css`, `platform.proto`, dan
`admin/page.tsx`.

## Yang sedang menunggu, berurutan

1. **B2** Meter pemakaian — batas ditegakkan tapi tidak ada yang tahu siapa
   mendekatinya.
2. **B3** Halaman detail tenant.
3. **Tahap 3 Dashboard Travel** — CRM Leads, WhatsApp, rundown, tier kamar.
4. **SEO & Konten** — `sitemap.xml` dan `robots.txt` sudah ada dan sadar-host.
   Sisanya S3: Search Console, menunggu proyek Google Cloud milik pemilik.
5. **Corong pengunjung** — K1–K5 selesai. Layar travel di `/dashboard/reports`
   → tab **Corong Pengunjung**; layar platform di `/admin` → tab **Corong**.
   Sisanya: **K6** dokumen insiden data pribadi + ukur biaya rollup, **K2.8**
   geolokasi (GeoLite2) — sampai itu dipasang bagian "Asal Daerah" kosong dan
   layarnya mengatakan begitu — dan **K5.6** yang menunggu B3 (halaman
   `/admin/tenant/[id]` belum ada).

**Dunning masih mode kering.** Ia berjalan tiap 24 jam, mengisi `dunning_log`,
dan **tidak mengirim apa pun** sampai `DUNNING_LIVE=true` diset. Bandingkan satu
siklus dengan daftar manual sebelum menyalakannya — satu tagihan salah kirim ke
travel yang sudah membayar lebih mahal daripada menunda sepekan.

**Belum diverifikasi di browser:** Paket & Kuota, Routing & Log, serta alur baru
Langganan (mass billing, grace, prorata). Tidak ada browser yang terhubung saat
pass terakhir; semuanya juga butuh sesi admin platform dengan 2FA. Pemilik perlu
membukanya sekali.

## Kondisi terverifikasi (3 September 2026)

```
go build · go vet            bersih
suite Go                     16 paket lulus, 0 gagal, tiga jalan berturut-turut
tsc --noEmit · next lint     bersih
next build                   sukses
migrasi                      148, terpasang di DB dev dan DB uji
working tree                 bersih
belum di-push                18 commit
```

`main` = deploy. **Jangan push tanpa perintah pemilik.**

## Aturan yang berlaku di kedua jalur

- proto → migrasi goose → sqlc → repository → service → handler → UI.
  Repository tidak boleh mengimpor service.
- Operasi yang bisa terulang butuh kunci idempotensi **di database**.
- `requirePlatformAdmin` terlihat di awal setiap metode `PlatformService`.
- Setiap RPC platform baru diuji dua arah: tanpa sesi → `unauthenticated`,
  owner operator asli → `permission_denied`, admin platform → berhasil,
  dicabut → ditolak pada panggilan **berikutnya**.
- Commit tiap unit yang selesai **dan terverifikasi**. Tandai `[x]` di berkas
  tugas beserta hash commit.
- Setiap animasi dibungkus `prefers-reduced-motion`.
- `KYC_ENCRYPTION_KEY` wajib ada untuk membuat jamaah, di lingkungan mana pun.

## Keputusan pemilik yang sudah diambil

- **Trial 10 hari** (2 Sep), dan harus jadi setelan, bukan konstanta. Mengubah
  setelan tidak boleh memendekkan trial yang sedang berjalan.
- **Layar Kesehatan menampilkan yang sehat juga**, hijau, dengan waktu
  pemeriksaan terakhirnya.
- **Implementasi panel SaaS dikerjakan Codex**; Claude menulis spesifikasi dan
  menjalankan pass verifikasi.
- **Live location mentah tidak dibagikan ke keluarga** — momen dan kabar boleh.
- **Corong pengunjung memakai tabel sendiri**, bukan Google Analytics atau
  Plausible: nilainya justru pada sambungan ke data pendaftaran dan pembayaran,
  dan itu hilang kalau datanya di server orang lain.
- **Tanpa cookie dan tanpa menyimpan IP**, supaya storefront pelanggan tidak
  perlu banner persetujuan. Konsekuensinya atribusi lintas hari tidak akurat,
  dan itu harus tertulis di layar.
- Marketplace B2B, aplikasi terpisah, AI berfatwa: **ditunda/ditolak**.

## Pekerjaan pemilik yang belum beres

- [ ] **Repo masih PUBLIC**
- [ ] `BANK_FEED_SECRET` belum diset — poller bank tidak bisa jalan
- [ ] Cron backup R2 belum dipasang
- [ ] Salin `backup-key.locked.pem` ke media kedua, hapus yang belum terkunci

## Kalau sebuah sesi terputus di tengah

Ini sudah terjadi. Yang menyelamatkannya:

1. **Periksa working tree lebih dulu** — `git status`. Pekerjaan yang belum
   di-commit adalah tempat paling rapuh.
2. **Nilai sebelum menyelamatkan**, jangan sebaliknya: `go build`, `go vet`,
   suite, dan cek migrasi terpasang di **kedua** DB.
3. **Commit apa adanya**, sebut siapa penulisnya dan apa yang belum selesai.
4. Perbarui berkas tugas dan berkas ini.

## Jebakan yang sudah menipu kami

Tulis di sini setiap kali ketemu lagi.

- **Volume Docker lokal bisa hilang saat daemon mati.** 3 September 2026:
  Docker Desktop berhenti di tengah suite, dan saat dihidupkan lagi kedua
  volume (`safrat_postgres_data`, `safrat_redis_data`) sudah tidak ada —
  Postgres bootstrap cluster kosong. Image-nya selamat (umur dua minggu), jadi
  disk VM tidak dihapus; yang hilang hanya container dan volumenya. Gejalanya
  menyesatkan: tesnya gagal dengan *connection refused*, lalu setelah DB naik
  lagi gagal dengan *relation does not exist* — dua-duanya terlihat seperti bug
  kode. **Periksa `docker volume inspect <nama> --format '{{.CreatedAt}}'`**:
  kalau waktunya sama dengan `docker compose up` barusan, volumenya baru, bukan
  yang lama. Membangun ulang: `goose up` (berhenti di 025 karena tabel `"user"`
  belum ada) → `npx @better-auth/cli migrate -y` dari `apps/web` → `goose up`
  lagi → `createdb -T safrat safrat_limit_test`. Tidak ada data produksi yang
  tersentuh; yang hilang hanya DB dev dan DB uji lokal.

- **Kode hasil sqlc gitignored.** `git stash` tidak mengembalikannya, jadi uji
  "di HEAD" bisa memakai kode yang sudah berubah dan menghasilkan kesimpulan
  terbalik.
- **DB uji tertinggal dari DB dev.** Gejalanya galat kolom tidak ada, bukan
  pesan migrasi.
- **`psql` mengembalikan 0 walau gagal** tanpa `ON_ERROR_STOP=1`. Skrip uji
  bisa hijau padahal seed-nya sendiri gagal — ini menghasilkan **tujuh
  penolakan palsu** sekali waktu.
- **`head -20` pada sapuan audit** membuat pekerjaan dilaporkan selesai padahal
  terpotong.
- **`serviceError` menelan galat tak terpetakan** ke Sentry, yang no-op saat
  `SENTRY_DSN` kosong. Di dev galat itu hilang tanpa jejak.
- **Skrip pengaman bisa jadi usang** dan gagal karena aturan baru, bukan karena
  ada yang bocor. Selalu baca teks galatnya, jangan hanya kode keluar.
- **Menghitung paket lulus bukan hasil.** `grep -c "^ok"` mengembalikan sukses
  walau ada yang gagal, dan sebuah commit sempat lolos dengan dua tes merah.
  Periksa `^--- FAIL` secara eksplisit.
- **Tes bisa flaky karena asersi global.** Paket uji berjalan paralel terhadap
  satu database; hitungan lintas-tenant dan nominal transfer yang dikarang akan
  bertabrakan. Jalankan suite beberapa kali sebelum percaya ia hijau.
