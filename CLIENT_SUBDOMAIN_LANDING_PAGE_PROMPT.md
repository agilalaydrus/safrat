# Technical Specification & Prompt: Client Subdomain Landing Page (Umrah & Hajj Luxury Editorial)

Gunakan dokumen instruksi prompt ini untuk AI Coding Agent (Codex / Claude / GPT) agar dapat mereproduksi atau mengembangkan Landing Page Subdomain Klien Travel Umrah & Haji dengan standar visual editorial kelas dunia.

---

## 🎯 Role & Objective
Anda adalah Senior Frontend Architect & Luxury Editorial Designer. Tugas Anda adalah membangun **Client Subdomain Landing Page** berorientasi B2C untuk biro travel penyelenggara Umrah (PPIU) dan Haji Khusus (PIHK) yang ditenagai oleh platform sistem operasi **TawafiqHub**.

Landing page ini harus menghindari template klise SaaS/AI (*No generic cards, no neon gradients, no purple blobs*), melainkan mengadopsi estetika **Majalah Editorial Premium (National Geographic / Leica documentary style)** dengan nuansa spiritual yang khusyuk, anggun, dan berwibawa.

---

## 🎨 Design System & Visual Guidelines

### 1. Palet Warna (Obsidian Dark & Sand Luxury)
- **Background (Dark Mode Default)**: `Obsidian Black` (`#0A0F0D` / `#0E1411`) untuk kenyamanan membaca di malam hari tanpa menyilaukan mata jamaah.
- **Background (Light Mode)**: `Alabaster Warm White` (`#FAFAF9` / `#FFFFFF`).
- **Primary Text**: `Off-White` (`#F8FAFC`) pada Dark Mode / `Deep Slate` (`#0F172A`) pada Light Mode.
- **Accent Primary**: `Sand Beige` (`#C5A880` / `#D8BE98`) untuk tombol CTA tunggal & badge prestisius.
- **Secondary Accent**: `Royal Forest Emerald` (`#064E3B` / `#059669`) & `Warm Terracotta` (`#8C3A27`).

### 2. Skala Tipografi (Typography Hierarchy)
- **Display Headings**: `Cormorant Garamond` (Serif Display, Bold, 48px - 64px, `leading-[1.1]`).
- **Body Text**: `Inter` / `Plus Jakarta Sans` (Sans-serif, Regular 16px, `line-height: 1.65 - 1.7`).
- **Data & Numbers**: `Tabular Mono` untuk harga, kuota seat, dan tanggal hijriah.

### 3. Standar Gambar & Optimasi
- Foto beresolusi 4K dengan kompresi modern **AVIF / WebP**.
- Efek pencahayaan foto natural, bukan gradien sintetis. Gunakan teknik *natural vignette* untuk keterbacaan teks di atas foto.

---

## 🏛️ 7 Komponen Utama Struktur Landing Page

### 1. Hero Section (Visual Emosional & Slow-Shutter Tawaf)
- **Background**: Foto candid 4K jamaah memandang Ka'bah dari kejauhan saat senja/fajar dengan efek *slow-shutter motion blur* pada arus thawaf.
- **Overlay**: Teknik *natural vignette* (tepi menggelap halus, pusat foto bercahaya lembut).
- **Headline (Serif Cormorant Garamond)**:
  > *"Langkah Kecil Menuju Perjalanan Besar yang Dinanti."*
- **Sub-headline (Sans-serif Inter)**:
  > *"Layanan Umrah & Haji yang mengedepankan khidmat, kenyamanan, dan bimbingan ibadah sesuai sunnah."*
- **Single Solid CTA Button**: Tombol tunggal warna **Sand Beige Solid** bertuliskan `"Eksplorasi Paket"` dengan scroll halus menuju katalog.
- **Trust Badges**: Izin PPIU Kemenag RI, Hotel Bintang 5 Dekat Pelataran, Jadwal Pasti 100%, Manasik Sesuai Sunnah.

### 2. Layout Editorial (Tentang Kami & Filosofi Pelayanan)
- **Layout Asimetris**: Foto vertikal rasio 3:4 di sisi kiri (candid asatidz membimbing jamaah di selasar marmer Masjid Nabawi).
- **Overlapping Typography Card**: Kontainer teks narasi menumpuk sebagian di atas tepi foto sebelah kanan.
- **Narasi Utama**:
  > *"Sejak [Tahun], kami percaya bahwa Umrah bukan sekadar perjalanan fisik, melainkan kepulangan spiritual..."*
- Menyampaikan komitmen pembebasan jamaah dari kerumitan teknis (tiket, visa Nusuk, hotel) agar murni fokus pada kekhusyukan ibadah.

