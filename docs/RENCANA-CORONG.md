# Rencana: Corong Pengunjung

Dari orang membuka halaman sampai jadi jamaah atau jadi tenant — dan di mana
mereka berhenti.

Ditulis 2 September 2026. Antre setelah pekerjaan panel SaaS yang sedang
berjalan; lihat [TUGAS-CORONG.md](TUGAS-CORONG.md).

---

## 1. Kenapa ini ada

Hari ini **tidak ada satu pun pelacakan pengunjung** di seluruh proyek. Tidak
ada Google Analytics, tidak ada Plausible, tidak ada apa pun. Berapa orang
membuka storefront sebuah travel hari ini, tidak ada yang tahu.

Dasar corongnya sudah ada — `crm_leads`, `pilgrim_registrations`,
`season_waitlists` — tetapi semuanya baru tercatat **setelah orang mengisi nama
dan nomor telepon**. Yang datang lalu pergi tidak meninggalkan jejak apa pun.

Yang hilang justru bagian yang bisa diperbaiki: berapa yang datang, berapa yang
mulai mengisi, dan **di langkah mana mereka berhenti**.

Satu lubang konkret yang ikut tertambal: `crm_leads.source` dan
`crm_leads.campaign` hari ini **diketik manual oleh staf travel**. Dengan corong
ini, sebuah lead tahu sendiri dari mana ia datang.

## 2. Dua corong, satu tabel

| | **Corong Travel** | **Corong TawafiqHub** |
|---|---|---|
| Situs | `/p/[slug]` (storefront tenant) | `/` (situs kita sendiri) |
| Pengunjung | calon jamaah | travel yang menimbang berlangganan |
| Ujungnya | pendaftaran / waitlist | tenant baru |
| Dilihat oleh | travel di `/dashboard` **dan** pemilik platform di `/admin` | pemilik platform di `/admin` |
| `operator_id` | terisi | **NULL** |

Satu tabel melayani keduanya. `operator_id` NULL berarti situs platform, dan itu
juga yang menjadi **batas kepemilikan data**: sebuah travel hanya boleh melihat
barisnya sendiri, dan itu ditegakkan di lapisan repository seperti isolasi
cabang — bukan di handler.

Pemilik platform melihat keduanya, karena panel SaaS memang satu-satunya
permukaan yang menembus batas tenant. Setiap tabel lintas-tenant di sana diberi
label tetap **"Lintas seluruh travel"**, sesuai §2 DESAIN-PANEL-SAAS.

## 3. Kenapa tabel sendiri, bukan alat pihak ketiga

Bukan sikap soal data — hitungan biaya.

**Bahannya sudah dibayar.** Postgres jalan, worker asynq jalan, `middleware.ts`
sudah menyentuh setiap permintaan halaman (matcher-nya mencakup semuanya kecuali
aset statis), panel sudah punya tempat, dan backup terenkripsi sudah mencakup
database itu. Yang ditambahkan hanya satu tabel dan satu rollup.

**Self-hosted Plausible/Umami** gratis lisensinya tetapi menjadi satu layanan
lagi di VPS: RAM sendiri, database sendiri, backup sendiri, patch sendiri, dan
satu hal lagi yang bisa mati diam-diam. Itu biaya berulang, bukan sekali.

**Google Analytics** gratis dan lima menit, tetapi datanya keluar ke server
Google — bertentangan dengan hosting Indonesia dan prosedur UU PDP yang sudah
kita tulis.

Dan keduanya sama-sama tidak bisa menjawab satu pertanyaan yang paling
menentukan:

> *Dari 100 pengunjung dari Instagram, berapa yang mendaftar, dan berapa yang
> akhirnya membayar?*

Alat luar tahu 100-nya. Postgres tahu yang membayar. Tidak ada yang bisa
menyambungkan. Di satu database, itu satu `JOIN`.

## 4. Identitas pengunjung — dan kenapa tanpa banner persetujuan

```
visitor_hash = SHA256(salt ‖ tanggal ‖ IP ‖ user_agent)
```

- **Tidak bisa dibalik** menjadi IP. Yang tersimpan hanya hash, jadi tidak ada
  data pribadi baru yang masuk ke sistem.
- **Berganti tiap hari**, karena tanggal ikut di-hash. Seseorang tidak bisa
  dilacak lintas hari, bahkan oleh kita.
- **Tanpa cookie**, jadi tidak perlu banner persetujuan di storefront pelanggan
  — dan banner itu sendiri menurunkan konversi mereka.

Cukup untuk membedakan 50 pengunjung dari 50 kali muat halaman oleh satu orang.
Itu yang dibutuhkan; lebih dari itu tidak.

