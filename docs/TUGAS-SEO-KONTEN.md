# Tugas: SEO & Konten Storefront

Membuat storefront travel bisa ditemukan, dan membuat kontennya terukur.

Dibuat 2 September 2026. **Belum ada yang dikerjakan.**
Berpasangan dengan [TUGAS-CORONG.md](TUGAS-CORONG.md): yang ini membuat orang
datang, yang itu mengukur apa yang terjadi setelah mereka datang.

> **Kenapa ini penting sekarang.** Pemilik memilih konten dan SEO sebagai arah
> nilai jual. Tanpa sitemap, artikel baru bisa berminggu-minggu tidak terindeks
> — jadi strategi kontennya tidak akan terlihat hasilnya, dan tidak akan
> ketahuan kenapa.

## Yang sudah benar — jangan diulang

Diperiksa sebelum menulis daftar ini:

- `generateMetadata` sudah ada di storefront **dan** halaman artikel: judul,
  deskripsi, dan Open Graph terisi dari CMS tenant.
- **Canonical sudah menunjuk ke domain klien**, bukan ke subdomain platform.
  Itu detail halus dan sudah dipikirkan: tanpa itu, storefront yang bisa dibuka
  di dua host akan dihitung sebagai konten ganda.

## Aturan

- Semua keluaran **sadar host**. Satu storefront bisa dibuka di subdomain
  platform dan di domain kliennya sendiri; menyajikan sitemap atau robots yang
  sama untuk keduanya membuat mesin pencari salah membaca kepemilikan.
- Commit tiap unit yang selesai **dan terverifikasi**. Jangan push ke `main`.

---

# TAHAP S1 — Bisa ditemukan

- [x] **S1.1** `robots.txt` sadar host (`c3ed52e`). Untuk host platform: izinkan `/`,
      larang `/dashboard`, `/admin`, `/leader`, `/pilgrim`, `/api`. Untuk host
      tenant: izinkan storefront dan artikelnya saja. **Subdomain yang
      dicadangkan dan domain yang belum terverifikasi harus `Disallow: /`** —
      halaman yang belum resmi milik siapa pun tidak boleh terindeks.
- [x] **S1.2** `sitemap.xml` per tenant (`c3ed52e`), dibangun dari host: halaman utama,
      tiap paket/musim yang aktif, tiap artikel. `lastmod` dari `updated_at`
      yang sebenarnya, bukan waktu render — `lastmod` yang selalu "sekarang"
      diabaikan Google setelah beberapa kali.
- [x] **S1.3** Sitemap platform terpisah (`c3ed52e`).
- [x] **S1.4** Diuji dua arah terhadap dua tenant nyata di server berjalan: nol
      kemunculan silang (`c3ed52e`)
- [x] **S1.5** Sitemap ditautkan dari `robots.txt`; `Content-Type` diverifikasi
      `application/xml` (`c3ed52e`)

# TAHAP S2 — Data terstruktur

- [x] **S2.1** `TravelAgency` di storefront, termasuk nomor izin sebagai
      `identifier` — hal pertama yang dicari calon jamaah yang teliti (`c8ed3ae`)
- [x] **S2.2** `Article` di blog dan berita. `dateModified` **tidak dikarang**
      dari `datePublished` — mengaku diperbarui padahal tidak persis jenis
      ketidakjujuran kecil yang diawasi (`c8ed3ae`)
- [x] **S2.3** `Product` selalu; `Offer` **hanya** bila ada harga numerik, dan
      angka itu juga yang ditampilkan halaman — satu sumber, jadi mustahil
      berbeda (`c8ed3ae`)

> **Jangan mengarang harga.** `priceLabel` adalah teks bebas yang diketik
> operator: bisa "Mulai Rp 25 juta", bisa "Hubungi kami". Google menuntut
> `Offer.price` berupa angka yang **sama persis dengan yang terlihat di
> halaman**, dan harga yang tidak cocok adalah pelanggaran kebijakan yang bisa
> mencabut rich result seluruh domain itu.
>
> Jadi: keluarkan `Offer` **hanya** bila ada harga numerik yang benar-benar
> ditampilkan. Kalau labelnya teks bebas, keluarkan `Product` tanpa `Offer`.
> Mem-parse angka dari teks bebas adalah cara paling cepat kehilangan rich
> result untuk semua pelanggan sekaligus.

- [x] **S2.4** Delapan aturan diuji langsung terhadap modulnya, bukan dibaca:
      Offer ada/tidak untuk tiga kondisi harga, `dateModified` absen, alamat
      tidak dikeluarkan bila hanya negara, dan `</script>` di-escape (`c8ed3ae`).
      **Belum diuji di Rich Results Test Google** — butuh URL publik

# TAHAP S3 — Search Console

Ini yang menjawab pertanyaan pemilik: *siapa dan berapa orang yang mencari
"Visa Umroh Mandiri"*. Kata pencarian **tidak pernah** sampai ke server kita —
Google berhenti mengirimnya di referrer sejak 2011 — jadi Search Console adalah
satu-satunya sumber yang sah, dan datanya agregat.

- [ ] **S3.1** Verifikasi kepemilikan otomatis. **Kita yang menyajikan situsnya**,
      jadi platform bisa menaruh berkas verifikasi Google di setiap domain tenant
      tanpa meminta apa pun dari mereka. Ini keunggulan yang tidak dimiliki alat
      analitik pihak ketiga.
- [ ] **S3.2** Tarik Search Analytics harian per properti: query, impresi, klik,
      posisi rata-rata. Simpan **agregat**, bukan per pengguna.
- [ ] **S3.3** Hormati batas kuota API dan mundur dengan rapi. Search Console
      membatasi permintaan; penarik yang tidak mundur akan diblokir dan diam.
- [ ] **S3.4** Layar di `/dashboard`: query yang membawa orang, posisi, klik,
      dan **artikel mana yang menjawab query itu** — sambungan yang membuat
      travel tahu konten mana yang harus ditulis berikutnya.
- [ ] **S3.5** Layar di `/admin`: query lintas seluruh tenant, dan topik yang
      banyak dicari **tetapi belum ada artikelnya di tenant mana pun**. Itu
      daftar konten yang bisa Anda tulis sekali dan tawarkan ke semua pelanggan.

# TAHAP S4 — Kecepatan

Peringkat dipengaruhi Core Web Vitals, dan storefront adalah halaman yang paling
sering dibuka orang dari luar.

- [ ] **S4.1** Ukur dulu: LCP, CLS, INP pada satu storefront nyata. **Jangan
      optimalkan sebelum ada angkanya** — tanpa dasar, tidak ada yang tahu apakah
      perubahan membantu.
- [ ] **S4.2** Gambar storefront lewat `next/image` dengan ukuran eksplisit;
      gambar tanpa dimensi adalah penyebab CLS paling umum.
- [ ] **S4.3** Font: `display: swap` dan preload untuk yang dipakai di paruh atas.

---

## Yang sengaja TIDAK dikerjakan

- **AMP.** Sudah tidak diistimewakan Google sejak 2021.
- **Meta keywords.** Diabaikan seluruh mesin pencari sejak lama.
- **Membuat konten otomatis dengan AI untuk tenant.** Google menurunkan konten
  massal tanpa nilai tambah, dan risikonya menimpa domain pelanggan — bukan
  domain kita.
- **Menjanjikan posisi tertentu di Google.** Tidak ada yang bisa, dan menjanjikannya
  merusak kepercayaan saat tidak tercapai.
