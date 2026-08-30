# Desain Dashboard Admin — spesifikasi tingkat layar

Pendamping [RENCANA-DASHBOARD-ADMIN.md](RENCANA-DASHBOARD-ADMIN.md). Kalau
dokumen itu menjawab *modul apa yang kurang*, dokumen ini menjawab **kenapa
layar mereka terasa lebih matang dan bagaimana menyamainya.**

Ditulis 30 Agustus 2026 dari pembacaan bundle `admmeeqt` — 7.050 string UI,
548 nama kolom data, dan seluruh pasangan judul–subjudulnya.

---

## 1. Delapan pola yang membuat layar mereka lebih baik

Ini bukan soal warna atau bayangan. Semuanya bisa kita kerjakan tanpa mengubah
satu piksel pun dari palet kita.

### Pola 1 — Subjudul membawa data hidup, bukan kalimat mati

Setiap judul bagian diikuti subjudul yang **menghitung isi layar saat itu**:

> **Hotel** · *"7 hotel terdaftar · 4 Makkah · 3 Madinah"*
> **Perjalanan & Manajemen Trip** · *"9 rombongan · 2 live di Tanah Suci · tim petugas, manifes, roomlist, dan tracking perjalanan"*
> **Data Jamaah** · *"412 jamaah terdaftar · 128 aktif dalam pipeline · Rp 3,4 M dana terkumpul"*
> **Jaringan Cabang** · *"6 cabang · 34 agen aktif · 1.240 jamaah terealisasi tahun ini"*

Kita hari ini menulis judul saja. Ini perubahan termurah dengan hasil terbesar:
pengguna tahu di mana ia berdiri sebelum membaca satu baris tabel pun.

### Pola 2 — "Pusat Tindakan" di setiap modul

Pola paling khas mereka. Setiap modul besar punya satu kartu berisi rekomendasi
otomatis yang **terurut menurut kepentingan**:

| Modul | Nama kartunya |
|---|---|
| Gudang | Pusat Tindakan Gudang |
| Keuangan | Pusat Tindakan Keuangan |
| CRM | Pusat Tindakan — *"7 lead butuh perhatian tim hari ini"* |
| Cabang | Pusat Aksi — *"5 rekomendasi otomatis"* |

Isinya konkret dan bisa langsung ditindak:

> *"Bantal leher di bawah minimum — Sisa 12/40 pcs · cukup 6 hari lagi"*
> *"PO-118 melewati ETA — Vendor Sinar Jaya · ETA 12 Agu · 200 unit tertahan"*
> *"GRP-2609 kekurangan 8 unit — Umroh Reguler · berangkat 3 Sep (H-14)"*
> *"Selisih opname Koper 24" — Sistem 140 vs fisik 137 pcs · oleh Doni"*

### Pola 3 — Peringatan menyebutkan nilainya dalam rupiah

Mereka tidak berhenti di "ada 23 tagihan telat". Mereka bilang berapa nilainya
dan apa artinya:

> *"23 tagihan lewat jatuh tempo — Nilai Rp 412 jt dari 19 jamaah. **Setiap Rp 100 jt tertagih menambah laba bersih ±Rp 18 jt.**"*
> *"7 transaksi belum diverifikasi — Senilai Rp 96 jt belum diakui dalam rekonsiliasi kas periode Juli."*
> *"15 cicilan jatuh tempo ≤ 7 hari — Potensi kas masuk Rp 41 jt pekan ini. Kirim pengingat otomatis agar tidak bergeser ke bucket menunggak."*

Kalimat terakhir itu penting: ia menjelaskan **akibat kalau diabaikan**.

### Pola 4 — Keadaan kosong yang menunjukkan jalan

Tidak pernah "Belum ada data". Selalu menyebut langkah berikutnya dan **di mana
langkah itu berada**:

> *"Belum ada segmen Bus di Rangkaian Perjalanan — Tambahkan segmen Bus di tab Rangkaian sebelum mengelola armada."*
> *"Belum ada kurikulum manasik — Terapkan template dari katalog Template Manasik, sesinya akan otomatis tercatat di menu Agenda."*
> *"Paket ini belum punya keberangkatan — Buat keberangkatan dulu dari halaman Detail Paket atau menu Keberangkatan, lalu jamaah ini bisa dimasukkan."*
> *"Tidak ada rombongan tujuan yang tersedia — Keberangkatan lain pada paket ini sudah berjalan, lewat tanggal, atau kursinya penuh."*

