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
      peramban sungguhan. Penyaring volume menyusul di rollup: token dengan
      lebih dari 60 kejadian sehari dibuang **utuh** — setengah sesi crawler
      bukan seorang manusia (`fa98c91`)

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
      di form sehingga sekali saja pada kolom pertama yang disentuh. Terpasang
      di ketiga form publik: pendaftaran, waitlist, dan `/apply`
      (`bd09302`, `35832e7`).
- [x] **K2.3** `KIRIM` dan `SELESAI` terpisah di handler pendaftaran, dan
      `SELESAI` membawa id pendaftarannya sehingga corong bisa di-join ke apa
      yang dihasilkannya. Diuji dengan percobaan yang **ditolak**, bukan hanya
      yang berhasil. Waitlist ikut, dan di sana `SELESAI` hanya ditulis bila
      entri benar-benar dibuat — `is_full=false` berarti musim masih ada kuota
      dan pengunjung dialihkan ke pendaftaran, jadi menghitungnya akan
      melaporkan konversi yang tidak terjadi (`bd09302`, `35832e7`).
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

- [x] **K3.1** Worker harian mengisi `funnel_daily`, mengulang **kemarin dan
      hari ini** karena hari baru lengkap setelah berakhir (`dfe9fec`)
- [x] **K3.2** Idempoten, diuji dengan menjalankan dua kali (`dfe9fec`)
- [x] **K3.3** Purge baris mentah > 90 hari dengan lantai 30 hari, ringkasan
      disimpan selamanya (`dfe9fec`)
- [x] **K3.4** Retensi ditulis di layar, bukan hanya di dokumen (lihat K4).

# TAHAP K4 — Layar travel

- [x] **K4.1** Tab **Corong Pengunjung** di `/dashboard/reports` (layar
      `/dashboard/analytics` hanya redirect ke sana), `FunnelDashboard.tsx`
- [x] **K4.2** Sumber diurutkan menurut **pendaftar**, bukan pengunjung. Kanal
      dengan 1.000 penonton dan nol pendaftar bukan kanal yang bagus.
      Angka pendaftar diambil dari `pilgrim_registrations.utm_source`, bukan
      dari kejadian SELESAI: atribusi di baris pendaftaran selamat dari penanda
      pengunjung yang diganti tiap tengah malam.
- [x] **K4.5** Jam aktif (0–23 WIB) **dan** tren harian. Keduanya dari
      `occurred_at`, dikelompokkan menurut Asia/Jakarta, bukan UTC.
- [x] **K4.6** Asal daerah per provinsi/kota. **Hanya pengunjung** — pendaftaran
      tidak menyimpan lokasi, jadi kolom pendaftar per daerah tidak dibuat
      daripada menampilkan kolom yang selalu nol. Bagian ini tetap kosong sampai
      K2.8 (GeoLite2) dipasang, dan layarnya mengatakan itu.
- [x] **K4.7** Usia pendaftar per kanal dari `date_of_birth`. Usia pengunjung
      tidak ditebak.
- [x] **K4.8** Kinerja artikel: pembaca (orang, bukan buka halaman) dan berapa
      yang menyelesaikan pendaftaran **di hari yang sama**.
- [x] **K4.3** Catatan Metodologi di kaki layar: hitungan per hari, tanpa cookie,
      atribusi lintas hari tidak akurat, ambang perayap 60 kejadian/hari dibuang
      seluruhnya, zona WIB, usia hanya dari pendaftar, dan retensi baris mentah.
- [x] **K4.4** Uji dua arah di `funnel_report_isolation_integration_test.go`:
      travel melihat corongnya sendiri **dan tidak bisa** melihat milik travel
      lain. Diverifikasi dengan merusak filter operator — uji gagal dengan pesan
      yang benar (`kanal "instagram" punya 7900 penonton`), lalu lulus lagi
      setelah dikembalikan.

**Catatan desain yang diputuskan saat mengerjakan:**

- Bilah corong diukur terhadap pembuka halaman depan, dan ARTIKEL boleh melebihi
  100%: artikel adalah pintu masuk tersendiri — orang dari mesin pencari masuk
  langsung ke artikel tanpa pernah membuka `/`. Bilahnya dipotong di 100% dan
  alasannya ditulis, bukan diskalakan ulang supaya kelihatan rapi.
- Label langkah menyebut yang benar-benar dicatat. Tidak ada halaman katalog
  terpisah, jadi KATALOG diberi label "Membuka halaman pendaftaran".

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
