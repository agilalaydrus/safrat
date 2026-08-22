import type { LucideIcon } from "lucide-react";
import {
  Bed,
  Bus,
  Calculator,
  MoonStar,
  ShoppingBag,
  Siren,
  UserCog,
  Users,
} from "lucide-react";

export const NAV_LINKS = [
  ["Fitur Operasional", "#fitur"],
  ["Interactive Preview", "#preview"],
  ["Simulasi Live", "#simulasi"],
  ["Kalkulator ROI", "#roi"],
  ["FAQ", "#faq"],
] as const;

export const TRUST_POINTS = [
  {
    title: "Kemenag & Siskopatuh Ready",
    desc: "Validasi NIK, nomor paspor, dan nomor porsi jamaah, format datanya sudah disiapkan untuk kebutuhan pelaporan resmi.",
  },
  {
    title: "100% Patuh Aturan Mahram",
    desc: "Sistem otomatis menolak kalau ada percobaan satu kamar campur yang bukan mahram, bukan cuma diingatkan lewat memo.",
  },
  {
    title: "Terhubung ke Lapangan Saudi",
    desc: "Muthowif dan supir bus bisa buka manifes langsung dari HP masing masing, tidak perlu install apa apa.",
  },
  {
    title: "Data Jamaah Terenkripsi",
    desc: "Paspor dan data pribadi jamaah tersimpan lewat koneksi terenkripsi, dan tiap travel terisolasi dari data travel lain.",
  },
] as const;

export type FeatureModule = {
  icon: LucideIcon;
  title: string;
  tagline: string;
  points: string[];
};

export const FEATURE_MODULES: FeatureModule[] = [
  {
    icon: Users,
    title: "Jamaah & Rombongan",
    tagline: "Satu tempat untuk semua data jamaah, dari daftar sampai pulang",
    points: [
      "Impor data dari Excel atau CSV, tidak perlu ketik ulang satu satu",
      "Sistem otomatis memberi peringatan kalau paspor sisa masa berlakunya di bawah 7 bulan",
      "Riwayat perubahan data tercatat, gampang ditelusuri kalau ada yang tanya",
    ],
  },
  {
    icon: MoonStar,
    title: "Kloter & Musim Hijriah",
    tagline: "Kelola musim keberangkatan sesuai kalender Hijriah yang sebenarnya dipakai",
    points: [
      "Timeline per kloter, dari bandara keberangkatan sampai Madinah dan Makkah",
      "Satu musim bisa menampung banyak kloter sekaligus, tetap rapi per kloter",
      "Kapasitas dan sisa kuota tiap kloter kelihatan langsung, tidak perlu hitung manual",
    ],
  },
  {
    icon: Bed,
    title: "Akomodasi & Aturan Mahram",
    tagline: "Rooming list yang tervalidasi otomatis, bukan hasil tebak tebakan",
    points: [
      "Alokasi kamar otomatis mempertimbangkan kapasitas, gender, dan hubungan mahram",
      "Percobaan menempatkan jamaah non mahram satu kamar akan ditolak sistem",
      "Ganti jamaah karena batal tetap bisa tanpa membongkar susunan kamar yang sudah jadi",
    ],
  },
  {
    icon: Bus,
    title: "Transportasi & Bus Saudi",
    tagline: "Manifes armada yang bisa dibuka langsung dari HP supir dan Muthowif",
    points: [
      "Kursi per jamaah tercatat jelas, tidak ada lagi drama kursi tertukar",
      "Manifes bisa dikirim lewat WhatsApp ke supir tanpa perlu cetak kertas",
      "Setiap pergerakan bus, dari Jeddah ke Madinah ke Makkah, ada jejak waktunya",
    ],
  },
  {
    icon: Siren,
    title: "SOS Real Time 10 Menit",
    tagline: "Satu tombol darurat untuk jamaah, dengan eskalasi otomatis",
    points: [
      "Jamaah bisa kirim sinyal darurat satu tombol, langsung dari HP sendiri",
      "Kalau 10 menit tidak direspons Muthowif, otomatis dieskalasi ke direksi PPIU",
      "Semua kasus darurat tercatat lengkap dengan waktu penanganannya",
    ],
  },
  {
    icon: ShoppingBag,
    title: "Produk Digital & Add On",
    tagline: "Jual layanan tambahan tanpa perlu sistem kasir terpisah",
    points: [
      "eSIM roaming Saudi, sewa kursi roda, sampai paket oleh oleh bisa dijual lewat aplikasi",
      "Jamaah bisa beli sendiri atau dicatatkan manual oleh admin travel",
      "Semua transaksi tercatat rapi, gampang direkap di akhir musim",
    ],
  },
  {
    icon: UserCog,
    title: "Manajemen Agen & Komisi",
    tagline: "Rekap komisi yang jelas, tidak ada lagi ribut di akhir musim",
    points: [
      "Setiap jamaah yang didaftarkan agen otomatis tercatat sumbernya",
      "Komisi dihitung otomatis per transaksi, sesuai persentase yang disepakati",
      "Agen bisa lihat sendiri rekap dan status pencairan komisinya",
    ],
  },
];

