# Tugas: Dashboard Admin Travel

Berkas kerja untuk agen berikutnya (Codex). Dibuat 30 Agustus 2026.
**Belum ada satu tugas pun yang dikerjakan** — semuanya masih T0.

## Cara memakai berkas ini

1. Baca `docs/RENCANA-DASHBOARD-ADMIN.md` (modul apa yang kurang & kenapa).
2. Baca `docs/DESAIN-DASHBOARD-ADMIN.md` (bagaimana layarnya harus terlihat).
3. Kerjakan tugas di bawah **berurutan**. Nomor kecil memblokir nomor besar.
4. Tandai `[x]` di berkas ini setiap tugas selesai **dan terverifikasi**,
   sertakan hash commit-nya. Ringkasan di chat tidak bertahan; berkas ini iya.

## Referensi

| Berkas | Isi |
|---|---|
| `docs/BENCHMARK-MEEQOT.md` | Analisa pesaing: rencana, harga, roadmap, apa yang tidak ditiru |
| `docs/RENCANA-DASHBOARD-ADMIN.md` | Peta 22 rute mereka vs 27 menu kita, urutan gelombang |
| `docs/DESAIN-DASHBOARD-ADMIN.md` | 8 pola UI, sistem `tone`, anatomi halaman, §2b resep visual |
| `docs/referensi/meeqot/rute.md` | Rute lengkap 9 aplikasi mereka |
| `docs/referensi/meeqot/judul-subjudul.md` | **310 pasang judul+subjudul** — contoh nyata pola subjudul hidup |
| `docs/referensi/meeqot/angka-desain.md` | Hitungan radius/shadow/transition/ring/hover, token `tone` |
| `docs/referensi/meeqot/model-data.md` | Nama kolom per modul, sebagai daftar periksa skema |

**Sumber aslinya** (masih hidup per 30 Agustus 2026, semuanya dummy tanpa
backend — boleh dibuka untuk melihat tata letaknya):
`admmeeqt.dul.co.id` (admin pusat — yang kita tiru) · `admcmeeqot.dul.co.id`
(cabang) · `jamaahmeeqot.dul.co.id` · `appsjamaah.dul.co.id` ·
`appskeluarga.dul.co.id` · `appspetugas.dul.co.id` · `devmeeqot.dul.co.id` ·
`saasmeeqot.dul.co.id/login` · `meeqot.id` (pemasaran).

Semua ekstraksi dibaca dari bundle JS publik. **Tidak ada akun yang dipakai dan
tidak ada endpoint yang diprobe — pertahankan batas itu.**

---

## Aturan yang berlaku untuk semua tugas

- Alur: proto → migrasi goose → sqlc → repository → service → handler → UI.
  **Repository tidak boleh mengimpor service.**
- Setiap operasi yang bisa diulang butuh kunci idempotensi yang **dipaksakan di
  database** (unique / partial unique index), bukan SELECT-lalu-INSERT.
- Commit setiap unit yang selesai dan terverifikasi. **Jangan push ke `main`** —
  push memicu deploy ke produksi; hanya pemilik yang memutuskan.
- `KYC_ENCRYPTION_KEY` wajib ada untuk membuat jamaah di lingkungan mana pun.
- Ekspor apa pun ditulis **streaming** sejak awal.
- Setiap animasi dibungkus `@media (prefers-reduced-motion: reduce)`.

---

# TAHAP 0 — Fondasi visual

Tidak menyentuh backend. Menutup sebagian besar jarak yang terasa tanpa satu
modul baru pun. **Kerjakan ini dulu.**

## T0.1 — Pindahkan primitif dari inline style ke CSS berkelas 🔴 PEMBLOKIR

- [x] `apps/web/components/ui/Button.tsx` (`696397d`)
- [x] `apps/web/components/ui/StatCard.tsx` (`696397d`)
- [x] `apps/web/components/ui/PageHero.tsx` (`696397d`)

**Kenapa memblokir:** ketiganya ditulis sebagai inline style object. Inline
style **tidak bisa menyatakan `:hover`, `:focus-visible`, atau `:active`.**
Selama masih begitu, T0.3 sampai T0.5 tidak bisa ditulis sama sekali.

