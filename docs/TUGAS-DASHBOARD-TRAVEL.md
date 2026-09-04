# Tugas: Dashboard Travel (`/dashboard`)

> **Yang mana panel ini.** Berkas ini soal **`/dashboard`** — dashboard yang
> dipakai staf travel pelanggan kita, satu tenant saja. **Bukan** `/admin`,
> panel platform milik pemilik TawafiqHub yang melihat seluruh tenant. Panel itu
> punya berkasnya sendiri: [RENCANA-PANEL-SAAS.md](RENCANA-PANEL-SAAS.md) dan
> [TUGAS-PANEL-SAAS.md](TUGAS-PANEL-SAAS.md).


Berkas kerja untuk agen berikutnya (Codex). Dibuat 30 Agustus 2026.
Tahap 0, 1, dan 2 selesai (lihat centang di bawah). Sisa: Tahap 3 dan 4.

## Cara memakai berkas ini

> **Panel SaaS punya berkas sendiri:** [TUGAS-PANEL-SAAS.md](TUGAS-PANEL-SAAS.md).
> Berkas ini hanya dashboard operator. Keduanya bisa dikerjakan paralel — tidak
> ada berkas yang beririsan kecuali `globals.css` dan `platform.proto`.
>
> **Penanda pemilik** dipakai di kedua berkas: `[C]` Codex, `[K]` Claude,
> `[ ]` belum diklaim. Klaim sebelum menyentuh kode.

1. Baca `docs/RENCANA-DASHBOARD-TRAVEL.md` (modul apa yang kurang & kenapa).
2. Baca `docs/DESAIN-DASHBOARD-TRAVEL.md` (bagaimana layarnya harus terlihat).
3. Kerjakan tugas di bawah **berurutan**. Nomor kecil memblokir nomor besar.
4. Tandai `[x]` di berkas ini setiap tugas selesai **dan terverifikasi**,
   sertakan hash commit-nya. Ringkasan di chat tidak bertahan; berkas ini iya.

## Referensi

| Berkas | Isi |
|---|---|
| `docs/BENCHMARK-MEEQOT.md` | Analisa pesaing: rencana, harga, roadmap, apa yang tidak ditiru |
| `docs/RENCANA-DASHBOARD-TRAVEL.md` | Peta 22 rute mereka vs 27 menu kita, urutan gelombang |
| `docs/DESAIN-DASHBOARD-TRAVEL.md` | 8 pola UI, sistem `tone`, anatomi halaman, §2b resep visual |
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

- [x] Skema: Bayar penuh · DP 50% + pelunasan · Cicilan 6× · Cicilan 12× ·
      Bonus Pelunasan Tunai (`d49add5`)
- [x] Jadwal angsuran per jamaah, dengan **kunci idempotensi per angsuran**
      yang dipaksakan database (`d49add5`)
- [x] Umur piutang (aging) menurut jatuh tempo (`481816f`)
- [x] Verifikasi transaksi manual + Kirim Kwitansi (`d49add5`, `209993d`)
- [x] Pengingat jatuh tempo (massal & satuan) (`209993d`)
- [x] Layar sesuai §4.3 DESAIN (`481816f`, `209993d`)

> Ini cara mayoritas jamaah Indonesia membayar. Bukan fitur pembeda — **syarat
> masuk pasar.** Tanpanya banyak travel tidak bisa memakai TawafiqHub sama sekali.

**Selesai:** ledger cicilan append-only, skema dan jadwal beku, idempotensi
database, reversal, aging, isolasi cabang dua arah, entitlement, workspace UI,
kwitansi email, serta pengingat satuan dan massal tersedia di `d49add5`,
`481816f`, dan `209993d`. Integration test membuktikan overpayment konkuren
ditolak, KPI kas membaca ledger, dan antrean email tidak bocor antar cabang.
Worker memakai SMTP TLS dengan lease, retry, stable Message-ID, dan dead-letter.

---

# TAHAP 3 — Modul komersial

- [x] **T3.1** CRM Leads `/dashboard/crm` — tahap `baru→kontak→penawaran→hot→closing`, §4.4
      (`aecde68`, `b4bed4c`, `029ea6d`)
- [ ] **T3.2** Gateway WhatsApp — wizard 4 langkah + Skor Kesiapan + Jam Tenang, §4.7.
      Tiru juga ide mereka yang lebih dalam: **WhatsApp sebagai antarmuka
      cadangan** untuk jamaah lansia, bukan sekadar kanal notifikasi.
