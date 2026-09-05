# Tugas: Panel SaaS TawafiqHub

Rancangan: [RENCANA-PANEL-SAAS.md](RENCANA-PANEL-SAAS.md).
Spesifikasi layar: [DESAIN-PANEL-SAAS.md](DESAIN-PANEL-SAAS.md).
Sistem desain: [DESAIN-DASHBOARD-TRAVEL.md](DESAIN-DASHBOARD-TRAVEL.md).
Dibuat 2 September 2026. **Belum ada tugas yang dikerjakan.**

> **Yang mana panel ini.** Berkas ini soal **`/admin`** — panel platform milik
> pemilik TawafiqHub, satu-satunya permukaan yang melihat seluruh tenant.
> **Bukan** `/dashboard`, yang dipakai staf travel pelanggan kita. Panel itu
> punya berkasnya sendiri: [TUGAS-DASHBOARD-TRAVEL.md](TUGAS-DASHBOARD-TRAVEL.md).

## Pembagian kerja

Tandai pemilik sebelum mulai supaya dua agen tidak menyentuh berkas yang sama:

- `[C]` Codex — implementasi panjang & terspesifikasi

> **Keputusan pemilik 2 September 2026:** seluruh implementasi panel SaaS
> dikerjakan Codex. Claude menulis spesifikasi dan menjalankan pass verifikasi
> setelah tiap tahap. Karena itu setiap butir di berkas ini berpenanda `[C]`
> kecuali disebut lain.
>
> **Diperluas 5 September 2026:** pemilik meminta Claude melanjutkan
> pekerjaan implementasi Codex juga, bukan hanya spesifikasi. Butir yang
> Claude ambil alih implementasinya ditandai `[K, diimplementasikan]` di
> tempatnya masing-masing, dengan tanggal dan commit — supaya jelas mana
> yang benar-benar sudah dikerjakan siapa, bukan cuma "jatahnya siapa".
- `[K]` Claude — spesifikasi, verifikasi, keputusan lintas-modul & keamanan
- `[ ]` belum diklaim

**Aturan tabrakan.** Satu working tree dipakai bersama, jadi:
1. Klaim tugas di berkas ini **sebelum** menyentuh kode.
2. Jangan dua agen menyunting sekaligus. Kalau perlu paralel, pakai
   `git worktree` — direktori dan branch terpisah.
3. Berkas panas yang mudah bentrok: `apps/web/app/globals.css`,
   `apps/web/app/admin/page.tsx`, `proto/hajj/v1/platform.proto`, dan berkas
   tugas ini sendiri.
4. Setelah tiap tahap, jalankan pass verifikasi: `go build`, `go vet`, suite Go,
   `tsc --noEmit`, `next lint`, dan skrip pengaman yang relevan — **dan periksa
   apakah ada yang hijau karena alasan salah.** Itu sudah terjadi dua kali di
   proyek ini.

## Aturan yang berlaku untuk semua tugas

DDL untuk semua tabel baru sudah dirancang di **§9 DESAIN** — pakai itu,
jangan merancang ulang. `dunning_log`, `privileged_actions`, dan
`impersonation_sessions` adalah **bukti, bukan cache**: ikuti pola
`REVOKE UPDATE, DELETE` migrasi 125.

- proto → migrasi goose → sqlc → repository → service → handler → UI.
  Repository **tidak boleh** mengimpor service.
- `requirePlatformAdmin` tetap terlihat di awal setiap metode baru, bukan di
  interceptor. Tidak ada apa pun di `PlatformService` yang di-scope tenant.
- Setiap operasi yang bisa terulang (terbit invoice, kirim dunning,
  menangguhkan) butuh kunci idempotensi **yang dipaksakan di database**.
- Setiap perubahan komersial dan setiap **pembacaan data pribadi tenant** masuk
  `audit_logs`.
- Pakai komponen dan `tone` yang sudah ada. Jangan bikin bahasa visual baru.
- Commit tiap unit yang selesai **dan terverifikasi**. **Jangan push ke `main`**.

---

# TAHAP A — Yang sudah terlanjur mendarat tanpa kendali

## A1 — Paket & Kuota 🔴 paling mendesak

> **A1 SELESAI.** Backend oleh Codex (`789d7d9`) sebelum kuotanya habis; layar
> oleh Claude. Enam RPC sekarang punya pemanggil.

T2.2 sudah menegakkan batas lewat trigger, tapi `plan_limits` dan
`plan_overrides` **tidak punya satu pun RPC**. Menaikkan kuota satu pelanggan
hari ini = menulis SQL di produksi.

- [x] Proto: `ListPlanLimits`, `SetPlanLimit`, `ListPlanOverrides`,
      `SetPlanOverride`, `DeletePlanOverride`, `PreviewPlanLimitChange` (`789d7d9`)
- [x] Override wajib punya **alasan**; kolom `expires_at` + worker harian yang
      mencabut yang kedaluwarsa (`789d7d9`)
- [x] `PreviewPlanLimitChange` mengembalikan tenant beserta **namanya** (`789d7d9`)
- [x] Grandfathering: dikunci di angka lama, tidak ditendang (`789d7d9`)
- [x] Perubahan batas ditulis ke `audit_logs`, plus `privileged_actions` (`789d7d9`)
- [x] Tab **Paket & Kuota** di `/admin` (`5140128`)
- [x] Uji dua arah: override naik untuk satu tenant, tidak bocor ke tenant lain;
      plus uji idempotensi (kunci sama payload beda → `ErrConflict`) (`789d7d9`)

## A2 — Routing produk & log supplier 🟠

Menutup mesin tanpa pemicu. RPC-nya sudah ada, teruji, tidak dipanggil siapa pun.

- [x] Layar routing memakai `ListProductRoutes` + `SaveProductRoute` (`7107574`)
- [x] Produk **tanpa routing** jadi antrean kerja di atas. Query-nya diperbaiki
      lebih dulu — dulu berangkat `FROM product_routes` sehingga yang belum
      dirutekan tidak pernah muncul (`ff77d8d`)
- [x] Log supplier memakai `ListSupplierLogs`: permintaan, respons, HTTP,
      hasil, referensi, biaya. **Latensi tidak ada di data** — tidak dikarang
      (`7107574`)
- [x] Tautan dari transaksi menggantung → log supplier terkait (`98d64fa`)

---

# TAHAP B — Mesin komersial

## B1 — Langganan & dunning 🔴

- [x] Siklus tagihan massal: tinjau dulu daftar invoice + nominalnya, terbitkan
      sekaligus (`7692809`)
