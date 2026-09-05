# Di mana kita sekarang

Satu halaman, selalu diperbarui. **Titik masuk pertama untuk agen mana pun** —
Claude, Codex, atau siapa pun berikutnya. Kalau hanya sempat membaca satu
berkas, baca ini.

Diperbarui: **5 September 2026** (Corong Pengunjung selesai 33/33 — K2.5 formulir kontak storefront, K2.8 geolokasi DB-IP)

---

## Dua jalur berjalan paralel

| Jalur | Rute | Berkas tugas | Posisi |
|---|---|---|---|
| **Dashboard Travel** | `/dashboard` | [TUGAS-DASHBOARD-TRAVEL.md](TUGAS-DASHBOARD-TRAVEL.md) | Tahap 0–2 selesai · **Tahap 4 selesai** (2 butir sengaja dicatat belum dibangun), sisa T3.2 |
| **Panel SaaS** | `/admin` | [TUGAS-PANEL-SAAS.md](TUGAS-PANEL-SAAS.md) | **A1, A2, B1, B2 selesai** · berikutnya **C4** atau **C5** (kotak masuk Support, baru — lihat T4.11), jatah Codex |
| **Corong Pengunjung** | `/dashboard` + `/admin` | [TUGAS-CORONG.md](TUGAS-CORONG.md) | **33/33 selesai** |
| **SEO & Konten** | storefront | [TUGAS-SEO-KONTEN.md](TUGAS-SEO-KONTEN.md) | **12/17 selesai** — sisa TAHAP S3 menunggu proyek Google Cloud pemilik |

Keduanya tidak beririsan berkas kecuali `globals.css`, `platform.proto`, dan
`admin/page.tsx`.

## Yang sedang menunggu, berurutan

1. **C4 Rotasi kunci & ekspor auditor** — jatah Codex (implementasi Panel
   SaaS dikerjakan Codex; Claude hanya menulis spesifikasi dan verifikasi).
   Rotasi kunci API dengan tumpang tindih 24 jam, dan ekspor auditor (CSV +
   manifes hash).
2. **Tahap 3 Dashboard Travel** — sisa **T3.2** gateway WhatsApp (butuh
   kredensial penyedia dari pemilik). **T3.3 sudah selesai (4 September
   2026)**: Rangkaian, Rundown, dan Armada Bus melengkapi Manifes & Roomlist
   yang sudah ada duluan. CRM Leads dan tier kamar juga sudah selesai.
   **Tahap 4 (T4.1–T4.11) sudah selesai, 4–5 September 2026** di
   `/dashboard/inventaris`, `/dashboard/manasik`, `/dashboard/agenda`,
   `/dashboard/layanan-tambahan`, `/dashboard/momen` (+ `/track/[code]`
   untuk keluarga), `/dashboard/pilgrims/baru`, tab **Laba Rugi** di
   `/dashboard/reports`, tab **Kebijakan Keamanan** + **Notifikasi** di
   `/dashboard/settings`, dan `/dashboard/support` — cakupan lengkap ada di
   TUGAS-DASHBOARD-TRAVEL.md. Dipotong dan dicatat jujur di sana: dua grafik
   Inventaris, pembayaran Layanan Tambahan yang masih penanda lunas/belum,
   Momen yang foto saja (video belum), wizard tanpa "simulasi cicilan",
   laba rugi yang mengecualikan biaya pokok produk yang belum tercatat
   harganya, T4.11 yang baru sisi operator (kotak masuk platform dicatat
   sebagai **C5** di TUGAS-PANEL-SAAS.md, jatah Codex), dan dua butir T4.10
   yang sengaja tidak dibangun: **matriks hak akses** (butuh mesin izin
   granular yang belum ada sama sekali di model peran saat ini) dan
   **aturan eskalasi** (satu-satunya logika eskalasi nyata adalah aturan
   SOS 10 menit — mesin keselamatan, bukan pengaturan bisnis, dan tidak
   diubah tanpa izin eksplisit pemilik).

   **Ditemukan sebelum membangun T4.10, sama seperti kejadian B2 di Panel
   SaaS:** dua dari tiga "kesenjangan keamanan" yang disebut §4.9 DESAIN
   ternyata sudah lama diterapkan tanpa syarat (2FA wajib untuk seluruh
   staf, satu sesi aktif per akun) — dokumen rancangan belum diperbarui
   setelah kapabilitasnya lahir dari komit lain.

   **Tahap 4 selesai. Sisa Dashboard Travel: T3.2** gateway WhatsApp,
   terblokir menunggu kredensial penyedia dari pemilik.

   **Prasyarat data lokal yang belum jelas sebelumnya, ditemukan saat
   menguji wizard di peramban (5 September 2026):** `KYC_ENCRYPTION_KEY`
   ternyata wajib bahkan untuk `CreatePilgrim` biasa, bukan hanya alur KYC —
   kini diisi di `.env` lokal. Produk baru juga butuh baris
   `product_markups` (migrasi 111) sebelum bisa dijual lewat
   `CreateManualOrder`, kalau tidak akan gagal dengan pesan "markup produk
   belum diatur" yang tidak menyebut migrasinya.

   **Bug lama ditemukan dan diperbaiki saat mengerjakan Momen (5 September
   2026):** setiap tautan lihat-foto yang di-presign (termasuk bukti serah
   terima pengiriman yang sudah lama ada) gagal dimuat di peramban sungguhan
   melawan MinIO — `AccessDenied: headers present in the request which were
   not signed`. aws-sdk-go-v2 menyalakan validasi checksum secara default
   sejak ~v1.30, menambahkan header yang tidak pernah dikirim balik oleh tag
   `<img>` atau `fetch` biasa. Diperbaiki di `storage.New`
   (`RequestChecksumCalculation`/`ResponseChecksumValidation` → `WhenRequired`),
   memperbaiki semua fitur presigned-view sekaligus. Baru ketahuan sekarang
   karena belum ada yang menguji jalur ini lewat peramban sungguhan
   melawan MinIO sampai Momen memaksanya.