**Garamnya rahasia dan dirotasi.** Tanpa garam, siapa pun yang punya dump bisa
menghitung ulang hash dari daftar IP dan membalik anonimitasnya — ruang IPv4
cukup kecil untuk itu. Garam disimpan seperti kunci lain: environment variable,
tidak di database.

**IP mentah tidak pernah ditulis.** Bukan disamarkan, bukan dipotong — tidak
pernah masuk kolom mana pun. Itu yang membuat tabel ini tidak perlu masuk daftar
data pribadi di [INSIDEN-DATA-PRIBADI.md](INSIDEN-DATA-PRIBADI.md).

## 5. Bentuk data

```sql
-- Baris mentah. Umur pendek dan sengaja: yang bertahan adalah ringkasannya.
CREATE TABLE funnel_events (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  -- NULL = situs TawafiqHub. Terisi = storefront travel, dan inilah batas
  -- kepemilikan datanya.
  operator_id  UUID REFERENCES operators(id) ON DELETE CASCADE,
  visitor_hash TEXT NOT NULL,
  step         TEXT NOT NULL CHECK (step IN (
                 'LANDING','KATALOG','MULAI_ISI','KIRIM','SELESAI')),
  path         TEXT NOT NULL DEFAULT '',
  referrer_host TEXT NOT NULL DEFAULT '',
  utm_source   TEXT NOT NULL DEFAULT '',
  utm_campaign TEXT NOT NULL DEFAULT '',
  -- Diisi saat langkah SELESAI, supaya corong bisa disambungkan ke uang.
  entity_id    UUID,
  occurred_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX funnel_events_rollup_idx ON funnel_events (occurred_at);
CREATE INDEX funnel_events_operator_idx ON funnel_events (operator_id, occurred_at DESC);

-- Ringkasan harian. Layar membaca ini, tidak pernah menghitung ulang baris
-- mentah — pola yang sama dengan usage_counters (B2). Inilah yang membuat
-- layar tetap cepat setahun lagi.
CREATE TABLE funnel_daily (
  operator_id   UUID REFERENCES operators(id) ON DELETE CASCADE,
  day           DATE NOT NULL,
  step          TEXT NOT NULL,
  utm_source    TEXT NOT NULL DEFAULT '',
  visitors      INTEGER NOT NULL DEFAULT 0,  -- visitor_hash unik
  events        INTEGER NOT NULL DEFAULT 0,
  computed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (operator_id, day, step, utm_source)
);
```

`operator_id` di PK `funnel_daily` bisa NULL, dan Postgres memperlakukan NULL
sebagai tidak sama dengan NULL dalam indeks unik — jadi baris platform butuh
`COALESCE(operator_id, '00000000-...')` atau `NULLS NOT DISTINCT` (PG15+).
**Pilih satu dan tulis alasannya di migrasi**; ini persis jenis detail yang
kalau dilewatkan akan menggandakan baris ringkasan platform diam-diam.

## 6. Di mana dicatat

**Lima titik, bukan setiap halaman.** Mencatat semua itu mahal dan hampir tidak
menjawab apa pun; lima langkah ini yang membentuk corong.

| Langkah | Dicatat di | Arti |
|---|---|---|
| `LANDING` | middleware, saat `/p/[slug]` atau `/` dimuat | Ada yang datang |
| `KATALOG` | middleware, saat halaman paket/artikel dibuka | Cukup tertarik untuk melihat isi |
| `MULAI_ISI` | klien, saat kolom pertama form disentuh | Berniat, belum tentu jadi |
| `KIRIM` | server, saat RPC pendaftaran dipanggil | Mengirim |
| `SELESAI` | server, saat pendaftaran/tenant benar-benar tercipta | Jadi |

Beda `KIRIM` dan `SELESAI` itu penting: jarak antara keduanya adalah **kegagalan
validasi dan penolakan server** — orang yang berusaha mendaftar dan ditolak
sistem kita. Itu angka yang paling langsung bisa ditindak, dan corong yang
menggabungkan keduanya menyembunyikannya.

Rute yang relevan hari ini: `/p/[slug]`, `/p/[slug]/blog/...`,
`/register/[operatorId]/[seasonId]`, `/waitlist/[operatorId]/[seasonId]`,
`/apply/[operatorId]` untuk corong travel; `/`, `/sign-up`, `/onboarding` untuk
corong platform.

**Pencatatan tidak boleh menghambat halaman.** Middleware menulis lewat jalur
yang gagal diam-diam: kalau pencatatan bermasalah, pengunjung tetap dilayani.
Analitik yang bisa menjatuhkan storefront pelanggan lebih merugikan daripada
tidak punya analitik.

## 7. Bot