- [x] Dunning H+1, H+7, H+14 → penangguhan otomatis H+21, jadwalnya dari
      `platform_settings` (`db9fa97`)
- [x] **Kunci idempotensi di database.** Bukan `(invoice_id, stage)` seperti
      rancangan: invoice kedaluwarsa tepat saat akses habis lalu diterbitkan
      ulang, jadi bukan jangkar yang stabil. Kuncinya
      `(operator_id, lapsed_at, stage)` (`db9fa97`)
- [x] Siklus massal tunduk pada indeks nominal unik transfer — kegagalan
      sufiks dilaporkan **per baris**, tidak membatalkan seluruh siklus (`7692809`)
- [x] Grace period yang bisa diatur, global dan override per tenant; akses,
      dunning, dan penangguhan membaca batas efektif yang sama (`7b7d834`)
- [x] Void invoice menyimpan barisnya + alasan + audit; yang sudah lunas tidak
      bisa dibatalkan (`9233cf1`)
- [x] Prorata saat upgrade/downgrade di tengah periode: ledger adjustment
      append-only, upgrade aktif sesudah dibayar tanpa menambah masa akses,
      downgrade menjadi kredit yang dipakai saat renewal lunas (`0504d63`, `fb120ef`)
- [x] Penangguhan lewat **waktu, bukan status** + `suspended_at`. Interceptor
      tidak berubah sama sekali (`db9fa97`)
- [x] Pembayaran kapan pun **memulihkan akses**, termasuk sesudah H+21, tanpa
      campur tangan manual — di dalam `extendAccess` supaya kedua jalur
      pembayaran melewatinya (`db9fa97`)
- [x] Tab **Langganan** di `/admin` (`4a8f514`)

## B2 — Meter pemakaian ✅

- [x] Tabel `usage_counters` (`0c32e52`)
- [x] Worker harian, satu transaksi ber-`FOR SHARE` atas daftar operator supaya
      tenant yang dihapus di tengah jalan tidak merobek snapshot (`0c32e52`)
- [x] Metrik: jamaah, cabang, penyimpanan. **Panggilan API dan pesan WhatsApp
      sengaja tidak ada** — belum ada yang mencatatnya, dan nol palsu akan
      terbaca sebagai travel yang tidak memakainya (`0c32e52`)
- [x] Tab **Pemakaian**: pemakaian vs batas, peringatan 80% dan 100% (`0c32e52`)
- [x] Subjudul menyebut kapan dihitung, dan mengaku angkanya bisa tertinggal
      beberapa jam (`0c32e52`)

## B3 — Detail tenant `/admin/tenant/[id]` ✅

- [x] Langganan & riwayat tagihan · pemakaian vs kuota · override berlaku
- [x] Jamaah, cabang, musim, produk, pendaftaran, transaksi tertahan, KYC
- [x] Tim & status 2FA · domain · jejak audit tenant itu
- [x] Corong storefront 30 hari, sehingga langganan, pemakaian, dan permintaan
      terbaca bersama (menutup **K5.6**)
- [x] Tombol **Lihat sebagai travel ini** (C1), dengan alasan yang harus
      diketik dan riwayat sesi lihat-saja di bawahnya.
- [ ] Tombol ubah override dan tangguhkan — **sengaja tidak di sini**. Keduanya
      sudah punya konfirmasi dan jejak di tabnya masing-masing; menyalinnya ke
      sini berarti dua jalan menuju tindakan yang tidak bisa ditarik. Halaman
      ini hanya membaca, dan mengatakannya di kaki halaman.

**Catatan teknis:**

- Satu RPC `GetTenantDetail`, bukan halaman yang memanggil delapan RPC daftar
  lalu menyaring di peramban. Cara kedua menarik langganan, pemakaian, dan
  tagihan **seluruh tenant** ke layar tentang satu tenant, dan lebih lambat.
- Batas pemakaian dibaca dengan bentuk kueri yang sama persis dengan
  `ListUsage` (paket dari `operators`, batas dari `plan_limits` ditimpa
  override yang masih berlaku). Dua definisi berbeda antara daftar dan detail
  berarti dukungan menyebut satu angka sementara penegakan memakai angka lain.
- `DISTINCT ON` pada `usage_counters`: tanpa itu setiap periode lama ikut
  tampil sebagai baris pemakaian hari ini.
- Gerbang akses platform dipindah ke `PlatformGate` supaya halaman baru tidak
  menyalin ulang logika empat keadaannya.
- Diuji: `platform_tenant_http_test.go` memastikan halaman ini hanya berisi
  travel yang diminta — jejak audit travel lain tidak terbaca, dan travel
  pertama tidak bocor ke halaman travel kedua. Diverifikasi dengan membuang
  `WHERE a.operator_id = $1`: uji gagal dengan `jejak audit travel lain
  terbaca di halaman ini`. Id yang tidak dikenal dan id ngawur sama-sama
  `not_found`, bukan galat internal.

---

## Pass visual panel platform — babak dua ✅

Empat tab yang saya bangun (Corong, Analitik, Kesehatan, Audit) awalnya memakai
**inline style** — persis penyebab "kaku" yang didiagnosis
[DESAIN-DASHBOARD-TRAVEL.md](DESAIN-DASHBOARD-TRAVEL.md) §2b: sembilan dari
sepuluh aturan visualnya (cincin fokus, bayangan bernama, transisi bertingkat,
hover naik satu langkah) **tidak bisa ditulis sama sekali** dari objek style
JavaScript.

Sekarang memakai komponen bersama yang sudah ada — `PageHeader`, `StatCard`,
`Badge`, `EmptyState`, `MethodologyNote`, `Button` — plus satu blok kelas
`.admin-*` di `globals.css` untuk tata letaknya.

**Diperiksa di peramban sungguhan, bukan dikira-kira.** Halaman contoh berisi
markup tiap kelas dirender dengan Playwright, nilai gaya terhitungnya dibaca
kembali, dan tangkapan layarnya dilihat. Tiga cacat yang hanya kelihatan dari
gambarnya:

1. `.tw-card` **tidak punya padding** — setiap pemakainya menyediakan sendiri,
   dan versi pertama layar Analitik tidak. Isinya menempel ke tepi kartu.
   Diperbaiki dengan `.admin-panel`.
2. Kelompok tombol periode **terentang penuh** selebar kolom grid, sehingga
   terlihat seperti bilah navigasi alih-alih pilihan kecil. `justify-self: start`.
3. Baris pergerakan MRR terlalu rapat sampai label dan bilah menyatu jadi satu
   blok padat. Jarak dinaikkan dari 12px ke 18px.

