# Benchmark: Meeqot — rencana & maksud produknya

Ditinjau 30 Agustus 2026 dari sembilan aplikasi publik di `*.dul.co.id` dan
`meeqot.id`. Semua dibaca dari bundle JavaScript publik: tidak ada akun dipakai,
tidak ada endpoint diprobe.

Meeqot masih dummy — backend-nya belum ada, dan pemiliknya tahu itu. Karena itu
dokumen ini **tidak** menilai apa yang mereka jalankan, melainkan **apa yang
mereka rencanakan**. Sebuah prototipe selengkap ini adalah pernyataan niat yang
sangat rinci, dan itu justru benchmark yang berguna: ia memperlihatkan bentuk
akhir yang mereka tuju, tanpa harus menunggu mereka sampai.

## Tesis produk mereka

Meeqot tidak memposisikan diri sebagai aplikasi manajemen jamaah. Mereka menulis
**"Travel Umroh & Haji OS"** — sistem operasi, bukan modul. Konsekuensinya
terlihat di struktur: sembilan aplikasi, satu untuk setiap *peran manusia* dalam
perjalanan umroh, bukan satu untuk setiap *fungsi perangkat lunak*.

| Peran | Aplikasi |
|---|---|
| Pemilik / kantor pusat | `admmeeqt` |
| Kepala cabang & staf | `admcmeeqot` |
| Jamaah (sebelum berangkat) | `jamaahmeeqot` |
| Jamaah (di Tanah Suci) | `appsjamaah` |
| Keluarga di rumah | `appskeluarga` |
| Muthawwif / petugas lapangan | `appspetugas` |
| Pemilik platform | `devmeeqot` |

TawafiqHub memakai pembagian yang sama tetapi lebih rapat — satu Next.js dengan
`/dashboard`, `/leader`, `/pilgrim`, `/agent`, `/admin`. Itu keputusan yang lebih
baik dan tidak perlu diubah; yang perlu ditiru adalah **kelengkapan perannya**,
bukan jumlah aplikasinya. Dua peran yang belum kita punya sama sekali: **kepala
cabang** dan **keluarga**.

## Rencana komersial mereka — bagian yang paling matang

Ini bagian di mana mereka paling jauh di depan, dan anehnya paling murah untuk
dikejar karena tidak menyentuh domain haji sama sekali.

### Paket

| Tier | Harga | Batas | Yang dijanjikan |
|---|---|---|---|
| Starter | Rp 990.000 | 200 jamaah, 1 cabang | dashboard pusat, paket & jamaah, cash & cicilan, website subdomain, support email |
| Growth | Rp 2.490.000 | 1.000 jamaah, 5 cabang | + dashboard cabang, app mobile jamaah, live location & panic button, broadcast WhatsApp, QR check-in, support prioritas |
| Enterprise | Rp 5.990.000 | tanpa batas | + AI Assistant Muthawwif, Wallet & eVisa, custom domain & white-label, API akses penuh, account manager, SLA 99,9% |

TawafiqHub: STARTER Rp 589.000 · GROWTH Rp 789.000 · PRO Rp 2.489.000.

Dua hal yang perlu dibaca dari tabel ini:

1. **Paket mereka membatasi; paket kita tidak.** Di TawafiqHub, `plan` hanya
   menggerbangi satu hal — domain kustom (`operator_domain.go`). Tidak ada batas
   jumlah jamaah, tidak ada batas cabang, tidak ada fitur yang dikunci. Artinya
   tidak ada alasan teknis apa pun bagi pelanggan untuk naik dari STARTER ke PRO.
   **Tangga harga tanpa anak tangga.**
2. **PRO kita Rp 2.489.000, Growth mereka Rp 2.490.000** — selisih seribu rupiah,
   isi jauh berbeda. Ini bukan soal fitur saja, ini soal posisi.

### Mesin penagihan yang mereka rancang

Dari panel SaaS mereka, dan hampir semuanya belum ada di TawafiqHub:

| Konsep | Meeqot | TawafiqHub |
|---|---|---|
| Trial 14 hari, tanpa kartu | ya, seluruh fitur Growth | status `TRIALING` ada, alur belum |
| Kuota per paket | ya | — |
| **Override kuota per tenant** | ya | — |
| Add-on (storage 100 GB, API +1 juta call, seat) | ya | — |
| Meter pemakaian 30 hari (storage, API, WhatsApp) | ya | — |
| Dunning + auto-suspend H+30 | ya | `PAST_DUE` ada, tindak lanjut tidak |
| Grace period | ya | — |
| **Grandfathering saat harga naik** | ya | — |
| Void invoice + pulihkan | ya | — |
| Siklus tagihan massal | ya | invoice satuan |
| Downgrade setelah musim haji | ya, disebut eksplisit | — |
| Biaya pihak ketiga diteruskan apa adanya | ya, "tanpa markup" | — |