- [x] **T3.3** Perjalanan & rundown — selesai (4 September 2026).
      - [x] **Manifes** di `/dashboard/kloter/[id]`: seluruh jamaah satu
            penerbangan dengan dokumen yang masih kurang, disebut namanya dan
            berurutan, plus rekap "yang paling menahan keberangkatan" dan unduh
            CSV.
      - [x] **Rangkaian** (`KloterItinerary.tsx`, migrasi 155
            `kloter_itinerary_segments`) — daftar segmen Transportasi/Hotel
            yang diurutkan operator, disimpan sebagai satu set lewat
            `SetKloterItinerary` (ganti-semua, bukan edit satu-satu — sama
            seperti tier kamar, supaya layar tidak pernah membaca urutan yang
            separuh tersimpan). Wajib mulai & akhiri dengan Transportasi,
            ditegakkan di repository dan dibuktikan dengan merusak:
            pengecekan bookend dibuang → test gagal persis pada kasus
            "Rangkaian mulai dari Hotel". Movement/hotel juga divalidasi
            benar-benar milik kloter/operator yang sama, bukan cuma dipercaya
            dari klien.
      - [x] **Rundown** per hari (`KloterRundown.tsx`, tabel
            `kloter_rundown_items`) — jadwal operasional harian yang dipegang
            koordinator/muttawwif di lapangan, berbeda dari
            `product_itinerary_days` yang sifatnya materi jualan sebelum
            pembelian.
      - [x] **Armada Bus** (`KloterArmadaBus.tsx`) — memakai ulang
            `TransportService` yang sudah ada (`VehicleFormDialog`,
            `VehicleManifestPanel`) tapi disaring hanya pada movement Bus yang
            benar-benar sudah jadi segmen di Rangkaian; kosong berkata persis
            seperti §4.5 DESAIN: *"Belum ada segmen Bus di Rangkaian
            Perjalanan — Tambahkan segmen Bus di tab Rangkaian sebelum
            mengelola armada."*
      - [x] **Roomlist** dikelompokkan per kota → hotel → kamar, dengan unduh
            CSV. Menampilkan yang **belum dapat kamar sama sekali** (baris
            paling mendesak, dan tidak mungkin muncul dari join yang berangkat
            dari tabel alokasi), tempat tidur kosong, dan menandai **kamar
            keluarga berisi laki-laki dan perempuan yang mahramnya tidak ada di
            kamar itu**.

            Penandaan itu satu-satunya hal yang tidak bisa ditangkap aturan
            alokasi: alokasi sudah menolak laki-laki masuk kamar berperuntukan
            perempuan, tapi kamar `family` memang menerima siapa saja. Jadi ini
            disurfacekan, bukan diblokir — pasangan suami istri sekamar itu
            biasa, dua orang asing beda jenis kelamin sekamar tidak.

            Tautan mahram hanya tercatat di satu sisi, jadi pemeriksaannya
            melihat dua arah. Diverifikasi dengan merusak: hanya melihat satu
            arah → gagal `pasangan suami istri di kamar keluarga ikut
            ditandai`; saringan "belum punya alokasi" dibuang → gagal `jamaah
            tanpa kamar tidak muncul` dengan seluruh roster ikut terbawa.

      **Dua penilaian yang membuat manifes ini terpakai, bukan sekadar benar:**

      1. **Jamaah yang sudah digantikan tidak ada di manifes.** Mereka tidak
         berangkat; menghitungnya membuat penerbangan penuh terlihat kurang dan
         manifes siap terlihat belum.
      2. **Buku nikah hanya diminta dari yang berangkat dengan mahram.**
         Menuntutnya dari semua orang menandai sebagian besar manifes "kurang"
         untuk dokumen yang memang tidak diminta dari mereka — dan angka
         kesiapan yang salah untuk separuh daftar adalah angka yang tidak
         dipercaya siapa pun.

      Keduanya diverifikasi dengan merusak: buku nikah dijadikan wajib untuk
      semua → gagal `jamaah lengkap dianggap kurang: [BUKU_NIKAH]`; filter
      `is_substituted` dibuang → gagal `5 baris, mau 4`.

      CSV dibuat di peramban dari baris yang sama dengan yang ditampilkan, jadi
      berkas dan layar tidak bisa berbeda. Diawali BOM, karena tanpa itu Excel
      di Windows membaca berkasnya sebagai Latin-1 dan setiap nama dengan
      diakritik sampai dalam keadaan rusak.