Buat kelas `.tw-btn`, `.tw-card`, `.tw-stat` di `globals.css`, senapas dengan
`.dashboard-*` dan `.tenant-*` yang sudah ada di sana.

Catatan: `StatCard` sekarang memakai `border: 1px solid rgba(224,212,176,.6)` —
garis krem sisa tema lama yang **tidak ada lagi** di palet Emerald. Ganti ke
`--color-cream-300`.

**Selesai bila:** tidak ada `style={{...}}` tersisa di ketiga berkas, dan hover
+ focus ring terlihat di browser.

## T0.2 — Token baru di `globals.css`

- [x] Trio warna peringatan (`571b4ff`; palet kita **tidak punya** warna amber sama sekali,
      jadi semua peringatan terpaksa memakai merah bahaya):
      `--color-warning-700:#b45309` `--color-warning-600:#d97706`
      `--color-warning-200:#fde68a` `--color-warning-50:#fffbeb`
- [x] Tiga bayangan bernama `--shadow-soft` / `--shadow-lift` / `--shadow-glow` (`571b4ff`)
      (nilai persisnya ada di §2b DESAIN)
- [x] Keyframe `fade-up` + kelas `.tw-enter` (`571b4ff`)
- [x] Blok `@media (prefers-reduced-motion: reduce)` (`571b4ff`)

## T0.3 — Terapkan resep visual §2b

- [x] `rounded-xl` (12px) bawaan · `rounded-full` lencana · `rounded-2xl` kartu besar (`96004e7`)
- [x] Bayangan: `soft` diam → `lift` saat hover · `glow` hanya elemen merek (`96004e7`)
- [x] Cincin 4px merek-muda menggantikan garis keras, **termasuk untuk fokus** (`96004e7`)
- [x] Transisi 150 ms warna · 300 ms transform · 700 ms bilah & grafik, `ease-out` (`96004e7`)
- [x] Hover **naik satu langkah pada skala yang sama**, jangan ganti warna (`96004e7`)
- [x] `fade-up` 260 ms saat masuk, bertingkat 28 ms, maksimal 8 butir (`96004e7`)
- [x] Denyut **hanya** untuk rombongan berjalan dan SOS (`96004e7`)

Jangan tambahkan gradien di mana-mana — di bundle mereka gradien cuma dipakai
7 kali dari 3,3 MB, semuanya pada elemen merek. Kelembutan datang dari radius,
bayangan lebar-tipis, cincin, dan gerakan.

## T0.4 — Sebelas komponen bersama

- [x] `PageHeader` (judul + subjudul hidup + satu aksi primary) (`07b70f7`)
- [x] `StatCard` (nilai, **satuan**, label, delta, sparkline, `tone`) (`07b70f7`)
- [x] `ActionCenter` (rekomendasi + dampak rupiah + keadaan bersih) (`7a40403`)
- [x] `Badge` (`tone` → trio `-50`/`-200`/`-700`) (`07b70f7`)
- [x] `EmptyState` (judul, **sebab**, langkah berikutnya, tautan ke tempatnya) (`7a40403`)
- [x] `DataTable` (pencarian deskriptif, filter, ekspor, klik baris → panel) (`0a28a68`)
- [x] `DetailDrawer` (panel samping, mempertahankan konteks daftar) (`0a28a68`)
- [x] `Wizard` (langkah bernomor + skor kesiapan + validasi silang) (`a8e882e`)
- [x] `ChartFrame` (judul + subjudul penjelas sumbu — **wajib**) (`758097d`)
- [x] `ProgressBar` (transisi ±700 ms) (`758097d`)
- [x] `MethodologyNote` (`758097d`)

Enam `tone`: `success` `info` `brand` `warning` `danger` `neutral`.
Sembilan nada mereka dipadatkan — lebih sedikit, lebih konsisten.

## T0.5 — Sapu 27 layar yang sudah ada

- [x] **Subjudul hidup** di setiap layar — yang **menghitung**, bukan menjelaskan (`15bcace`)
      konsep. Contoh nyata: `docs/referensi/meeqot/judul-subjudul.md`
- [ ] **Keadaan kosong yang mengajar** — sebutkan sebabnya dan langkah
      berikutnya beserta tempatnya