Nilai terhitung yang diverifikasi: bilah bergerak **700 ms** (aturan 4), ujung
membulat 999px bukan siku, kartu sinyal 16px, cincin fokus 4px merek-muda,
`shadow-soft` pada kartu, dan `prefers-reduced-motion` mematikan transisinya.

**Babak dua**, setelah pemilik meminta perapian menyeluruh: kelas tata letak
yang tadinya bernama `.admin-*` dipindah ke ruang nama `.tw-*` di
`globals.css` (`tw-panel`, `tw-signal`, `tw-flow`, `tw-table`, `tw-segmented`,
`tw-stat-grid`, dst) — kelasnya milik sistem desain, bukan milik satu modul,
jadi layar lain (dashboard travel, kloter) bisa memakainya tanpa menulis
ulang. `TenantDetail.tsx` (halaman terbesar dan terakhir yang masih inline)
dipindah ke `PageHeader`, `StatCard`, `Badge`, `EmptyState`.

Diperiksa lagi dengan Playwright, bukan diasumsikan selesai karena typecheck
lulus. Halaman tenant sungguhan dirender dan dilihat.

**Yang sengaja *tidak* disentuh:** `KloterManifest`, `KloterRoomlist`,
`RoomTierEditor`. Ketiganya sudah rapi — token warna, radius, dan jarak
konsisten — hanya mengikuti konvensi inline `KloterDetail`/`PricingDashboard`
yang menaunginya, bukan `tw-*`. Memigrasikan hanya potongan kecil di dalam
halaman yang seluruhnya masih inline akan membuat satu halaman punya dua
bahasa visual sekaligus — lebih buruk daripada konsisten inline. Migrasi
penuhnya adalah **T0.5** ("sapu 27 layar") di
[TUGAS-DASHBOARD-TRAVEL.md](TUGAS-DASHBOARD-TRAVEL.md), tugas tersendiri yang
lebih besar dari perapian ini.

---

# TAHAP C — Keamanan sebelum tim bertambah

## C1 — Impersonate dengan jejak penuh ✅

- [x] Sesi impersonasi ditandai berbeda di seluruh sistem: spanduk oranye
      lengket di **setiap** halaman, dengan nama travel dan hitung mundur, dan
      tidak bisa ditutup selain dengan mengakhiri sesinya.
- [x] **Read-only, tanpa mode tulis sama sekali.** Ditegakkan di interceptor
      dengan **tolak-secara-baku**: hanya awalan `List`/`Get`/`Preview`/`Count`/
      `Am` yang lolos, dan `PlatformService`/`FunnelService` tertutup penuh.
      RPC baru yang ditambahkan tahun depan otomatis tertolak sampai seseorang
      memutuskan sebaliknya.
- [x] Berbatas waktu (maksimal 60 menit, dipaksa di repository), berakhir
      sendiri lewat kueri pencarian — bukan lewat timer yang harus diingat
      seseorang untuk dijalankan.
- [x] Dicatat lengkap sebelum sesinya mulai: siapa, travel mana, IP, user agent,
      alasan (minimal 10 huruf, dipaksa oleh CHECK di database), durasi. Entri
      audit ditulis pada **travel itu**, bukan pada platform — rekam jejak
      pelanggan seharusnya memperlihatkan bahwa kami membuka akunnya.
- [x] Token tidak pernah disimpan apa adanya, hanya SHA-256-nya. Diuji: token
      hasil `StartImpersonation` tidak ditemukan di kolom mana pun.
- [x] Kunci idempotensi unik di database, bukan cek-lalu-tulis: tombol yang
      diklik dua kali tidak membuka dua sesi.

**Diuji, dan diverifikasi dengan merusak:**

- `impersonation_test.go` menelusuri **323 prosedur** dari deskriptor yang
  dihasilkan dan memastikan tidak satu pun nama yang berawalan kata kerja tulis
  bisa lolos (115 boleh dibaca). Daftar kata kerjanya ditulis terpisah dari
  aturannya — kalau diturunkan dari aturan yang diuji, pernyataannya hanya
  berbunyi "aturan ini setuju dengan dirinya sendiri".
- `impersonation_http_test.go` menjalankan `CreateSeason` sungguhan lewat
  interceptor sungguhan. Permintaannya sengaja **valid sepenuhnya**, supaya
  tanpa pengaman barisnya benar-benar masuk. Dibuktikan: dengan `Create`
  ditambahkan ke daftar awalan yang diizinkan, panggilannya **berhasil** dan
  musim baru tertulis di akun pelanggan.
- Token palsu, sesi kedaluwarsa, dan sesi yang sudah ditutup semuanya
  `unauthenticated` dan tidak bisa dibedakan satu sama lain.

**Keputusan yang perlu diketahui:**

- Impersonasi **melewati gerbang langganan**. Akun yang terkunci justru yang
  paling perlu dilihat, dan ini aman hanya karena sesinya tidak bisa menulis.
- Header impersonasi **ditolak** (bukan diabaikan) pada prosedur yang tertutup,
  di semua jalur autentikasi. Peramban sendiri tidak pernah mengirimkannya ke
  `PlatformService` — panel platform selalu dijalankan sebagai admin itu
  sendiri, dan itu juga yang membuat sesinya bisa ditutup: tombol "Akhiri"
  adalah panggilan `PlatformService`.
- Token disimpan di `sessionStorage`, bukan `localStorage`: sesi yang selamat
  dari tab yang ditutup adalah sesi yang dilupakan orang.

## C2 — Four-eyes untuk tindakan tak bisa ditarik ✅ (sebagian, dengan alasan)

- [x] **Konfirmasi dengan mengetik nama tenant**, dibandingkan di repository
      terhadap nama sungguhannya. Dicek di sana, bukan di service, karena di
      situlah nama aslinya diketahui — konfirmasi yang dibandingkan dengan nilai
      yang dikirim pemanggil sendiri tidak mengkonfirmasi apa pun. Huruf besar-
      kecil dan spasi di ujung dimaafkan; katanya tidak.
- [x] `privileged_actions` **sudah ada sejak migrasi 138**, dan sudah dipakai
      `SET_PLAN_LIMIT`. Yang belum: `SUSPEND` dideklarasikan tapi **tidak ada
      satu pun jalur yang menulisnya** — tidak ada RPC penangguhan manual sama
      sekali. Sekarang ada, plus `REINSTATE` (migrasi 150) sebagai jenis
      tersendiri: "kenapa dikunci" dan "siapa yang membuka" adalah dua
      pertanyaan berbeda, dan field di dalam payload tidak bisa diindeks
      atau dibaca sekilas.
