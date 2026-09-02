# Desain Panel SaaS (`/admin`) — spesifikasi tingkat layar

Pendamping [RENCANA-PANEL-SAAS.md](RENCANA-PANEL-SAAS.md), yang menjawab *apa*
dan *kenapa*. Dokumen ini menjawab **bagaimana tiap layar bekerja** — kolom,
keadaan, alur, dan bentuk datanya.

Sistem visualnya sama persis dengan [DESAIN-DASHBOARD-TRAVEL.md](DESAIN-DASHBOARD-TRAVEL.md).
Panel ini tidak punya bahasa visual sendiri.

---

## 1. Yang sudah diberikan skema — jangan dirancang ulang

Membaca skema lebih dulu menghemat separuh rancangan ini.

**`subscriptions`** — satu per operator (`UNIQUE`), dan komentarnya menyebut
alasannya: dua langganan membuat *"apakah operator ini lunas?"* ambigu, padahal
itu satu-satunya pertanyaan yang tabel ini ada untuk menjawabnya.

> **Akses diberikan oleh waktu, bukan status.** `access_until > NOW()` yang
> menentukan, sehingga status basi tidak pernah bisa membagikan akses gratis.

Ini menentukan rancangan penangguhan (§5): **menangguhkan bukan mengubah status,
melainkan berhenti memperpanjang `access_until`** — ditambah satu penanda
eksplisit untuk membedakan "belum bayar" dari "dibekukan sengaja".

**`subscription_invoices`** — sudah punya `period_start/end`, `due_at`,
`base_amount_idr` vs `amount_idr` (yang kedua membawa sufiks unik untuk
pencocokan transfer), `external_id`, `checkout_url`, dan:

```sql
CREATE UNIQUE INDEX subscription_invoices_transfer_amount_idx
  ON subscription_invoices (amount_idr)
  WHERE status = 'PENDING' AND channel = 'BANK_TRANSFER';
```

Dua transfer tertunda dengan nominal sama akan membuat mutasi masuk mustahil
diatribusikan, dan uang masuk ke travel yang salah. Database menolak keadaan itu
ada. **Siklus tagihan massal (§4.2) harus tunduk pada indeks ini** — menerbitkan
40 invoice sekaligus berarti 40 nominal unik, dan bila kolam sufiks habis, yang
gagal harus dilaporkan per baris, bukan membatalkan seluruh siklus.

**`plan_limits` / `plan_overrides`** — sudah ada, lengkap dengan `note`.
`NULL` berarti tanpa batas; nol adalah batas sungguhan, bukan sinonim tanpa
batas. Trigger `assert_operator_entitlement` memakai
`pg_advisory_xact_lock` sehingga dua penulisan serentak tidak bisa sama-sama
lolos `COUNT(*)` yang basi.

**Yang belum ada dan perlu ditambah:** `expires_at` pada `plan_overrides`
(kelonggaran sementara harus punya akhir), `usage_counters`, tabel jejak
dunning, penanda penangguhan, dan tabel sesi impersonasi.

---

## 2. Navigasi

Urutannya adalah urutan §3 RENCANA: uang → tenant → pertumbuhan → kesehatan.

```
Ringkasan            ← Pusat Tindakan, halaman muka
Uang        Transfer · Transaksi · Langganan · Harga Modal
Pelanggan   Tenant · Paket & Kuota · Pemakaian · Identitas (KYC)
Platform    Supplier & Routing · Katalog · Kesehatan
Tumbuh      Analitik · Pengumuman
Akses       Akun · Audit
```

Enam grup, tidak ada yang lebih dari empat item. Delapan permukaan yang sudah
ada masuk ke sini tanpa ditulis ulang — hanya berpindah tempat.

**Setiap tabel lintas-tenant diberi label tetap "Lintas seluruh tenant"** di
header. Ini bukan hiasan: di panel inilah satu-satunya tempat angka bukan milik
satu travel, dan salah baca di sini berujung pada keputusan uang yang salah.

---

## 3. Ringkasan — Pusat Tindakan

