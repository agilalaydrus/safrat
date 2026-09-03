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

## B2 — Meter pemakaian 🔴

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
- [ ] **Menghapus tenant** — sengaja belum. Itu D6, dan syaratnya lebih berat:
      tawarkan ekspor data lebih dulu (hak portabilitas UU PDP), dan
      `audit_logs` tidak ikut terhapus.
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

## C4 — Rotasi kunci & ekspor auditor 🟡

- [C] Rotasi kunci API dengan tumpang tindih 24 jam (pola yang sama dengan kunci
      KYC)
- [C] Ekspor auditor: CSV + hash manifes, ditandatangani kunci platform

---

# TAHAP D — Siklus hidup tenant

Lihat §7 DESAIN. Bagian ini sebelumnya tidak dirancang di mana pun.

- [x] **D1** `TrialDays` → **10 hari**, dibaca dari `platform_settings` lewat
      `platform_trial_days()` saat langganan dibuat. Mengubah setelan tidak bisa
      memendekkan trial yang sedang berjalan, karena panjangnya hanya dibaca
      sekali di awal. Nilai rusak jatuh ke 10, bukan menghentikan pendaftaran
      (`80fa6fd`)
- [C] **D2** Perpanjang trial per tenant, alasan wajib
- [C] **D3** Layar Langganan menampilkan trial yang berakhir pekan ini
- [C] **D4** Antrean tenant baru 7 hari terakhir + penanda kelengkapan
      (sudah ada musim? jamaah? login kedua?)
- [C] **D5** Pembatalan: `cancelled_at`, akses tetap sampai `access_until`
      berjalan habis — sisa periode yang sudah dibayar adalah haknya
- [C] **D6** Penghapusan setelah 90 hari: **tawarkan ekspor data lebih dulu**
      (hak portabilitas UU PDP), four-eyes, dan **`audit_logs` tidak ikut
      dihapus** — ia bukti bahwa penghapusan itu sah
- [C] **D7** Hitung mundur penghapusan tampil sebagai tanggal, dan masuk Pusat
      Tindakan saat mendekat

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
- [C] **E2** Pengumuman ke tenant (§10.1 DESAIN): wizard 4 langkah, penerima
      **dihitung dari data**, Skor Kesiapan termasuk pemeriksaan "sudah ada
      pengumuman lain ke penerima sama dalam 24 jam", riwayat baca.
      **Tidak bisa diedit setelah terkirim** — kalau salah, kirim ralat
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
- [C] **E4** Audit global (§10.3 DESAIN): saringan per tenant / aktor / tindakan
      istimewa / impersonasi / pembacaan data pribadi. **Read-only tanpa
      pengecualian** — jangan tawarkan tombol yang pasti gagal
- [C] **E5** Ekspor auditor: CSV + hash manifes, ditandatangani kunci platform,
      streaming sejak awal

---

# TAHAP F — Lintas semua tugas

Berlaku di sepanjang pengerjaan, bukan di akhir.

- [C] **F1** Dunning & pengumuman memakai outbox `cascade_events` yang ada,
      **jangan buat jalur pengiriman baru** (§11 DESAIN). Efeknya harus
      idempoten karena pengirimannya at-least-once — itulah gunanya PK
      `(invoice_id, stage)`
- [C] **F2** `service/errors.go`: galat tak terpetakan **juga** ditulis ke
      `slog` level error dengan nama metodenya. Sekarang hanya ke Sentry, dan
      `sentry.Init` no-op saat `SENTRY_DSN` kosong — di pengembangan galat itu
      hilang tanpa jejak (§12 DESAIN). Pesan ke klien tetap `internal error`
- [C] **F3** `scripts/uji-batas-platform.sh` — menguji constraint §9 langsung
      terhadap skema, dan **dibuktikan bisa gagal** dengan mematikan salah satu
      constraint, seperti pada skrip cabang
- [C] **F4** Setiap RPC platform baru diuji **dua arah**: tanpa sesi →
      `unauthenticated`, sesi owner operator asli → `permission_denied`, admin
      platform → berhasil, dicabut → ditolak pada panggilan **berikutnya**
- [C] **F5** Uji jejak: panggil RPC, periksa `audit_logs` bertambah — dan
      **gagal kalau tidak**
- [C] **F6** Uji idempotensi dengan **menjalankan dua kali**, bukan membaca kode

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