- [x] `approved_by = requested_by` selama admin masih satu, **dan barisnya
      menyebut berapa admin yang ada saat itu**. Satu berarti persetujuan orang
      kedua memang belum mungkin — bukan bahwa aturannya dilewati. Hari ada
      admin kedua, tanda tangannya punya tempat dan baris-baris lama tidak
      berpura-pura pernah memilikinya.
- [x] Masuk `audit_logs` dengan alasannya, di transaksi yang sama dengan
      perubahannya.
- [x] Bisa dibaca: tabel **Tindakan yang tidak bisa ditarik** di halaman detail
      tenant. Tabel yang tidak pernah dibaca adalah tabel yang tidak ada yang
      sadar kalau kosong.
- [x] **Menghapus tenant** — bagian dari `privileged_actions` di atas, tapi
      syaratnya lebih berat: tawarkan ekspor data lebih dulu (hak
      portabilitas UU PDP), dan `audit_logs` tidak ikut terhapus. Lihat D6.
- [ ] **Rekening settlement** — tidak ada yang bisa dirutekan. Nomornya hanya
      ada di `.env.prod` di VPS, tidak pernah di database dan tidak pernah di
      repo. Mengubahnya berarti masuk ke server, bukan menekan tombol. Kalau
      suatu hari pindah ke database, jalurnya lewat sini.

**Penangguhan bekerja lewat satu kolom, dan itu disengaja:**
`suspended_at` diisi, `access_until` **tidak disentuh**. Akses ditentukan oleh
`suspended_at IS NULL AND effective_access_until > NOW()`, jadi kolom itu
menutup pintu seketika sementara waktu yang sudah dibayar terus berjalan di
bawahnya. Membuka kembali mengembalikan persis yang mereka beli, tanpa
aritmetika yang bisa salah.

**Diuji, dan diverifikasi dengan merusak:** `suspension_http_test.go` memastikan
nama yang salah tidak menangguhkan siapa pun, penangguhan benar-benar menutup
akses travel yang **masih punya sisa waktu 30 hari**, `access_until` tidak
berubah, pengulangan dengan kunci sama tidak membuat baris kedua, dan pemulihan
mengembalikan waktu yang sama persis. Dengan pemeriksaan nama dilumpuhkan,
ujinya gagal: `nama salah = unknown` — artinya nama yang salah benar-benar
menangguhkan travelnya.

## C3 — Audit pembacaan data pribadi ✅

- [x] `personal_data_reads` (migrasi 151): satu baris per orang, per layar, per
      travel, per hari, dengan penghitung. Bukan satu baris per permintaan —
      itu akan jadi puluhan ribu baris yang tidak pernah dibaca, dan pertanyaan
      yang benar-benar ditanyakan dijawab oleh hitungannya.
- [x] Dicatat di interceptor untuk **setiap** pembacaan lewat sesi lihat-saja,
      diklasifikasikan per **service** (bukan per prosedur) supaya RPC baru pada
      layar yang sama ikut tercakup hari ia ditambahkan, bukan hari seseorang
      ingat mendaftarkannya.
- [x] `GetKycRecord` tetap menulis entri audit **dan** baris pembacaan: entri
      audit menyebut siapa orangnya (yang dibutuhkan saat menyelidiki insiden),
      baris pembacaan menyimpan hitungan hariannya (yang memperlihatkan pola).
      `ListKycRecords` yang sebelumnya sama sekali tidak tercatat, sekarang
      tercatat.
- [x] Bisa dibaca di halaman detail tenant — **Pembacaan data pribadi oleh
      TawafiqHub**, dengan penanda apakah dari panel atau dari sesi lihat-saja.
- [x] Inventaris di [INSIDEN-DATA-PRIBADI.md](INSIDEN-DATA-PRIBADI.md)
      diperbarui, termasuk dua batasan yang disebut jujur.

**Dua batasan yang disebut, bukan disembunyikan:**

- Angkanya menghitung **percobaan**, bukan baris yang dikembalikan. Dicatat
  sebelum permintaannya dilayani, supaya proses yang gagal di tengah tidak
  meninggalkan data terbaca tanpa catatannya.
- Pembacaan oleh staf travel atas datanya sendiri **tidak** dicatat. Itu bukan
  peristiwa privasi, dan mencatatnya akan mengubur yang memang penting.

**Diverifikasi dengan merusak:** `PilgrimService` dihapus dari daftar layar data
pribadi → uji gagal dengan `pembacaan data pribadi tidak tercatat: no rows in
result set`. Uji juga memastikan `ListSeasons` **tidak** tercatat: musim bukan
orang, dan mencatat semuanya sama saja dengan tidak mencatat apa pun.

## C4 — Kunci penandatangan platform & ekspor auditor ✅

> **Diperjelas 5 September 2026 (Claude, `[K]`).** Judul lama "Rotasi kunci
> API" menyiratkan ada sistem kunci API publik (operator memanggil sesuatu
> dengan kunci mereka sendiri) — **itu tidak ada di produk ini sama sekali**.
> Dua butir itu sebenarnya **satu fitur**: kunci milik platform sendiri,
> dipakai untuk menandatangani ekspor auditor. Butir ini juga sebelumnya
> dobel tercatat sebagai **E5** di TAHAP E — sudah digabung ke sini. E5
> sekarang hanya menunjuk balik ke sini.
>
> **Diimplementasikan 5 September 2026 (Claude, `[K, diimplementasikan]`,
> `e88e524`)** — pemilik meminta implementasi Panel SaaS dilanjutkan juga,
> bukan hanya spesifikasinya. Dua hal berubah dari spesifikasi di atas saat
> benar-benar dibangun:
>
> - **Tidak ada `AUDIT_EXPORT_SIGNING_KEY_PREVIOUS`.** Tidak ada kode di
>   aplikasi ini yang pernah memverifikasi ulang ekspornya sendiri, jadi
>   tidak ada yang benar-benar butuh kunci lama saat runtime — beda dengan
>   `KYC_ENCRYPTION_KEY` yang harus bisa membuka data lama yang tersimpan.
>   Manifes tiap ekspor sudah mencatat `key_fingerprint`-nya sendiri; rotasi
>   cukup deploy nilai baru.
> - **CSV dikirim sebagai bita mentah, bukan pesan baris bertipe** — beda
>   dari pola `StreamProfitLossExport` yang dirujuk di atas. Alasannya: hash
>   di manifes harus sama persis dengan berkas yang diunduh. Kalau baris
>   dikirim bertipe dan klien merakit ulang CSV-nya sendiri, hash di server
>   dan berkas di komputer auditor adalah dua hasil enkode terpisah yang
>   harus kebetulan sama persis — rapuh. Dengan bita mentah, klien tidak
>   pernah menyandikan ulang apa pun, jadi tidak mungkin beda.