Satu-satunya layar tanpa tabel. Isinya hanya apa yang perlu dikerjakan hari ini.

**Subjudul:** `{n} tenant aktif · {m} trial · {x} menunggak · Rp {y} belum direkonsiliasi`

**KPI:** MRR · Tenant Aktif · Kas Belum Direkonsiliasi · Transaksi Menggantung

**Pusat Tindakan**, urut menurut seberapa mahal salah menunda:

| Butir | Dampak yang ditampilkan | Tujuan |
|---|---|---|
| Mutasi bank belum dicocokkan | `Rp {jumlah}` | Transfer |
| Transaksi `HELD` | `Rp {jumlah}` · `{n} tenant` | Transaksi |
| Fulfilment menggantung > 24 jam | `{n} pesanan` | Transaksi |
| Invoice langganan lewat jatuh tempo | `Rp {jumlah}` · `{n} tenant` | Langganan |
| Produk terjual tanpa harga modal | `{n} produk` | Harga Modal |
| Produk tanpa routing supplier | `{n} produk` | Supplier & Routing |
| Tenant ≥ 100% kuota | `{n} tenant` | Pemakaian |
| KYC menunggu > 48 jam | `{n} berkas` | Identitas |

Tiap butir menyebut akibat kalau diabaikan, bukan hanya jumlahnya. Contoh nada:

> **Rp 41.250.000 mutasi bank belum dicocokkan** — Sembilan travel sudah
> mentransfer dan langganannya belum diperpanjang. Setiap hari tertunda adalah
> hari mereka membayar tanpa mendapat akses.

> **6 produk terjual tanpa harga modal** — Margin tidak diketahui dan tidak ada
> lantai harga di bawahnya. Satu penjualan rugi tidak akan terlihat sampai
> rekonsiliasi bulan depan.

Keadaan bersih: **"Tidak ada yang tertunda"** — bukan kartu kosong.

---

## 4. Layar per layar

### 4.1 Paket & Kuota

**Subjudul:** `3 paket · {n} override aktif · perubahan berlaku ke seluruh tenant pada paket itu`

**Bagian 1 — Batas per paket.** Tiga baris (STARTER/GROWTH/PRO), tiap baris:
`max_pilgrims`, `max_branches`, dan flag fitur. Sel kosong berarti **tanpa
batas**, dan itu ditulis sebagai kata "Tanpa batas", bukan dibiarkan kosong —
nol adalah batas sungguhan dan tidak boleh terbaca sama.

**Bagian 2 — Override per tenant.** Kolom: Tenant · Paket · Batas jamaah ·
Batas cabang · Flag · **Alasan** · Berlaku sampai · Diubah oleh · Kapan.

Alasan **wajib**. Override tanpa alasan adalah utang yang tidak bisa ditagih
enam bulan kemudian.

**Pratinjau dampak — wajib sebelum simpan.** Menurunkan `max_pilgrims` GROWTH
dari 500 ke 300 membuka dialog:

> **7 tenant akan seketika melampaui batas baru**
> Al-Hijrah Travel (412) · Barokah Tour (388) · … *(daftar penuh, bukan jumlah saja)*
> Mereka tidak akan kehilangan data, tetapi **tidak bisa menambah jamaah baru**
> sampai turun di bawah 300 — kecuali dikunci di angka lamanya.
> `[ Kunci di angka lama ]` `[ Terapkan tetap ]` `[ Batal ]`

"Kunci di angka lama" menulis `plan_overrides` untuk masing-masing dengan alasan
otomatis *"grandfathered dari batas GROWTH 500, {tanggal}"*. Kenaikan harga atau
pengetatan kuota tidak boleh mengubah kondisi pelanggan lama tanpa keputusan
sadar.

**Proto:**
```proto
rpc ListPlanLimits(...) returns (...);
rpc PreviewPlanLimitChange(PreviewPlanLimitChangeRequest) returns (PreviewPlanLimitChangeResponse);
rpc SetPlanLimit(...) returns (...);          // four-eyes (§6.2)
rpc ListPlanOverrides(...) returns (...);
rpc SetPlanOverride(...) returns (...);       // note wajib, expires_at opsional
rpc DeletePlanOverride(...) returns (...);
```

