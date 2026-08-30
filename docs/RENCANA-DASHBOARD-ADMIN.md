# Rencana: Dashboard Admin Travel

Dokumen kerja untuk melengkapi dashboard operator TawafiqHub (`/dashboard`)
sampai setara dan melampaui `admmeeqt.dul.co.id`, lalu menaikkan mutu UI/UX-nya
ke tingkat yang sama — dengan palet TawafiqHub sendiri.

Status: **rencana, belum ada yang dikerjakan.** Diperbarui 30 Agustus 2026.
Latar belakang dan analisa pesaing ada di [BENCHMARK-MEEQOT.md](BENCHMARK-MEEQOT.md).

---

## 1. Peta lengkap: mereka vs kita

22 rute di dashboard admin mereka, dipetakan ke 27 item navigasi kita. Tidak
ada yang dilewati.

| # | Modul Meeqot | Di TawafiqHub | Status |
|---|---|---|---|
| 1 | `/` Ringkasan | `/dashboard` | **ada** |
| 2 | `/jamaah` + `/jamaah/:id` + `/edit` | `/dashboard/pilgrims` | **ada** |
| 3 | `/hotel` | `/dashboard/accommodation` | **ada** |
| 4 | `/transportasi` | `/dashboard/transport` | **ada** |
| 5 | `/monitoring` | `/dashboard/monitoring` | **ada** |
| 6 | `/petugas` | `/dashboard/muttawwif` | **ada** |
| 7 | `/pencairan-komisi` | `/dashboard/agents` | **ada** |
| 8 | `/pengaturan` | `/dashboard/settings` | **ada** |
| 9 | `/paket` + `/paket/:id` | `/seasons` + `/products/harga` | **sebagian** |
| 10 | `/perjalanan` + `/:id` + `/baru` | `/kloter` + `/groups` | **sebagian** |
| 11 | `/pembayaran` | `/cashflow` + `/orders` | **sebagian** |
| 12 | `/laporan` | `/dashboard/reports` | **sebagian** |
| 13 | `/broadcast` | `/dashboard/communication` | **sebagian** |
| 14 | `/jamaah/baru` (wizard 4 langkah) | form biasa | **sebagian** |
| 15 | `/cabang` | — | **KOSONG** |
| 16 | `/crm` | — | **KOSONG** |
| 17 | `/inventaris` | — | **KOSONG** |
| 18 | `/template-manasik` | — | **KOSONG** |
| 19 | `/agenda` | — | **KOSONG** |
| 20 | `/layanan-tambahan` | — | **KOSONG** |
| 21 | `/kelebihan-bayar` | — | **KOSONG** |
| 22 | `/momen` | — | **KOSONG** |
| 23 | `/jamaah/:id/pindah-paket` | — | **KOSONG** |
| 24 | `/support` | — | **KOSONG** |
| 25 | `/market` (4 layar) | `/products` (beda tujuan) | **ditunda sengaja** |

### Yang kita punya dan mereka tidak

Jangan dikorbankan saat mengejar: **Musim** sebagai entitas kelas satu,
**Dokumen** tersendiri, **Pendaftaran**, **Asuransi & klaim**, **SOS**,
**Jamaah Terpisah**, **Jadwal Tim/Saya**, **Vendor**, **Pengiriman**,
**Refund & Saldo**, **Kloter**, **Waitlist**, **Langganan sendiri**, kebijakan
pembatalan per musim, checklist per musim, dan substitusi jamaah.

Hitungannya: mereka unggul 10 modul, kita unggul 14. Yang membuat mereka
*terasa* lebih lengkap bukan jumlahnya — melainkan bahwa 3 dari 10 milik mereka
adalah modul struktural (cabang, CRM, pembayaran bercicilan) yang menyentuh
seluruh produk, sementara keunggulan kita tersebar sebagai modul berdiri
sendiri.

---

## 2. Rincian setiap kekosongan

Diurutkan seperti akan dikerjakan. Setiap butir menyebut apa yang mereka punya
supaya tidak ada detail yang hilang.

### 2.1 Cabang — `/dashboard/cabang` 🔴 struktural

Yang mereka punya: daftar cabang (Pusat + 5 cabang), peran **Kepala Cabang**
yang tidak boleh menjabat dua cabang, target per cabang & capaian %, skor
gabungan omzet & jamaah, matriks produktivitas antar cabang, perbandingan &
tren, komisi cabang 2,5%, "Ajukan setoran ke pusat", cabang pendaftar tercatat
di setiap jamaah, agenda pusat vs agenda cabang, broadcast cabang, gudang per
cabang.