- [x] **T3.4** Tier kamar (Quad/Triple/Double) + kuota kursi — migrasi 152,
      `product_room_tiers`, editor di `/dashboard/products/harga`.

      **Harga tier disimpan sebagai selisih, bukan angka tersendiri.** Alasannya
      sama dengan alasan `ListProductPricing` menghitung dan tidak menyimpan:
      salinan absolut jadi basi begitu harga paketnya bergerak, lalu dua angka
      bertengkar tanpa ada yang bisa bilang mana yang benar. Selisih selamat
      dari semua itu.

      **Kuota ditegakkan database, bukan layanan.** Ada lebih dari satu jalur
      yang membuat pesanan (dashboard, checkout publik, agen, manual), jadi
      pemeriksaan di satu jalur adalah pemeriksaan yang hilang di jalur lain.
      Pemicu `assert_room_tier_quota` menolak pesanan yang melewati batas, dan
      memakai `pg_advisory_xact_lock` per tier.

      Kuota kosong = tanpa batas; nol = tier ada tapi habis. Keduanya tidak
      boleh terbaca sama, di database maupun di layar.

      **Diverifikasi dengan merusak — dua kali.** Versi pertama uji balapannya
      memakai autocommit dan **lulus walau kuncinya dilepas** (kedua pernyataan
      tidak pernah benar-benar bertumpuk) — jadi ia tidak membuktikan apa pun.
      Sekarang keduanya di transaksi terbuka masing-masing: dengan kunci
      dilepas, pemesanan kedua **selesai sebelum yang pertama di-commit** dan
      kursi terakhir terjual dua kali. Uji juga memastikan tier yang sudah
      terjual tidak bisa dihapus, kuotanya tidak bisa disetel di bawah yang
      sudah terjual, harga tidak bisa jatuh di bawah nol, dan travel lain tidak
      bisa membaca maupun menulis tier paket ini.

**Selesai:** pipeline CRM memakai timeline append-only, transisi tahap yang
divalidasi, idempotensi yang dipaksakan database, entitlement paket, serta
isolasi cabang di repository untuk seluruh baca dan mutasi. Integration test
membuktikan cabang dapat mengelola lead miliknya dan tidak dapat membaca atau
memutasikan lead cabang lain. Workspace UI mencakup KPI, pencarian dan filter,
kanban responsif, pembuatan lead, detail aktivitas, empty/error/loading state,
serta menu terkunci untuk paket tanpa hak akses. Proto, migrasi goose, sqlc,
repository, service, handler, dan UI telah terhubung utuh.

---

# TAHAP 4 — Kelengkapan

- [x] **T4.1** Inventaris & Purchase Order (§4.6 DESAIN) — selesai
      (4 September 2026). `/dashboard/inventaris`, migrasi 156
      (`inventory_items`, `inventory_stock_movements`, `purchase_orders`,
      `purchase_order_items`).

      Item gudang, kartu stok (`inventory_stock_movements`) sebagai satu-satunya
      sumber kebenaran, dan PO yang penerimaannya menulis stok + statusnya
      (`PARTIAL`/`RECEIVED`) dalam **satu transaksi** — sebuah baris PO tidak
      pernah bisa terbaca "diterima" sementara rak sendiri belum mencerminkan
      itu. Dibuktikan dengan merusak: rollup status PARTIAL/RECEIVED dibuang →
      test gagal persis pada transisi ke RECEIVED yang tidak pernah terjadi.

      **Dipotong dari cakupan, disebut jujur:** dari lima grafik di desain,
      hanya **Nilai Persediaan**, **Item di Bawah Minimum**, **PO Berjalan**,
      dan **Perputaran Stok** (rasio disederhanakan: kuantitas keluar 90 hari
      ÷ stok saat ini — perkiraan, bukan angka akuntansi COGS/rata-rata
      persediaan yang sebenarnya) yang dibangun sebagai KPI. **Radar Kesiapan
      Keberangkatan** (kebutuhan per kloter vs stok) dan **Performa Vendor**
      (ketepatan kirim 12 bulan) belum dibangun — keduanya butuh menghubungkan
      `per_pilgrim_qty` item ke roster kloter sungguhan, bukan hanya field di
      tabel. Pusat Tindakan Gudang juga baru mencakup "di bawah minimum"; "PO
      lewat ETA", "selisih opname", dan "melebihi kapasitas rak" belum ada.
