# Benchmark: Meeqot

Ditinjau 30 Agustus 2026. Sumber: delapan aplikasi publik di `*.dul.co.id`
dan situs pemasaran `meeqot.id`. Semua yang ada di sini dibaca dari bundle
JavaScript publik mereka — tidak ada akun yang dipakai, tidak ada endpoint
yang diprobe, tidak ada autentikasi yang disentuh.

## Temuan pertama, yang mengubah cara membaca sisanya

**Meeqot belum punya backend.**

Bukti, berurutan dari yang paling sulit dibantah:

- `api.meeqot.app`, `auth.meeqot.app`, `bayar.meeqot.app` — **tidak resolve**.
  Ketiganya dirujuk dari dalam bundle sebagai tujuan API, webhook, dan login.
- Bundle 3,3 MB (`admmeeqt`) berisi **satu** pemanggilan `fetch(`. Bundle 1,7 MB
  (`appspetugas`) berisi dua. Tidak ada instance axios yang dikonfigurasi,
  tidak ada base URL API, tidak ada alur token.
- Aplikasi petugas merender kalimatnya sendiri: `"Aplikasi memakai data simulasi"`.
- Delapan aplikasi memakai **satu cast nama palsu yang sama** — Ahmad Fauzan,
  Aisyah Putri, Andi Nurul Fadillah — dan satu tenant fiktif "Al-Hijrah Travel"
  dengan lima cabang fiktif.
- Daftar "pelanggan" di panel SaaS mereka adalah array hardcoded lengkap dengan
  nilai MRR.
- **Admin Pusat (`admmeeqt`) terbuka tanpa login sama sekali.** Bukan kebocoran —
  memang tidak ada yang menjaganya, karena tidak ada yang bisa menjaga.
- `saasmeeqot.dul.co.id` menyediakan halaman `/login` dengan akun dummy yang
  dibagikan terbuka. Bundle-nya identik dengan situs pemasaran ditambah satu
  rute, tanpa chunk tambahan, **dan nol pemakaian `localStorage`** — login yang
  sungguhan menyimpan token; yang ini tidak menyimpan apa pun.

Artinya Meeqot adalah **prototipe klik-able beresolusi tinggi**, bukan sistem
yang berjalan. Itu tidak membuatnya tidak relevan — justru sebaliknya. Sebagai
**benchmark cakupan dan desain** ia sangat berguna. Sebagai benchmark rekayasa,
ia tidak ada isinya.

Konsekuensi yang perlu dipegang: dalam demo penjualan, prototipe yang cantik
mengalahkan sistem yang bekerja. Kekalahan TawafiqHub bukan pada kemampuan,
melainkan pada **luas permukaan yang terlihat** dan pada desain.

## Peta produk mereka

Delapan aplikasi (pengguna menyebut lima; tiga sisanya ditemukan dari rujukan
silang di dalam bundle):

| Aplikasi | Peran | Rute |
|---|---|---|
| `meeqot.id` | Pemasaran | — |
| `admmeeqt` | **Admin Pusat** | agenda, broadcast, cabang, crm, hotel, inventaris, jamaah, kelebihan-bayar, laporan, layanan-tambahan, market, momen, monitoring, paket, pembayaran, pencairan-komisi, perjalanan, petugas, support, template-manasik, transportasi |
| `admcmeeqot` | **Dashboard Cabang** | agen, agen/cairkan-komisi, agenda, jadwal, jamaah, laporan, pembayaran, pendaftaran |
| `jamaahmeeqot` | Portal Jamaah (web) | bantuan, checklist, dokumen, jadwal, kartu, keluarga, pembayaran, rombongan |
| `appsjamaah` | Mobile Jamaah | ai, chat, doa-keluarga, dokumen, dompet, ganti-paket, haram, ibadah, kiblat, live, manasik, perjalanan, tasbih |
| `appskeluarga` | **Aplikasi Keluarga** | chat, doa, ibadah, kabar, kirim, momen, pantau, perjalanan |
| `appspetugas` | Aplikasi Petugas (Muthawwif) | absensi, absensi-manasik, broadcast, chat, insiden, laporan, momen, ops, peta |
| `devmeeqot` | Panel SaaS pemilik | ai, analitik, autentikasi, billing, deploy, domain, email, gateway, integrasi, keamanan, log, market, pelanggan, server, turnstile |

### Harga mereka (dari bundle, bukan dugaan)

| Tier | Harga | Batas | Isi |
|---|---|---|---|
| Free | Rp 0 | — | — |
| Starter | **Rp 990.000/bln** | 200 jamaah, 1 cabang | dashboard pusat, paket & jamaah, cash & cicilan, website subdomain, support email |
| Growth | **Rp 2.490.000/bln** | 1.000 jamaah, 5 cabang | + dashboard cabang, aplikasi mobile jamaah, live location & panic button, broadcast WhatsApp, QR check-in |
| Enterprise | **Rp 5.990.000/bln** | tanpa batas | + marketplace, API publik |

