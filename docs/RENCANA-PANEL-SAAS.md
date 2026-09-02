# Rencana: Panel SaaS TawafiqHub

Panel milik pemilik platform — `/admin`. Bukan dashboard operator, bukan
storefront. Ini satu-satunya permukaan di sistem ini yang **tidak** dibatasi
tenant, dan itu menentukan hampir semua keputusan di dokumen ini.

Ditulis 2 September 2026. Referensi pesaing: `devmeeqot.dul.co.id` (prototipe,
tanpa backend — lihat [BENCHMARK-MEEQOT.md](BENCHMARK-MEEQOT.md)).
Pendamping: [TUGAS-PANEL-SAAS.md](TUGAS-PANEL-SAAS.md).

---

## 1. Apa yang sudah ada — jujur

Panel ini bukan nol. `PlatformService` punya **30 RPC** dan `/admin` punya
delapan permukaan yang berjalan:

| Permukaan | Isi | RPC |
|---|---|---|
| **Travel** | Setiap tenant: paket, status langganan, jumlah jamaah & produk, transaksi `HELD` | `ListOperators` |
| **Harga Modal** | Produk tanpa harga modal didahulukan — itu yang terjual tanpa lantai harga | `ListProductsNeedingCost`, `SetProductSupplierCost`, `SetProductBasePrice` |
| **Katalog** | Katalog produk milik platform | `SavePlatformProduct`, `ListPlatformCatalogue` |
| **Akun** | Direktori akun, beri/cabut akses platform, cabut sesi | `ListAccounts`, `GrantPlatformAdmin`, `RevokePlatformAdmin`, `RevokeSessions` |
| **Identitas** | Tinjauan KYC | `ListKycRecords`, `GetKycRecord`, `SetKycStatus` |
| **Supplier** | Supplier, aturan respons, penguji aturan | `ListSuppliers`, `SaveSupplier`, `ListResponseRules`, `CreateResponseRule`, `SetResponseRuleActive`, `TestResponseRules` |
| **Transaksi** | Transaksi lintas tenant, penyelesaian fulfilment | `ListTransactions`, `ResolveFulfilment` |
| **Transfer** | Transfer tertunda, mutasi bank, rekonsiliasi | `ListPendingTransfers`, `ConfirmBankTransfer`, `ListBankMutations`, `SettleInvoiceWithMutation`, `IgnoreBankMutation` |

Model aksesnya sudah benar dan **tidak boleh diubah**:

- Akses adalah **baris di `platform_admins`**, bukan kolom di tabel milik Better
  Auth. Memberikannya butuh INSERT yang disengaja oleh orang dengan akses
  database. Itu memang maksudnya: panel yang dibuka olehnya adalah yang
  menghapus kebutuhan akses database untuk segalanya yang lain.
- `requirePlatformAdmin` dibaca **setiap permintaan**, bukan di-cache. Pencabutan
  menggigit di panggilan berikutnya.
- 2FA wajib untuk admin platform.
- Sudah diverifikasi lewat HTTP nyata: sesi owner operator asli →
  `permission_denied`.

## 2. Tiga mesin yang belum punya pemicu

Pola yang sudah berulang di proyek ini: RPC ada, terimplementasi, teruji — dan
tidak ada satu pun yang memanggilnya.

| RPC | Akibat hari ini |
|---|---|
| `ListProductRoutes` | Routing produk→supplier hanya terbaca lewat SQL |
| `SaveProductRoute` | **Routing hanya bisa diubah lewat terminal di produksi** |
| `ListSupplierLogs` | Saat supplier bermasalah, lognya tak terlihat dari panel |

`HANDOFF.md:941` sendiri menulis *"Still to build: the admin screens for all of
this (the RPCs exist, the UI does not)"* — dan itu masih benar. Ironisnya panel
ini lahir persis untuk menghapus kebutuhan membuka terminal.

**Dan yang lebih parah:** `plan_limits` serta `plan_overrides` mendarat lewat
T2.2, hidup di database, menegakkan batas lewat trigger — **tanpa satu pun RPC,
apalagi layar**. Menaikkan kuota satu pelanggan hari ini berarti menulis SQL di
produksi. Entitlement yang tidak bisa dikelola bukan fitur komersial; itu
sekadar tembok.

## 3. Tesis produk

Panel ini melayani **satu orang** — pemilik platform — dan menjawab empat
pertanyaan, berurutan menurut seberapa mahal salah jawabnya:

1. **Apakah ada uang yang macet atau salah tempat?** Transfer belum
   direkonsiliasi, fulfilment menggantung, transaksi `HELD`, tagihan langganan
   menunggak.