- [x] **T4.2** Manasik: kurikulum + sesi + absensi — selesai (4 September
      2026). `/dashboard/manasik`, migrasi 157 (`manasik_curricula`,
      `manasik_sessions`, `manasik_attendance`). Musim-scoped seperti
      Checklist; sesi opsional dikaitkan ke satu topik kurikulum dan/atau
      satu kloter, tapi sebagian besar manasik berjalan untuk seluruh musim
      sebelum kloter final.

      **Absensi adalah upsert, bukan insert.** Roll-call diambil satu nama
      pada satu waktu; koreksi tanda yang salah (`RecordManasikAttendance`
      dipanggil ulang untuk pasangan sesi+jamaah yang sama) harus menimpa
      baris yang sama, bukan menambah baris kedua yang akan menggandakan
      hitungan hadir/tidak hadir. Ditegakkan oleh
      `UNIQUE(session_id, pilgrim_id)` + `ON CONFLICT DO UPDATE`. Diverifikasi
      dengan merusak: klausa `ON CONFLICT` dibuang dari query lalu di-generate
      ulang lewat sqlc → percobaan koreksi kedua gagal persis dengan
      pelanggaran constraint unik, bukan diam-diam menggandakan baris.

      Berbeda sengaja dari `product_itinerary_days` (materi jualan sebelum
      pembelian) dan Rundown kloter (jadwal operasional kloter yang sudah
      berangkat) — Manasik adalah pelatihan sebelum keberangkatan.
- [x] **T4.3** Agenda (kalender kegiatan gabungan) — selesai (5 September
      2026). `/dashboard/agenda`, migrasi 158 (`agenda_events`, hanya untuk
      acara internal).

      Manasik dan keberangkatan/kepulangan kloter **tidak disalin** ke
      tabel baru — `ListAgenda` membacanya langsung dari `manasik_sessions`
      dan `kloter_itinerary_segments` (posisi pertama/terakhir yang
      `TRANSPORT` pada spine T3.3), supaya kedua tampilan tidak bisa
      terpisah. Kloter dengan hanya satu segmen TRANSPORT (rangkaian belum
      lengkap) sengaja dilewati lewat `HAVING MIN(position) <> MAX(position)`
      — dibuktikan dengan menghapus klausanya: satu-satunya segmen
      terhitung dua kali sebagai keberangkatan sekaligus kepulangan.

      **Pusat vs cabang** hanya berlaku untuk acara internal — manasik dan
      kloter bukan milik cabang manapun (satu operator, satu kloter),
      jadi saringan cabang tidak pernah menyembunyikannya. Diuji dan
      dibuktikan dengan merusak: melonggarkan saringan cabang di query
      (`OR e.branch_id IS NULL`) membuat acara pusat bocor ke tampilan
      cabang — uji gagal persis di titik itu.

      Diperiksa di peramban sungguhan lewat akun fixture: linimasa
      tergabung dan terurut lintas tiga sumber, buat/ubah/hapus acara
      internal lewat drawer, tanpa galat konsol.
- [x] **T4.4** Layanan tambahan per jamaah — selesai (5 September 2026).
      `/dashboard/layanan-tambahan`, migrasi 159 (`addon_items` katalog
      per-musim, `pilgrim_addons` penetapan per jamaah).

      **Harga di-snapshot saat ditetapkan**, bukan dibaca ulang dari katalog.
      `pilgrim_addons.unit_price_idr` disalin sekali dari `addon_items` saat
      `AssignPilgrimAddon` dan tidak pernah disentuh lagi — mengubah harga
      katalog setelahnya tidak mengubah apa yang sudah disepakati jamaah yang
      sudah ditetapkan. Dibuktikan dengan merusak: query dibuat membaca harga
      hidup dari `addon_items` alih-alih baris `pilgrim_addons` — uji gagal
      persis di titik itu (harga jamaah lama ikut naik).

      **Pembayarannya sengaja masih penanda lunas/belum**, bukan lewat mesin
      order/cicilan/komisi yang sudah ada. Layanan tambahan bukan bagian dari
      paket musim (tidak ikut `platformMargin`/`operatorMargin`/
      `agentMargin`), jadi menyalurkannya lewat `OrderService` berarti
      menambah jalur komisi baru untuk sesuatu yang bukan produk musim.
      Kalau nanti perlu ditagih formal, itu perluasan tersendiri.

      **"Jamaah ber-add-on di grup ini" dijawab lewat saringan grup di
      halaman ini** (`ListPilgrimAddons` menerima `group_id`), bukan dengan
      menambah badge di `/dashboard/groups/[id]`. Halaman itu sudah stabil
      dan tidak disentuh — kalau pemilik minta lencana di roster grup,
      itu tambahan kecil yang bisa menyusul.

      Diperiksa di peramban sungguhan lewat akun fixture: tambah katalog,
      tetapkan ke jamaah lewat drawer pencarian nama, tandai lunas — semua
      round-trip tanpa galat konsol.
- [x] **T4.5** Kelebihan bayar — panel **Kelebihan Bayar** di Profil Jamaah,
      dibangun bersama T4.6 (satu tabel `pilgrim_credits`, karena kelebihan
      bayar hanya pernah muncul dari sana). Status terbuka/dipakai/
      dikembalikan, dan menyelesaikan kredit yang sama dua kali ditolak.
      **Belum ada:** antrean lintas seluruh travel di luar profil per jamaah —
      desain hanya memintanya di layar Profil Jamaah, jadi itu yang dibangun.