Perhatikan yang terakhir: ia menjelaskan **kenapa kosong**, bukan hanya bahwa
kosong.

### Pola 5 — Grafik menjelaskan sumbunya sendiri

> *"Peta Profitabilitas Paket — Sumbu X nilai kontrak · sumbu Y marjin kotor · ukuran gelembung = kursi terisi"*
> *"Peta sebaran jaringan — Ukuran titik = omzet · warna = skor kinerja · klik untuk memilih"*
> *"Posisi terhadap cabang lain — Skor kinerja gabungan (70% omzet · 30% jamaah)"*

Grafik yang tidak menjelaskan encoding-nya adalah dekorasi. Ini aturan yang
harus kita pegang untuk setiap chart.

### Pola 6 — Petunjuk interaksi ditulis, bukan ditebak

> *"Klik baris untuk membuka profil jamaah"* · *"Klik untuk memfilter daftar"*
> *"Klik sel untuk mengubah level: — → Lihat → Ubah → Penuh"*
> *"Klik kartu untuk mengubah status: belum ada → diproses → lengkap"*
> *"Klik kartu untuk menyorot posisinya di peta"*
> *"Scroll untuk zoom, geser untuk menjelajah · klik titik untuk detail, klik zona untuk memfilter"*

### Pola 7 — Catatan Metodologi

Di laporan keuangan mereka ada bagian **"Catatan Metodologi — Agar angka pada
laporan ini dibaca dengan konteks yang tepat"**. Menjelaskan bagaimana angka
dihitung.

Ini jarang ada di produk lokal, dan untuk laporan yang dipakai mengambil
keputusan uang, ia membedakan alat serius dari dasbor pajangan.

### Pola 8 — Pemeriksaan sebelum langkah tak bisa ditarik

Wizard broadcast mereka bernomor dengan tujuan — *"1 · Pilih Audiens"*,
*"2 · Kanal Pengiriman"*, *"3 · Susun Pesan"*, *"4 · Jadwal & Estimasi Biaya"* —
dan sebelum kirim ada **"Skor Kesiapan — Periksa sebelum pesan dilepas ke ribuan
kontak"** dengan daftar periksa: judul jelas, isi memadai, di bawah 700
karakter, minimal satu kanal, estimasi biaya.

Bandingkan dengan peringatan mereka yang jujur: *"Jam tenang nonaktif —
notifikasi dapat terkirim kapan saja, termasuk tengah malam."*

---

## 2. Sistem desain mereka, diterjemahkan ke palet kita

Kekonsistenan visual mereka datang dari satu hal: **`tone` sebagai prop kelas
satu**, dipakai 810 kali. Setiap lencana, kartu, statistik, dan peringatan
menerima satu token nada. Tidak ada warna yang ditulis lepas.

### Peta nada → token TawafiqHub

| `tone` mereka | Dipakai untuk | Token kita |
|---|---|---|
| `green` (115×) | berhasil, lunas, selesai, aktif | `--color-success-600` `#059669` |
| `blue` (102×) | informasi netral, terjadwal | `--color-cyan-600` `#0891b2` |
| `gold` (84×) | sorotan, unggulan, premium | `--tenant-brand` + `--color-teal-600` |
| `amber` (73×) | **perlu perhatian, belum gagal** | **`--color-warning-600` (perlu ditambah)** |
| `red` (65×) | gagal, telat, bahaya | `--color-danger-600` `#e11d48` |
| `gray` (44×) | nonaktif, kosong, arsip | `--color-warm-400` `#64748b` |
| `sky` (32×) | sekunder informatif | `--color-cyan-600` lebih terang |
| `navy` (30×) | penekanan tenang | `--color-warm-900` `#0f172a` |
| `purple` (15×) | kategori khusus | *tidak perlu — gabung ke teal* |

Sembilan nada mereka cukup dipadatkan jadi **enam** untuk kita: `success`,
`info`, `brand`, `warning`, `danger`, `neutral`. Lebih sedikit lebih baik,
asalkan konsisten.