3. **SEO & Konten** — `sitemap.xml` dan `robots.txt` sudah ada dan sadar-host.
   Sisanya S3: Search Console, menunggu proyek Google Cloud milik pemilik.
4. **Corong pengunjung — 33/33 selesai (5 September 2026).** Layar travel di
   `/dashboard/reports` → tab **Corong Pengunjung**; layar platform di
   `/admin` → tab **Corong**. Beban terukur pada 180.000 baris: rollup
   68 ms/hari, layar travel 177 ms, layar platform 1 ms. **K5.6 sudah
   tertutup** oleh B3. **K2.5** (`d567974`): `crm_leads` tidak pernah punya
   jalur dari pengunjung website ke `source`/`campaign` — `CreateLead`
   adalah CRUD staf, tidak menyentuh browser. Dibangun formulir "Hubungi
   Kami" di storefront + tabel `storefront_inquiries` (migrasi 164, terpisah
   dari `crm_leads` karena itu butuh fitur CRM berbayar dan aktor staf asli)
   + tombol "Jadikan lead" di panel CRM yang mengisi
   `Source=WEBSITE`/`Campaign` dari `utm_campaign` pengunjung sendiri.
   **K2.8** (`252d514`): geolokasi kota/provinsi dari IP, dibangun pakai
   **DB-IP City Lite** bukan MaxMind — skema dan tingkat kota sama, tapi
   tidak butuh akun/kunci lisensi apa pun (dipilih pemilik setelah dijelaskan
   perbandingannya). Berkasnya diunduh dan diperbarui otomatis oleh worker
   tiap bulan; server memantau berkas yang sama tiap 10 menit tanpa perlu
   restart. IP mentah tetap tidak pernah ditulis ke kolom mana pun.

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
migrasi                      152, terpasang di DB dev dan DB uji
working tree                 bersih
belum di-push                10 commit
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

## FUNNEL_SALT/FUNNEL_INGEST_SECRET sudah diisi (4 September 2026)

Pemilik mengisi keduanya langsung di VPS lewat SSH. Saat menerapkannya secara
manual, ditemukan masalah kedua yang jauh lebih serius — lihat di bawah.

## Jebakan yang sudah menipu kami