`PreviewPlanLimitChangeResponse` mengembalikan `repeated AffectedTenant
{operator_id, name, current_usage}` — **nama, bukan jumlah**.

**Migrasi:** `ALTER TABLE plan_overrides ADD COLUMN expires_at TIMESTAMPTZ`
plus worker harian yang mencabut override kedaluwarsa dan mencatatnya di audit.
Kelonggaran yang tidak pernah berakhir bukan kelonggaran, itu paket baru yang
tidak pernah diberi nama.

### 4.2 Langganan

**Subjudul:** `{n} langganan aktif · {m} trial · {x} lewat jatuh tempo senilai Rp {y}`

**KPI:** MRR · Akan Jatuh Tempo 30 Hari · Lewat Jatuh Tempo · Trial Berakhir Pekan Ini

**Tabel:** Tenant · Paket · Status · `access_until` · Invoice terakhir ·
Tahap dunning · Nominal · Kanal.

Kolom **`access_until` ditampilkan apa adanya**, karena itulah yang benar-benar
menentukan akses. Status hanya label.

**Siklus tagihan massal.** Tiga langkah, dan langkah pertama tidak menulis
apa pun:

1. **Tinjau** — daftar invoice yang akan terbit: tenant, paket, periode,
   nominal. Total di bawah.
2. **Terbitkan** — satu tombol, satu transaksi per invoice, bukan satu untuk
   semua. Kegagalan sufiks nominal unik (§1) dilaporkan **per baris**;
   sisanya tetap terbit.
3. **Hasil** — berapa terbit, berapa gagal, dan kenapa masing-masing.

Kunci idempotensi: `(operator_id, period_start)` unik pada
`subscription_invoices`. Worker yang berjalan dua kali tidak boleh menagih dua
kali, dan cek-lalu-tulis akan meloloskan dua proses serentak.

### 4.3 Pemakaian

**Subjudul:** `Meter 30 hari · reset setiap tanggal 1 · terakhir dihitung {waktu}`

Tanggal reset disebut karena angka kuota tanpanya tidak bisa ditindak.
Waktu hitung disebut karena angkanya berasal dari worker harian, bukan
real-time — dan mengaku begitu lebih baik daripada terlihat langsung.

**Tabel:** Tenant · Jamaah `n/batas` · Cabang `n/batas` · Penyimpanan ·
Panggilan API · Pesan WhatsApp. Setiap sel bilah kemajuan dengan `tone`:
`success` < 80% · `warning` 80–99% · `danger` ≥ 100%.

**Teknis:** `usage_counters (operator_id, metric, period_start, value, computed_at)`,
PK `(operator_id, metric, period_start)`. Diisi worker harian. **Jangan**
dihitung ulang per permintaan — menghitung jamaah seluruh tenant setiap panel
dibuka akan menjadi query termahal di sistem ini.

### 4.4 Tenant `/admin/tenant/[id]`

Halaman yang hari ini tidak ada; tab Travel masih tabel datar.

**Subjudul:** `{paket} · {status} · akses sampai {tanggal} · bergabung {tanggal}`

**Enam bagian:** Langganan & riwayat tagihan · Pemakaian vs kuota (dengan
override yang berlaku dan alasannya) · Jamaah, cabang, dan agen · Transaksi &
transfer · Tim, status 2FA, dan domain · **Jejak audit tenant ini**.

**Aksi:** Ubah override · Tangguhkan (four-eyes) · Impersonate · Ekspor data
tenant.

Bagian audit ada di sini, bukan hanya di layar Audit global, karena pertanyaan
yang muncul saat insiden selalu berbentuk *"siapa menyentuh travel ini?"* —
bukan "apa yang terjadi kemarin".

### 4.5 Supplier & Routing

Menutup dua mesin tanpa pemicu.