Kenapa duluan: ini satu-satunya yang **mengubah bentuk data**. Menyentuh setiap
query multi-tenant, setiap pemeriksaan RBAC, dan setiap laporan. Menundanya
membuatnya makin mahal setiap minggu.

Pekerjaan: migrasi `branches` (operator_id, nama, kota, target, PIC) →
`branch_id` nullable di `pilgrims`, `registrations`, `agents`, `orders` →
peran `BRANCH_HEAD` di `user_role` → penyaringan wajib di repository (bukan di
handler) → agregasi laporan per cabang → layar perbandingan cabang.

**Peringatan:** jangan menambahkan `branch_id` sebagai kolom yang bisa diabaikan.
Kalau penyaringannya tidak dipaksakan di lapisan repository, kepala cabang
Bandung akan bisa melihat jamaah Medan, dan itu pelanggaran UU PDP, bukan bug
tampilan.

### 2.2 Batas paket yang sungguhan 🔴 struktural

Bukan modul, tapi harus di sini. Hari ini `plan` hanya menggerbangi domain
kustom. Tidak ada batas jamaah, tidak ada batas cabang, tidak ada fitur
terkunci. Setiap fitur yang ditambahkan dari titik ini langsung bocor gratis ke
STARTER.

Pekerjaan: tabel `plan_limits` (plan, max_pilgrims, max_branches, flag fitur) →
`plan_overrides` per operator → satu fungsi `entitlement.Check()` yang dipanggil
service, bukan handler → layar di `/dashboard/langganan` yang menampilkan
pemakaian vs batas.

### 2.3 Pembayaran bercicilan — perluas `/dashboard/cashflow` 🔴

Yang mereka punya: skema Bayar penuh / DP 50% + pelunasan / Cicilan 6× / Cicilan
12×, "DP di muka, sisanya dibagi rata 6 bulan", Bonus Pelunasan Tunai, jadwal
cicilan per jamaah dengan nomor angsuran, "Cicilan lewat jatuh tempo", "15
tagihan cicilan jatuh tempo dalam 7 hari, total Rp 41.250.000", umur piutang
(aging), verifikasi transaksi manual, Kirim Kwitansi, Export Mutasi Pembayaran,
Kirim Reminder Pembayaran + pengingat massal, kanal bayar per transaksi.

Kenapa penting: ini cara mayoritas jamaah Indonesia membayar. Tanpanya banyak
travel tidak bisa memakai TawafiqHub sama sekali — ini bukan fitur pembeda,
ini syarat masuk.

**Idempotensi:** setiap angsuran butuh identifier stabil per percobaan, dan
pencocokan mutasi bank harus lewat kunci unik di database — bukan cek-lalu-tulis.

### 2.4 CRM Leads — `/dashboard/crm` 🟠

Yang mereka punya: pipeline **Lead Baru → Follow-up → Penawaran Terkirim →
Deal → Closing**, kanal masuk (website/IG/WhatsApp/walk-in/referral), "Follow-up
telat", Kontak Terakhir, Lead time, Kirim Penawaran Resmi, omzet closing,
konversi bulanan, **segmentasi alumni** untuk penawaran umroh berikutnya,
"Belum berstatus alumni", catatan per lead.

Kita punya waitlist dan registrations — itu hanya **dasar** corong. Alumni
adalah penjualan berikutnya yang paling murah bagi travel.

### 2.5 Gateway WhatsApp — perluas `/dashboard/communication` 🟠

Yang mereka punya: broadcast tersegmentasi dengan aturan penerima, penjadwalan,
**jam tenang** ("notifikasi dapat terkirim kapan saja, termasuk tengah malam"
sebagai peringatan), matriks notifikasi per peristiwa, template pesan dengan
placeholder, Kirim Uji Coba, hitung karakter ("ideal di bawah 700"), jam kirim
optimal, tingkat baca, biaya per kanal, pilih beberapa kanal sekaligus dengan
peringatan biaya.

Ide mereka yang lebih dalam dan layak ditiru: **WhatsApp sebagai antarmuka
cadangan** untuk jamaah lansia — "semua fitur penting juga berjalan lewat
WhatsApp" — bukan sekadar kanal notifikasi.

### 2.6 Perjalanan & rundown — perluas `/dashboard/kloter` 🟠

