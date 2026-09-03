# Tugas: Corong Pengunjung

Rancangan: [RENCANA-CORONG.md](RENCANA-CORONG.md).
Dibuat 2 September 2026. **Belum ada yang dikerjakan.**

> Antre **setelah** pekerjaan panel SaaS yang sedang berjalan
> ([TUGAS-PANEL-SAAS.md](TUGAS-PANEL-SAAS.md)). Ditulis sekarang supaya tidak
> hilang, bukan supaya disela.

Menyentuh dua permukaan: `/dashboard` (travel melihat corongnya sendiri) dan
`/admin` (pemilik platform melihat semuanya). Satu tabel melayani keduanya.

## Aturan

- proto → migrasi goose → sqlc → repository → service → handler → UI.
- **Penyaringan `operator_id` dipaksakan di repository, bukan handler.** Sebuah
  travel yang bisa membaca corong travel lain adalah kebocoran data, bukan bug
  tampilan — aturan yang sama dengan isolasi cabang.
- **IP mentah tidak pernah ditulis ke kolom mana pun.**
- Pencatatan gagal diam-diam. Analitik tidak boleh menjatuhkan storefront.
- Commit tiap unit yang selesai **dan terverifikasi**. Jangan push ke `main`.

---

# TAHAP K1 — Fondasi

- [x] **K1.1** Migrasi 146. Bukan PRIMARY KEY — PK tidak boleh memuat kolom
      nullable, jadi itu akan memaksa baris platform membawa operator palsu.
      Dipakai `UNIQUE ... NULLS NOT DISTINCT`, dan dibuktikan: tanpa klausul itu
      dua ringkasan platform identik lolos (`fa98c91`)
- [x] **K1.2** `FUNNEL_SALT` di environment, didokumentasikan di `DEPLOY.md`
      bersama alasan dan cara rotasinya (`fa98c91`)
- [x] **K1.3** `internal/funnel`, diuji: stabil dalam satu hari, berubah lintas
      hari, terpengaruh garam, dan **menolak mencatat tanpa garam layak** —
      tabel yang hanya terlihat anonim lebih buruk daripada tabel kosong
      (`fa98c91`)
- [x] **K1.4** Penyaring user-agent, diuji dua arah terhadap enam bot dan tiga
      peramban sungguhan. Penyaring berbasis pola lalu lintas menyusul di K3
      saat sudah ada data untuk mengujinya (`fa98c91`)

# TAHAP K2 — Pencatatan

> **Jalur pencatatannya sudah ada** (`2e242df`): `FunnelService.RecordEvent`,
> publik dan ber-rate-limit, dengan penyaring bot dan penolakan slug asing.
> Yang tersisa di tahap ini adalah **memanggilnya** dari lima titik corong.

- [x] **K2.1** Hook di `middleware.ts`, lewat `waitUntil` sehingga respons
      keluar lebih dulu dan setiap kegagalan ditelan (`ff4ebea`).
      **`KATALOG` dipetakan ulang**: tidak ada halaman katalog terpisah — paket
      tampil di halaman utama — jadi ia dipasang pada saat pengunjung membuka
      formulir satu paket. Itu sinyal yang sebenarnya dimaksud: berpindah dari
      melihat travel ke melihat satu perjalanan.
- [x] **K2.2** `MULAI_ISI` di `PublicRegistrationForm`, satu `onFocusCapture`
      di form sehingga sekali saja pada kolom pertama yang disentuh (`bd09302`).
      Form waitlist dan `/apply` menyusul.
- [x] **K2.3** `KIRIM` dan `SELESAI` terpisah di handler pendaftaran, dan
      `SELESAI` membawa id pendaftarannya sehingga corong bisa di-join ke apa
      yang dihasilkannya. Diuji dengan percobaan yang **ditolak**, bukan hanya
      yang berhasil (`bd09302`). Waitlist menyusul.
- [x] **K2.4** Corong platform: `LANDING` dari middleware, `KIRIM`/`SELESAI`
      saat operator dibuat, `operator_id` sengaja NULL — barisnya lalu lintas
      TawafiqHub, bukan milik tenant barunya (`8f83596`)
- [ ] **K2.5** Isi `crm_leads.source` dan `campaign` dari `utm_source` bila ada.
      Hari ini kolom itu diketik manual oleh staf travel.
- [x] **K2.6** `utm_source`/`utm_campaign` di `pilgrim_registrations`
      (migrasi 147), dibaca dari URL halaman form (`bd09302`).
      `season_waitlists` menyusul.