- [x] Kunci penandatangan: `AUDIT_EXPORT_SIGNING_KEY`, HMAC bukan enkripsi
      (`internal/crypto.Signer`, bukan `Sealer`). Kosong = ditolak dengan
      jelas (`FailedPrecondition`), bukan mengekspor tanpa tanda tangan.
- [x] `PlatformService.ExportAuditTrail`, streaming, saringan sama persis
      dengan layar Audit (`ListAuditTrail`) minus batas baris — sebuah
      ekspor yang terpotong demi hemat memori adalah jawaban salah untuk
      "semua yang cocok".
- [x] Manifes: `sha256`, `signed_at`, `key_fingerprint`, `hmac_sha256`.
      Tombol **Ekspor auditor** di layar Audit mengunduh dua berkas: CSV
      dan `manifest.json`.
- [x] Setiap ekspor masuk `audit_logs` miliknya sendiri (siapa, kapan,
      berapa baris) — pola yang sama dengan C3, dicatat setelah stream
      selesai supaya klien yang putus di tengah tidak tercatat berhasil.
- [x] Diuji dengan merusak: hash yang dihitung dari input kosong (bukan
      stream sungguhan) membuat pengujian gagal dengan pesan yang benar,
      dipulihkan. Dibuktikan juga di luar aplikasi — manifes yang diunduh
      dicocokkan ke CSV yang diunduh pakai `openssl` biasa, bukan kode dari
      proyek ini, karena itulah yang sebenarnya dilakukan seorang auditor.

---

## C5 — Kotak masuk Support (sisi platform) ✅

Sisi operator sudah selesai — lihat T4.11 di TUGAS-DASHBOARD-TRAVEL.md:
`/dashboard/support`, tabel `support_tickets` + `support_ticket_messages`
(migrasi 162), `SupportService` untuk operator membuat tiket, membalas, dan
menutup tiketnya sendiri.

> **Diimplementasikan 5 September 2026 (Claude, `[K, diimplementasikan]`,
> `034f5e5`)** — bagian kedua yang diambil alih implementasinya dari Codex,
> setelah C4. Satu celah nyata ditemukan **saat membangun**, bukan saat
> menulis spesifikasi ini: butir "jangan bisa CLOSED" di bawah cuma dicegah
> lewat `buf.validate` di batas API — penjaga `WHERE` di query SQL-nya
> sendiri hanya menolak tiket yang **sudah** `CLOSED`, tidak menolak
> **mengatur** ke `CLOSED`. Kalau pemanggil melewati lapisan proto (uji
> langsung ke repository, bukan lewat RPC), tiket tetap bisa didorong ke
> `CLOSED`. Ditambahkan `AND $2 != 'CLOSED'` di `WHERE`-nya — dibuktikan
> dengan memanggil repository langsung sambil sengaja melewati validasi
> proto, dan merusak perbaikannya untuk memastikan ada uji yang benar-benar
> menangkap regresinya.

- [x] Layar di `/admin`: daftar tiket **lintas semua tenant**, saring per
      status/prioritas, urut berdasarkan yang lewat target respons dulu —
      pakai `domain.SupportTicket.ResponseDueAt()`/`.ResponseOverdue()` yang
      sudah ada, tidak dihitung ulang.
- [x] Balas sebagai staf platform — `author_is_platform = true`, thread
      operator menampilkannya tanpa perubahan apa pun di sisi operator.
- [x] Ubah status **OPEN → IN_PROGRESS → RESOLVED saja**. `CLOSED` tetap
      eksklusif milik `SupportService.CloseSupportTicket` — dijaga dua
      lapis: `buf.validate` di proto **dan** `WHERE` di SQL (lihat catatan
      di atas soal kenapa keduanya perlu).
- [x] RPC baru di `PlatformService` (bukan `SupportService`), `SupportTicket`
      memakai field yang sama dengan sisi operator (`operator_id`/
      `operator_name` ditambahkan, kosong di sisi operator) — bukan tipe
      pesan kedua yang terpisah.
- [x] Diuji lintas tenant sungguhan: admin yang hanya anggota organisasinya
      sendiri berhasil melihat, membalas, dan mengubah status tiket milik
      travel lain sepenuhnya, lewat HTTP asli. Diverifikasi juga langsung di
      `/admin` — balasan staf platform muncul seketika di thread yang sama.

---

# TAHAP D — Siklus hidup tenant

Lihat §7 DESAIN. Bagian ini sebelumnya tidak dirancang di mana pun.

- [x] **D1** `TrialDays` → **10 hari**, dibaca dari `platform_settings` lewat
      `platform_trial_days()` saat langganan dibuat. Mengubah setelan tidak bisa
      memendekkan trial yang sedang berjalan, karena panjangnya hanya dibaca
      sekali di awal. Nilai rusak jatuh ke 10, bukan menghentikan pendaftaran
      (`80fa6fd`)
- [x] **D2** Perpanjang trial per tenant (`[K, diimplementasikan]`,
      5 September 2026, `1f7f3a0`). Bukan tindakan four-eyes — beda dengan
      SUSPEND/DELETE_TENANT, tidak ada yang sulit ditarik balik di sini —
      jadi mengikuti bentuk `SetGracePeriod` yang sudah ada (kunci advisory
      + anti-duplikat + audit_logs biasa), bukan ledger `privileged_actions`.
      Hanya menambah hari ke `access_until`, dan **hanya** kalau langganan
      masih `TRIALING` — memperpanjang akses tenant yang sudah bayar adalah
      tindakan lain (kredit/diskon), bukan ini. Konfirmasi nama travel
      diketik tangan, pola yang sama dengan Suspend/SetGracePeriod.
- [x] **D3** Layar Langganan menampilkan trial yang berakhir pekan ini
      (`[K, diimplementasikan]`, 5 September 2026, `b9b2b5d`). Filter murni di
      frontend atas data `ListOperators`/`ListSubscriptions` yang sudah ada —
      tidak ada RPC baru.