- **`docker compose up -d` yang dijalankan tangan, tanpa `IMAGE_TAG`, memuat
  gambar `latest` yang basi (dibangun 21 Agustus) — bukan commit yang
  sebenarnya berjalan.** `docker-compose.prod.yml` memakai
  `${IMAGE_TAG:-latest}`; hanya workflow **Deploy to VPS** yang menyetel
  `IMAGE_TAG=<git sha>` dengan benar sebelum memanggil compose. Saat mengisi
  FUNNEL_SALT secara manual dan menjalankan `docker compose up -d` begitu saja
  untuk menerapkannya, kontainer `api` dan `worker` dibuat ulang memakai
  gambar `latest` yang basi — dan gambar basi itu masih memerlukan
  `DATABASE_URL` (kode lama, sebelum commit 21 Agustus 19:16 yang
  menghapusnya), sementara `docker-compose.prod.yml` yang sekarang sengaja
  tidak menyetelnya lagi. Hasilnya: `api` dan `worker` crash-loop dengan
  `"DATABASE_URL is required"` — **production benar-benar turun** selama
  proses ini, bukan cuma corong yang diam.

  **Perbaikan:** `export IMAGE_TAG=<sha commit yang sedang di-checkout>`
  sebelum `docker compose ... pull` dan `... up -d`. Setelah itu ketiga
  kontainer (`api`, `web`, `worker`) terkonfirmasi memakai
  `ghcr.io/.../safrat-api:<sha>` yang benar, sehat, tanpa galat.

  **Aturan ke depan: jangan pernah menjalankan `docker compose up -d` di VPS
  dengan tangan tanpa menyetel `IMAGE_TAG` lebih dulu.** Kalau hanya perlu
  menerapkan `.env.prod` yang berubah tanpa kode baru, tetap sertakan
  `IMAGE_TAG=<sha commit yang sudah berjalan sekarang>` supaya compose tidak
  diam-diam jatuh ke `latest`. Lebih aman lagi: picu ulang workflow **Deploy
  to VPS** dari GitHub Actions, karena itu selalu menyetel tag dengan benar.

## Deploy terakhir GAGAL — dan alasannya benar

Commit `a6bf2ad` sudah di-push, **CI lulus, Deploy gagal**. Produksi masih
menjalankan versi sebelumnya (`d836b97`) dan sehat — deploy berhenti di
`docker compose config`, sebelum menyentuh container yang berjalan.

```
error: required variable FUNNEL_SALT is missing a value
error: required variable FUNNEL_INGEST_SECRET is missing a value
```

`docker-compose.prod.yml` menandai keduanya `:?` (wajib). Itu **disengaja**:
tanpa garam, corong pengunjung diam-diam tidak merekam apa pun, dan kegagalan
diam adalah yang paling mahal di proyek ini. Gerbangnya bekerja persis seperti
maksudnya.

Yang harus dilakukan pemilik — di VPS, **bukan di sesi agen** (kunci produksi
tidak pernah dibuat di sini):

```
ssh <vps>
cd /opt/safrat   # atau di mana .env.prod berada
printf 'FUNNEL_SALT=%s\n' "$(openssl rand -hex 32)" >> .env.prod
printf 'FUNNEL_INGEST_SECRET=%s\n' "$(openssl rand -hex 32)" >> .env.prod
grep -c FUNNEL .env.prod   # harus 2
```

Lalu jalankan ulang workflow **Deploy to VPS** dari GitHub Actions. `BANK_FEED_SECRET`
hanya memberi peringatan (default kosong), jadi ia tidak memblokir deploy —
tapi selama kosong, pencocokan mutasi bank tidak berjalan.

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

- **Uji yang bergantung pada setelan global bisa gagal karena paket lain.**
  3 September 2026: `TestDunningSequence...` gagal dua kali dari sepuluh
  jalan, lalu tidak bisa diulang. Sebabnya fixture-nya membiarkan
  `grace_period_days` NULL — yang artinya "ikut setelan platform" — sementara
  paket uji lain bisa mengubah setelan itu. Grace ditambahkan ke `access_until`,
  jadi grace 2 hari memindahkan langganannya ke tahap yang salah. Dibuktikan
  dengan menyetel `grace_period_days='2'` lalu menjalankan fixture lama: gagal
  dengan `tahap = "H7", mau H14 (telat 12 hari)`. Fixture sekarang mengunci
  nilainya sendiri. **Setiap fixture yang membiarkan kolom NULL karena "nanti
  ada defaultnya" mewarisi keadaan global yang bisa berubah di bawahnya.**

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