**Yang wajib ditambahkan ke `globals.css`** — palet kita tidak punya warna
peringatan sama sekali:

```css
--color-warning-700: #b45309;
--color-warning-600: #d97706;
--color-warning-200: #fde68a;
--color-warning-50:  #fffbeb;
```

Tanpa ini setiap peringatan terpaksa memakai merah bahaya, dan merah yang
dipakai untuk hal tidak gawat berhenti berarti gawat.

Pola lencana: **tiga token sekaligus** — latar `-50`, garis `-200`, teks `-700`.
Persis seperti mereka, konsisten di seluruh dashboard.

### Varian tombol — dan pelajaran yang terkandung di angkanya

| Varian | Dipakai |
|---|---|
| `ghost` | 112× |
| `outline` | 106× |
| `secondary` | 34× |
| `gold` | 11× |
| `primary` | **9×** |
| `dangerSoft` | 5× |
| `danger` | 3× |

Antarmuka mereka **hampir seluruhnya tenang.** Primary hanya sembilan kali di
seluruh dashboard — satu tindakan utama per layar, tidak lebih. Ini kenapa layar
mereka terlihat rapi meski padat: tidak ada dua tombol yang berebut perhatian.

Aturan untuk kita: **satu `primary` per layar.** Sisanya `outline` atau `ghost`.

### Angka selalu membawa satuan

`unit` muncul sebagai prop tersendiri: `pcs`, `set`, `pax`, `seat`,
`room-night`, `pax/hari`, `paket`, `bus`, `trip`, `box`, `kamar`, `rombongan`,
`jamaah`, `cabang`, `GB`, `kit`. Tidak ada angka telanjang.

---

## 3. Anatomi halaman baku

Setiap layar dashboard mengikuti kerangka yang sama. Ini yang membuat produk
padat tetap terasa tenang.

```
┌───────────────────────────────────────────────────────────┐
│ Judul Modul                              [1 tombol primary]│
│ subjudul dengan angka hidup dari data saat ini             │
├───────────────────────────────────────────────────────────┤
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │
│ │ KPI +   │ │ KPI +   │ │ KPI +   │ │ KPI +   │  4 kartu   │
│ │ delta   │ │ delta   │ │ delta   │ │ delta   │  + satuan  │
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘            │
├───────────────────────────────────────────────────────────┤
│ ┌───────────────────────────┐ ┌─────────────────────────┐ │
│ │ Grafik utama              │ │ ⚡ Pusat Tindakan        │ │
│ │ subjudul menjelaskan sumbu│ │ n rekomendasi otomatis  │ │
│ │                           │ │ • butir + nilai rupiah  │ │
│ └───────────────────────────┘ └─────────────────────────┘ │
├───────────────────────────────────────────────────────────┤
│ [cari yang menyebut apa saja yang dicari] [filter] [ekspor]│
│ ┌───────────────────────────────────────────────────────┐ │
│ │ Tabel — "Klik baris untuk membuka …"                  │ │
│ │ baris membuka panel samping, bukan pindah halaman     │ │
│ └───────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────┘
```

Aturan yang menyertainya:

- **Satu tombol primary.** Selebihnya outline/ghost.
- **Subjudul wajib**, dan harus menghitung, bukan menjelaskan konsep.
- **Pusat Tindakan wajib** di setiap modul yang punya keadaan bisa memburuk.
  Kalau kosong, tampilkan keadaan bersih yang menenangkan — mereka menulis
  *"Antrean bersih"* dan *"Toko bersih 🎉"*.
- **Baris tabel membuka panel samping**, mempertahankan konteks daftar.
- **Placeholder pencarian menyebut isinya**: *"Cari nama, ID, kota, paket, PIC,
  atau tag…"* — bukan "Cari…".
- **Setiap grafik menjelaskan sumbunya** di subjudul.

---

## 4. Spesifikasi per modul

Format tiap modul: subjudul hidup · KPI · grafik · Pusat Tindakan · tabel.

### 4.1 Ringkasan `/dashboard`

