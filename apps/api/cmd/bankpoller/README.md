# bankpoller — membaca mutasi rekening dan mengirimkannya

Endpoint `/webhooks/bank-feed` sudah ada dan menunggu; ini yang memberinya
makan.

## Kenapa proses terpisah, bukan task worker

Pembaca mutasi adalah bagian paling rapuh di sistem ini — ia mengurai format
ekspor orang lain, atau lebih buruk, HTML mereka. Ia juga mungkin perlu
berjalan di tempat lain: jaringan berbeda, mesin dengan browser, atau laptop
yang dijalankan seseorang seminggu sekali. Di luar proses API, kegagalannya
tidak bisa ikut menjatuhkan API.

## Pakai hari ini, tanpa akses API bank

Setiap bank bisa mengekspor mutasi ke CSV. Itu cukup:

```bash
BANK_FEED_SECRET=… bankpoller \
  -file mutasi-agustus.csv \
  -endpoint https://api.tawafiqhub.id/webhooks/bank-feed \
  -date-column Tanggal -amount-column Nominal \
  -description-column Keterangan -reference-column Referensi \
  -date-layout 02/01/2006
```

Coba dulu dengan `-dry-run`: ia menampilkan apa yang terbaca tanpa mengirim
apa pun.

**Aman dijalankan berulang pada berkas yang sama.** Endpoint mengunci kredit
berdasarkan id eksternalnya, jadi impor ulang tidak mencatat apa pun yang baru
dan tidak melunasi apa pun dua kali. "Jalankan lagi kalau ragu" memang saran
yang benar di sini.

## Yang diperiksa, dan kenapa

**Hanya kredit.** Debit yang lolos ke feed akan tercatat sebagai uang masuk —
satu-satunya kesalahan yang seluruh jalur ini ada untuk mencegahnya. Nominal
negatif dilewati, bukan dijadikan positif.

**Format angka Indonesia.** `1.500.000` dan `1,500,000` sama-sama muncul di
ekspor bank. Pemisah dianggap desimal hanya bila diikuti satu atau dua digit;
tiga digit berarti ribuan. Salah di sini mengubah Rp1.500.000 jadi Rp1.500 —
yang lalu tidak cocok dengan tagihan mana pun dan terbaca seperti pelanggan
yang tidak membayar. Ini bug nyata yang tertangkap tes saat dibangun.

**Gagal urai menghentikan seluruh batch.** Setengah mutasi yang terkirim
terlihat persis seperti setengah pelanggan yang belum bayar.

## Batasan yang perlu diketahui

Kalau ekspornya **tidak punya kolom referensi**, id diturunkan dari isi baris
(tanggal + nominal + keterangan). Akibatnya: dua transfer yang benar-benar
terpisah, di hari yang sama, dengan nominal dan keterangan sama, dianggap satu.

Itu mengurangi hitungan, bukan melunasi dua kali — arah yang lebih aman. Tapi
ia alasan kuat untuk memakai ekspor yang memuat nomor referensi, dan untuk
memeriksa antrean "belum ada tagihannya" ketika sebuah travel bilang sudah
membayar.

## Menyambung API bank sungguhan nanti

Implementasikan `bankfeed.Source` — dua method, `Name()` dan `Fetch()`. Tidak
ada yang di hilirnya berubah: pencocokan, idempotensi, dan jalur manual sudah
tidak peduli dari mana mutasinya datang.