- [x] **T4.6** Pindah paket — migrasi 153 (`pilgrim_plan_changes`,
      `pilgrim_credits`), `OrderService.ChangeOrderProduct`, panel di Profil
      Jamaah → tab Dokumen & Pembayaran.

      **Pesanan lama tidak pernah ditulis ulang seolah menjadi paket baru.**
      Kolom harganya diperbarui supaya mencerminkan paket yang sedang dijalani
      sekarang, tapi `paid_amount_idr` — uang yang benar-benar diterima —
      tidak pernah disentuh, dan `pilgrim_plan_changes` menyimpan angka lama
      supaya tidak hilang.

      **Hanya untuk pesanan berstatus PAID.** Pesanan yang belum dibayar
      diedit langsung, bukan "dipindah" — kerangka membandingkan yang sudah
      dibayar dengan harga baru cuma masuk akal setelah uang benar-benar
      berpindah.

      **Ditemukan saat menguji, bukan setelah dikirim:** `MarkOrderPaidManually`
      (pembayaran tunai/manual) **tidak pernah mengisi** `paid_amount_idr` —
      kolom itu tetap NULL walau status sudah PAID. Tanpa perbaikan, setiap
      penjualan tunai akan terbaca sebagai "membayar nol", dan pindah paket
      apa pun dari pesanan tunai akan salah dilaporkan sebagai kekurangan
      bayar penuh. Diperbaiki dengan `COALESCE(paid_amount_idr,
      total_price_idr)` — NULL di sini berarti "lunas penuh, angka
      persisnya tidak dicatat", bukan "belum bayar apa-apa". Diverifikasi
      dengan merusak: `COALESCE`-nya dibuang → kegagalan pemindaian nilai
      NULL ke kolom yang tidak boleh kosong, persis seperti yang akan terjadi
      di produksi.

      **Diverifikasi lagi dengan merusak arah kelebihan/kekurangan bayar**:
      ditukar terbalik → gagal `kelebihan bayar = 0, mau 15.000.000`.

      Komisi agen disesuaikan lewat **satu entri selisih** (`ADJUSTMENT`),
      bukan membalik lalu mencatat ulang dengan kunci pesanan yang sama —
      itu akan bentrok dengan entri `EARNED` asli pesanan itu sendiri.

      **Celah ini sudah ditutup (4 September 2026):** tier kamar sekarang ikut
      dihitung di jalur pembuatan pesanan biasa (`CreateOrder`,
      `CreateManualOrder`, `CreateOrderForPilgrim`) lewat `applyRoomTier` di
      `internal/service/order.go` — sqlc → domain → repository → proto →
      service semua diperbarui supaya `orders.room_tier` benar-benar tertulis,
      yang berarti trigger kuota (`assert_room_tier_quota`, hanya menyala
      kalau `room_tier` terisi) akhirnya ikut aktif di pembelian pertama, dan
      selisih harga tier tidak lagi hilang. Dibuktikan dengan merusak:
      panggilan `applyRoomTier` dan `RoomTier:` di parameter dibuang →
      `TestCreateManualOrderPricesAndEnforcesRoomTierIntegration` gagal persis
      seperti bug aslinya (`total = 30.000.000, mau 35.000.000`). UI pemilihan
      tier ditambahkan di `CreateOrderDialog.tsx` (dashboard) dan
      `SellPackageDialog.tsx` (agen) — muncul otomatis kalau produk punya
      tier aktif.