Yang mereka punya: Perjalanan sebagai entitas (kode `GRP-2608`, TL, penerbangan,
kursi terisi vs tersedia), **Rangkaian/Rundown per hari** yang bisa disusun,
itinerari, segmen hotel + segmen transportasi, alokasi bus per segmen dengan
peringatan kapasitas, **Manifes** dengan kolom kelengkapan dokumen dan ekspor,
"Manifes final dikirim ke muassasah & maskapai", Atribut Rombongan, Koreksi
Paket Rombongan, Export Jadwal Keberangkatan.

Kita punya kloter dan groups, tapi tidak punya penyusun rundown maupun ekspor
manifes.

### 2.7 Paket bertingkat — perluas `/dashboard/products/harga` 🟠

Yang mereka punya: harga per tier kamar (**Quad / Triple / Double**) dalam satu
paket, kuota kursi per keberangkatan, add-on kursi eksekutif dengan kuota kabin
bisnis/ekonomi, "vs harga dasar", diskon grup, jadwal keberangkatan per paket,
"Belum ada keberangkatan terjadwal untuk paket ini — bisa dilengkapi belakangan".

Kita sudah punya harga berlapis (`base_price_idr` + `product_markups`) — yang
kurang adalah **tier kamar** dan **kuota kursi**.

### 2.8 Inventaris & Purchase Order — `/dashboard/inventaris` 🟡

Yang mereka punya: gudang (pusat + per cabang) dengan **rak** (`Rak A1-03`),
stok minimum + peringatan "Di bawah stok minimum — segera buat purchase order",
Buat Purchase Order, rasio pemakaian ("1 koper/jamaah", "1 kit/5 jamaah"),
barang pinjaman yang kembali ke gudang (kursi roda, dicek rem & ban tiap
kembali), stok opname ("Cocok — hitung ulang 2 rak"), kapasitas rak ideal,
supplier per barang, lead time.

Kita punya pengiriman/fulfilment — yaitu barang **keluar**, bukan stok.

### 2.9 Manasik sebagai modul — `/dashboard/manasik` 🟡

