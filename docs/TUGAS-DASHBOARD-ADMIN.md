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
- Setiap fakta keuangan memakai **encapsulated update**: jurnal dan mutasi uang
  bersifat append-only; kesalahan dikoreksi dengan entry pembalik yang merujuk
  transaksi asal, bukan `UPDATE`/`DELETE`. Nominal, beneficiary, serta sumber
  transaksi tidak boleh berubah setelah tercatat. Perubahan saldo dan status
  terkait harus atomik dalam satu transaksi ACID, dengan aktor, waktu, alasan,
  idempotency key, dan audit trail. Status workflow boleh berubah hanya bila
  tidak menulis ulang fakta uang yang sudah terjadi.
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

- [x] **Subjudul hidup** di setiap layar — yang **menghitung**, bukan menjelaskan
      konsep. Contoh nyata: `docs/referensi/meeqot/judul-subjudul.md` (`15bcace`)
- [x] **Keadaan kosong yang mengajar** — sebutkan sebabnya dan langkah
      berikutnya beserta tempatnya (`1a54a2d`)
- [x] **Placeholder pencarian menyebut isinya** — *"Cari nama, ID, kota, paket,
      PIC, atau tag…"*, bukan *"Cari…"* (`e5e60fc`)
- [x] **Satu tombol `primary` per layar.** Di dashboard mereka `primary` hanya
      muncul 9 kali, lawan `ghost` 112 dan `outline` 106. Inilah sebabnya layar
      mereka tenang meski padat. (`b6b39a2`, `98aa32f`, `4e1ab35`, `4251601`)
- [x] **Setiap grafik menjelaskan sumbunya** di subjudul (`82746fb`)
- [x] **Setiap angka membawa satuan** (`bbfe4a7`, `c33b784`)
      Aturan yang dipakai: satuan ditambahkan hanya bila **label belum menyebut
      benda yang dihitung**. "Kapasitas 34/40" ambigu → diberi satuan;
      "Total Grup 12" dan "Jamaah 34/40" sudah jelas → dibiarkan, karena
      mengulang jadi derau. Satuan ditulis dengan `<span class="tw-stat__unit">`
      supaya tampil kecil dan redup, bukan ikut besar-tebal seperti angkanya.

Daftar 27 layar ada di `apps/web/app/dashboard/(shell)/layout.tsx:49`.

---

# TAHAP 1 — Pusat Tindakan pada data yang sudah ada

Tanpa modul baru; datanya sudah ada.

- [x] **T1.1** Jamaah — paspor <6 bulan, berkas kurang, belum ada kloter/grup (`c33b784`)
- [x] **T1.2** Arus kas — vendor lewat jatuh tempo, defisit 30 hari, jamaah belum lunas (`363f46e`)
- [x] **T1.3** Monitoring — SOS, kesehatan berat, grup tanpa muttawwif, grup basi (`d0d8d6e`)
- [x] **T1.4** Dokumen — paspor/vaksin/pasfoto kurang, dihitung per jamaah (`ed2d2e2`)

Setiap butir **wajib** menyebut nilainya dalam rupiah dan akibat kalau
diabaikan. Contoh nada dari mereka:

> *"23 tagihan lewat jatuh tempo — Nilai Rp 412 jt dari 19 jamaah. Setiap Rp 100 jt tertagih menambah laba bersih ±Rp 18 jt."*
> *"15 cicilan jatuh tempo ≤ 7 hari — Potensi kas masuk Rp 41 jt pekan ini. Kirim pengingat otomatis agar tidak bergeser ke bucket menunggak."*

Kalau kosong, tampilkan keadaan bersih yang menenangkan, bukan kartu kosong.

**Catatan pelaksanaan.** Rupiah dipakai di tempat datanya memang ada — arus kas
punya nominal vendor sungguhan. Di layar jamaah, dokumen, dan monitoring tidak
ada harga per jamaah di basis data, jadi lencana dampak memakai satuan yang
benar (`n jamaah`, `n grup`) dan akibatnya ditulis di deskripsi. Menurunkan
rupiah dari pendapatan pesanan akan menghasilkan angka karangan yang terlihat
presisi.