### 3. Katalog Paket Umrah (Desain List Horizontal Premium)
*Wajib menghindari susunan 3 kolom kartu yang monoton.*
- **Struktur Barisan Horizontal (List Style)**:
  - **Kolom 1**: Nomor urut, Nama Paket (e.g. *Umrah I’tikaf Khidmat*, *Umrah Reguler Barakah*, *Umrah Plus Turki*, *Haji Khusus Furoda*), & Badge Pilihan.
  - **Kolom 2**: Durasi Perjalanan (9 Hari, 12 Hari, 16 Hari, 25 Hari).
  - **Kolom 3**: Harga Mulai Dari (`Rp 30.xxx.xxx / pax`).
  - **Kolom 4**: Tombol interaktif `"Lihat Detail"` (dengan icon Chevron Expand).
- **Expandable Seasonal Sub-Section**: Saat baris diklik, area meluas ke bawah menampilkan 3 pilihan musim:
  - 📅 **Musim Awal (Sept - Okt)**
  - ❄️ **Musim Dingin (Des - Jan)**
  - 🌙 **Musim Ramadhan (Awal & Lailatul Qadar)**
- **Spesifikasi Musim**: Nama hotel Makkah & Madinah + rating bintang, maskapai penerbangan direct, sisa kuota kursi real-time, rincian harga per tipe kamar (*Quad, Triple, Double*), serta tombol order/reservasi.

### 4. Blog & Warta Berita (Magazine Style & SEO SERP Preview)
- **Sisi Kiri (Besar, 7 Kolom)**: Artikel utama bercover foto dokumenter Masjid Nabawi senja dengan judul:
  > *"Panduan Memilih Musim Terbaik untuk Ibadah Nyaman & Khusyuk."*
- **Sisi Kanan (Kecil, 5 Kolom)**: 3 pembaruan warta terkini:
  - Update Regulasi Saudi: Aturan Tasreh Raudhah & Biometrik Nusuk.
  - Jadwal Keberangkatan Tambahan Musim Dingin Direct Madinah.
  - 5 Tips Manasik & Fisik Praktis Jamaah Lansia.
- **Google SEO SERP Preview Box**: Simulasi tampilan hasil pencarian Google dengan URL slug `https://[subdomain].tawafiqhub.com/blog/...` dan meta description informatif.

### 5. Galeri Dokumenter ("The Human Touch")
- **Layout Masonry Grid**: Susunan foto dengan variasi aspek dinamis (*tall 3:4*, *wide 16:9*, *square 1:1*).
- **Konten Humanis & Pembangun Kepercayaan**:
  - Close-up makro butiran tasbih kayu zaitun di tangan jamaah.
  - Senyuman haru & syukur jamaah lansia di depan Baitullah.
  - Kehangatan makan malam bersama mencicipi hidangan nusantara di hotel.
  - Pemandangan ketenangan payung hidrolik Masjid Nabawi di senja hari.
- **Fitur Interaksi**: Klik foto untuk membuka *Fullscreen Lightbox Modal* dengan takarir (caption) dokumenter lengkap.

### 6. Pendaftaran Mitra Agen & Tour Leader (CTA Banner)
- **Background Banner**: Nuansa hangat *Deep Olive* atau *Terracotta* dengan ambient glow.
- **Headline**:
  > *"Jadilah jembatan kebaikan. Bergabung sebagai Agen atau Tour Leader kami."*
- **Form Minimalis Konversi Tinggi**:
  - Pilihan role: *Mitra Agen Syiar* vs *Tour Leader / Muthowif*.
  - Input: **Nama Lengkap** & **Nomor WhatsApp Aktif**.
  - Tombol Submit yang langsung mengarahkan pesan terstruktur ke WhatsApp Manajer Kemitraan Travel.

### 7. Footer (Professional, Clean & Terintegrasi)
- **Kolom Kiri**: Logo Resmi Travel, tagline, dan ringkasan profil biro.
- **Kolom Tengah**: Link navigasi cepat & **Kartu Akreditasi Izin Resmi Kemenag (No. Izin PPIU & PIHK)** terverifikasi SISKOPATUH.
- **Kolom Kanan**: Alamat kantor lengkap, telepon, email, serta **Interactive Location Maps Preview** dengan tautan langsung ke Google Maps.
- **Bottom Bar**:
  ```text
  © 2024 [Nama Travel Anda]. All Rights Reserved. Powered by TawafiqHub.
  ```

---

## 🛠️ State Management & Fitur Interaktif
1. **Live Subdomain Bar & Customizer**:
   - Bar atas yang menampilkan URL live subdomain: `https://[subdomain].tawafiqhub.com`.
   - Tombol toggle instan *Obsidian Dark Mode* vs *Light Mode*.
   - Modal customizer untuk mengganti nama travel, izin PPIU, WhatsApp, dan aksen warna secara langsung di sisi browser.
2. **Booking & Consultation Modal**:
   - Modal responsif saat jamaah mengklik tombol *"Pesan Kursi Musim Ini"* pada katalog paket.
   - Pilihan tipe kamar (*Quad, Triple, Double*), jumlah jamaah, catatan khusus lansia/kursi roda, dan direct forward ke WhatsApp biro travel.