- **Subjudul**: `{n} jamaah aktif · {m} rombongan berjalan · {x} berangkat 30 hari ke depan`
- **KPI**: Jamaah Aktif · Pendapatan Bulan Ini · Keberangkatan Terdekat · Kesiapan Dokumen %
- **Grafik**: *Pendaftar vs Keberangkatan* (12 bulan) · *Pendapatan Bulanan* ·
  *Pipeline Jamaah* (sebaran per tahap)
- **Panel**: *Agenda Terdekat* (5 kegiatan berikutnya) · *Notifikasi Terbaru*
  (aktivitas sistem & lapangan)

### 4.2 Cabang `/dashboard/cabang` 🔴

- **Subjudul**: `{n} cabang · {m} agen aktif · {x} jamaah terealisasi tahun ini`
- **KPI**: Omzet Jaringan · Capaian Target % · Jamaah Terealisasi · Cabang di Bawah Target
- **Grafik**: *Papan Peringkat* (skor gabungan 70% omzet · 30% jamaah) ·
  *Peta sebaran* (ukuran titik = omzet, warna = skor) · *Sebaran wilayah*
- **Pusat Aksi**: cabang tertinggal target, setoran menunggu, kepala cabang kosong
- **Tabel**: Cabang · Kepala · Kota · Target vs Realisasi (bilah) · Jamaah ·
  Agen · Skor · Kolektibilitas — klik buka panel detail
- **Panel detail cabang**: pencapaian target · tren omzet & jamaah 12 bulan ·
  posisi terhadap cabang lain · pipeline jamaah aktif · kesiapan dokumen ·
  jatuh tempo terdekat · rekening cabang

### 4.3 Pembayaran & Cicilan `/dashboard/cashflow` 🔴

- **Subjudul**: `Arus kas masuk, verifikasi transaksi, umur piutang, dan penagihan cicilan`
- **KPI**: Kas Masuk Periode · Piutang Berjalan · Tunggakan · Kolektibilitas %
- **Grafik**: *Arus Kas Masuk* · *Umur Piutang (Aging)* — sebaran menurut jatuh
  tempo · *Kanal Pembayaran* · *Kolektibilitas per Cabang* · *Rencana Bayar &
  Ketepatan*
- **Pusat Tindakan Keuangan**, dengan nilai rupiah dan akibatnya:
  tagihan lewat jatuh tempo · transaksi belum diverifikasi · cicilan ≤ 7 hari
- **Tabel 1** *Tagihan Jatuh Tempo & Tunggakan* — 30 hari ke depan, urut paling mendesak
- **Tabel 2** *Buku Piutang per Jamaah* — sisa tagihan, progres, status risiko
- **Skema**: Bayar penuh · DP 50% + pelunasan · Cicilan 6× · Cicilan 12× ·
  Bonus Pelunasan Tunai

### 4.4 CRM Leads `/dashboard/crm` 🟠

- **Subjudul**: `{n} prospek aktif dari {m} kanal pemasaran · diperbarui {waktu}`
- **Tahapan**: `baru → kontak → penawaran → hot → closing` (+ `batal`)
- **KPI**: Aktif dalam Pipeline · Nilai Pipeline · Konversi Bulan Ini · Follow-up Telat
- **Grafik**: *Corong Konversi* · *Sumber Lead* (klik untuk memfilter papan) ·
  *Performa Kanal Pemasaran* (belanja iklan · CPL) · *Papan Peringkat PIC* ·
  *Minat Paket* · *Umur Lead Aktif* — *"semakin tua, semakin dingin"*
- **Pusat Tindakan**: `{n} lead butuh perhatian tim hari ini`
- **Data per lead**: stage · source · campaign · lastContact · leadTime ·
  nextAction · assignee · pax · nilai estimasi · catatan

### 4.5 Perjalanan & Rundown `/dashboard/kloter` 🟠

- **Subjudul**: `{n} rombongan · {m} live di Tanah Suci · tim petugas, manifes, roomlist, dan tracking`
- **Tab**: Rangkaian · Rundown · Manifes · Armada Bus · Roomlist Hotel ·
  Tim Petugas · Checklist · Absensi
- **Rangkaian**: *"Urut sesuai ditambahkan — mulai & akhiri dengan Transportasi"*
  — segmen transportasi & hotel bergantian
- **Armada Bus**: *"Assign jamaah ke bus, tervalidasi terhadap segmen Bus di tab
  Rangkaian"* — kosong berkata: tambahkan segmen Bus dulu