TawafiqHub hari ini: STARTER Rp 589.000 · GROWTH Rp 789.000 · PRO Rp 2.489.000.

Growth mereka (Rp 2.490.000) dan PRO kita (Rp 2.489.000) praktis identik. Yang
mereka janjikan di harga itu jauh lebih banyak. **Selisih Rp 1.000 dengan
selisih isi sebesar itu adalah masalah posisi harga, bukan masalah fitur saja.**

## Yang TawafiqHub sudah punya dan mereka tidak

Ini bukan penghiburan — ini yang tidak boleh dikorbankan saat mengejar.

- **Backend yang benar-benar ada.** 36 service Connect RPC, Postgres 127 migrasi,
  sqlc, worker asynq. Milik mereka nol.
- **Uang yang benar-benar bergerak.** Order, refund, payout agen, payout refund,
  wallet, langganan, rekonsiliasi mutasi bank, batas belanja harian.
- **Enkripsi at-rest** (AES-256-GCM) untuk KYC dan nomor paspor, dengan blind
  index dan sidik jari kunci. Meeqot menampilkan string "AES-256-GCM (envelope)"
  di layar panel — sebagai teks.
- **Jejak audit yang tidak bisa ditulis ulang** oleh peran aplikasi, retensi 24
  bulan, plus prosedur insiden UU PDP yang sudah dilatih terhadap skema nyata.
- **Storefront white-label** dengan domain sendiri, verifikasi domain, blog/berita,
  dan on-demand TLS.
- **Pelacakan keluarga yang privacy-first** — sengaja tanpa GPS mentah, tanpa
  nomor kamar, tanpa paspor. Lihat catatan di bawah; ini keunggulan, bukan
  kekurangan.
- **Kloter, substitusi jamaah, kebijakan pembatalan, klaim asuransi, vendor,
  arus kas, waitlist, sertifikat** — semuanya nyata.

## Yang kurang, diurutkan menurut yang paling menentukan

### Tingkat 1 — struktural, mahal untuk ditambal belakangan

1. **Hierarki cabang (pusat ↔ cabang).** Tidak ada sama sekali di TawafiqHub;
   satu operator = satu organisasi datar. Meeqot menjadikannya jualan utama:
   aplikasi cabang terpisah, konsolidasi real-time, target & komisi per cabang,
   "Ajukan setoran ke pusat", laporan keuangan per cabang.
   Ini gap terbesar dan yang paling sulit ditambahkan nanti karena menyentuh
   **setiap query multi-tenant, setiap pemeriksaan RBAC, dan setiap laporan.**
   Kalau hanya satu hal yang dikerjakan dari dokumen ini, ini orangnya.

2. **Aplikasi Keluarga.** Kita punya `/track/[code]` — satu halaman status.
   Mereka punya aplikasi: pantau, momen, kabar, doa bersama, chat, kirim uang saku.
   Rasa sakit yang mereka kutip nyata dan sering: *"Keluarga menelepon kantor
   tiap hari menanyakan kabar jamaah."*
   **Catatan penting:** mereka membagikan live location ke keluarga. Kita sengaja
   tidak. Pilihan kita lebih benar menurut UU PDP dan harus dipertahankan —
   tetapi *momen, kabar, dan chat* bisa kita berikan tanpa membocorkan posisi.
   Itulah bentuk yang harus dikejar: kehangatannya, bukan pelacakannya.

3. **CRM / pipeline leads.** Kita punya waitlist dan registrations — itu hanya
   *dasar* corong. Tidak ada penangkapan leads dari web/IG/WA, tidak ada
   follow-up, tidak ada segmentasi alumni untuk umroh berikutnya. Untuk travel,
   alumni adalah sumber penjualan berikutnya yang paling murah.

### Tingkat 2 — komersial, terasa langsung saat demo

4. **Gateway WhatsApp.** Milik kita hanya *kolom nomor kontak* di storefront.
   Mereka: broadcast tersegmentasi, pengingat jatuh tempo, notifikasi keluarga,
   dari nomor bisnis operator. Di Indonesia ini nyaris syarat minimum.
   Kita sudah punya BroadcastService dan web push — kanalnya yang kurang.

5. **Cicilan & tabungan umroh.** Mereka: cicilan tetap syariah, tabungan,
   kalkulator cicilan di website, pengingat otomatis, aging piutang.
   Kita: pembayaran dan invoice, tanpa mesin rencana cicilan, tanpa buku besar
   tabungan, tanpa umur piutang. Ini cara mayoritas jamaah Indonesia membayar.