- [ ] **Placeholder pencarian menyebut isinya** — *"Cari nama, ID, kota, paket,
      PIC, atau tag…"*, bukan *"Cari…"*
- [ ] **Satu tombol `primary` per layar.** Di dashboard mereka `primary` hanya
      muncul 9 kali, lawan `ghost` 112 dan `outline` 106. Inilah sebabnya layar
      mereka tenang meski padat.
- [ ] **Setiap grafik menjelaskan sumbunya** di subjudul
- [ ] **Setiap angka membawa satuan**

Daftar 27 layar ada di `apps/web/app/dashboard/(shell)/layout.tsx:49`.

---

# TAHAP 1 — Pusat Tindakan pada data yang sudah ada

Tanpa modul baru; datanya sudah ada.

- [ ] **T1.1** Jamaah — dokumen belum lengkap, paspor mendekati kedaluwarsa
- [ ] **T1.2** Arus kas — tagihan telat, transaksi belum diverifikasi
- [ ] **T1.3** Monitoring — rombongan berjalan tanpa petugas, SOS terbuka
- [ ] **T1.4** Dokumen — berkas kurang per rombongan

Setiap butir **wajib** menyebut nilainya dalam rupiah dan akibat kalau
diabaikan. Contoh nada dari mereka:

> *"23 tagihan lewat jatuh tempo — Nilai Rp 412 jt dari 19 jamaah. Setiap Rp 100 jt tertagih menambah laba bersih ±Rp 18 jt."*
> *"15 cicilan jatuh tempo ≤ 7 hari — Potensi kas masuk Rp 41 jt pekan ini. Kirim pengingat otomatis agar tidak bergeser ke bucket menunggak."*

Kalau kosong, tampilkan keadaan bersih yang menenangkan, bukan kartu kosong.

---

# TAHAP 2 — Modul struktural

## T2.1 — Hierarki cabang 🔴 paling mahal kalau ditunda

- [ ] Migrasi: tabel `branches` (operator_id, nama, kota, target_jamaah,
      target_revenue, kepala, rekening)
- [ ] `branch_id` nullable di `pilgrims`, `registrations`, `agents`, `orders`
- [ ] Peran `BRANCH_HEAD` di enum `user_role`
- [ ] **Penyaringan dipaksakan di lapisan repository, bukan handler**
- [ ] Agregasi laporan per cabang
- [ ] Layar `/dashboard/cabang` sesuai §4.2 DESAIN
- [ ] Uji **dua arah**: kepala cabang Bandung **bisa** melihat jamaahnya, dan
      **tidak bisa** melihat jamaah Medan

> **Bahaya:** kalau penyaringan `branch_id` hanya ada di handler, kepala cabang
> Bandung akan bisa membaca data jamaah Medan. Itu **pelanggaran UU PDP**, bukan
> bug tampilan. Lihat `docs/INSIDEN-DATA-PRIBADI.md`.

## T2.2 — Batas paket yang sungguhan 🔴 sebelum fitur apa pun

- [ ] Tabel `plan_limits` (plan, max_pilgrims, max_branches, flag fitur)
- [ ] Tabel `plan_overrides` per operator
- [ ] Satu fungsi `entitlement.Check()` dipanggil **service**, bukan handler
- [ ] Layar pemakaian vs batas di `/dashboard/langganan`
- [ ] Menu di luar paket tetap terlihat tapi terkunci — itu yang menjual naik paket

> Hari ini `plan` hanya menggerbangi domain kustom
> (`apps/api/internal/repository/operator_domain.go:31`). Tidak ada batas
> jamaah, tidak ada batas cabang, tidak ada fitur terkunci. **Tangga harga tanpa
> anak tangga** — dan setiap fitur baru setelah ini langsung bocor gratis ke
> STARTER. Karena itu tugas ini mendahului semua modul.

## T2.3 — Pembayaran bercicilan

- [ ] Skema: Bayar penuh · DP 50% + pelunasan · Cicilan 6× · Cicilan 12× ·
      Bonus Pelunasan Tunai
