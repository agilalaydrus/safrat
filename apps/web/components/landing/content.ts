// Structured copy/data for the marketing landing page. Section markup lives in
// the individual components under components/landing/*; this file holds the
// repeated arrays so copy edits stay in one place.

export const NAV_LINKS: [string, string][] = [
  ["Solusi", "#solusi"],
  ["Live Preview", "#demo"],
  ["Kalkulator ROI", "#kalkulator"],
  ["Paket Layanan", "#harga"],
  ["FAQ", "#faq"],
];

export const HERO_TRUST: string[] = [
  "Standar Kemenag RI",
  "Kepatuhan Syariah & Mahram",
  "Bantuan Setup Data Excel Gratis",
];

type BoldItem = { bold: string; rest: string };

export const PROBLEM_ITEMS: BoldItem[] = [
  { bold: "Lembur berjam-jam H-1:", rest: "Mengatur susunan kamar dan bus di Excel hingga larut malam." },
  { bold: "Drama salah kamar mahram:", rest: "Pria dan wanita bukan mahram tidak sengaja sekamar karena salah formula nama." },
  { bold: "Kursi bus tertukar:", rest: "Supir Saudi dan Muthowif memegang versi kertas cetak yang berbeda." },
  { bold: "Panik saat jamaah terpisah:", rest: "Hanya mengandalkan chat grup WhatsApp yang tenggelam." },
];

export const SOLUTION_ITEMS: BoldItem[] = [
  { bold: "Rooming list instan:", rest: "Alokasi Quad, Triple, Double otomatis terverifikasi dalam hitungan detik." },
  { bold: "Validasi mahram otomatis:", rest: "Sistem memblokir penempatan jika terdeteksi bukan pasangan mahram sah." },
  { bold: "Manifest bus digital:", rest: "Supir dan Muthowif cukup buka link web via ponsel tanpa aplikasi berat." },
  { bold: "Gateway SOS 10 Menit:", rest: "Pelacakan lokasi GPS darurat jamaah dengan auto-escalation ke pimpinan." },
];

export type Testimonial = { quote: string; name: string; role: string };

export const TESTIMONIALS: Testimonial[] = [
  {
    quote:
      "Musim kemarin kami terbantu sekali saat ada 2 jamaah sakit H-2 dan harus diganti. Di Tawafiq Hub tinggal substitusi 1 klik tanpa pusing utak-atik rumus Excel kamar hotel.",
    name: "H. Muhammad Rofi'i",
    role: "Direktur Utama - Mabroor Tour (Surabaya)",
  },
  {
    quote:
      "Validasi mahramnya sangat menyelamatkan. Pernah hampir keliru menaruh jamaah pria dan wanita satu kamar, langsung ada tanda bahaya merah dari sistem.",
    name: "Hj. Nurul Aini",
    role: "Manajer Operasional - Al-Multazam Travel (Jakarta)",
  },
  {
    quote:
      "Driver bus Saudi tinggal dikirimi link via WA, langsung tahu siapa saja yang naik di gate mana. Sangat efisien dan mengurangi salah antar koper.",
    name: "Ust. Ahmad Fauzan",
    role: "Kepala Lapangan Saudi - Roudhoh Wisata (Bandung)",
  },
];

export type FaqItem = { q: string; a: string };

export const FAQ_ITEMS: FaqItem[] = [
  {
    q: "Apakah kami masih bisa mengimpor data jamaah dari format Excel lama?",
    a: "Tentu saja! Tawafiq Hub mendukung impor massal file Excel/CSV pendaftaran jamaah. Sistem akan otomatis mencocokkan kolom nama, NIK, nomor paspor, dan relasi keluarga tanpa perlu ketik ulang.",
  },
  {
    q: "Apakah Muthowif dan Supir di Saudi perlu mengunduh aplikasi di PlayStore?",
    a: "Tidak perlu. Muthowif dan Supir bus cukup mengakses link web mobile ringan yang dikirim via WhatsApp. Tampilan dirancang cepat dan hemat kuota bahkan saat sinyal roaming lambat.",
  },
  {
    q: "Bagaimana cara kerja validasi mahram di dalam kamar hotel?",
    a: "Sistem mencocokkan ID Mahram dan gender setiap jamaah. Jamaah laki-laki dan perempuan yang bukan mahram akan otomatis ditolak oleh sistem jika dimasukkan ke dalam kamar yang sama.",
  },
  {
    q: "Apakah tim Tawafiq Hub membantu proses setup awal musim?",
    a: "Ya, setiap pelanggan mendapatkan sesi onboarding 1-on-1 bersama tim spesialis kami untuk migrasi data manifest dan training staf operasional travel Anda.",
  },
];

export type PricingTier = {
  name: string;
  blurb: string;
  // Prices are intentionally placeholders until finalized. `unit` is the period label.
  price: string;
  unit: string;
  features: string[];
  cta: string;
  highlighted?: boolean;
};

export const PRICING_TIERS: PricingTier[] = [
  {
    name: "Starter PPIU",
    blurb: "Cocok untuk travel perintis / skala 1-3 kloter per musim.",
    price: "Hubungi Sales",
    unit: "harga menyusul",
    features: [
      "Hingga 300 Jamaah per Musim",
      "Alokasi Kamar & Validasi Mahram",
      "Manifest Bus & Mobile Link Supir",
      "Ekspor Rooming List Format Saudi",
    ],
    cta: "Pilih Paket Starter",
  },
  {
    name: "Growth Enterprise",
    blurb: "Solusi lengkap untuk PPIU aktif dengan keberangkatan rutin.",
    price: "Hubungi Sales",
    unit: "harga menyusul",
    features: [
      "Hingga 1.500 Jamaah per Musim",
      "Semua Fitur Starter PPIU",
      "Gateway Darurat SOS 10-Menit",
      "Modul Komisi Agen & Mitra Cabang",
      "Migrasi Data Excel Jamaah Dibantu",
    ],
    cta: "Pilih Paket Growth",
    highlighted: true,
  },
  {
    name: "PIHK & Konsorsium",
    blurb: "Untuk travel haji khusus, konsorsium & volume jamaah besar.",
    price: "Custom",
    unit: "kontrak musim",
    features: [
      "Unlimited Jamaah & Kloter",
      "Dedicated Account Manager 24/7",
      "Integrasi API Siskopatuh Custom",
      "Server Private & SLA 99.9%",
    ],
    cta: "Hubungi Enterprise Sales",
  },
];

export const FOOTER_MODULES: [string, string][] = [
  ["Manajemen Jamaah & Paspor", "#solusi"],
  ["Rooming List & Validasi Mahram", "#demo"],
  ["Manifest Bus & Supir Saudi", "#demo"],
  ["Gateway Darurat SOS 10 Menit", "#demo"],
];

export const FOOTER_LAYANAN: [string, string][] = [
  ["Standar Siskopatuh Kemenag", "#solusi"],
  ["Paket Layanan Travel", "#harga"],
  ["Bantuan Migrasi Data Excel", "#faq"],
  ["Panduan Onboarding 1447H", "#faq"],
];

export const WA_LINK = "https://wa.me/628119876000?text=Halo%20TawafiqHub,%20saya%20tertarik%20untuk%20demo%20sistem";