- [x] **T4.7** Momen — selesai (5 September 2026). `/dashboard/momen` (staf),
      digabung ke `/track/[code]` (keluarga, publik). Migrasi 160
      (`pilgrim_moments`).

      **Foto & kabar boleh, GPS mentah tidak** — dipertahankan sebagaimana
      mestinya. Tabel `pilgrim_moments` tidak punya kolom lokasi sama sekali,
      jadi tidak ada yang bisa bocor lewat sini walau ada bug di tempat lain.

      **Menyasar satu jamaah ATAU satu grup, tidak pernah dua-duanya** —
      `CHECK ((pilgrim_id IS NOT NULL) <> (group_id IS NOT NULL))` di database,
      supaya petugas lapangan yang memotret satu bus tidak perlu mengunggah
      empat puluh kali. Sisi baca keluarga (`ListFamilyMoments`, publik,
      autentikasi `app_access_code` saja seperti `GetFamilyStatus`) mencocokkan
      `pilgrim_id` **atau** `group_id` milik jamaah itu. Diuji dan dibuktikan
      dengan merusak: kueri dilonggarkan menjadi "tampilkan semua momen
      bergrup" tanpa mencocokkan grup jamaah yang sebenarnya — uji gagal
      persis di titik itu (jamaah luar grup ikut melihat momen grup orang
      lain).

      Foto diunggah langsung ke S3 lewat presigned URL (pola yang sama dengan
      bukti serah terima pengiriman) — tidak pernah lewat server ini, dan
      kunci objek dipastikan benar-benar ada (`HEAD`) sebelum baris database
      ditulis. **Video sengaja dipotong dari cakupan** — foto saja untuk saat
      ini; video butuh validasi format/ukuran dan kendali pemutaran yang
      belum dibangun.

      **Ditemukan saat verifikasi peramban sungguhan, bukan dikira beres
      karena `go build` lulus:** tautan lihat foto yang di-presign gagal
      dengan `AccessDenied: headers present in the request which were not
      signed` di MinIO — aws-sdk-go-v2 sejak ~v1.30 menyalakan validasi
      checksum secara default dan menambahkan header `x-amz-checksum-mode`
      ke setiap presigned GET, sementara klien biasa (tag `<img>`, `fetch`
      polos) tidak pernah mengirim header itu kembali. Ini **bug lama yang
      sudah ada**, bukan sesuatu yang baru diperkenalkan momen ini — bukti
      serah terima pengiriman (`PresignHandoverView`, dipakai sejak
      Kelebihan Bayar/Pindah Paket) memakai pola presign yang persis sama dan
      pasti kena bug yang sama, hanya belum pernah diuji lewat peramban
      sungguhan sampai sekarang. Diperbaiki sekali di `storage.New` —
      `RequestChecksumCalculation`/`ResponseChecksumValidation` diset ke
      `WhenRequired` pada level klien S3 — sehingga memperbaiki **semua**
      fitur presigned-view sekaligus, bukan hanya Momen.