Tiga layar ternyata sudah punya peringatan sendiri yang tidak bisa ditindak —
banner defisit di arus kas dan deretan pil di monitoring. Keduanya dilipat ke
Pusat Tindakan, bukan dibiarkan berdampingan: peringatan yang sama di dua
tempat membuat keduanya berhenti dibaca.

---

# TAHAP 2 — Modul struktural

## T2.1 — Hierarki cabang 🔴 paling mahal kalau ditunda

- [x] Migrasi: tabel `branches` (operator_id, nama, kota, target_jamaah,
      target_revenue, kepala, rekening) (`f3d6c38`, pengaman tenant `c85733e`)
- [x] `branch_id` nullable di `pilgrims`, `registrations`, `agents`, `orders`
      (`f3d6c38`)
- [x] Peran `BRANCH_HEAD` di enum `user_role` (`0db8968`; otorisasi aktif tetap
      memakai keanggotaan Better Auth + `branch_members`, bukan enum lama)
- [x] **Penyaringan dipaksakan di lapisan repository, bukan handler**
      (`6af60db`–`adfa952`, penutupan audit `4911b00`, `226e1cc`, `36e26aa`,
      `ebcf198`, `f917d6b`, `90876d0`, `753003c`)
- [x] Agregasi laporan per cabang — omzet bersih, jamaah, agen, capaian
      target, kolektibilitas, kesiapan dokumen, dan tren 12 bulan; laporan
      kepala cabang tetap dipaksa hanya untuk cabangnya (`f72cbf6`)
- [x] Layar `/dashboard/cabang` sesuai §4.2 DESAIN (`971a182`, penyempurnaan
      produk `6c78315`; KPI hidup, ranking, Pusat Aksi, pencarian, progres,
      detail drawer, form cabang, dan browser test; data performa nyata melalui
      BranchService, dengan menu terkunci oleh entitlement)
- [x] Uji **dua arah**: kepala cabang Bandung **bisa** melihat jamaahnya, dan
      **tidak bisa** melihat jamaah Medan (`6af60db`, integration test langsung
      di lapisan repository)

> **Bahaya:** kalau penyaringan `branch_id` hanya ada di handler, kepala cabang
> Bandung akan bisa membaca data jamaah Medan. Itu **pelanggaran UU PDP**, bukan
> bug tampilan. Lihat `docs/INSIDEN-DATA-PRIBADI.md`.