- **Roomlist**: dikelompokkan per kota/hotel
- **Kesiapan Dokumen**: rekap manifes, 6 dokumen wajib
- **Linimasa Operasional**: keberangkatan & kepulangan terdekat

### 4.6 Inventaris `/dashboard/inventaris` 🟡

- **Subjudul**: `{gudang} — stok koper, ihram, seragam, dan atribut rombongan · sinkron {waktu}`
- **KPI**: Nilai Persediaan · Item di Bawah Minimum · PO Berjalan · Perputaran Stok
- **Grafik**: *Arus Stok Gudang* (masuk vs keluar + nilai akhir bulan) ·
  *Radar Kesiapan Keberangkatan* (kebutuhan per kloter vs stok) ·
  *Ketahanan Stok* (berapa lama bertahan pada laju saat ini) ·
  *Perputaran Stok* · *Performa Vendor* (ketepatan kirim & nilai 12 bulan)
- **Pusat Tindakan Gudang**: di bawah minimum · PO lewat ETA · kloter kekurangan ·
  selisih opname · mepet lead time vendor · melebihi kapasitas rak
- **Data per item**: sku · stock · minStock · maxStock · unit · unitCost ·
  lastRestock · perJamaah + catatannya · moq · leadTime · vendorId · rak

### 4.7 Broadcast `/dashboard/communication` 🟠

Wizard empat langkah:
1. **Pilih Audiens** — *"Segmen dihitung langsung dari data jamaah terkini"*
2. **Kanal Pengiriman** — *"Jangkauan dihitung dari ketersediaan kontak tiap jamaah"*
3. **Susun Pesan** — *"Gunakan variabel agar pesan terasa personal"* + Pratinjau Langsung
4. **Jadwal & Estimasi Biaya** — *"Atur waktu kirim dan tinjau pemakaian kredit"*

- **Skor Kesiapan** sebelum kirim
- **Analitik**: Tren Pengiriman & Tingkat Baca · Corong Interaksi ·
  **Peta Jam Emas** (tingkat baca per hari & jam) · Performa per Kanal ·
  Broadcast Berperforma Terbaik
- **Metrik**: targeted · delivered · read · clicked · replied · openRate · clickRate

### 4.8 Laporan `/dashboard/reports` 🟡

- **Subjudul**: `Konsolidasi {n} cabang — periode {x} · data per {tanggal}`
- **Indikator Kunci** terhadap target & portofolio
- **Insight Otomatis** — *"Dibaca dari data periode aktif — ikut berubah saat periode diganti"*
- *Peta Profitabilitas Paket* — X nilai kontrak · Y marjin kotor · ukuran = kursi terisi
- *Kontribusi Laba per Paket* · *Kinerja Keuangan Cabang* · *Arus Kas Masuk Mingguan*
- **Pusat Tindakan Keuangan**
- **Catatan Metodologi**
- Ekspor CSV & PDF — **streaming sejak awal**

### 4.9 Pengaturan `/dashboard/settings` 🟡

Jauh lebih dalam dari milik kita. Bagian-bagiannya:

- **Identitas Perusahaan** — *"Dipakai pada invoice, e-ticket, kontrak jamaah, dan aplikasi mobile"*
- **Dokumen Legal & Perizinan** — *"Masa berlaku dipantau otomatis — pengingat H-90"*
- **Cabang Terdaftar** · **Branding & Tampilan** · **Regional & Format**
- **Anggota Tim** + **Matriks Hak Akses** — *"Klik sel untuk mengubah level: — → Lihat → Ubah → Penuh"*
- **Template pesan** + Pratinjau Jamaah + Performa Template (90 hari)
- **Kunci API & Webhook** · **Status Layanan** (24 jam terakhir)
- **Matriks Notifikasi** — kanal per event · **Jadwal Pengingat Cicilan** ·
  **Jam kirim otomatis** · **Jam Tenang** · **Aturan Eskalasi**
- **Kebijakan Keamanan**: 2FA wajib · pembatasan IP · satu perangkat per akun ·
  keluar otomatis · kebijakan kata sandi · pencadangan harian · retensi log audit