2. **Apakah ada tenant yang sedang gagal?** Melewati kuota, langganan lewat
   jatuh tempo, produk terjual tanpa harga modal, KYC menumpuk.
3. **Apakah bisnis ini tumbuh?** MRR, tenant aktif, konversi trial, churn.
4. **Apakah platformnya sehat?** Supplier gagal, aturan respons meleset,
   antrean tersendat.

Urutan itu juga urutan navigasinya. Uang lebih dulu, karena kesalahan di sana
tidak bisa ditarik.

**Yang panel ini bukan.** Ia bukan konsol infrastruktur. Meeqot menaruh layar
deploy, layar server, restart node, dan konsol anggaran AI multi-provider di
dalam panelnya — itu membangun PaaS di dalam SaaS. Operasional milik perkakas
operasional (Dokploy, systemd, log agregator), bukan milik layar produk. Lihat
§8.

## 4. Yang harus dibangun

### 4.1 Entitlement & kuota — `/admin` tab **Paket & Kuota** 🔴

Paling mendesak, karena T2.2 sudah menegakkan batas yang tidak bisa dikelola.

**Fungsi:**
- Baca dan ubah `plan_limits` per paket (`max_pilgrims`, `max_branches`,
  `feature_flags`).
- Override per tenant (`plan_overrides`) — dengan alasan wajib dan tanggal
  kedaluwarsa opsional. Prinsip yang benar dan layak ditiru dari mereka:
  > *"Add-on lebih aman dipakai untuk kebutuhan satu tenant daripada mengubah
  > batas paket untuk semua."*
- **Pratinjau dampak sebelum simpan.** Menurunkan `max_pilgrims` GROWTH dari
  500 ke 300 harus menampilkan berapa tenant yang seketika melampaui batas,
  dan siapa. Tanpa itu, satu perubahan angka bisa memblokir pendaftaran di
  belasan travel sekaligus.
- **Grandfathering**: tenant yang sudah melewati batas baru tidak ditendang;
  mereka dikunci di angka lamanya sampai turun sendiri. Kenaikan harga tidak
  boleh mengubah tagihan pelanggan lama tanpa keputusan sadar.

**Teknis:** RPC baru `ListPlanLimits`, `SetPlanLimit`, `ListPlanOverrides`,
`SetPlanOverride`, `DeletePlanOverride`, `PreviewPlanLimitChange`. Perubahan
batas ditulis ke `audit_logs` — ini keputusan komersial, bukan konfigurasi.

### 4.2 Meter pemakaian — tab **Pemakaian** 🔴

Tanpa ini, batas ditegakkan tapi tidak ada yang tahu siapa mendekatinya.

- Pemakaian per tenant terhadap batasnya: jamaah, cabang, penyimpanan, panggilan
  API, pesan WhatsApp.
- Rentang 30 hari, dan **tanggal reset yang eksplisit** — Meeqot menulisnya di
  subjudul: *"Reset setiap tanggal 1 · perhitungan real-time"*. Angka kuota
  tanpa tanggal reset tidak bisa ditindak.
- Peringatan pada 80% dan 100%, dengan nama tenantnya.

**Teknis:** tabel `usage_counters` (operator_id, metric, period_start, value)
yang diisi worker harian, bukan dihitung ulang per permintaan — menghitung
jamaah lintas tenant setiap kali panel dibuka akan menjadi query paling mahal
di sistem ini.

### 4.3 Tagihan langganan & dunning — tab **Langganan** 🔴

`subscription_invoices` dan status `PAST_DUE` sudah ada; **tidak ada yang
menindaklanjutinya**.

- Siklus tagihan massal: tinjau daftar invoice yang akan terbit beserta
  nominalnya, lalu terbitkan sekaligus.
- **Rangkaian dunning** H+1, H+7, H+14, lalu penangguhan otomatis H+21.
  Setiap tahap adalah pesan, bukan hanya perubahan status.
- Grace period yang bisa diatur, per tenant kalau perlu.
- **Void invoice + pulihkan** — invoice salah terbit harus bisa dibatalkan
  dengan jejak, bukan dihapus.
- Prorata saat upgrade/downgrade di tengah periode.
- Penangguhan **memutus akses, tidak menghapus data**. Kalimat mereka tepat:
  *"Akses seluruh pengguna tenant diputus. Data tetap utuh."*

**Teknis:** penangguhan lewat kolom pada `subscriptions`, dibaca interceptor
yang sudah ada. **Idempotensi wajib**: setiap penerbitan invoice dan setiap
pesan dunning butuh kunci unik di database — worker yang berjalan dua kali
tidak boleh menagih dua kali. Lihat [[feedback_idempotency_first]].