Satu prinsip mereka layak dicuri utuh, karena benar:

> *"Add-on lebih aman dipakai untuk kebutuhan satu tenant daripada mengubah
> batas paket untuk semua."*

Dan satu lagi yang kebetulan sama dengan keputusan kita: panel mereka **hanya
menyimpan prefix, sidik jari, dan metadata** kunci API — tidak pernah kuncinya.
Itu persis pola `key_fingerprint` kita. Bukti bahwa arah keamanan kita benar.

### Cara mereka merebut pelanggan

Ini yang paling perlu diperhatikan, karena bukan fitur:

- **Migrasi Excel/CSV gratis dikerjakan tim mereka, selesai 3–7 hari kerja.**
  Travel Indonesia hidup di Excel. Ini penghalang adopsi terbesar, dan mereka
  menghapusnya dengan jasa, bukan dengan kode. Ini kemungkinan besar lever
  akuisisi paling kuat di seluruh daftar ini.
- **Trial 14 hari yang dashboard-nya sudah terisi data contoh, bisa dikosongkan
  sekali klik.** Ini menjelaskan kenapa seluruh prototipe penuh data dummy: data
  contoh *adalah* strateginya. Travel bisa melihat produk terisi sebelum
  memasukkan satu jamaah pun.
- **"Tidak ada biaya per jamaah, tidak ada biaya tersembunyi."** Posisi harga
  langsung melawan pesaing yang menagih per jamaah.
- **Onboarding didampingi + pelatihan admin & kepala cabang**, disebut termasuk
  di semua paket.
- **Referral antar travel, komisi 15% untuk 3 bulan pertama.**
- **Roadmap digerakkan permintaan tenant**, dinyatakan terbuka di FAQ.

### Cara mereka menjawab keamanan

FAQ mereka: *"dienkripsi saat transit dan tersimpan (AES-256), di-hosting di
data center Indonesia sesuai UU PDP, backup otomatis harian, kontrol akses per
peran. Data Anda milik Anda — ekspor kapan saja."*

Itu klaim; kita sudah **punya** hampir seluruhnya secara nyata — AES-256-GCM
at-rest untuk KYC dan paspor, blind index, audit yang tak bisa ditulis ulang,
backup terenkripsi ke R2, prosedur insiden UU PDP yang sudah dilatih. Yang kita
belum punya adalah **ekspor data mandiri oleh operator** ("ekspor kapan saja"),
dan itu bukan sekadar jualan: hak portabilitas data adalah kewajiban UU PDP.

## Roadmap yang mereka nyatakan

Terbaca dari changelog, pengumuman, dan penanda rilis di bundle:

- **Meeqot 3.0** — AI Assistant Muthawwif aktif untuk semua tenant
- **Mobile Jamaah v2 + Mode Ibadah**
- **QR v2** (`/changelog/qr-v2`)
- **Rilis Q4 2026** — sesuatu yang belum dinamai, dijual sebagai akses awal
- **Early-bird Haji 1448H** — kuota manifes ekstra untuk Enterprise
- Keyring menyimpan 2 kunci terakhir dengan grace period 14 hari — *rotasi kunci
  dengan masa tumpang tindih*, hal yang sama yang sudah kita kerjakan

Integrasi yang mereka rencanakan: Midtrans, Xendit, Doku, Flip (pembayaran);
SES dengan Brevo sebagai cadangan otomatis (email); WhatsApp Business API;
Cloudflare R2/Registrar/Access/Turnstile; Kemenag RI; muassasah Saudi; OCR
paspor. Sisi AI: OpenAI, Anthropic, Gemini, DeepSeek, Qwen, Mistral, Llama
lokal via Ollama — dengan pagu anggaran bulanan dan hard-stop.

Marketplace mereka juga bukan generik. Isinya **inventaris kuota**: charter
Garuda CGK–JED dengan kuota kabin bisnis/ekonomi, bus Saudi 49 seat itinerary
9 hari, allotment hotel, asuransi Saudi + tasreh Raudhah, badal umroh
bersertifikat — dengan verifikasi legalitas penjual (izin usaha, kontrak
muassasah, lisensi muthawwif, kuota resmi Kemenag, sertifikat BNSP).