Tanpa penyaringan, angka ini akan bohong. Perayap dan pemindai bisa melebihi
lalu lintas manusia di situs kecil, dan corong yang menghitung mereka membuat
tingkat konversi terlihat jauh lebih buruk dari kenyataan.

Penyaringan minimum yang jujur: buang user-agent yang mengaku bot, dan buang
`visitor_hash` yang hanya pernah menyentuh `LANDING` di banyak path berbeda
dalam hitungan detik. Tidak sempurna, dan **layar harus mengatakan bahwa angka
ini sudah disaring** — bukan diam-diam.

## 8. Layar

### 8.1 Dashboard travel — `/dashboard/analytics`, bagian baru

Untuk pemilik travel, tentang dirinya sendiri.

- **Subjudul hidup**: `{n} pengunjung 30 hari · {m} mulai mengisi · {x} mendaftar`
- **Corong lima batang**, dengan persentase antar langkah, bukan hanya jumlah
- **Sumber**: pengunjung dan pendaftar per `utm_source`, urut menurut **pendaftar**
  bukan pengunjung — kanal yang mendatangkan 1.000 penonton dan nol pendaftar
  bukan kanal yang bagus
- **Catatan Metodologi**: apa yang dihitung, apa yang dibuang sebagai bot, dan
  bahwa pengunjung dihitung per hari sehingga orang yang sama di dua hari
  terhitung dua kali

### 8.2 Panel SaaS — tab **Corong** di `/admin`

Untuk pemilik platform, lintas seluruh tenant. **Ini yang membedakannya dari
sekadar analitik**, karena hanya di sini dua corong bisa dibaca berdampingan.

- **Corong platform**: pengunjung `/` → `/sign-up` → tenant aktif. Ini corong
  penjualan Anda sendiri, dan hari ini sama sekali tidak terlihat.
- **Corong agregat seluruh travel**: total pengunjung dan pendaftar di semua
  storefront. Angka yang bisa dikutip saat menjual.
- **Papan peringkat storefront**: travel dengan konversi tertinggi dan terendah.
  Yang terendah adalah daftar kerja — storefront yang ramai tapi tidak
  menghasilkan pendaftar biasanya salah pasang harga atau formulirnya rusak,
  dan itu bisa dibantu.
- **Storefront tanpa pengunjung sama sekali** masuk Pusat Tindakan: travel yang
  membayar untuk sesuatu yang tidak dipakai adalah travel yang akan berhenti
  berlangganan.

Menautkan ke `/admin/tenant/[id]` (B3) supaya corong satu tenant terbaca
bersama langganan dan pemakaiannya.

## 9. Retensi

- **Baris mentah 90 hari**, lalu dihapus oleh worker harian. Data mentah yang
  tidak pernah dihapus menjadi beban penyimpanan dan tanggung jawab sekaligus.
- **Ringkasan harian disimpan selamanya.** Ia agregat, tidak bisa dikembalikan
  ke individu, dan justru bagian ini yang berguna setahun kemudian.

Retensi ditulis di layar, bukan hanya di dokumen: pengguna yang melihat "90
hari" tahu kenapa data lama tidak bisa dibedah lebih dalam.

## 10. Yang sengaja tidak dibangun

- **Session replay dan heatmap.** Mahal dirawat, berat secara UU PDP, dan tidak
  mengubah satu keputusan pun.
- **Pelacakan per orang lintas hari.** Butuh cookie dan persetujuan, dan
  mengubah tabel ini dari agregat menjadi data pribadi.
- **Angka real-time.** Rollup harian sudah cukup untuk keputusan yang diambil
  dari layar ini. Real-time berarti query mahal setiap panel dibuka.
- **Menyimpan IP**, disamarkan atau tidak.

## 11. Yang membuat rancangan ini gagal

- **Menghitung bot sebagai manusia.** Tingkat konversi terlihat jauh lebih buruk
  dari kenyataan, dan keputusan diambil dari angka yang salah.
- **Pencatatan yang bisa menjatuhkan halaman.** Analitik tidak boleh menjadi
  jalur kritis storefront pelanggan.
- **Layar yang menghitung ulang baris mentah.** Cepat hari ini, tidak bisa
  dibuka setahun lagi — persis kesalahan yang dihindari `usage_counters`.
- **Garam yang tidak dirotasi atau ikut tersimpan di database.** Hash-nya
  menjadi bisa dibalik, dan tabel ini berubah menjadi data pribadi tanpa ada
  yang menyadari.
- **NULL `operator_id` di kunci unik tanpa dipikirkan.** Baris ringkasan
  platform menggandakan diri diam-diam.
- **Menggabungkan `KIRIM` dan `SELESAI`.** Menyembunyikan orang yang berusaha
  mendaftar dan ditolak sistem kita sendiri.