Yang mereka punya: **Kurikulum** ("Umroh Reguler — 3 Sesi", "Umroh Plus/Haji —
2 Sesi"), template manasik, sesi bertanggal yang dihitung dari rangkaian
perjalanan, **absensi per sesi** dengan Buka/Tutup Sesi, Konfirmasi Kehadiran,
riwayat absensi, materi per sesi ("Rukun & Wajib Umroh", "Praktik Thawaf & Sa'i",
"Simulasi Bandara & Bagasi", "Manasik Akbar + Pelepasan").

Kita punya rituals dan checklist — bukan kurikulum dan kehadiran.

### 2.10 Agenda — `/dashboard/agenda` 🟡

Kalender kegiatan gabungan: manasik, keberangkatan, kepulangan, acara internal.
Agenda pusat vs agenda cabang. Kita punya `/schedule` (jadwal tugas staf) dan
`/my-schedule` — beda hal.

### 2.11 Layanan tambahan — `/dashboard/layanan-tambahan` 🟡

Add-on per jamaah: kursi eksekutif, handling VIP bandara, lounge & fast track,
badal umroh bersertifikat + video, asuransi plus, katering khusus, air zamzam
kargo. Dengan harga satuan dan penandaan "jamaah ber-add-on di grup ini".

### 2.12 Kelebihan bayar — `/dashboard/kelebihan-bayar` 🟡

Alur tersendiri saat jamaah membayar lebih: dikembalikan, atau dialihkan ke
angsuran berikutnya, atau ke layanan tambahan. Kita hanya punya refund.

### 2.13 Pindah paket — `/dashboard/pilgrims/[id]/pindah-paket` 🟡

Memindahkan jamaah ke paket lain dengan selisih harga terhitung. Kita punya
substitusi (mengganti **orang**), bukan perpindahan **paket**.

### 2.14 Momen — `/dashboard/momen` 🟡

Foto, video singkat, dan catatan kabar dari petugas lapangan untuk keluarga.
Retensi emosional; murah dibuat, besar dampaknya pada persepsi.

Batasi sesuai keputusan kita: **momen dan kabar boleh, posisi GPS mentah
tidak.** `FamilyStatus` kita sengaja menolak GPS, nomor kamar, dan paspor —
pertahankan, dan jadikan bahan jualan.

### 2.15 Wizard pendaftaran 4 langkah 🟡

"Empat langkah terpandu — identitas, paket, pembayaran, konfirmasi." Dengan
validasi silang antar langkah ("Isi nomor WhatsApp pada langkah Data Diri",
"Isi Agen/Perujuk terlebih dahulu di langkah Data Diri"). Kita punya form biasa.

### 2.16 Laporan laba rugi — perluas `/dashboard/reports` 🟡

Yang mereka punya: laba kotor, laba bersih, **laba per unit**, margin kotor,
target vs realisasi omzet, kontribusi %, omzet per agen, omzet per cabang,
5 periode terakhir, ekspor CSV & PDF.

Catatan teknis dari changelog mereka sendiri: worker ekspor PDF mereka mati
pada 1.240 jamaah karena heap 512 MB, solusinya streaming per 200 baris.
**Ekspor kita harus streaming sejak awal**, bukan setelah pelanggan pertama
gagal.

### 2.17 Support 🟢

Tiket ke platform dengan prioritas dan target waktu respons. Kecil, tapi
merupakan bukti bahwa ada yang bertanggung jawab.

---

## 3. Rencana UI/UX

### 3.1 Kenapa punya mereka terasa lebih maju

Setelah membaca seluruh teksnya, jawabannya **bukan** visual. Tiga hal:

**a. Setiap halaman menjelaskan dirinya.** Setiap judul punya subjudul yang
menyebut isinya: *"Arus kas masuk, verifikasi transaksi, umur piutang, dan
penagihan cicilan"*. Bukan sekadar "Pembayaran".

**b. Keadaan kosong yang mengajar, bukan yang menyerah.** Bukan "Belum ada
data", melainkan *"Rundown disusun per Perjalanan — buat Perjalanan pertama
untuk paket ini lewat tombol di atas"* dan *"Isi Rangkaian Perjalanan dulu di
atas supaya tanggal sesi bisa dihitung"*. Keadaan kosong mereka memberi tahu
langkah berikutnya.

**c. Sistem menyarankan tindakan.** *"8 jamaah GRP-2609 belum mengunggah paspor.
Kirim pengingat otomatis?"* · *"Di bawah stok minimum — segera buat purchase
order."* · *"15 tagihan cicilan jatuh tempo dalam 7 hari, total Rp 41.250.000."*

Ini yang harus ditiru lebih dulu, dan ini pekerjaan **penulisan**, bukan CSS.
Murah, dan efeknya paling besar.

Detail kecil yang juga layak: placeholder pencarian menyebutkan apa yang bisa
dicari — *"Cari nama, ID, kota, paket, PIC, atau tag…"* — bukan "Cari…".

### 3.2 Palet — TawafiqHub, bukan mereka

Sumber kebenaran: `apps/web/app/globals.css`. **Jangan memperkenalkan warna
baru.** Meeqot memakai biru-brand + amber; kita Emerald. Yang ditiru bentuknya,
bukan warnanya.

| Peran | Token |
|---|---|
| Primer / merek | `--tenant-brand` `#059669` (Emerald-600) |
| Primer gelap | `--tenant-secondary` `#07825f` |
| Aksen | `--color-teal-600` `#0d9488`, `--color-cyan-600` `#0891b2` |
| Latar halaman | `--tenant-light-bg` `#f7f8f5` / gelap `#07110d` |
| Permukaan | `--tenant-light-surface` `#ffffff` / gelap `#101b16` |
| Judul | `--tenant-light-heading` `#142019` |
| Teks | `--tenant-light-body` `#46544b` |
| Redup | `--tenant-light-muted` `#66736b` |
| Sukses | `--color-success-600` `#059669` |
| Bahaya | `--color-danger-600` `#e11d48` |
| Netral | skala `--color-warm-*` (Slate) |

**Kekurangan yang harus ditambal:** palet kita tidak punya warna **peringatan**
(kuning/amber). Meeqot memakainya di mana-mana untuk "perlu perhatian tapi belum
gagal" — stok menipis, cicilan mendekati jatuh tempo, dokumen belum lengkap.
Tanpa itu, semua peringatan terpaksa memakai merah bahaya, dan merah yang
dipakai untuk hal tidak gawat berhenti berarti gawat.

Usulan: tambahkan **satu** trio amber, sekali saja, di `globals.css` —
`--color-warning-600: #d97706`, `--color-warning-200: #fde68a`,
`--color-warning-50: #fffbeb`. Amber duduk baik di sebelah Emerald dan tidak
bertabrakan dengan merah rose.

Pola lencana status: tiga token sekaligus (latar `-50`, garis `-200`, teks
`-700`), persis seperti yang mereka lakukan. Konsisten di seluruh dashboard.

### 3.3 Bentuk yang dipinjam, diterjemahkan ke Emerald

- **Kartu KPI** di puncak setiap modul: angka besar, label, delta vs periode
  lalu, sparkline. Latar `--tenant-surface`, garis `--color-cream-300`.
- **Bilah kemajuan** untuk capaian target dengan transisi lembut (mereka pakai
  `duration-700` — cukup lambat untuk terbaca sebagai perubahan).
- **Penanda live** berdenyut untuk rombongan yang sedang berjalan; hijau
  Emerald untuk normal, rose untuk SOS.
- **Pola titik halus** sebagai latar header (`mq-dots` versi kita, Emerald 6%
  opacity) — hanya di header, jangan di seluruh halaman.
- **Overlay blur** untuk dialog, bukan gelap pekat.
- **Baris tabel yang bisa diklik** membuka panel samping, bukan pindah halaman —
  mempertahankan konteks daftar.

Tipografi: pertahankan Playfair untuk judul di app chrome sesuai keputusan yang
sudah ada di `globals.css`.

### 3.4 Navigasi setelah semua modul masuk

27 item hari ini akan jadi ±38. Sidebar datar akan pecah. Usulan pengelompokan:

```
Ringkasan
Penjualan     CRM Leads · Pendaftaran · Waitlist · Paket & Harga · Agen
Jamaah        Musim · Jamaah · Dokumen · Grup · Kloter · Asuransi
Perjalanan    Perjalanan & Rundown · Agenda · Manasik · Akomodasi ·
              Transportasi · Muttawwif · Jadwal Tim
Lapangan      Monitoring · SOS · Jamaah Terpisah · Momen
Keuangan      Pembayaran & Cicilan · Arus Kas · Pesanan · Refund & Saldo ·
              Kelebihan Bayar · Pencairan Komisi · Vendor
Logistik      Inventaris · Pengiriman · Layanan Tambahan
Laporan       Laporan & Analitik · Perbandingan Cabang
Pengaturan    Cabang · Pengaturan · Langganan · Support
```

Delapan grup, tidak ada yang lebih dari tujuh item. Item yang tidak termasuk
paket pelanggan tetap terlihat tapi terkunci — itu yang menjual kenaikan paket.

---

## 4. Urutan pengerjaan

**Gelombang 1 — struktural, dahulukan mutlak**
1. Cabang (2.1) — mengubah bentuk data
2. Batas & entitlement paket (2.2) — sebelum menambah fitur apa pun
3. Pembayaran bercicilan (2.3) — syarat masuk pasar

**Gelombang 2 — komersial**
4. CRM Leads (2.4) · 5. WhatsApp (2.5) · 6. Perjalanan & rundown (2.6) ·
7. Tier kamar & kuota kursi (2.7)

**Gelombang 3 — kelengkapan**
8. Inventaris & PO · 9. Manasik · 10. Agenda · 11. Layanan tambahan ·
12. Kelebihan bayar · 13. Pindah paket · 14. Momen · 15. Wizard ·
16. Laba rugi · 17. Support

**Berjalan sepanjang semua gelombang — dan boleh dimulai hari ini**
- Subjudul penjelas di setiap halaman
- Keadaan kosong yang mengajar
- Saran tindakan pada kartu KPI
- Placeholder pencarian yang menyebut isinya
- Trio warna peringatan + lencana status yang konsisten

Yang terakhir ini tidak bergantung pada apa pun, tidak menyentuh backend, dan
merupakan sebagian besar dari alasan produk mereka terasa lebih matang.

---

## 5. Catatan pelaksanaan

- Setiap modul baru: proto → migrasi → repository → service → handler → UI.
  Repository tidak boleh mengimpor service.
- Setiap operasi yang bisa diulang butuh kunci idempotensi yang dipaksakan
  **di database**, bukan cek-lalu-tulis.
- Cabang menyentuh RBAC: uji **dua arah** — kepala cabang Bandung harus bisa
  melihat jamaahnya, dan harus **tidak** bisa melihat jamaah Medan.
- Ekspor apa pun ditulis streaming sejak awal.
- `KYC_ENCRYPTION_KEY` kini wajib untuk membuat jamaah di lingkungan mana pun.