## Standar minimum yang saya usulkan, dan urutannya

Bukan urut kemiripan dengan Meeqot, tetapi urut **biaya menunda**.

1. **Hierarki cabang (pusat ↔ cabang).** Satu-satunya yang mengubah bentuk data:
   menyentuh setiap query multi-tenant, setiap pemeriksaan RBAC, setiap laporan.
   Makin lama ditunda makin mahal, dan ini jualan utama mereka di tier Growth.
2. **Paket yang benar-benar membatasi.** Hari ini tangga harga kita tidak punya
   anak tangga. Batas jumlah jamaah dan penguncian fitur per paket harus ada
   sebelum menambah fitur baru apa pun — kalau tidak, setiap fitur baru langsung
   bocor gratis ke STARTER.
3. **Gateway WhatsApp.** BroadcastService sudah ada; yang kurang kanalnya. Dampak
   terbesar per baris kode. Dan perhatikan ide mereka yang lebih dalam: WhatsApp
   sebagai **antarmuka cadangan** untuk jamaah lansia, bukan sekadar kanal
   notifikasi.
4. **Cicilan & tabungan umroh** dengan pengingat jatuh tempo dan umur piutang.
   Ini cara mayoritas jamaah Indonesia membayar; tanpanya banyak travel tidak
   bisa memakai TawafiqHub sama sekali.
5. **Migrasi dari Excel.** Jasa, bukan fitur — tetapi butuh importer yang benar.
   Penghalang adopsi terbesar, dan paling murah dihilangkan.
6. **Ekspor data mandiri.** Sekaligus kewajiban portabilitas UU PDP.
7. **CRM leads + segmentasi alumni.** Waitlist dan registrations kita hanya dasar
   corong; alumni adalah penjualan berikutnya yang paling murah.
8. **Kehangatan untuk keluarga** — momen, kabar, chat. Lihat catatan di bawah.
9. Sisanya modul, tidak struktural: manasik sebagai kurikulum + absensi,
   inventaris & PO, kelebihan bayar, pindah paket, layanan tambahan, insiden
   lapangan, peta langsung, QR check-in, dompet & uang saku, eVisa sebagai
   entitas.

## Yang berlebihan — jangan ditiru

- **Sembilan aplikasi.** Portal Jamaah (web) dan Aplikasi Jamaah (mobile) adalah
  permukaan yang sama dua kali. Satu PWA `/pilgrim` lebih murah dirawat dan tidak
  memecah perhatian pengguna. Tiru kelengkapan perannya, bukan jumlah binernya.
- **Konsol anggaran AI multi-provider, layar deploy, layar server, konfigurasi
  Turnstile sebagai UI.** Ini membangun PaaS di dalam SaaS. Operasional milik
  perkakas operasional.
- **AI Ustadz yang menjawab fikih.** Satu jawaban manasik keliru dari aplikasi
  bermerek travel menjadi masalah travel itu, bukan masalah model. Kalau AI
  dipakai, batasi pada meringkas data operator sendiri — bukan berfatwa.
- **Tasbih & kiblat.** Komoditas; puluhan aplikasi gratis melakukannya.
- **Marketplace B2B.** Produk kedua, bukan fitur, dan bersinggungan dengan
  keputusan yang sudah diambil bahwa jalur API ke supplier hanya milik
  TawafiqHub. Tunda sampai inti selesai.
- **Live location mentah ke keluarga.** Mereka melakukannya; kita sengaja tidak —
  `FamilyStatus` kita menolak GPS mentah, nomor kamar, dan paspor. Pertahankan,
  dan jadikan bahan jualan: *keluarga tahu kabar tanpa jamaah dilacak.* Yang
  perlu dikejar dari mereka adalah kehangatannya, bukan pengawasannya.

## Yang kita punya dan mereka baru janjikan

Perlu dicatat supaya tidak dikorbankan saat mengejar: backend Connect RPC dengan
36 service, Postgres 127 migrasi, worker asynq, uang yang benar-benar bergerak
(order, refund, payout agen, wallet, rekonsiliasi mutasi bank, batas belanja
harian), enkripsi at-rest yang sungguhan dengan blind index dan rotasi kunci,
audit yang tak bisa ditulis ulang peran aplikasi, storefront white-label dengan
domain sendiri dan TLS on-demand, kloter, substitusi jamaah, kebijakan
pembatalan, klaim asuransi, vendor, arus kas, dan sertifikat.

Meeqot menuliskan "AES-256-GCM" di layar. Kita menjalankannya.
