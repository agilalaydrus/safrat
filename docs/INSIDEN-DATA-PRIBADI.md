# Prosedur insiden data pribadi (UU PDP)

UU 27/2022 Pasal 46 memberi **3×24 jam** sejak kegagalan perlindungan data
pribadi diketahui, untuk memberi tahu dua pihak:

1. **pemilik data** — jamaah yang datanya terdampak;
2. **Lembaga PDP**.

Isinya wajib memuat tiga hal: **data apa yang terungkap**, **kapan dan
bagaimana**, dan **apa yang sudah dan sedang dilakukan**.

Dokumen ini ada karena bagian tersulit bukan menulis suratnya. Bagian tersulit
adalah menjawab *"siapa dan apa"* dengan tepat, di bawah tekanan, dalam
hitungan jam — dan itu tidak bisa disiapkan saat kejadian.

---

## 1. Jam mulai berjalan saat diketahui, bukan saat selesai diselidiki

Tiga hari dihitung dari saat kamu **tahu** ada kegagalan, bukan dari saat kamu
selesai memastikan luasnya. Menunda notifikasi sampai penyelidikan tuntas
adalah cara paling umum melewati tenggat.

**Satu orang memutuskan.** Owner menyatakan "ini insiden" dan mencatat jamnya.
Tanpa itu, enam jam pertama habis untuk saling menunggu kepastian.

Catat waktunya di tempat yang tidak bisa hilang — pesan ke diri sendiri,
catatan bertanggal, apa pun yang bukan ingatan.

---

## 2. Enam jam pertama: hentikan dulu, baru selidiki

Memberi tahu orang sementara kebocoran masih berjalan hanya memperbesar
kerusakan.

| Kalau yang bocor | Lakukan segera |
|---|---|
| kredensial database (`.env.prod`, `DATABASE_URL`) | ganti password `safrat` dan `safrat_app`, restart, cabut sesi |
| `KYC_ENCRYPTION_KEY` | rotasi ke kunci baru — `DEPLOY.md` §12c. Kunci lama tetap disimpan sampai backup pra-rotasi habis retensinya |
| akun staf travel | `RevokeSessions` di panel admin, paksa ganti kata sandi |
| akun platform admin | `RevokePlatformAdmin`, lalu `RevokeSessions` |
| arsip backup | kunci privat backup **tidak** ada di VPS, jadi arsipnya tetap tersegel. Tetap rotasi kunci backup |
| `BANK_FEED_SECRET` | ganti, dan periksa `bank_mutations` untuk mutasi yang tidak kamu kenali |

---

## 3. Enam sampai empat puluh delapan jam: siapa dan apa

Ini bagian yang menentukan apakah notifikasimu memenuhi Pasal 46. *"Kemungkinan
ada data yang terekspos"* tidak memenuhinya — dan lebih buruk, membuat setiap
jamaah menganggap dirinya terdampak.

### Data pribadi apa yang disimpan

| Tabel | Berisi | Terenkripsi? |
|---|---|---|
| `pilgrims` | nama, nomor paspor, tanggal lahir, telepon, email, alamat | nomor paspor **ya** (AES-256-GCM), sisanya tidak |
| `pilgrim_registrations` | nama, nomor paspor, tanggal lahir, telepon, email, alamat | **tidak** — pendaftaran yang belum disetujui |
| `kyc_records` | nama, tanggal lahir, alamat, nomor identitas | nomor identitas **ya** (AES-256-GCM), sisanya tidak |
| `pilgrim_refund_payout_requests` | tujuan rekening/e-wallet | **ya**, kunci sama dengan KYC |
| `"user"` (Better Auth) | nama, email | tidak. Kata sandi di-hash scrypt |
| `agents` | nama, telepon | tidak |

**Konsekuensi yang harus dipahami sebelum insiden:** kalau yang bocor adalah
dump database, nomor identitas KYC, tujuan pencairan, dan **nomor paspor
jamaah** tetap tersegel. Yang terbaca: nama, tanggal lahir, telepon, email, dan
alamat.