- [x] **D4** Antrean tenant baru 7 hari terakhir + penanda kelengkapan
      (sudah ada musim? jamaah? login kedua?) (`[K, diimplementasikan]`,
      5 September 2026, `b9b2b5d`). "Login kedua" ternyata tidak bisa diukur
      dari riwayat sesi — `session.create.after` menghapus sesi lama setiap
      kali seseorang masuk, jadi tabel `session` tidak pernah menyimpan lebih
      dari satu baris per akun. Didekati: sesi yang aktif sekarang
      dibandingkan dengan `operators.created_at` — kalau sesi itu lebih baru
      dari dua jam setelah pendaftaran, seseorang pasti masuk lagi. Disebut
      terang-terangan sebagai perkiraan, di kode maupun di layar.
      Ditemukan sekaligus diperbaiki saat mengerjakan ini: `CancelledAt` dari
      D5 tidak pernah disalin ke pesan proto `ListOperators`, jadi filter
      "belum dibatalkan" di layar sejak D5 selalu membandingkan dengan nilai
      yang tidak mungkin terisi.
- [x] **D5** Pembatalan (`[K, diimplementasikan]`, 5 September 2026,
      `1f7f3a0`): `cancelled_at` diisi, `access_until` **sama sekali tidak
      disentuh** — sisa periode yang sudah dibayar tetap haknya. Percobaan
      membatalkan langganan yang sudah dibatalkan **ditolak**, bukan
      diperlakukan sebagai tidak melakukan apa-apa — admin kedua yang
      mencoba lagi harus tahu tidak ada yang berubah, bukan mengira
      berhasil. Sama seperti D2, bukan four-eyes: `access_until` yang sudah
      mengatur akses sejak awal (dipakai di mana-mana, dari pengecekan
      entitlement sampai proses dunning) tidak berubah sama sekali oleh
      aksi ini — yang berubah hanya penagihan berhenti mencoba lagi.
- [x] **D6** Penghapusan setelah 90 hari: **tawarkan ekspor data lebih dulu**
      (hak portabilitas UU PDP), four-eyes, dan **`audit_logs` tidak ikut
      dihapus** — ia bukti bahwa penghapusan itu sah (`[K, diimplementasikan]`,
      5 September 2026, `0a0a6f6` + `faf088b`).

      Tiga syarat, semua diperiksa ulang dalam satu transaksi berkunci
      advisory yang sama dengan Suspend/Reinstate: (1) akses sudah berakhir
      **90 hari** — dihitung dari `subscription_effective_access_until`
      (akses + masa tenggang), bukan `access_until` mentah, supaya trial yang
      habis tanpa pernah dibatalkan tetap terhitung; (2) ekspor data **READY**
      sudah pernah dibuat — dibuktikan ada, bukan diklaim sudah diunduh,
      karena membuktikan itu tidak perlu dan nyaris tidak bisa ditegakkan; (3)
      nama travel diketik ulang persis, pola yang sama dengan Suspend.

      Penghapusannya sendiri `DELETE FROM operators` biasa: hampir seluruh
      tabel di skema ini sudah `ON DELETE CASCADE` dari `operators(id)` —
      itulah yang membuat satu DELETE ini penghapusan yang lengkap tanpa
      menyisakan apa pun, dibanding memilih satu-satu tabel mana yang
      "data pribadi" (rawan lupa satu tabel). Diperiksa langsung dua
      pengecualian yang bukan CASCADE (`seat_assignments` — aman lewat rantai
      cascade tabel lain; `product_routes` — `SET NULL`, tidak menghalangi)
      dan satu yang sengaja tidak ber-FK ke `operators` sama sekali
      (`privileged_actions` — menyimpan nama & id tenant sebagai teks JSON,
      jadi selamat dengan sendirinya, pola yang sama dipakai Suspend/Reinstate).

      `audit_logs` adalah pengecualian yang disengaja: migrasi 165 mengubah
      FK-nya dari CASCADE ke SET NULL, supaya baris tentang tenant yang sudah
      dihapus tetap ada — dibaca sebagai "peristiwa ini soal tenant yang
      sudah tidak ada", pola NULL yang sama yang migrasi 108 pakai untuk
      "tidak pernah punya tenant sama sekali".

      Diuji dengan mematahkan tiap syarat satu-satu (masing-masing dibuktikan
      cukup sendirian untuk menolak, meski dua syarat lain sudah terpenuhi),
      nama salah tetap ditolak, penghapusan sungguhan menghapus barisnya,
      `audit_logs` bertahan dengan `operator_id` NULL, dan pengulangan dengan
      kunci yang sama menyelesaikan tindakan yang sama tanpa mengeksekusi
      ulang.
- [x] **D7** Hitung mundur penghapusan tampil sebagai tanggal, dan masuk Pusat
      Tindakan saat mendekat (`[K, diimplementasikan]`, 5 September 2026,
      `0a0a6f6` + `faf088b`). `deletion_eligible_at` masuk `ListOperators` dan
      `GetTenantDetail` — tanggal yang sama di kedua tempat, dihitung dari
      fungsi SQL yang sama. Blok Pusat Tindakan di `OperatorsTab` menampilkan
      tenant yang sudah atau akan (dalam 14 hari) bisa dihapus, di atas
      antrean tenant baru D4.

      Diverifikasi langsung di browser sungguhan dengan tenant fixture
      sekali-pakai: tombol hapus terkunci sampai tanggal itu lewat, tombol
      "minta ekspor" memicu `RequestTenantDataExport`, pesan penolakan
      menyebut alasan sebenarnya (belum 90 hari / belum ada ekspor / nama
      salah), dan penghapusan sungguhan berhasil sampai baris tenantnya
      hilang.

      Verifikasi langsung ini menemukan dua bacaan nyata, keduanya diperbaiki
      di commit yang sama: (1) `GetTenantDetail` (repositori terpisah dari
      `ListOperators`) ternyata tidak pernah menghitung `deletion_eligible_at`
      sama sekali — halaman detail tenant, tempat tombol hapus itu sendiri
      berada, selalu menampilkan "belum pernah punya langganan" apa pun
      keadaan tenantnya; (2) tidak berkaitan dengan D6/D7, tersingkap justru
      karena migrasi 165 membuat baris `audit_logs` lama di database
      pengujian bertahan alih-alih ikut CASCADE terhapus:
      `IssueBillingPeriod` menulis jejak audit tagihan dengan
      `idempotency_key` kosong memakai aktor tetap `"system"` — karena
      keunikan `audit_logs` adalah `(user_id, action, idempotency_key)` tanpa
      `operator_id` sama sekali, tagihan pertama yang pernah diterbitkan
      worker penagihan berulang untuk **tenant mana pun** akan menghalangi
      tagihan pertama tenant lain berikutnya selamanya. Diperbaiki dengan
      memakai id invoice yang baru dibuat sebagai kuncinya.