**Kemajuan scoping repository.** Vertical jamaah sudah dipaksa di query untuk
baca/list/statistik, mutasi, substitusi, dokumen, asuransi, roster kloter, dan
create yang otomatis mewarisi cabang aktor (`6af60db`). Inbox registrasi juga
sudah terisolasi; registrasi publik mewarisi cabang agen referral dan agent ID
lintas tenant ditolak oleh query (`dfb2e62`). Kotak penyaringan utama tetap
terbuka. Profil, KYC, dokumen, create, dan aplikasi referral agen sudah memakai
batas yang sama (`822b895`); payout agen, saldo, histori, komisi, dan request
pencairan sekarang juga terisolasi (`de0a3b1`). Order sekarang mewarisi cabang
pembeli dan dipagari saat dibaca, dihitung, dibayar
manual, atau diselesaikan dari status held (`ac5e846`). Kotak penyaringan utama
tetap terbuka sampai agregat dashboard/laporan cabang ditutup dengan batas yang
sama. Ringkasan musim, pendapatan pesanan, timeline pembayaran, statistik agen,
pengisian kloter, dan okupansi hotel kini juga menerima batas cabang dari
repository; uji integrasi membuktikan angka Bandung tidak memuat Medan,
sementara kantor pusat tetap operator-wide (`4c6db22`). Checklist per jamaah
dan statistik penyelesaiannya juga dipagari di query, termasuk mutasi upsert
(`daefeab`). Tagihan vendor kini memiliki `branch_id` nullable: tagihan lama
tetap tercatat sebagai kewajiban pusat, sedangkan tagihan baru dari kepala
cabang otomatis mewarisi cabangnya; daftar, mutasi, proyeksi, dan ringkasan
arus kas memakai batas yang sama (`1090e32`).
Monitoring SOS, kesehatan, progres ritual grup, dan timeline kepulangan juga
sekarang dibatasi oleh cabang di repository (`ba05834`). Laporan kesehatan
individual—termasuk list, cek kondisi BERAT untuk aturan perjalanan, pembuatan,
dan penyelesaian laporan—kini dipagari kembali di repository; uji integrasi
dua arah memastikan kepala Bandung tidak dapat menyentuh data Medan (`d93b4ab`).
Alert SOS individual dan daftar SOS per kloter juga kini memakai batas cabang
yang sama saat dibaca, dikonfirmasi, atau diselesaikan; uji integrasi dua arah
mengunci perilaku tersebut (`970da01`).
Laporan jamaah hilang, yang mencakup koordinat dan nomor telepon, kini dibatasi
untuk daftar serta penyelesaian di repository; tes dua arah membuktikan cabang
tidak dapat mengubah laporan cabang lain (`a216849`).
Pembatalan jamaah dan riwayat refund sekarang juga dipagari langsung di
repository, termasuk transaksi pembatalan atomik; test dua arah memverifikasi
Bandung tidak dapat melihat atau membatalkan jamaah Medan (`7117fb3`).
Daftar grup, kloter, dan roster kini menghitung hanya jamaah cabang aktif;
perubahan struktur grup/kloter dibatasi untuk kantor pusat di repository, dengan
uji integrasi dua arah (`8b474bc`).
Manifest kendaraan—termasuk nama, paspor, kebutuhan kursi roda, pemasangan, dan
pelepasan kursi—kini dibatasi per cabang di repository dan dikunci oleh uji
integrasi dua arah (`19d050b`).
Alokasi kamar kini juga dipagari di query/repository untuk manifest, daftar
penempatan, pemasangan, pelepasan, dan transfer substitusi; uji integrasi dua
arah memastikan kepala Bandung tidak dapat membaca atau mengubah penghuni
Medan (`0fa3006`). Kapasitas fisik kamar tetap dihitung operator-wide agar
kamar bersama tidak dapat terpesan melebihi kapasitas oleh dua cabang
(`519084b`). UI kamar ikut diselaraskan: kontras kartu diperbaiki, ringkasan
okupansi ditambahkan, identitas lintas cabang disembunyikan tanpa memalsukan
sisa kapasitas, seluruh jamaah dimuat bertahap, dan panel penempatan kini
memakai drawer aksesibel bersama (`56c817d`).
Status perjalanan jamaah kini dipagari untuk baca, agregat kloter, mutasi,
log, dan cascade grup/kloter langsung di query/repository. Uji dua arah
memastikan kepala Bandung tidak dapat melihat atau mengubah status perjalanan
Medan, sementara kantor pusat tetap operator-wide (`0027bf8`).
Progres ritual kini menghitung dan menampilkan nama jamaah hanya dari cabang
aktif; penyelesaian individual maupun bulk dipagari di query, ritual lintas
operator ditolak, dan kepala cabang dilarang membuat template operator-wide.
Uji dua arah membuktikan Bandung dapat menyelesaikan ritual jamaahnya tanpa
membaca atau mengubah Medan (`1e44d38`).
Chat grup kini memvalidasi bahwa pesan jamaah benar-benar dikirim ke grupnya.
Untuk staf cabang, thread hanya dapat dibaca atau dikirimi pesan bila seluruh
jamaah aktif di dalamnya berasal dari cabang yang sama; ini menjaga konteks
percakapan utuh tanpa membocorkan pesan grup lain. Akses publik jamaah dan
akses kantor pusat tetap berfungsi, dengan uji dua arah (`bbd8364`).
Broadcast musim diklasifikasikan sebagai komunikasi operator-wide: kepala
cabang tetap dapat membaca pengumuman pusat yang juga tampil di portal jamaah,
tetapi pembuatan dan penghapusan hanya boleh dilakukan kantor pusat. Pengaman
berada di repository dan dikunci integration test (`16977ec`).
Scope ritual kini bertahan melewati batas asynchronous: `branch_id` dibaca di
dalam transaksi, disimpan pada payload outbox, diteruskan worker/Firebase, dan
dipakai query token grup. Event lama tanpa field tersebut tetap operator-wide
untuk kompatibilitas antrean produksi. Integration test membuktikan hanya token
Bandung yang dipilih dan pasangan `operator_id`/`pilgrim_id` lintas tenant tidak
dapat didaftarkan (`adfa952`).
`MyAccess` kini mengekspos `branch_id` sebagai kontrak proto agar frontend
tidak menebak authority dari jumlah cabang. Layar komunikasi memakai nilai itu
untuk menampilkan composer/hapus hanya bagi kantor pusat; kepala cabang mendapat
mode baca dengan penjelasan cakupan, dan kegagalan pemeriksaan akses bersifat
fail-closed. Lint, typecheck, API suite, serta build produksi 70 halaman lulus
(`e349c03`).
Refund order dan payout refund kini memaksa scope berdasarkan pemilik transaksi
di repository; kepala Bandung tidak dapat mengunci order, membaca riwayat, atau
memproses payout Medan (`4911b00`, `226e1cc`). Klaim asuransi dipagari untuk
create, list, perubahan status, dan ekspor data medis (`36e26aa`). Antrean
pengiriman—termasuk nama, telepon, alamat, resi, dan bukti serah-terima—dipagari
untuk seluruh baca dan mutasi (`ebcf198`). Jadwal petugas hanya terlihat untuk
kloter yang memuat jamaah cabang aktif, sedangkan perubahan struktur petugas
tetap khusus kantor pusat (`f917d6b`). Audit trail kini membekukan `branch_id`
saat append sehingga perpindahan staf tidak menulis ulang makna bukti lama;
legacy log tidak di-backfill dengan tebakan, dan pembacaan dipagari per cabang
(`90876d0`). PII antrean tunggu yang belum punya assignment cabang tetap menjadi
inbox pusat sampai CRM menyediakan alokasi eksplisit; kepala cabang tidak dapat
membaca atau memutasinya, sementara jalur publik dan worker tetap bekerja
(`753003c`). Setiap unit memiliki integration test dua arah dan kantor pusat
tetap operator-wide.