6. **Alur eVisa / muassasah** sebagai objek kelas satu — status tersinkron,
   checklist agar pengajuan tidak ditolak, submit batch per rombongan.
   Kita punya dokumen dan pelacakan kedaluwarsa paspor, tetapi visa bukan
   entitas.

7. **Marketplace B2B antar travel** — seat grup, allotment hotel, visa, katering.
   Kita punya lapisan dispatch supplier (platform → supplier untuk produk
   digital), bukan pasar travel ↔ travel. Lihat "terlalu banyak" di bawah.

### Tingkat 3 — modul, masing-masing kecil

8. **Manasik sebagai modul** — template/kurikulum, sesi, absensi manasik. Kita
   punya rituals dan checklist, bukan kurikulum dan kehadiran.
9. **Inventaris & Purchase Order** — stok perlengkapan, PO ke vendor. Kita punya
   pengiriman/fulfilment, bukan stok.
10. **Kelebihan bayar** sebagai alur penyelesaian tersendiri (kita hanya refund).
11. **Pindah paket** dengan selisih harga (kita punya substitusi *orang*, bukan
    perpindahan *paket*).
12. **Layanan tambahan** per jamaah — bagasi ekstra, kursi eksekutif, badal umroh.
13. **Momen** — petugas mengunggah foto, keluarga bereaksi. Retensi emosional,
    murah dibuat, berdampak besar pada persepsi.
14. **Insiden lapangan** untuk petugas — serah-terima penanganan, alarm darurat.
15. **Peta langsung** untuk petugas: posisi rombongan, peringatan keluar zona,
    peringatan baterai jamaah lemah. Kita tidak memakai pustaka peta sama sekali.
16. **QR check-in** di bandara, hotel, bus. Kita punya check-in, belum berbasis QR.
17. **Dompet jamaah & uang saku** dari keluarga.
18. **Fitur ibadah di aplikasi jamaah** — kiblat, tasbih, doa per lokasi dengan
    geofence (Multazam, Raudhah), audio panduan manasik, mode hemat baterai
    saat thawaf.
19. **Kedalaman panel SaaS.** `/admin` kita punya 6 tab. Mereka menampilkan
    billing, domain, email + penanganan bounce, gateway pembayaran, analitik,
    log, pengumuman, keamanan.

## Yang berlebihan — jangan ditiru

- **Delapan aplikasi.** Portal Jamaah (web) *dan* Aplikasi Jamaah (mobile) adalah
  permukaan yang sama dua kali. Satu PWA `/pilgrim` kita lebih baik, lebih murah
  dirawat, dan tidak memecah perhatian pengguna.
- **Konsol anggaran AI** (Anthropic/Qwen/DeepSeek dengan pengatur anggaran),
  layar **deploy** dan **server**, konfigurasi **Turnstile** sebagai layar UI.
  Ini membangun PaaS di dalam SaaS. Operasional milik perkakas operasional,
  bukan milik layar produk.
- **AI Ustadz yang menjawab pertanyaan fikih.** Risiko tanggung jawab yang nyata:
  satu jawaban manasik yang keliru dari aplikasi *bermerek travel* adalah
  masalah travel itu, bukan masalah model. Kalau AI dipakai, batasi pada
  meringkas data milik operator sendiri — bukan berfatwa.
- **Tasbih & kiblat.** Komoditas; puluhan aplikasi gratis melakukannya. Murah
  ditambahkan kapan saja, hampir tidak membedakan. Jangan dahulukan.
- **Marketplace B2B.** Produk kedua, bukan fitur. Juga bersinggungan dengan
  keputusan yang sudah diambil: jalur API ke supplier hanya milik TawafiqHub.
  Tunda sampai inti selesai.
- **Live location mentah ke keluarga.** Mereka melakukannya; kita sengaja tidak.
  Pertahankan. Ini justru bahan jualan: *keluarga tahu kabar tanpa jamaah
  dilacak*.

## Cara membaca ini sebagai standar minimum

Urutan yang saya sarankan, dan alasannya:

1. **Cabang** — karena ia mengubah bentuk data. Setiap minggu yang lewat membuat
   penambahannya lebih mahal.
2. **WhatsApp** — dampak paling besar per baris kode; kanal, bukan fitur baru.
3. **Cicilan & tabungan** — karena begitulah jamaah membayar, dan tanpanya
   TawafiqHub tidak bisa dipakai banyak travel sama sekali.
4. **CRM leads + alumni** — pendapatan berikutnya.
5. **Momen + kabar untuk keluarga** — kehangatan, tanpa pelacakan.
6. Sisanya menyusul; tidak ada yang struktural.

Dan satu hal yang tidak ada di daftar mana pun tetapi mengalahkan semuanya
dalam demo: **desain**. Meeqot tidak punya backend dan tetap terlihat seperti
produk yang lebih matang. Itu pekerjaan Codex, dan nilainya tidak lebih rendah
dari daftar di atas.
