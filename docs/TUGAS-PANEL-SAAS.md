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
- [C] Tautan dari transaksi menggantung → log supplier terkait

---

# TAHAP B — Mesin komersial

## B1 — Langganan & dunning 🔴

- [C] Siklus tagihan massal: tinjau dulu daftar invoice + nominalnya, terbitkan
      sekaligus
- [C] Dunning H+1, H+7, H+14 → penangguhan otomatis H+21
- [C] **Kunci idempotensi di database**: `(operator_id, period_start)` unik
      pada invoice, `(invoice_id, stage)` unik pada jejak dunning
- [C] Siklus massal tunduk pada indeks nominal unik transfer — kegagalan
      sufiks dilaporkan **per baris**, tidak membatalkan seluruh siklus
- [C] Grace period yang bisa diatur, per tenant bila perlu
- [C] Void invoice + pulihkan, dengan jejak (jangan DELETE)
- [C] Prorata saat upgrade/downgrade di tengah periode
- [C] Penangguhan lewat **waktu, bukan status**: berhenti memperpanjang
      `access_until` + kolom `suspended_at`. Interceptor tidak berubah
- [C] Pembayaran kapan pun **membatalkan rangkaian dan memulihkan akses**,
      termasuk sesudah H+21, tanpa campur tangan manual
- [C] Tab **Langganan** di `/admin`

## B2 — Meter pemakaian 🔴

- [C] Tabel `usage_counters` (operator_id, metric, period_start, value)
- [C] Worker harian yang mengisinya — **jangan** hitung ulang per permintaan;
      menghitung jamaah lintas tenant tiap panel dibuka akan jadi query
      termahal di sistem ini
- [C] Metrik: jamaah, cabang, penyimpanan, panggilan API, pesan WhatsApp
- [C] Tab **Pemakaian**: pemakaian vs batas, peringatan 80% dan 100%
- [C] Subjudul menyebut **tanggal reset** — kuota tanpa tanggal reset tidak bisa
      ditindak

## B3 — Detail tenant `/admin/tenant/[id]` 🟠

- [C] Langganan & riwayat tagihan · pemakaian vs kuota · override berlaku
- [C] Jamaah & cabang · transaksi & transfer · KYC
- [C] Tim & status 2FA · domain · jejak audit tenant itu
- [C] Tombol tindakan: ubah override, tangguhkan, impersonate (lihat C1)

---

# TAHAP C — Keamanan sebelum tim bertambah

## C1 — Impersonate dengan jejak penuh 🔴

- [C] Sesi impersonasi ditandai berbeda di seluruh sistem
- [C] **Read-only, tanpa mode tulis sama sekali.** Perubahan untuk pelanggan
      lewat RPC platform yang punya jejaknya sendiri — jangan menyamar
- [C] Berbatas waktu, otomatis berakhir
- [C] Dicatat lengkap: siapa, tenant mana, IP, alasan, durasi
- [C] Uji: sesi impersonasi tidak bisa menulis sebelum ditingkatkan

## C2 — Four-eyes untuk tindakan tak bisa ditarik 🔴

Berlaku untuk: menangguhkan tenant, menghapus tenant, mengubah `plan_limits`
global, mengubah rekening settlement.

- [C] Selama admin platform hanya satu: **konfirmasi ulang dengan mengetik nama
      tenant**
- [C] Tabel `privileged_actions` sejak hari pertama, dengan
      `approved_by = requested_by` selama admin masih satu
- [C] Setiap tindakan ini masuk `audit_logs` dengan alasannya

## C3 — Audit pembacaan data pribadi 🟠

- [C] Membaca KYC / paspor / data jamaah dari panel platform tercatat, bukan
      hanya perubahannya
- [C] Perbarui tabel inventaris data di
      [INSIDEN-DATA-PRIBADI.md](INSIDEN-DATA-PRIBADI.md)

## C4 — Rotasi kunci & ekspor auditor 🟡

- [C] Rotasi kunci API dengan tumpang tindih 24 jam (pola yang sama dengan kunci
      KYC)
- [C] Ekspor auditor: CSV + hash manifes, ditandatangani kunci platform

---

# TAHAP D — Siklus hidup tenant

Lihat §7 DESAIN. Bagian ini sebelumnya tidak dirancang di mana pun.

- [C] **D1** `TrialDays` → **10 hari**, dan jadi setelan yang bisa diubah dari
      panel, bukan konstanta di `repository/subscription.go:20` (sekarang 3).
      **Keputusan pemilik 2 September 2026.** Mengubah setelan **tidak boleh**
      memendekkan trial yang sedang berjalan — tenant memakai angka yang berlaku
      saat ia mulai.
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

- [C] **E1** Analitik: MRR & pergerakannya, tenant aktif, konversi trial, churn,
      NRR — dengan **Catatan Metodologi**. Komisi market **bukan** MRR; tulis
      itu di layar. Skor churn ditandai sebagai heuristik, bukan vonis.
- [C] **E2** Pengumuman ke tenant (§10.1 DESAIN): wizard 4 langkah, penerima
      **dihitung dari data**, Skor Kesiapan termasuk pemeriksaan "sudah ada
      pengumuman lain ke penerima sama dalam 24 jam", riwayat baca.
      **Tidak bisa diedit setelah terkirim** — kalau salah, kirim ralat
- [C] **E3** Kesehatan platform (§10.2 DESAIN): antrean tertinggal,
      **event outbox dead-letter**, webhook gagal, poller bank berhenti,
      supplier gagal beruntun, backup terakhir, invoice macet. Setiap butir
      menyebut **berapa tenant terdampak**, dan yang sehat tetap ditampilkan
      hijau — layar yang hanya menunjukkan masalah tak bisa dibedakan dari layar
      yang rusak
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