- [x] **K2.7** Langkah `ARTIKEL` dengan `article_slug` (`ff4ebea`)
- [ ] **K2.8** Geolokasi kota/provinsi dari IP (MaxMind GeoLite2). **IP tidak
      pernah ditulis** — hanya nama daerahnya. Tingkat kota, tidak lebih halus.

# TAHAP K3 — Rollup & retensi

- [ ] **K3.1** Worker harian mengisi `funnel_daily`. Layar **tidak pernah**
      menghitung ulang baris mentah — pola yang sama dengan `usage_counters`.
- [ ] **K3.2** Rollup idempoten: dijalankan dua kali untuk hari yang sama
      menimpa, tidak menggandakan. Diuji dengan **menjalankan dua kali**.
- [ ] **K3.3** Purge harian baris mentah > 90 hari. Ringkasan tetap.
- [ ] **K3.4** Retensi ditulis di layar, bukan hanya di dokumen.

# TAHAP K4 — Layar travel

- [ ] **K4.1** Bagian corong di `/dashboard/analytics` (§8.1)
- [ ] **K4.2** Sumber diurutkan menurut **pendaftar**, bukan pengunjung. Kanal
      dengan 1.000 penonton dan nol pendaftar bukan kanal yang bagus.
- [ ] **K4.5** Jam aktif: sebaran pengunjung per jam dan per hari, agregat dari
      `occurred_at`. Dipakai untuk jam publikasi artikel dan jam kirim broadcast.
- [ ] **K4.6** Asal daerah: pengunjung dan pendaftar per provinsi/kota
- [ ] **K4.7** Usia pendaftar per kanal, dari `pilgrim_registrations.date_of_birth`
      yang sudah ada. **Usia pengunjung tidak ada dan tidak boleh ditebak.**
- [ ] **K4.8** Kinerja artikel: dibaca berapa kali, berapa pembacanya lanjut
      mendaftar. Ini yang membuat strategi konten terukur.
- [ ] **K4.3** Catatan Metodologi: apa yang dihitung, apa yang dibuang sebagai
      bot, bahwa pengunjung dihitung **per hari** — orang yang sama di dua hari
      terhitung dua kali — dan bahwa **atribusi lintas hari tidak akurat** karena
      tidak memakai cookie, sehingga angka kanal bias ke yang mengonversi cepat.
- [ ] **K4.4** Uji dua arah: travel melihat corongnya sendiri, **dan tidak bisa**
      melihat milik travel lain.

# TAHAP K5 — Layar panel SaaS

- [ ] **K5.1** Tab **Corong** di `/admin` (§8.2), berlabel "Lintas seluruh travel"
- [ ] **K5.2** Corong platform sendiri: `/` → `/sign-up` → tenant aktif. Hari ini
      sama sekali tidak terlihat.
- [ ] **K5.3** Corong agregat seluruh travel — angka yang bisa dikutip saat menjual
- [ ] **K5.4** Papan peringkat storefront: konversi tertinggi dan terendah.
      **Yang terendah adalah daftar kerja**, bukan papan malu.
- [ ] **K5.5** Storefront **tanpa pengunjung sama sekali** masuk Pusat Tindakan.
      Travel yang membayar untuk sesuatu yang tidak dipakai akan berhenti
      berlangganan.
- [ ] **K5.6** Tautan ke `/admin/tenant/[id]` (B3) supaya corong satu tenant
      terbaca bersama langganan dan pemakaiannya.

# TAHAP K6 — Penutup

- [ ] **K6.1** Perbarui [INSIDEN-DATA-PRIBADI.md](INSIDEN-DATA-PRIBADI.md):
      catat `funnel_events` **dan alasan ia bukan data pribadi** — tanpa IP,
      tanpa cookie, hash bergaram yang berganti harian. Kalau salah satu syarat
      itu berubah, statusnya ikut berubah.
- [ ] **K6.2** Verifikasi tidak ada kolom mana pun yang memuat IP, dengan uji.
- [ ] **K6.3** Ukur beban: rollup pada 30 hari data harus selesai dalam hitungan
      detik, bukan menit.

---

## Yang sengaja TIDAK dikerjakan

- Session replay, heatmap
- Pelacakan per orang lintas hari (butuh cookie dan persetujuan)
- Angka real-time
- Menyimpan IP, disamarkan atau tidak