- [x] **T4.8** Wizard pendaftaran 4 langkah (§4.10 DESAIN) — selesai (5
      September 2026). `/dashboard/pilgrims/baru`, tautan "Pendaftaran
      Terpandu" di halaman Jamaah.

      **Murni frontend — tidak ada RPC baru.** Empat langkah memanggil RPC
      yang sudah ada: `CreatePilgrim`/`UpdatePilgrim` (Data Diri),
      `ListProducts`/`ListProductRoomTiers` (Paket & Kamar, harga dihitung di
      klien dari `price_idr` + `price_delta_idr`), `CreateManualOrder`
      (Pembayaran), dan `PilgrimDocumentChecklist` yang sudah ada dipakai
      ulang di Konfirmasi. Setiap langkah menulis lewat RPC-nya sendiri saat
      "Lanjut" ditekan — bukan formulir raksasa yang dikirim sekali di akhir
      — supaya jamaah dan pesanan sudah tercatat walau wizard ditinggal
      sebelum langkah terakhir.

      **Validasi silang nyata, bukan contoh kosong:** kalau nomor WhatsApp
      belum diisi di Data Diri, langkah Pembayaran menolak lanjut dan
      mengarahkan kembali — nomor itu dipakai kirim tautan Xendit dan
      pengingat pembayaran, dan tidak ada tempat lain untuk mengisinya.

      **Sengaja dipotong dari cakupan, dicatat di sini:** langkah Pembayaran
      hanya menangani metode `CreateManualOrder` (tunai/transfer/tautan
      Xendit — pesanan langsung lunas atau menunggu webhook). "Simulasi
      cicilan" di rancangan **tidak** dibangun di sini — skema `Bayar
      Penuh/DP 50/6x/12x/Bonus Tunai` (`installment_plans`) tidak berelasi
      dengan tabel `orders` di skema database sama sekali (dibuktikan dengan
      membaca migrasi 135 langsung); menciptakan hubungan pesanan↔cicilan
      yang tidak ada di skema berisiko salah pada logika keuangan yang
      justru paling mahal untuk disalahkan. Operator yang perlu cicilan
      membuat rencana pembayaran di halaman Arus Kas setelah pesanan ini
      dibuat — layar itu sendiri sudah utuh sejak T2.3.

      Diperiksa di peramban sungguhan lewat akun fixture, seluruh empat
      langkah dari nol: pilgrim dibuat, paket+kamar dipilih dengan pratinjau
      harga, pesanan tercatat lunas, dan checklist dokumen tampil — tanpa
      galat konsol. Menyingkap dua prasyarat data yang belum jelas
      terdokumentasi: `KYC_ENCRYPTION_KEY` wajib ada bahkan untuk
      `CreatePilgrim` biasa (bukan hanya alur KYC), dan sebuah produk baru
      butuh baris `product_markups` (migrasi 111) sebelum bisa dijual —
      tanpanya `CreateManualOrder` menolak dengan "markup produk belum
      diatur", bukan galat yang jelas menyebut penyebabnya.
- [x] **T4.9** Laporan laba rugi + Catatan Metodologi, ekspor **streaming** —
      selesai (5 September 2026). Tab **Laba Rugi** baru di
      `/dashboard/reports`, tanpa migrasi — seluruhnya dihitung on-read dari
      `orders`/`products`/`branches`/`agents` yang sudah ada.

      **Lintas waktu, bukan per musim.** Laba rugi adalah laporan keuangan
      operator, bukan laporan satu musim — 5 bulan terakhir dihitung lintas
      musim, sama seperti Analitik (E1) di panel platform.

      **Definisi laba bersih ditulis dan diuji eksplisit, bukan diasumsikan
      benar:** `laba bersih = pendapatan − fee platform − komisi agen − biaya
      pokok yang diketahui` — bukan `pendapatan − biaya` saja, yang akan
      menghitung bagian platform dan agen seolah milik operator. Dibuktikan
      dengan merusak: definisi disederhanakan jadi `pendapatan − biaya` →
      uji gagal dengan selisih persis sebesar total fee platform + komisi
      agen yang seharusnya dikurangi.

      **Biaya pokok yang tidak diketahui tidak pernah dianggap nol.**
      `products.supplier_cost_idr` nullable (banyak paket travel tidak
      mengisinya) — pesanan dengan biaya tidak diketahui dikeluarkan dari
      `cost_idr`, dan `orders_missing_cost`/`revenue_missing_cost_idr`
      membawa berapa banyak yang tidak tercakup, ditulis eksplisit di
      Catatan Metodologi setiap kali ada. Pesanan `REFUNDED` tidak pernah
      dihitung sebagai pendapatan.

      **Ekspor sungguhan streaming, dibuktikan sampai lapisan SQL.**
      `StreamProfitLossExport` (server-streaming Connect RPC, pola yang sama
      dengan `MonitoringService.StreamEvents`) sengaja **tidak** memakai
      query `:many` sqlc yang membuffer seluruh baris ke memori Go sebelum
      mengirim satupun — repository membaca `pgxpool.Pool` langsung dan
      memanggil `rows.Next()` satu-satu, mengirim tiap baris lewat
      `stream.Send` saat itu juga. Inilah yang gagal di kompetitor pada
      1.240 jamaah menurut changelog mereka sendiri (dicatat di RENCANA);
      menukar ke `:many` akan lulus semua test unit tapi mengembalikan
      persis kegagalan yang sama pada skala besar.

      Diperiksa di peramban sungguhan: laporan dengan tiga pesanan lunas
      lintas cabang dan satu agen menghasilkan setiap angka yang cocok
      dengan perhitungan tangan (laba bersih, capaian target cabang,
      kontribusi), dan ekspor CSV yang diklik sungguhan mengunduh berkas
      dengan baris yang cocok — pesanan `REFUNDED` tidak ikut terekspor.
- [x] **T4.10** Pengaturan yang lebih dalam (§4.9 DESAIN) — **sebagian**,
      selesai (5 September 2026). Tab **Kebijakan Keamanan** baru di
      `/dashboard/settings`, migrasi 161 (`operator_security_settings`).

      **Penemuan sebelum membangun apa pun: dua dari tiga "yang belum kita
      punya" menurut §4.9 sudah lama ada — hanya belum ada layarnya.**
      2FA sudah wajib untuk seluruh staf tanpa syarat (`RequireTwoFactor
      mode="enforce"` di seluruh shell dashboard) dan satu sesi per akun
      sudah ditegakkan tanpa syarat (`databaseHooks.session.create` di
      `apps/web/lib/auth.ts` mencabut sesi lain yang sama persis saat sesi
      baru dibuat) — keduanya **lebih ketat** daripada "bisa diatur per
      operator" yang diminta rancangan, jadi dibangun sebagai klaim
      **tetap** (tidak bisa dimatikan dari layar ini), bukan sakelar. Sama
      persis dengan kejadian B2 di Panel SaaS — dokumen rancangan tidak
      diperbarui setelah kapabilitasnya lahir dari komit lain.

      **Satu-satunya kesenjangan nyata: pembatasan IP.** Baru dan opsional
      (nonaktif secara default, tidak mengubah perilaku operator manapun
      yang tidak pernah membukanya). **Pengaman yang paling penting**:
      server menolak mengaktifkan daftar yang tidak menyertakan IP
      pemanggil sendiri saat itu — dibuktikan dengan merusak (guard
      dilonggarkan → uji gagal tepat di titik itu) dan diperiksa di
      peramban sungguhan: aktifkan dengan IP sendiri, muat ulang halaman,
      pastikan tidak terkunci. `SecuritySettingsService` sendiri dikecualikan
      dari pemeriksaan IP-nya sendiri (pola yang sama dengan
      `billingProcedures` — lihat komentarnya) supaya operator yang salah
      mengetik CIDR tetap bisa membuka layar yang memperbaikinya.

      **Sesi Aktif**: daftar + cabut sesi, dibaca langsung dari tabel
      `session` Better Auth (pola SQL mentah yang sama dengan
      `internal/middleware/auth.go`). Dicabut dan dibuktikan dengan
      merusak: kondisi penyaring operator dihapus dari kueri cabut → uji
      gagal dengan operator A berhasil mencabut sesi milik operator B.

      **Ditemukan saat menguji, diperbaiki sebelum sempat dipakai:** IP
      pemanggil di middleware awalnya selalu string kosong secara lokal
      (`clientIP(header, "")` — argumen peer address tidak pernah
      dialirkan dari `WrapUnary`/`WrapStreamingHandler`), yang berarti
      pengaktifan pembatasan IP akan **selalu ditolak** di luar produksi
      (di belakang nginx). Diperbaiki dengan mengalirkan `request.Peer().Addr`/
      `conn.Peer().Addr` sampai ke `authenticate()`, pola yang sama yang
      sudah dipakai `ratelimit.go`.

      **Sengaja tidak dibangun sesi ini, dicatat jujur:** matriks hak
      akses (klik sel untuk ubah level Lihat/Ubah/Penuh), matriks
      notifikasi per event, jam tenang, dan aturan eskalasi. Keempatnya
      adalah sistem sendiri-sendiri (mesin izin granular per RPC, mesin
      perutean notifikasi) yang menyentuh permukaan jauh lebih luas
      daripada satu tabel pengaturan — membangunnya tergesa demi
      menyelesaikan daftar berisiko menghasilkan layar yang **terlihat**
      menegakkan sesuatu padahal tidak, persis peringatan §4.9 tentang
      "klaim di layar" kompetitor yang coba dihindari proyek ini.
- [ ] **T4.11** Support (tiket ke platform)
- [x] **T4.12** Ekspor data mandiri oleh operator — **kewajiban portabilitas
      UU PDP**. Migrasi 154 (`operator_data_exports`), tab **Ekspor Data Saya**
      di Pengaturan.

      **Disusun dari repository yang sama dengan yang dipakai setiap layar,
      tidak pernah dari SQL mentah terhadap tabelnya.** Nomor paspor dan
      identitas tersegel (AES-256-GCM) di database — kueri mentah akan
      mengembalikan cipherteks ke operator sendiri. Repository sudah
      mendekripsinya; itu sebabnya ekspor memanggil `PilgrimRepository.List`,
      bukan `SELECT * FROM pilgrims`.

      **Diproses di latar belakang, bukan menahan permintaan HTTP.** Tabel
      antrean sama dengan pola outbox yang sudah ada di proyek ini: baris
      ditulis PENDING, worker mengklaim dengan `FOR UPDATE SKIP LOCKED`
      (`@every 1m`), menyusun berkas ZIP (CSV per tabel + `BACA-DULU.txt`),
      mengunggahnya, menandai READY. Tautan unduhan **dibuat baru setiap
      diminta** (berlaku 15 menit) — tidak pernah disimpan sebagai URL tetap,
      dan berkasnya sendiri kedaluwarsa 7 hari lewat sapuan terpisah
      (`@every 6h`) yang benar-benar menghapus objeknya, bukan hanya
      mengubah status di database.

      **Diverifikasi dengan merusak dua kali:** batasan operator pada
      `Get` dibuang → gagal `travel lain bisa membaca ekspor travel ini`;
      uji tingkat worker membuktikan kegagalan penyusunan berkas tercatat
      dengan alasannya (bukan diam-diam ditelan) dan sapuan kedaluwarsa
      benar-benar menghapus objek dari penyimpanan, bukan cuma mengubah
      baris.

      **Dipotong dari cakupan, disebut jujur:** hanya musim, paket, jamaah,
      dan transaksi yang diekspor — agen, grup, dan kloter adalah struktur
      organisasi, bukan data pribadi jamaah, dan `BACA-DULU.txt` di dalam
      berkas mengatakan itu kepada operator secara langsung.

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
