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

T2.2 sudah menegakkan batas lewat trigger, tapi `plan_limits` dan
`plan_overrides` **tidak punya satu pun RPC**. Menaikkan kuota satu pelanggan
hari ini = menulis SQL di produksi.

- [ ] Proto: `ListPlanLimits`, `SetPlanLimit`, `ListPlanOverrides`,
      `SetPlanOverride`, `DeletePlanOverride`, `PreviewPlanLimitChange`
- [ ] Override wajib punya **alasan**; tambah kolom `expires_at` di
      `plan_overrides` + worker harian yang mencabut yang kedaluwarsa
- [ ] `PreviewPlanLimitChange` mengembalikan tenant yang akan seketika melampaui
      batas baru, **beserta namanya** — bukan hanya jumlahnya
- [ ] Grandfathering: tenant yang sudah lewat batas dikunci di angka lamanya,
      tidak ditendang
- [ ] Perubahan batas ditulis ke `audit_logs` (keputusan komersial, bukan
      konfigurasi)
- [ ] Tab **Paket & Kuota** di `/admin`
- [ ] Uji dua arah: override menaikkan batas satu tenant **dan** tidak bocor ke
      tenant lain

## A2 — Routing produk & log supplier 🟠

Menutup mesin tanpa pemicu. RPC-nya sudah ada, teruji, tidak dipanggil siapa pun.

- [ ] Layar routing memakai `ListProductRoutes` + `SaveProductRoute`
- [ ] Produk **tanpa routing** ditampilkan sebagai antrean kerja, bukan
      disembunyikan — inilah yang memicu respons "Produk Belum di Atur Routing"
- [ ] Log supplier memakai `ListSupplierLogs`: permintaan, respons, latensi,
      aturan yang cocok
- [ ] Tautan dari transaksi menggantung → log supplier terkait

---

# TAHAP B — Mesin komersial

## B1 — Langganan & dunning 🔴

- [ ] Siklus tagihan massal: tinjau dulu daftar invoice + nominalnya, terbitkan
      sekaligus
- [ ] Dunning H+1, H+7, H+14 → penangguhan otomatis H+21
- [ ] **Kunci idempotensi di database**: `(operator_id, period_start)` unik
      pada invoice, `(invoice_id, stage)` unik pada jejak dunning
- [ ] Siklus massal tunduk pada indeks nominal unik transfer — kegagalan
      sufiks dilaporkan **per baris**, tidak membatalkan seluruh siklus
- [ ] Grace period yang bisa diatur, per tenant bila perlu
- [ ] Void invoice + pulihkan, dengan jejak (jangan DELETE)
- [ ] Prorata saat upgrade/downgrade di tengah periode
- [ ] Penangguhan lewat **waktu, bukan status**: berhenti memperpanjang
      `access_until` + kolom `suspended_at`. Interceptor tidak berubah
- [ ] Pembayaran kapan pun **membatalkan rangkaian dan memulihkan akses**,
      termasuk sesudah H+21, tanpa campur tangan manual
- [ ] Tab **Langganan** di `/admin`

## B2 — Meter pemakaian 🔴

- [ ] Tabel `usage_counters` (operator_id, metric, period_start, value)
- [ ] Worker harian yang mengisinya — **jangan** hitung ulang per permintaan;
      menghitung jamaah lintas tenant tiap panel dibuka akan jadi query
      termahal di sistem ini
- [ ] Metrik: jamaah, cabang, penyimpanan, panggilan API, pesan WhatsApp
- [ ] Tab **Pemakaian**: pemakaian vs batas, peringatan 80% dan 100%
- [ ] Subjudul menyebut **tanggal reset** — kuota tanpa tanggal reset tidak bisa
      ditindak

## B3 — Detail tenant `/admin/tenant/[id]` 🟠

- [ ] Langganan & riwayat tagihan · pemakaian vs kuota · override berlaku
- [ ] Jamaah & cabang · transaksi & transfer · KYC
- [ ] Tim & status 2FA · domain · jejak audit tenant itu
- [ ] Tombol tindakan: ubah override, tangguhkan, impersonate (lihat C1)

---

# TAHAP C — Keamanan sebelum tim bertambah

## C1 — Impersonate dengan jejak penuh 🔴

- [ ] Sesi impersonasi ditandai berbeda di seluruh sistem
- [ ] **Read-only, tanpa mode tulis sama sekali.** Perubahan untuk pelanggan
      lewat RPC platform yang punya jejaknya sendiri — jangan menyamar
- [ ] Berbatas waktu, otomatis berakhir
- [ ] Dicatat lengkap: siapa, tenant mana, IP, alasan, durasi
- [ ] Uji: sesi impersonasi tidak bisa menulis sebelum ditingkatkan

## C2 — Four-eyes untuk tindakan tak bisa ditarik 🔴

Berlaku untuk: menangguhkan tenant, menghapus tenant, mengubah `plan_limits`
global, mengubah rekening settlement.

- [ ] Selama admin platform hanya satu: **konfirmasi ulang dengan mengetik nama
      tenant**
- [ ] Tabel `privileged_actions` sejak hari pertama, dengan
      `approved_by = requested_by` selama admin masih satu
- [ ] Setiap tindakan ini masuk `audit_logs` dengan alasannya

## C3 — Audit pembacaan data pribadi 🟠

- [ ] Membaca KYC / paspor / data jamaah dari panel platform tercatat, bukan
      hanya perubahannya
- [ ] Perbarui tabel inventaris data di
      [INSIDEN-DATA-PRIBADI.md](INSIDEN-DATA-PRIBADI.md)