**Tab Routing:** Produk · Supplier utama · Cadangan · Prioritas · Terakhir
diubah. **Produk tanpa routing ditampilkan paling atas sebagai antrean kerja**,
bukan disembunyikan di filter — itulah yang memicu respons *"Produk Belum di
Atur Routing"* ke operator.

**Tab Log:** waktu · supplier · produk · permintaan · respons · latensi ·
**aturan mana yang cocok**. Kolom terakhir itu alasan tab ini ada: saat
transaksi menggantung, yang ingin diketahui bukan "apa responsnya" tapi
"kenapa dibaca begitu".

Tautan dua arah: dari transaksi menggantung → log terkait, dan dari baris log →
transaksinya.

### 4.6 Analitik

MRR & pergerakannya · tenant aktif · konversi trial · churn · NRR.

**Catatan Metodologi wajib**, memuat dua kejujuran ini apa adanya:

> Komisi marketplace masuk ke pendapatan lain, **bukan MRR**. Mencampurnya
> membuat pertumbuhan terlihat lebih cepat dari sebenarnya.
> Skor risiko churn adalah **heuristik internal** — penanda prioritas, bukan
> vonis. Jangan dipakai untuk memutus hubungan dengan pelanggan.
> NRR di bawah 100% berarti ekspansi tidak menutup churn.

Grafik menjelaskan sumbunya. Angka yang tidak menyatakan batasnya akan dipakai
untuk keputusan yang tidak bisa ditopangnya.

---

## 5. Mesin keadaan

### 5.1 Daur hidup invoice

```
PENDING ──bayar──> PAID
   │
   ├──lewat due_at──> PENDING (dunning berjalan)
   ├──batalkan──────> CANCELLED   (void, jejak disimpan, TIDAK dihapus)
   └──kedaluwarsa───> EXPIRED     (sufiks nominal dilepas kembali ke kolam)
```

`EXPIRED` melepas nominal unik agar kolam sufiks tidak habis — sapuan itu sudah
ada (`ExpireOverdueInvoices`). **Void ≠ hapus.** Invoice salah terbit tetap
menjadi bagian catatan; menghapusnya membuat riwayat tagihan berlubang tepat di
tempat yang paling ingin diperiksa.

### 5.2 Dunning dan penangguhan

```
H+0   jatuh tempo lewat          → status PAST_DUE
H+1   pengingat pertama          → email + notifikasi in-app
H+7   pengingat kedua            → + PIC ditandai
H+14  peringatan penangguhan     → menyebut tanggal pastinya
H+21  ditangguhkan               → akses diputus, DATA UTUH
```

**Penangguhan bekerja lewat waktu, bukan status**, mengikuti rancangan skema:
`access_until` berhenti diperpanjang, ditambah kolom `suspended_at` untuk
membedakan "belum bayar" dari "dibekukan sengaja". Interceptor yang sudah ada
tidak perlu diubah.

Pembayaran di titik mana pun **membatalkan seluruh rangkaian** dan memperpanjang
`access_until` — termasuk sesudah H+21. Memulihkan tidak boleh butuh campur
tangan manual; travel yang sudah membayar tidak boleh menunggu kita bangun.

Setiap tahap butuh kunci idempotensi `(invoice_id, stage)` unik di database.
Worker yang berjalan dua kali tidak boleh mengirim dua peringatan — dan pelanggan
yang menerima dua kali tidak akan percaya yang ketiga.

Layarnya menampilkan tahap dunning tiap tenant sebagai lencana, dan halaman
tenant menampilkan **tanggal penangguhan yang dijadwalkan**, bukan hanya "H+21".

---

## 6. Alur keamanan

### 6.1 Impersonate

```
Pilih tenant → tulis alasan (wajib) → sesi impersonasi dimulai
   ├─ read-only, selalu
   ├─ berbatas 30 menit, mundur terlihat di layar
   ├─ spanduk tetap di atas: "Melihat sebagai {tenant} · read-only · sisa {mm:ss}"
   └─ tercatat: siapa, tenant, IP, alasan, mulai, selesai, jumlah permintaan
```