### 4.4 Routing produk & log supplier — lengkapi tab **Supplier** 🟠

Menutup dua dari tiga mesin tanpa pemicu.

- Daftar routing produk→supplier, dengan supplier cadangan dan urutannya.
- Simpan routing dari panel. Produk tanpa routing harus terbaca jelas —
  di TawafiqHub responsnya sudah *"Produk Belum di Atur Routing"*, dan panel
  harus menampilkan daftar itu sebagai antrean kerja.
- Log supplier: permintaan, respons, latensi, dan aturan mana yang cocok.
  Inilah yang dibuka saat ada transaksi menggantung.

### 4.5 Detail tenant — `/admin/tenant/[id]` 🟠

Hari ini tab Travel adalah tabel datar. Satu tenant butuh halamannya sendiri:
langganan & riwayat tagihan, pemakaian vs kuota, override yang berlaku, jamaah
& cabang, transaksi dan transfer, KYC, tim & status 2FA, domain, dan jejak audit
tenant itu.

Ini juga tempat yang benar untuk **Impersonate** (§6).

### 4.6 Analitik pertumbuhan — tab **Analitik** 🟡

MRR dan pergerakannya, tenant aktif, konversi trial, churn, NRR.

Dua kejujuran yang mereka tulis dan layak ditiru apa adanya:

> *"Komisi market masuk ke pendapatan lain, bukan MRR — jangan dicampur saat membaca Analitik."*
> *"Skor risiko churn adalah heuristik internal — pakai sebagai penanda prioritas, bukan vonis."*

Angka yang tidak menjelaskan batasnya sendiri akan dipakai untuk keputusan yang
tidak bisa ditopangnya. Sertakan **Catatan Metodologi** seperti di laporan
operator.

### 4.7 Pengumuman ke tenant — tab **Pengumuman** 🟡

Tidak ada kanal apa pun dari platform ke pelanggan hari ini. Minimal:
pengumuman terjadwal, tertarget (semua / per paket / tenant trial / multi-cabang),
dengan pratinjau dan riwayat kirim.

### 4.8 Kesehatan platform — tab **Kesehatan** 🟡

Bukan konsol infrastruktur. Hanya yang berdampak ke pelanggan: antrean worker
tertinggal, webhook gagal, supplier bermasalah, poller bank berhenti, backup
terakhir berhasil kapan. Setiap butir menyebut **berapa tenant terdampak**.

---

## 5. Rancangan UI/UX

Mengikuti sistem yang sudah dibangun di Tahap 0 —
[DESAIN-DASHBOARD-TRAVEL.md](DESAIN-DASHBOARD-TRAVEL.md). Panel ini **tidak
boleh** punya bahasa visual sendiri.

- Komponen yang sama: `PageHeader`, `StatCard`, `ActionCenter`, `Badge`,
  `EmptyState`, `DataTable`, `DetailDrawer`, `ChartFrame`, `MethodologyNote`.
- Enam `tone` yang sama. Palet Emerald yang sama.
- **Satu tombol `primary` per layar.**
- Subjudul menghitung isi layar: *"38 tenant · 12 trial · 3 menunggak"*.
- Keadaan kosong yang mengajar, menyebut sebab dan langkah berikutnya.
- Setiap grafik menjelaskan sumbunya.

**Pusat Tindakan panel** ada di halaman muka, dan urutannya mengikuti §3:
uang macet → tenant gagal → pertumbuhan → kesehatan. Setiap butir menyebut
nilai rupiah bila datanya ada, dan akibat kalau diabaikan. Di panel ini rupiah
hampir selalu ada — itu bedanya dengan layar jamaah.

**Satu pembeda visual yang dibenarkan:** panel ini melihat data seluruh tenant,
jadi setiap layar harus menyatakan cakupannya. Setiap tabel lintas-tenant diberi
label tetap *"Lintas seluruh tenant"* di header. Bukan hiasan — ia mencegah
salah baca bahwa yang tampil adalah satu travel.

---

## 6. Keamanan — bagian paling menentukan

Ini satu-satunya permukaan yang menembus batas tenant. Semua isolasi cabang dan
operator yang dibangun di Tahap 2 **tidak berlaku di sini**. Karena itu:

**Yang sudah benar, pertahankan:** baris `platform_admins` tanpa jalur
self-service; `requirePlatformAdmin` dibaca tiap permintaan; 2FA wajib;
`audit_logs` yang tidak bisa ditulis ulang peran aplikasi (migrasi 125) dengan
retensi 24 bulan (migrasi 126).