## T2.2 — Batas paket yang sungguhan 🔴 sebelum fitur apa pun

- [x] Tabel `plan_limits` (plan, max_pilgrims, max_branches, flag fitur)
      (`e7bc6e2`; STARTER 200 jamaah/0 cabang, GROWTH 500/3, PRO tanpa batas)
- [x] Tabel `plan_overrides` per operator (`e7bc6e2`)
- [x] Satu fungsi `entitlement.Check()` dipanggil **service**, bukan handler
      (`401b6c7`; pembuatan jamaah sudah memakai cek ini, dengan trigger DB
      sebagai pengaman konkurensi)
- [x] Layar pemakaian vs batas di `/dashboard/langganan` (`281ebbe`)
- [x] Menu di luar paket tetap terlihat tapi terkunci — Cabang pada STARTER
      membawa operator ke langganan, bukan disembunyikan (`281ebbe`)

> Hari ini `plan` hanya menggerbangi domain kustom
> (`apps/api/internal/repository/operator_domain.go:31`). Tidak ada batas
> jamaah, tidak ada batas cabang, tidak ada fitur terkunci. **Tangga harga tanpa
> anak tangga** — dan setiap fitur baru setelah ini langsung bocor gratis ke
> STARTER. Karena itu tugas ini mendahului semua modul.

**Pengaman konkurensi.** Trigger PostgreSQL mengunci transaksi per operator dan
memeriksa penggunaan saat `INSERT` jamaah/cabang. Dua request bersamaan tidak
dapat sama-sama lolos dari limit; pemeriksaan service yang ramah-pengguna tetap
menyusul sebagai lapisan pengalaman, bukan satu-satunya pengaman.

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

**Progres aktif (belum menutup T2.3):** ledger cicilan append-only, skema dan
jadwal beku, idempotensi database, reversal, aging, isolasi cabang dua arah,
entitlement, serta workspace UI sudah tersedia di `d49add5` dan `481816f`.
Integration test juga membuktikan overpayment konkuren ditolak dan KPI kas
masuk membaca ledger. Yang masih memblokir tanda selesai: pengiriman kwitansi
melalui email, pengingat jatuh tempo satuan dan massal yang durable, serta uji
visual browser untuk layar final.

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