**Tidak ada mode tulis.** Kalau perlu mengubah sesuatu untuk pelanggan, itu
dilakukan lewat RPC platform yang punya jejaknya sendiri, bukan menyamar sebagai
mereka. Menulis atas nama orang lain menghapus perbedaan antara "kami
memperbaikinya" dan "mereka melakukannya" — dan perbedaan itu yang ditanya saat
sengketa.

**Tabel:** `impersonation_sessions (id, admin_user_id, operator_id, reason,
ip, started_at, ended_at, request_count)`.

### 6.2 Four-eyes

Berlaku untuk: menangguhkan tenant, menghapus tenant, mengubah `plan_limits`
global, mengubah rekening settlement.

Hari ini admin platform hanya satu (*"panel ADMIN ini hanya boleh diakses oleh
saya saja"*), jadi bentuknya: **ketik ulang nama tenant** untuk mengonfirmasi,
plus alasan wajib.

**Rancang jalurnya untuk dua orang sejak awal.** Tabel
`privileged_actions (id, kind, payload, requested_by, requested_at, approved_by,
approved_at, executed_at, reason)` ada sejak hari pertama, dengan
`approved_by = requested_by` selama masih satu admin. Saat admin kedua
ditambahkan, yang berubah hanya satu aturan — bukan seluruh alur.

### 6.3 Audit pembacaan

Setiap pembacaan data pribadi tenant dari panel ini masuk `audit_logs`, bukan
hanya perubahan. Membuka KYC seorang jamaah adalah pemrosesan data pribadi, dan
"siapa membaca data siapa" adalah pertanyaan pertama saat insiden UU PDP.

`audit_logs` sudah tidak bisa ditulis ulang oleh peran aplikasi (migrasi 125)
dengan retensi 24 bulan (migrasi 126), jadi yang perlu ditambah hanya
pemanggilannya. Perbarui tabel inventaris di
[INSIDEN-DATA-PRIBADI.md](INSIDEN-DATA-PRIBADI.md) setelahnya.

---

## 7. Siklus hidup tenant

Bagian yang belum pernah dirancang di mana pun, padahal panel ini yang
menjalankannya.

```
   dibuat ──> TRIALING ──bayar──> ACTIVE ──lewat tempo──> PAST_DUE
      │           │                  │                        │
      │           │                  │                    H+21 │
      │      trial habis             │                         ▼
      │           ▼                  │                    DITANGGUHKAN
      │      KEDALUWARSA             │                         │
      │           │                  ▼                    bayar│
      │           └──────────> DIBATALKAN <───────────────────┘
      │                             │
      └─────────────────────────────┴──90 hari──> DIHAPUS
```

### 7.1 Trial — tiga hari terlalu pendek

`TrialDays = 3` (`repository/subscription.go:20`). Untuk travel umroh itu
hampir tidak berguna: mereka perlu mengimpor data dari Excel, melatih admin,
dan mencoba satu pendaftaran sungguhan. Tiga hari kerja bisa jatuh di akhir
pekan.

Pembanding: Meeqot memberi **14 hari seluruh fitur Growth, tanpa kartu kredit,
dengan dashboard terisi data contoh yang bisa dikosongkan sekali klik.**

Ini keputusan komersial pemilik, bukan keputusan teknis, jadi rancangan ini
tidak mengubah angkanya — tetapi menuntut tiga hal:

- **`TrialDays` jadi setelan, bukan konstanta.** Baris di `plan_limits` atau
  tabel setelan platform, bisa diubah dari panel tanpa deploy.
- **Perpanjang trial per tenant** sebagai aksi panel, dengan alasan wajib.
  Prospek yang sedang serius mengevaluasi tidak boleh terkunci karena kalender.
- **Layar Langganan menampilkan "trial berakhir dalam n hari"** dan siapa saja
  yang berakhir pekan ini. Trial yang habis tanpa ada yang menyadari adalah
  pelanggan yang hilang tanpa pernah ditawari.

### 7.2 Provisioning

Operator lahir lewat `OperatorService.Create`, dan langganan trial dibuat
menyertainya. Panel tidak membuat tenant — pendaftaran mandiri yang membuatnya.
Yang panel lakukan adalah **melihat dan memperbaiki** apa yang lahir cacat:
slug bentrok, domain gagal verifikasi, trial yang perlu diperpanjang.

Antrean *"Tenant baru 7 hari terakhir"* di layar Tenant, dengan penanda
kelengkapan: sudah punya musim? sudah ada jamaah? sudah pernah login kedua kali?
Tenant yang mendaftar lalu tidak pernah kembali adalah sinyal paling awal
tentang onboarding yang rusak.

### 7.3 Pembatalan dan penghapusan

**Pembatalan bukan penghapusan.** `cancelled_at` diisi, akses berhenti pada
`access_until` yang berjalan — pelanggan tetap memakai sisa periode yang sudah
dibayar. Itu bukan kemurahan hati, itu memang haknya.

**Penghapusan menunggu 90 hari** sejak akses berakhir, dan sebelum dijalankan:

1. **Ekspor data tenant wajib ditawarkan** — hak portabilitas UU PDP. Kalau
   pelanggan pergi, datanya ikut, bukan hilang.
2. **Four-eyes** (§6.2). Ini tindakan paling tidak bisa ditarik di sistem.
3. Yang dihapus adalah data pribadi; **`audit_logs` tetap**, karena ia bukti
   dan retensinya 24 bulan (migrasi 126). Menghapus jejak audit bersama
   tenantnya akan menghapus justru catatan yang membuktikan penghapusan itu sah.

Layar Tenant menampilkan hitung mundur penghapusan sebagai tanggal, bukan
"90 hari" — dan tenant yang mendekatinya masuk Pusat Tindakan.

---

## 8. Delapan permukaan yang sudah ada

Semuanya berjalan dan tidak ditulis ulang. Yang berubah hanya penempatan di
navigasi baru (§2) dan penerapan sistem desain Tahap 0.

| Permukaan | Perubahan yang diperlukan |
|---|---|
| **Travel** | Jadi **Tenant**; baris bisa diklik → `/admin/tenant/[id]` (§4.4). Ini perubahan terbesar di antara delapan. |
| **Harga Modal** | Subjudul hidup: *"{n} produk tanpa harga modal · terjual tanpa lantai harga"*. Sudah benar mendahulukan yang kosong. |
| **Katalog** | Subjudul hitung; keadaan kosong yang mengajar. |
| **Akun** | Tambah kolom status 2FA dan sesi aktif. Beri/cabut akses platform masuk four-eyes (§6.2). |
| **Identitas** | Tambah umur antrean — KYC menunggu > 48 jam masuk Pusat Tindakan. Setiap pembukaan berkas masuk audit (§6.3). |
| **Supplier** | Tambah dua tab: Routing dan Log (§4.5). |
| **Transaksi** | Tautan dua arah ke log supplier. Transaksi `HELD` dan fulfilment menggantung masuk Pusat Tindakan. |
| **Transfer** | Sudah kuat. Tambahkan nilai rupiah belum direkonsiliasi ke Pusat Tindakan. |

Aturan yang berlaku ke delapan-delapannya: satu tombol `primary` per layar,
subjudul yang menghitung, keadaan kosong yang menyebut sebab dan langkah
berikutnya, dan label **"Lintas seluruh tenant"** pada setiap tabel.

---

## 9. Kontrak data

Bentuk tabel yang perlu ditambah. Ditulis di sini supaya keputusan integritas
diambil sekarang, bukan saat menulis migrasi.

```sql
-- Kelonggaran yang tidak pernah berakhir bukan kelonggaran; itu paket baru
-- yang tidak pernah diberi nama.
ALTER TABLE plan_overrides ADD COLUMN expires_at TIMESTAMPTZ;

-- Membedakan "belum bayar" dari "dibekukan sengaja". Akses tetap ditentukan
-- access_until; kolom ini hanya menjelaskan kenapa.
ALTER TABLE subscriptions ADD COLUMN suspended_at TIMESTAMPTZ;

-- Diisi worker harian. PK-nya mencegah dua baris untuk periode yang sama,
-- sehingga worker yang berjalan dua kali menimpa, bukan menggandakan.
CREATE TABLE usage_counters (
  operator_id  UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  metric       TEXT NOT NULL,              -- pilgrims | branches | storage | api | whatsapp
  period_start DATE NOT NULL,
  value        BIGINT NOT NULL CHECK (value >= 0),
  computed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (operator_id, metric, period_start)
);

-- Satu baris per tahap per invoice. Uniknya PK: worker yang berjalan dua kali
-- tidak bisa mengirim peringatan kedua, dan pelanggan yang menerima dua kali
-- tidak akan percaya yang ketiga.
CREATE TABLE dunning_log (
  invoice_id UUID NOT NULL REFERENCES subscription_invoices(id) ON DELETE CASCADE,
  stage      TEXT NOT NULL,                -- H1 | H7 | H14 | SUSPEND
  sent_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  channel    TEXT NOT NULL,
  PRIMARY KEY (invoice_id, stage)
);

-- Ada sejak hari pertama walau admin masih satu, supaya menambah admin kedua
-- hanya mengubah satu aturan, bukan seluruh alur.
CREATE TABLE privileged_actions (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kind          TEXT NOT NULL,             -- SUSPEND | DELETE_TENANT | SET_PLAN_LIMIT | SET_SETTLEMENT
  payload       JSONB NOT NULL,
  reason        TEXT NOT NULL CHECK (length(trim(reason)) > 0),
  requested_by  TEXT NOT NULL,
  requested_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  approved_by   TEXT NOT NULL,
  approved_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  executed_at   TIMESTAMPTZ
);

-- Tidak ada kolom untuk "mode tulis". Ketiadaannya disengaja: menulis atas
-- nama orang lain menghapus beda antara tindakan kami dan tindakan mereka.
CREATE TABLE impersonation_sessions (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id  TEXT NOT NULL,
  operator_id    UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  reason         TEXT NOT NULL CHECK (length(trim(reason)) > 0),
  ip             INET,
  started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at     TIMESTAMPTZ NOT NULL,
  ended_at       TIMESTAMPTZ,
  request_count  INTEGER NOT NULL DEFAULT 0
);
```

Semua tabel di atas mengikuti aturan yang sudah berlaku: peran aplikasi tidak
boleh menghapus `dunning_log`, `privileged_actions`, dan
`impersonation_sessions` — ketiganya bukti, bukan cache. Ikuti pola
`REVOKE UPDATE, DELETE` pada migrasi 125.

---

## 10. Yang membuat rancangan ini gagal

Ditulis supaya bisa diperiksa, bukan supaya terdengar aman.

- **Pratinjau dampak yang hanya menyebut jumlah.** "7 tenant terdampak" tidak
  bisa ditindak; tujuh nama bisa.
- **Meter pemakaian dihitung per permintaan.** Panel akan melambat seiring
  pertumbuhan — persis saat ia paling dibutuhkan.
- **Dunning tanpa kunci idempotensi.** Worker berjalan dua kali, pelanggan
  menerima dua peringatan, dan kepercayaan pada yang ketiga hilang.
- **Penangguhan yang mengubah status alih-alih waktu.** Melawan rancangan
  skema, dan status basi akan membagikan akses gratis.
- **Impersonate yang bisa menulis.** Menghapus perbedaan antara tindakan kami
  dan tindakan mereka.
- **Void invoice yang menghapus baris.** Riwayat berlubang tepat di tempat yang
  akan diperiksa.
- **Override tanpa alasan dan tanpa akhir.** Enam bulan kemudian tidak ada yang
  tahu kenapa satu tenant punya kuota berbeda.
- **Menghapus `audit_logs` bersama tenantnya.** Menghapus justru catatan yang
  membuktikan penghapusan itu sah.
- **Trial 3 hari dibiarkan sebagai konstanta di kode.** Angka komersial yang
  butuh deploy untuk diubah akan tetap salah selama berbulan-bulan.