- [ ] Jadwal angsuran per jamaah, dengan **kunci idempotensi per angsuran**
- [ ] Umur piutang (aging) menurut jatuh tempo
- [ ] Verifikasi transaksi manual + Kirim Kwitansi
- [ ] Pengingat jatuh tempo (massal & satuan)
- [ ] Layar sesuai §4.3 DESAIN

> Ini cara mayoritas jamaah Indonesia membayar. Bukan fitur pembeda — **syarat
> masuk pasar.** Tanpanya banyak travel tidak bisa memakai TawafiqHub sama sekali.

---

# TAHAP 3 — Modul komersial

- [ ] **T3.1** CRM Leads `/dashboard/crm` — tahap `baru→kontak→penawaran→hot→closing`, §4.4
- [ ] **T3.2** Gateway WhatsApp — wizard 4 langkah + Skor Kesiapan + Jam Tenang, §4.7.
      Tiru juga ide mereka yang lebih dalam: **WhatsApp sebagai antarmuka
      cadangan** untuk jamaah lansia, bukan sekadar kanal notifikasi.
- [ ] **T3.3** Perjalanan & rundown — Rangkaian, Rundown, Manifes, Armada Bus,
      Roomlist, §4.5
- [ ] **T3.4** Tier kamar (Quad/Triple/Double) + kuota kursi, §4.6 RENCANA

---

# TAHAP 4 — Kelengkapan

- [ ] **T4.1** Inventaris & Purchase Order (§4.6 DESAIN)
- [ ] **T4.2** Manasik: kurikulum + sesi + absensi
- [ ] **T4.3** Agenda (kalender kegiatan gabungan)
- [ ] **T4.4** Layanan tambahan per jamaah
- [ ] **T4.5** Kelebihan bayar
- [ ] **T4.6** Pindah paket (dengan selisih harga)
- [ ] **T4.7** Momen — **foto & kabar boleh, GPS mentah tidak.**
      `FamilyStatus` kita sengaja menolak GPS, nomor kamar, dan paspor.
      Pertahankan; jadikan bahan jualan.
- [ ] **T4.8** Wizard pendaftaran 4 langkah (§4.10 DESAIN)
- [ ] **T4.9** Laporan laba rugi + Catatan Metodologi, ekspor **streaming**
- [ ] **T4.10** Pengaturan yang lebih dalam (§4.9 DESAIN) — 2FA wajib,
      pembatasan IP, satu perangkat per akun, matriks hak akses, matriks
      notifikasi, jam tenang, aturan eskalasi
- [ ] **T4.11** Support (tiket ke platform)
- [ ] **T4.12** Ekspor data mandiri oleh operator — **kewajiban portabilitas
      UU PDP**, bukan sekadar fitur jualan

---

## Yang sengaja TIDAK dikerjakan

- **Marketplace B2B** — produk kedua, bukan fitur; bersinggungan dengan
  keputusan bahwa jalur API ke supplier hanya milik TawafiqHub
- **Aplikasi terpisah** — satu PWA `/pilgrim` lebih baik daripada portal web +
  app mobile seperti mereka
- **AI menjawab fikih** — satu jawaban manasik keliru dari aplikasi bermerek
  travel jadi masalah travel itu. Kalau AI dipakai, batasi pada meringkas data
  operator sendiri
- **Tasbih & kiblat** — komoditas, tidak membedakan
- **Konsol anggaran AI, layar deploy/server, konfigurasi Turnstile sebagai UI** —
  membangun PaaS di dalam SaaS
- **Live location mentah ke keluarga** — pilihan kita berbeda dan lebih benar
  menurut UU PDP
- **Emoji di teks sistem** — tidak cocok dengan nada TawafiqHub

---

## Pekerjaan pemilik (tidak bisa dikerjakan agen)

- [ ] **Repo masih PUBLIC** — termurah, paling berharga
- [ ] `BANK_FEED_SECRET` belum diset — poller bank tidak bisa jalan
- [ ] Cron backup R2 belum dipasang
- [ ] Salin `backup-key.locked.pem` ke media kedua, lalu hapus yang belum terkunci
- [ ] IP webhook Xendit, cutover peran DB, cutover Caddy

## Status repo saat berkas ini dibuat

7 commit lokal belum di-push (semuanya dokumen; belum ada kode yang berubah). `main` = deploy, jadi push hanya atas perintah
pemilik.
