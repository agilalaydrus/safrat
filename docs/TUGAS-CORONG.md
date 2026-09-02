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

- [ ] **K1.1** Migrasi: `funnel_events` + `funnel_daily` sesuai §5 RENCANA.
      **Putuskan dan tulis alasannya** bagaimana `operator_id` NULL diperlakukan
      di PK `funnel_daily` — `COALESCE` ke UUID nol atau `NULLS NOT DISTINCT`.
      Kalau dilewatkan, baris ringkasan platform menggandakan diri diam-diam.
- [ ] **K1.2** `FUNNEL_SALT` sebagai environment variable, **tidak di database**.
      Tanpa garam, ruang IPv4 cukup kecil untuk membalik hash dari sebuah dump.
      Dokumentasikan di `DEPLOY.md` bersama kunci lain.
- [ ] **K1.3** Helper hash: `SHA256(salt ‖ tanggal ‖ IP ‖ user_agent)`, dengan
      uji bahwa hash **berubah saat tanggal berganti** dan **sama dalam satu
      hari** untuk masukan yang sama.
- [ ] **K1.4** Penyaring bot (§7): buang user-agent yang mengaku bot, dan
      `visitor_hash` yang menyentuh banyak path dalam hitungan detik tanpa pernah
      melewati `LANDING`.

# TAHAP K2 — Pencatatan

- [ ] **K2.1** Hook di `middleware.ts` untuk `LANDING` dan `KATALOG`.
      Matcher-nya sudah menyentuh setiap permintaan halaman, jadi tidak ada
      matcher baru. **Tulisnya asinkron dan gagal diam-diam.**
- [ ] **K2.2** `MULAI_ISI` dari klien saat kolom pertama form disentuh —
      `PublicRegistrationForm`, form waitlist, dan `/apply/[operatorId]`.
- [ ] **K2.3** `KIRIM` dan `SELESAI` di sisi server, di service pendaftaran dan
      waitlist. **Keduanya terpisah**: jaraknya adalah orang yang berusaha
      mendaftar lalu ditolak sistem kita sendiri, dan itu angka paling bisa
      ditindak di seluruh corong.
- [ ] **K2.4** Corong platform: `LANDING` di `/`, `MULAI_ISI` di `/sign-up`,
      `SELESAI` saat operator tercipta. `operator_id` NULL.
- [ ] **K2.5** Isi `crm_leads.source` dan `campaign` dari `utm_source` bila ada.
      Hari ini kolom itu diketik manual oleh staf travel.

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
- [ ] **K4.3** Catatan Metodologi: apa yang dihitung, apa yang dibuang sebagai
      bot, dan bahwa pengunjung dihitung **per hari** — orang yang sama di dua
      hari terhitung dua kali.
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