**Yang harus ditambahkan:**

- **Impersonate dengan jejak penuh.** Menelusuri keluhan tanpa meminta kata
  sandi pelanggan adalah kebutuhan nyata. Syaratnya mutlak: sesi impersonasi
  ditandai berbeda, dicatat lengkap beserta IP dan alasannya, berbatas waktu,
  dan **read-only secara bawaan**. Kalimat mereka benar: *"Impersonate adalah
  tindakan sensitif: sesi tercatat lengkap di Akses & Audit beserta IP."*
- **Four-eyes untuk tindakan tak bisa ditarik.** Menangguhkan tenant, menghapus
  tenant, mengubah `plan_limits` global, dan mengubah rekening settlement
  membutuhkan persetujuan admin platform kedua. Selama hanya ada satu admin
  (kondisi hari ini — *"panel ADMIN ini hanya boleh diakses oleh saya saja"*),
  ini berarti **konfirmasi ulang dengan mengetik nama tenant**, dan siap
  dinaikkan ke dua orang saat tim bertambah.
- **Rotasi kunci API dengan tumpang tindih.** Kunci lama tetap berlaku 24 jam
  agar tidak ada permintaan yang putus di tengah rotasi. Pola ini sudah kita
  pakai untuk kunci KYC; terapkan sama di sini.
- **Ekspor auditor**: CSV beserta hash manifes, ditandatangani kunci platform.
  UU PDP menuntut kita bisa menunjukkan siapa membaca data pribadi siapa dan
  kapan; ekspor yang tidak bisa dibuktikan keutuhannya tidak menjawab itu.
- **Setiap pembacaan data pribadi tenant dari panel ini masuk audit** — bukan
  hanya perubahan. Membaca KYC seorang jamaah dari panel platform adalah
  pemrosesan data pribadi, dan itu yang paling ditanya saat insiden.

**Yang sengaja tidak dilakukan:** panel tidak menyimpan rahasia. Ia menyimpan
prefix, sidik jari, dan metadata — persis pola `key_fingerprint` yang sudah
berjalan. Kebetulan Meeqot merancang hal yang sama; itu penguat, bukan sumber.

---

## 7. Urutan pengerjaan

Urut biaya menunda, bukan urut kemudahan.

1. **Paket & Kuota** (§4.1) — entitlement sudah menegakkan batas yang tidak bisa
   dikelola. Setiap hari yang lewat adalah hari SQL manual di produksi.
2. **Langganan & dunning** (§4.3) — pendapatan berulang yang tidak ditagih
   adalah pendapatan yang hilang diam-diam.
3. **Meter pemakaian** (§4.2) — memberi makna pada nomor 1 dan 2.
4. **Routing produk & log supplier** (§4.4) — menutup mesin tanpa pemicu.
5. **Detail tenant** (§4.5) — tempat semuanya bertemu.
6. **Four-eyes + impersonate + audit baca** (§6) — sebelum ada admin kedua.
7. Analitik, pengumuman, kesehatan.

---

## 8. Yang tidak ditiru dari Meeqot

- **Konsol anggaran AI multi-provider** (Anthropic/Qwen/DeepSeek dengan pagu dan
  hard-stop), **layar deploy**, **layar server**, **restart node**, **mode
  maintenance sebagai tombol UI**, **konfigurasi Turnstile**. Ini PaaS di dalam
  SaaS. Kalau nanti butuh mode maintenance, tempatnya di perkakas deploy.
- **Market/seller center platform** — produk kedua, sudah diputuskan ditunda.
- **Skor risiko churn sebagai angka menonjol** — heuristik yang terlihat presisi
  akan dipakai sebagai vonis. Kalau dipasang, pasang dengan peringatannya.
- **Emoji di teks sistem.**

---

## 9. Catatan pelaksanaan

- Alur tetap: proto → migrasi goose → sqlc → repository → service → handler → UI.
  Repository tidak boleh mengimpor service.
- `requirePlatformAdmin` tetap **terlihat di awal setiap metode**, bukan
  disembunyikan di interceptor. Itu keputusan yang sudah diambil dan alasannya
  masih berlaku: tidak ada apa pun di `PlatformService` yang di-scope tenant.
- Setiap penerbitan invoice, pesan dunning, dan penangguhan butuh kunci
  idempotensi **di database**.
- Ekspor apa pun ditulis streaming sejak awal.
- Panel ini tidak boleh punya jalur tulis yang melewati audit.