---

# TAHAP E — Pertumbuhan & komunikasi

- [x] **E1** Tab **Analitik** di `/admin`: MRR dan pergerakannya (baru, naik
      paket, turun paket, berhenti), tenant per keadaan, konversi trial, NRR,
      dan rincian per paket — dengan **Catatan Metodologi** yang menyebut lima
      batasannya.

      **Definisi "membayar" dipakai persis sama dengan pemeriksaan akses.**
      Bukan trial, tidak dibatalkan, tidak ditangguhkan, dan masa bayarnya
      belum habis. MRR yang menghitung travel yang tidak bisa masuk adalah
      angka yang menyanjung kita tepat ketika seharusnya tidak.

      Diverifikasi dengan merusak: definisi dilonggarkan jadi "ACTIVE atau
      TRIALING dan tidak dibatalkan" → MRR fixture melonjak dari **Rp789.000
      menjadi Rp8.256.000** dari baris yang sama persis. Sepuluh kali lipat,
      tanpa satu pelanggan baru pun.

      Ekspansi dan kontraksi dibaca dari `subscription_adjustments`, bukan dari
      membandingkan paket hari ini dengan yang diingat — hanya ledger itu yang
      tahu satu tenant pindah paket dua kali.

      **Yang disebut jujur di layar:** komisi marketplace bukan MRR; MRR awal
      periode direkonstruksi dari pergerakan karena potret bulanan belum
      disimpan, sehingga travel yang naik paket lalu berhenti di periode yang
      sama muncul di churn bukan di ekspansi; konversi trial diukur pada
      rombongan yang mulai di periode itu sehingga angkanya baru mengendap
      belakangan.
- [x] **E2** Pengumuman ke tenant (§10.1 DESAIN): wizard 4 langkah, penerima
      **dihitung dari data**, Skor Kesiapan termasuk pemeriksaan "sudah ada
      pengumuman lain ke penerima sama dalam 24 jam", riwayat baca.
      **Tidak bisa diedit setelah terkirim** — kalau salah, kirim ralat
      (`[K, diimplementasikan]`, 5 September 2026, `434a08d`).

      Dibangun di atas `components/ui/Wizard.tsx` yang sudah ada tapi belum
      pernah dipakai satu layar pun — inilah pemakai pertamanya. Enam mode
      penerima (semua, per paket, trial, multi-cabang, menunggak, pilih
      manual), semuanya query langsung terhadap data hidup, dievaluasi ulang
      persis pada saat kirim (bukan saat preview) — pengumuman terjadwal
      menghitung ulang siapa yang cocok ketika benar-benar terkirim, bukan
      siapa yang cocok saat ditulis.

      Kanal email memakai `internal/mailer` dan `cascade_events` yang sudah
      ada (§11 DESAIN) — satu event per penerima, bukan satu event untuk
      seluruh pengiriman, supaya satu alamat email yang gagal tidak menahan
      yang lain. Dalam-aplikasi tidak perlu jalur pengiriman sama sekali:
      begitu baris `announcement_deliveries` ditulis, ia langsung terlihat.
      Dashboard mendapat lonceng notifikasi pertamanya
      (`components/announcements/AnnouncementBell.tsx`).

      Tidak bisa diedit setelah terkirim ditegakkan dua lapis: tidak ada RPC
      yang menawarkan itu, dan `title`/`body`/`recipient_filter` dicabut hak
      UPDATE-nya dari peran aplikasi di database (migrasi 166), pola
      pertahanan yang sama dengan migrasi 125 untuk `audit_logs`.

      Diuji lewat HTTP sungguhan: dua tenant masing-masing hanya melihat
      salinannya sendiri, satu tenant membaca tidak ikut menandai tenant
      lain, tenant tidak bisa menandai terbaca pengumuman yang tidak pernah
      dikirim ke mereka (not_found, bukan berhasil diam-diam), riwayat
      platform menghitung pembaca dengan benar, pengulangan kunci yang sama
      tidak menggandakan baris pengiriman, dan sapuan jadwal worker
      membiarkan pengumuman yang belum waktunya lalu mengirimnya tepat
      sekali setelah lewat jadwalnya — masing-masing dibuktikan dengan
      merusak kode penjaganya dan memastikan ujinya gagal sebelum diperbaiki
      kembali.

      Diverifikasi langsung di browser: alur wizard penuh empat langkah,
      dan lonceng dashboard. Verifikasi ini menemukan bug nyata: tombol
      lonceng memakai kelas CSS yang sama dengan tombol menu hamburger, yang
      sengaja disembunyikan (`display:none`) di lebar desktop — lonceng ikut
      lenyap. Diperbaiki dengan kelas `.dashboard-bell-button` sendiri.
- [x] **E3** Tab **Kesehatan** di `/admin`. Tujuh sinyal: event outbox yang
      sudah menyerah, antrean event tertinggal, poller mutasi bank, kegagalan
      supplier 24 jam, tagihan langganan macet, transaksi tertahan, dan backup.
      Setiap butir menyebut **berapa travel terdampak**, sejak kapan, dan
      **dari tabel mana angkanya** — supaya yang membacanya jam 2 pagi tidak
      perlu menebak.

      **Statusnya empat, bukan dua.** `OK` · `WARN` · `ALERT` ·
      **`UNMONITORED`**. Layar dengan dua nilai terpaksa menyebut sesuatu
      "aman" padahal tidak ada yang memeriksanya. Backup persis begitu: tidak
      ada apa pun di database ini yang tahu cron R2 sudah jalan atau belum,
      jadi ia ditandai *tidak dipantau*, bukan hijau. **Lampu hijau yang tidak
      memeriksa apa pun lebih buruk daripada tidak ada lampu sama sekali.**

      Poller bank membedakan "berhenti" dari "belum pernah dinyalakan" —
      memberi tahu orang bahwa sesuatu berhenti padahal mereka belum pernah
      menyalakannya mengirim mereka mencari kerusakan yang tidak ada.

      Diverifikasi dengan merusak: backup dijadikan hijau → gagal `backup
      dilaporkan "OK" padahal tidak ada yang memeriksanya`; hitungan tenant
      terdampak dinolkan → gagal `tidak menyebut berapa tenant terdampak`.

      Dead-letter outbox akhirnya terlihat. Komentar di `worker/outbox.go`
      menulis event yang menyerah "tinggal untuk diperiksa ops" — selama tidak
      ada layar yang menampilkannya, itu berarti tidak akan diperiksa.