export const PROBLEM_SOLUTION = [
  {
    problem: "Rekap Excel manual dan lembur H-1 sebelum jamaah terbang",
    solution: "Data sudah rapi dari awal, staf tinggal cek dashboard kapan saja",
  },
  {
    problem: "Salah tempatkan jamaah non mahram satu kamar",
    solution: "Sistem menolak duluan sebelum jadi masalah di hotel",
  },
  {
    problem: "Kursi bus tertukar dan manifes kertas gampang hilang",
    solution: "Kursi per jamaah tercatat, manifes bisa dibuka dari HP",
  },
  {
    problem: "Panik dan bingung waktu ada jamaah terpisah di Masjidil Haram",
    solution: "Tombol SOS sekali tekan, eskalasi otomatis kalau lambat direspons",
  },
] as const;

export const FAQ_ITEMS = [
  {
    q: "Data jamaah kami sudah ada di Excel, apa harus input ulang?",
    a: "Tidak perlu. Tinggal impor file Excel atau CSV yang sudah ada, sistem yang akan membaca dan mencocokkan kolomnya. Data lama tidak perlu diketik ulang satu per satu.",
  },
  {
    q: "Datanya sinkron otomatis ke Siskopatuh Kemenag?",
    a: "Tawafiq Hub memakai istilah dan alur kerja yang sama dengan operasional PPIU sehari hari, jadi datanya sudah siap dipakai untuk laporan resmi. Untuk saat ini datanya diekspor dan disesuaikan formatnya dulu, kami belum mengklaim ada integrasi otomatis langsung ke Siskopatuh.",
  },
  {
    q: "Bisa dipakai waktu sudah di Saudi, tanpa install apa apa?",
    a: "Bisa. Semua bagian, baik untuk admin, Muthowif, maupun jamaah, bisa dibuka lewat browser HP biasa, tidak perlu download aplikasi berat dari Play Store atau App Store.",
  },
  {
    q: "Kalau sinyal di Saudi lagi jelek atau lagi roaming, gimana?",
    a: "Halaman yang sudah pernah dibuka tetap bisa diakses walau sinyal hilang. Aksi penting seperti SOS, absen, dan chat akan tersimpan dulu di HP dan otomatis terkirim begitu sinyal kembali.",
  },
  {
    q: "Aman tidak data paspor dan data pribadi jamaah kami?",
    a: "Semua akses lewat sesi login yang terenkripsi, dan tiap travel datanya terisolasi, tidak bisa diakses oleh travel lain. Seluruh trafik juga berjalan lewat koneksi terenkripsi.",
  },
] as const;

export const ILLUSTRATIVE_TESTIMONIALS = [
  {
    name: "Direktur Utama (Contoh)",
    role: "Travel Umrah Ilustrasi, musim sekitar 900 jamaah",
    quote: "Yang paling kerasa itu tim di kantor dan Muthowif di lapangan akhirnya lihat data yang sama. Tidak perlu lagi nunggu rekap dari grup WhatsApp yang scroll-nya panjang banget.",
  },
  {
    name: "Manajer Operasional (Contoh)",
    role: "PIHK Ilustrasi, musim sekitar 1.300 jamaah",
    quote: "Susun kamar dulu paling bikin deg degan, takut ada yang salah pasang mahram. Sekarang sistemnya yang tolak duluan sebelum sempat jadi masalah di hotel.",
  },
  {
    name: "Muthowif Lapangan (Contoh)",
    role: "Rombongan Ilustrasi, 45 jamaah",
    quote: "Manifes bus dan absen jamaah bisa saya buka sendiri dari HP. Tidak perlu bawa bawa kertas lagi tiap mau berangkat.",
  },
] as const;