Paspor sengaja disegel lebih dulu di antara kolom `pilgrims` yang lain karena ia
satu-satunya yang memungkinkan **penyamaran identitas**, bukan sekadar
ketidaknyamanan. Nama dan alamat memalukan kalau bocor; nomor paspor bisa
dipakai orang lain.

**Yang belum tersegel dan harus disebut jujur:** `pilgrim_registrations` —
pendaftaran yang belum disetujui — menyimpan paspor dalam teks polos. Tabel itu
punya jalur tulis terpisah dan belum ikut dipindahkan.

### `funnel_events` dan `funnel_daily`: kenapa keduanya bukan data pribadi

Sengaja tidak masuk tabel di atas, dan alasannya harus bertahan diperiksa —
kalau salah satu syarat di bawah berubah, statusnya ikut berubah dan tabel ini
naik ke daftar itu.

1. **Tidak ada alamat IP di kolom mana pun.** IP dipakai sekali di memori untuk
   menghitung penanda, lalu dibuang. Ini dijaga oleh uji, bukan oleh ingatan:
   `funnel_no_ip_integration_test.go` menolak kolom bertipe `inet`/`cidr` dan
   kolom yang namanya menyiratkan alamat, di kedua tabel.
2. **Tidak ada cookie dan tidak ada penyimpanan di peramban.** Tidak ada yang
   ditanam di perangkat pengunjung, jadi tidak ada persetujuan yang perlu
   diminta dan tidak ada pengenal yang bisa dibaca kembali.
3. **Penanda pengunjung adalah hash bergaram yang berganti tiap hari.**
   `SHA256(garam ‖ tanggal ‖ IP ‖ user agent)`. Tanpa garamnya, hash tidak bisa
   dikembalikan; dan karena tanggalnya ikut masuk, penanda orang yang sama
   berbeda esok hari — tidak ada seorang pun yang bisa diikuti lintas hari,
   termasuk oleh kami.
4. **Baris mentah dihapus setelah 90 hari** (`repository.FunnelRetentionDays`).
   Yang disimpan selamanya hanya ringkasan harian, yang berisi angka.
5. **Lokasi hanya sampai tingkat kota**, disimpan sebagai nama daerah, bukan
   koordinat dan bukan IP asalnya.

**Kalau garamnya bocor**, hash tetap tidak berisi apa pun yang menunjuk orang:
untuk mengembalikannya seseorang harus sudah memegang daftar IP yang dicurigai
**dan** user agent-nya **dan** tanggalnya, lalu mencocokkan satu per satu. Itu
mengonfirmasi tebakan yang sudah dia punya; ia tidak mengungkap identitas baru.
Walau begitu, `FUNNEL_SALT` tetap diperlakukan sebagai rahasia produksi seperti
kunci lain.

**Yang akan mengubah status ini:** menambah kolom IP (walau disamarkan),
memakai cookie atau `localStorage` untuk mengenali pengunjung, menyimpan
penanda yang tidak berganti harian, atau menautkan penanda ke akun. Salah satu
saja, dan tabel ini menjadi data pribadi dengan segala kewajibannya.

### Kueri yang sudah disiapkan

Ditulis sekarang, bukan saat panik. Menyusun kueri sambil menghitung mundur
adalah cara membuat kesalahan yang lalu masuk ke notifikasi resmi.

**Akun staf dibobol — apa yang dibacanya, dan milik siapa:**

```sql
SELECT created_at, action, entity_id, COALESCE(metadata->>'message', '')
FROM audit_logs
WHERE user_id = :user_id
  AND created_at BETWEEN :mulai AND :selesai
ORDER BY created_at;
```

> Catatannya ada di `metadata->>'message'`, **bukan** kolom `message`. Versi
> pertama dokumen ini menulis `message` dan gagal saat dijalankan. Itu jenis
> kesalahan yang baru ketahuan di jam ke-12 sebuah insiden nyata — alasan
> setiap kueri di sini sekarang sudah dijalankan sungguhan, bukan ditulis dari
> ingatan atas skema.

`pilgrim_read` menyebut satu jamaah di `entity_id`. `pilgrims_listed` menyebut
musim di `entity_id` dan jumlahnya di `message` — jamaah yang terjangkau adalah
seluruh isi musim itu:

```sql
SELECT id, full_name, email, phone
FROM pilgrims
WHERE season_id = :season_id AND operator_id = :operator_id;
```

`kyc_record_read` menyebut catatan KYC yang dibuka — ini yang paling sensitif
dan paling harus lengkap.

**Dump database bocor — seluruh jamaah travel terdampak:**

```sql
SELECT p.id, p.full_name, p.email, p.phone, o.name AS travel
FROM pilgrims p JOIN operators o ON o.id = p.operator_id
WHERE p.operator_id = :operator_id;
```

**Batas jejak, dan ini harus jujur disebut dalam laporan:** pencatatan
pembacaan bersifat *best effort* — kegagalan menulisnya tidak menggagalkan
pembacaan, karena menolak menampilkan data jamaah demi sebuah baris log adalah
pertukaran yang salah. Jejak juga baru ada sejak 30 Agustus 2026; pembacaan
sebelum itu tidak terekam.

---

## 4. Empat puluh delapan sampai tujuh puluh dua jam: kirim

Dua notifikasi, isi berbeda.

### Ke Lembaga PDP

Sebutkan, tanpa mengecilkan:

- kapan diketahui, dan kapan diperkirakan mulai;
- data apa, untuk berapa orang, dari travel mana;
- bagaimana terjadinya, sejauh yang diketahui — dan katakan bagian yang belum
  diketahui sebagai belum diketahui;
- apa yang sudah dilakukan (§2) dan apa yang sedang berjalan;
- apa yang akan dicegah agar tidak berulang.

### Ke jamaah terdampak

Bahasa yang bisa dimengerti, bukan bahasa hukum. Yang mereka perlu tahu:

- data mereka yang mana yang terekspos — sebutkan spesifik: *"nama, nomor
  paspor, dan alamat Anda"*, bukan *"data pribadi Anda"*;
- apa risikonya bagi mereka, secara konkret;
- apa yang sebaiknya mereka lakukan — mengganti kata sandi, mewaspadai
  telepon yang mengaku dari travel;
- ke mana bertanya. Nomor CS: **081283031003**.

Kirim ke **kontak yang ada di catatan mereka**, dan siapkan CS untuk gelombang
pertanyaan. Notifikasi tanpa jalur bertanya menghasilkan kepanikan.

---

## 5. Sesudahnya

Tulis kronologi selagi ingat: kapan diketahui, apa yang dilakukan jam berapa,
apa yang lambat, apa yang tidak bisa dijawab. Bagian "tidak bisa dijawab" itu
yang paling berharga — ia menunjuk persis lubang yang harus ditutup sebelum
insiden berikutnya.

## Yang masih membuat prosedur ini lebih sulit dari seharusnya

Dicatat supaya jujur, bukan disembunyikan:

- **`pilgrim_registrations` masih teks polos**, termasuk paspornya. Kolom
  `pilgrims` sudah dipindahkan; tabel pendaftaran belum.
- **Nama dan alamat di `pilgrims` masih teks polos.** Keduanya dicari dan
  diurutkan, jadi tidak bisa disegel dengan cara yang sama seperti paspor —
  paspor bisa karena ia hanya dicocokkan sama-persis.
- **`audit_logs` tumbuh tanpa batas dan tanpa retensi.** Ia sendiri berisi data
  yang sensitif — siapa membaca catatan siapa.
- **Jejak baca baru ada sejak 30 Agustus 2026.**
- **Alurnya belum pernah dilatih utuh.** Kueri-kuerinya sudah, prosedurnya
  belum: siapa memutuskan, berapa lama mengumpulkan bukti, apakah CS siap.
  Prosedur yang belum pernah dijalankan adalah asumsi, sama seperti backup yang
  belum pernah dipulihkan.

---

## Lampiran: latihan kueri

Jalankan sekali sebulan, dan setelah setiap perubahan skema. Butuh kurang dari
satu menit, dan menangkap persis kegagalan yang tidak boleh ditemukan saat
insiden:

```bash
./scripts/latihan-insiden.sh
```

Ia hanya membaca — tidak mengubah apa pun. Kalau ada kueri yang gagal, skema
sudah bergeser dan dokumen ini harus ikut diperbarui **sebelum** dibutuhkan.