- **Sesi Aktif** · **Log Audit** · **Pemakaian Kuota** · **Riwayat Tagihan** · **Add-on**

Catatan: bagian keamanan mereka sebagian besar **klaim di layar**. Kita sudah
menjalankan pencadangan terenkripsi, retensi audit 24 bulan, dan audit yang tak
bisa ditulis ulang. Yang kita belum punya: 2FA wajib per operator, pembatasan
IP, satu perangkat per akun, dan **layar** yang menunjukkan semua itu.

### 4.10 Wizard Pendaftaran `/dashboard/pilgrims/baru` 🟡

*"Empat langkah terpandu — identitas, paket, pembayaran, konfirmasi."*

1. **Data Diri** — *"Tulis persis seperti yang tertera di KTP & paspor"*
2. **Paket & Kamar** · 3. **Pembayaran** (skema & simulasi) · 4. **Konfirmasi**

- **Kelengkapan Berkas**: *"Klik kartu untuk mengubah status: belum ada →
  diproses → lengkap"*
- Dokumen wajib: paspor · vaksin meningitis · foto biometrik · buku nikah · KTP · KK
- Validasi silang antar langkah, disebut jelas: *"Isi nomor WhatsApp pada langkah Data Diri"*

### 4.11 Profil Jamaah `/dashboard/pilgrims/[id]`

- **Tahapan Perjalanan Jamaah** — pipeline 8 tahap, pendaftaran → alumni
- **Paket yang Dimiliki** — *"{n} paket terdaftar atas NIK yang sama"*
- **Kelengkapan Dokumen** — 6 dokumen wajib
- **Kelebihan Bayar** — *"Muncul dari reprice (Pindah Paket/Kursi Eksekutif) yang lebih murah dari yang sudah dibayar"*
- Aksi: Edit · Pindah Paket · Kirim Kwitansi · Kirim Pengingat

---

## 5. Komponen yang perlu dibangun

Sebelum modul apa pun. Sekali dibuat, dipakai seluruh dashboard.

| Komponen | Isi |
|---|---|
| `PageHeader` | judul + subjudul hidup + satu aksi primary |
| `StatCard` | nilai, satuan, label, delta, sparkline, `tone` |
| `ActionCenter` | daftar rekomendasi + dampak rupiah + keadaan bersih |
| `Badge` | `tone` → trio `-50/-200/-700` |
| `EmptyState` | judul, sebab, langkah berikutnya, tautan ke tempatnya |
| `DataTable` | pencarian deskriptif, filter, ekspor, klik baris → panel |
| `DetailDrawer` | panel samping yang mempertahankan konteks daftar |
| `Wizard` | langkah bernomor + skor kesiapan + validasi silang |
| `ChartFrame` | judul + subjudul penjelas sumbu — **wajib** |
| `ProgressBar` | capaian target, transisi ±700 ms |
| `MethodologyNote` | catatan cara angka dihitung |

---

## 6. Urutan

**Tahap 0 — fondasi (tidak menyentuh backend, bisa mulai hari ini)**
1. Tambahkan trio warna `warning` ke `globals.css`
2. Bangun 11 komponen di atas
3. Terapkan ke 27 layar yang sudah ada: subjudul hidup, keadaan kosong yang
   mengajar, placeholder pencarian deskriptif, satu primary per layar

Tahap 0 saja sudah menutup sebagian besar jarak yang terasa, tanpa satu pun
modul baru.

**Tahap 1** Pusat Tindakan di modul yang datanya sudah ada — jamaah, dokumen,
arus kas, monitoring.

**Tahap 2 dan seterusnya** mengikuti gelombang di
[RENCANA-DASHBOARD-ADMIN.md](RENCANA-DASHBOARD-ADMIN.md).

---

## 7. Yang tidak ditiru

- **Sembilan nada warna.** Cukup enam. Lebih sedikit, lebih konsisten.
- **Emoji di teks sistem** (*"Toko bersih 🎉"*) — tidak cocok dengan nada
  TawafiqHub.
- **Kepadatan berlebih di layar Market.** Modul itu ditunda; jangan meniru
  kerapatannya ke modul lain.
- **Klaim keamanan yang hanya tampil di layar.** Kita menampilkan yang benar-
  benar berjalan, bukan yang ingin berjalan.