- [x] **E4** Tab **Audit** di `/admin`: jejak lintas seluruh travel dengan
      saringan per **tenant**, per **aktor** (dicari lewat email, bukan hanya id
      Better Auth — insiden bermula dari orang, dan tidak ada yang hafal id),
      dan per kategori bernama: **tindakan istimewa · sesi lihat-saja ·
      pembacaan data pribadi**.

      Kategori bernama, bukan kotak pencarian. Saat insiden tidak ada yang
      menggulir, dan tidak ada yang ingat ejaan persis sebuah tindakan di bawah
      tekanan.

      **Read-only tanpa pengecualian**, dan alasannya ditulis di layar: migrasi
      125 sudah mencabut UPDATE dan DELETE dari peran aplikasi, jadi tombol
      hapus di sini akan menjadi tombol yang pasti gagal.

      Daftar yang terpotong **mengatakan dirinya terpotong** — ekor yang kosong
      tidak boleh terbaca sebagai "tidak ada kejadian lain".

      Diverifikasi dengan merusak dua saringan: kategori diabaikan → gagal
      `saringan tindakan istimewa meloloskan "crm_lead_created"`; saringan
      tenant diabaikan → gagal `saringan tenant meloloskan travel lain`.
      Keduanya adalah bentuk kegagalan yang paling berbahaya untuk layar
      seperti ini: hasilnya terlihat masuk akal dan menjawab pertanyaan yang
      berbeda dari yang ditanyakan.

      **Membuka layar ini tidak dicatat**, dengan sengaja: layar audit yang
      menulis entri audit setiap kali dibuka akan memenuhi jejaknya dengan
      tindakan membacanya sendiri dan mengubur apa yang benar-benar terjadi.
- **E5 dipindahkan ke C4** (5 September 2026) — ekspor auditor adalah soal
  keamanan/kepatuhan, bukan pertumbuhan. Lihat **C4** untuk spesifikasi
  lengkapnya (kunci penandatangan, rotasi, format manifes).

---

# TAHAP F — Lintas semua tugas

Berlaku di sepanjang pengerjaan, bukan di akhir.

- [x] **F1** Dunning & pengumuman memakai outbox `cascade_events` yang ada,
      **jangan buat jalur pengiriman baru** (§11 DESAIN). Efeknya harus
      idempoten karena pengirimannya at-least-once — itulah gunanya PK
      `(invoice_id, stage)` (`[K, diperiksa]`, 5 September 2026). Sudah benar
      untuk dunning sejak awal; E2 mengikuti pola yang sama persis — satu
      `cascade_events` per penerima email, idempoten lewat
      `EnqueueIdempotentTx`, bukan jalur pengiriman baru.
- [x] **F2** `service/errors.go`: galat tak terpetakan **juga** ditulis ke
      `slog` level error dengan nama metodenya. Sekarang hanya ke Sentry, dan
      `sentry.Init` no-op saat `SENTRY_DSN` kosong — di pengembangan galat itu
      hilang tanpa jejak (§12 DESAIN) (`[K, diimplementasikan]`,
      5 September 2026, `bac7584`). Diuji dengan menangkap output `slog` ke
      buffer dan merusak baris logging-nya untuk membuktikan ujinya
      benar-benar memeriksa isi pesan, bukan cuma bahwa sesuatu terpanggil.
- [ ] **F3** `scripts/uji-batas-platform.sh` — menguji constraint §9 langsung
      terhadap skema, dan **dibuktikan bisa gagal** dengan mematikan salah satu
      constraint, seperti pada skrip cabang. **Belum dikerjakan** — perlu
      audit tersendiri atas semua constraint §9, bukan bagian dari tugas yang
      sedang berjalan.
- [x] **F4** Setiap RPC platform baru diuji **dua arah**: tanpa sesi →
      `unauthenticated`, sesi owner operator asli → `permission_denied`, admin
      platform → berhasil, dicabut → ditolak pada panggilan **berikutnya**
      (`[K, diperiksa untuk D6/D7/E2]`, 5 September 2026). Sudah dipenuhi
      untuk setiap RPC baru yang ditambahkan sepanjang TAHAP D dan E sesi ini
      (`platform_deletion_http_test.go`, `announcement_access_test.go`).
      **Belum diperiksa mundur** untuk RPC `PlatformService` yang sudah ada
      sebelum sesi ini — itu audit terpisah atas puluhan RPC lama, bukan
      bagian dari tugas yang sedang berjalan.
- [ ] **F5** Uji jejak: panggil RPC, periksa `audit_logs` bertambah — dan
      **gagal kalau tidak**. Dipenuhi untuk setiap RPC baru sesi ini
      (mis. `TestDeleteTenantRequiresGraceExportAndNameIntegration`), tapi
      **belum diperiksa mundur** ke RPC lama — audit terpisah, sama seperti F3.
- [ ] **F6** Uji idempotensi dengan **menjalankan dua kali**, bukan membaca kode.
      Dipenuhi untuk setiap RPC baru sesi ini, **belum diperiksa mundur** ke
      RPC lama — audit terpisah, sama seperti F3.

# TAHAP G — Rilis bertahap

Panel ini menyentuh uang dan data seluruh tenant. Urutannya (§14 DESAIN):

- [C] **G1** Rilis yang hanya membaca lebih dulu: Pemakaian, Kesehatan, Audit
- [C] **G2** Lalu tulis yang bisa ditarik: override kuota, routing, pengumuman
- [C] **G3** Lalu tulis yang tidak bisa ditarik, **setelah four-eyes dan audit
      berjalan**: penangguhan, `plan_limits` global, penghapusan tenant
- [C] **G4** Impersonate paling akhir, setelah audit terbukti mencatat semuanya
- [C] **G5** Dunning dijalankan **mode kering dulu**: rangkaian jalan,
      `dunning_log` terisi, tidak ada pesan keluar. Bandingkan daftar yang
      *akan* dihubungi dengan daftar manual sebelum satu email pun dikirim

---

## Yang sengaja TIDAK dikerjakan

- Konsol anggaran AI multi-provider, layar deploy, layar server, restart node,
  mode maintenance sebagai tombol UI, konfigurasi Turnstile — itu PaaS di dalam
  SaaS
- Market / seller center platform — produk kedua, sudah ditunda
- Skor churn sebagai angka menonjol tanpa peringatannya
- Emoji di teks sistem

## Pekerjaan pemilik

- [ ] **Repo masih PUBLIC**
- [ ] `BANK_FEED_SECRET` belum diset — poller bank tidak bisa jalan
- [ ] Cron backup R2 belum dipasang