## C4 — Rotasi kunci & ekspor auditor 🟡

- [ ] Rotasi kunci API dengan tumpang tindih 24 jam (pola yang sama dengan kunci
      KYC)
- [ ] Ekspor auditor: CSV + hash manifes, ditandatangani kunci platform

---

# TAHAP D — Siklus hidup tenant

Lihat §7 DESAIN. Bagian ini sebelumnya tidak dirancang di mana pun.

- [ ] **D1** `TrialDays` jadi setelan yang bisa diubah dari panel, bukan
      konstanta di `repository/subscription.go:20`. Nilainya sekarang **3 hari**
      — terlalu pendek untuk travel yang perlu impor Excel dan latih admin;
      pesaing memberi 14. Angkanya keputusan pemilik, tapi harus bisa diubah
      tanpa deploy.
- [ ] **D2** Perpanjang trial per tenant, alasan wajib
- [ ] **D3** Layar Langganan menampilkan trial yang berakhir pekan ini
- [ ] **D4** Antrean tenant baru 7 hari terakhir + penanda kelengkapan
      (sudah ada musim? jamaah? login kedua?)
- [ ] **D5** Pembatalan: `cancelled_at`, akses tetap sampai `access_until`
      berjalan habis — sisa periode yang sudah dibayar adalah haknya
- [ ] **D6** Penghapusan setelah 90 hari: **tawarkan ekspor data lebih dulu**
      (hak portabilitas UU PDP), four-eyes, dan **`audit_logs` tidak ikut
      dihapus** — ia bukti bahwa penghapusan itu sah
- [ ] **D7** Hitung mundur penghapusan tampil sebagai tanggal, dan masuk Pusat
      Tindakan saat mendekat

---

# TAHAP E — Pertumbuhan & komunikasi

- [ ] **E1** Analitik: MRR & pergerakannya, tenant aktif, konversi trial, churn,
      NRR — dengan **Catatan Metodologi**. Komisi market **bukan** MRR; tulis
      itu di layar. Skor churn ditandai sebagai heuristik, bukan vonis.
- [ ] **E2** Pengumuman ke tenant (§10.1 DESAIN): wizard 4 langkah, penerima
      **dihitung dari data**, Skor Kesiapan termasuk pemeriksaan "sudah ada
      pengumuman lain ke penerima sama dalam 24 jam", riwayat baca.
      **Tidak bisa diedit setelah terkirim** — kalau salah, kirim ralat
- [ ] **E3** Kesehatan platform (§10.2 DESAIN): antrean tertinggal,
      **event outbox dead-letter**, webhook gagal, poller bank berhenti,
      supplier gagal beruntun, backup terakhir, invoice macet. Setiap butir
      menyebut **berapa tenant terdampak**, dan yang sehat tetap ditampilkan
      hijau — layar yang hanya menunjukkan masalah tak bisa dibedakan dari layar
      yang rusak
- [ ] **E4** Audit global (§10.3 DESAIN): saringan per tenant / aktor / tindakan
      istimewa / impersonasi / pembacaan data pribadi. **Read-only tanpa
      pengecualian** — jangan tawarkan tombol yang pasti gagal
- [ ] **E5** Ekspor auditor: CSV + hash manifes, ditandatangani kunci platform,
      streaming sejak awal

---

# TAHAP F — Lintas semua tugas

Berlaku di sepanjang pengerjaan, bukan di akhir.

- [ ] **F1** Dunning & pengumuman memakai outbox `cascade_events` yang ada,
      **jangan buat jalur pengiriman baru** (§11 DESAIN). Efeknya harus
      idempoten karena pengirimannya at-least-once — itulah gunanya PK
      `(invoice_id, stage)`
- [ ] **F2** `service/errors.go`: galat tak terpetakan **juga** ditulis ke
      `slog` level error dengan nama metodenya. Sekarang hanya ke Sentry, dan
      `sentry.Init` no-op saat `SENTRY_DSN` kosong — di pengembangan galat itu
      hilang tanpa jejak (§12 DESAIN). Pesan ke klien tetap `internal error`
- [ ] **F3** `scripts/uji-batas-platform.sh` — menguji constraint §9 langsung
      terhadap skema, dan **dibuktikan bisa gagal** dengan mematikan salah satu
      constraint, seperti pada skrip cabang
- [ ] **F4** Setiap RPC platform baru diuji **dua arah**: tanpa sesi →
      `unauthenticated`, sesi owner operator asli → `permission_denied`, admin
      platform → berhasil, dicabut → ditolak pada panggilan **berikutnya**
- [ ] **F5** Uji jejak: panggil RPC, periksa `audit_logs` bertambah — dan
      **gagal kalau tidak**
- [ ] **F6** Uji idempotensi dengan **menjalankan dua kali**, bukan membaca kode

# TAHAP G — Rilis bertahap

Panel ini menyentuh uang dan data seluruh tenant. Urutannya (§14 DESAIN):

- [ ] **G1** Rilis yang hanya membaca lebih dulu: Pemakaian, Kesehatan, Audit
- [ ] **G2** Lalu tulis yang bisa ditarik: override kuota, routing, pengumuman
- [ ] **G3** Lalu tulis yang tidak bisa ditarik, **setelah four-eyes dan audit
      berjalan**: penangguhan, `plan_limits` global, penghapusan tenant
- [ ] **G4** Impersonate paling akhir, setelah audit terbukti mencatat semuanya
- [ ] **G5** Dunning dijalankan **mode kering dulu**: rangkaian jalan,
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
