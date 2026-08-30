import type { Icon } from "@tabler/icons-react";
import {
  IconBuildingStore,
  IconBus,
  IconCashBanknote,
  IconClipboardCheck,
  IconHeartRateMonitor,
  IconIdBadge2,
  IconPlaneDeparture,
  IconShieldLock,
  IconShoppingBag,
  IconUsersGroup,
} from "@tabler/icons-react";

export const NAV_LINKS: [string, string][] = [
  ["Platform", "#platform"],
  ["Solusi", "#solusi"],
  ["Cara Kerja", "#cara-kerja"],
  ["Harga", "#harga"],
  ["FAQ", "#faq"],
];

export type Capability = { title: string; description: string; icon: Icon };

export const CAPABILITIES: Capability[] = [
  {
    title: "Data jamaah",
    description: "Pendaftaran, dokumen, KYC, grup, dan manifest dalam satu profil.",
    icon: IconIdBadge2,
  },
  {
    title: "Keberangkatan",
    description: "Musim, kloter, jadwal, hotel, kamar, kendaraan, dan kursi.",
    icon: IconPlaneDeparture,
  },
  {
    title: "Operasional lapangan",
    description: "Check-in, penugasan tim, monitoring, kesehatan, SOS, dan jamaah terpisah.",
    icon: IconClipboardCheck,
  },
  {
    title: "Bisnis travel",
    description: "Agen, komisi, produk, pesanan, pembayaran, refund, dan arus kas.",
    icon: IconCashBanknote,
  },
];

export const PRODUCT_AREAS: [Capability, Capability, Capability, Capability, Capability, Capability] = [
  {
    title: "Ruang kerja operator",
    description: "Direksi dan tim operasional bekerja dari data yang sama, sesuai peran dan musim aktif.",
    icon: IconUsersGroup,
  },
  {
    title: "Storefront bermerek",
    description: "Travel memiliki katalog dan halaman publik sendiri melalui subdomain atau domain pilihan.",
    icon: IconBuildingStore,
  },
  {
    title: "Portal jamaah",
    description: "Jadwal, dokumen, pengumuman, kesehatan, dan bantuan perjalanan tetap dekat di ponsel.",
    icon: IconHeartRateMonitor,
  },
  {
    title: "Portal tour leader",
    description: "Daftar rombongan, check-in, manifest, komunikasi, dan respons lapangan tersedia saat bergerak.",
    icon: IconBus,
  },
  {
    title: "Jaringan agen",
    description: "Referral, penjualan, komisi, saldo, dan pencairan dapat ditelusuri tanpa rekap terpisah.",
    icon: IconShoppingBag,
  },
  {
    title: "Kontrol dan keamanan",
    description: "Hak akses berbasis peran, autentikasi dua langkah, jejak aktivitas, dan data KYC terenkripsi.",
    icon: IconShieldLock,
  },
];

export const WORKFLOW = [
  {
    title: "Buka pendaftaran",
    description: "Terima data jamaah, dokumen, pembayaran, dan referral agen dari satu alur.",
  },
  {
    title: "Siapkan keberangkatan",
    description: "Susun kloter, kamar, kendaraan, jadwal, perlengkapan, dan penugasan tim.",
  },
  {
    title: "Dampingi di lapangan",
    description: "Pantau check-in, kesehatan, komunikasi, SOS, dan pergerakan rombongan.",
  },
  {
    title: "Tutup perjalanan",
    description: "Selesaikan laporan, arus kas, refund, komisi, dan layanan setelah kepulangan.",
  },
];

export type FaqItem = { q: string; a: string };

export const FAQ_ITEMS: FaqItem[] = [
  {
    q: "Apakah data jamaah lama bisa dipindahkan?",
    a: "Bisa. Tim travel dapat mengimpor data jamaah dari CSV dan melanjutkan pengelolaan data tersebut di musim yang dipilih.",
  },
  {
    q: "Apakah jamaah dan tour leader harus memasang aplikasi?",
    a: "Tidak. Portal jamaah dan tour leader berbasis PWA, sehingga dapat dibuka lewat browser ponsel dan ditambahkan ke layar utama.",
  },
  {
    q: "Bisakah storefront memakai domain travel sendiri?",
    a: "Bisa pada paket Growth dan Custom. Paket Starter menggunakan subdomain TawafiqHub yang dapat dikustom dengan identitas travel.",
  },
  {
    q: "Bagaimana akses data diatur?",
    a: "Akses diberikan sesuai peran. Akun juga dapat memakai verifikasi dua langkah, termasuk akun yang masuk melalui Google.",
  },
  {
    q: "Apakah pembayaran otomatis sudah termasuk?",
    a: "Pencatatan pembayaran, rekonsiliasi transfer bank, refund, dan laporan tersedia. Koneksi payment gateway mengikuti konfigurasi penyedia pada akun travel.",
  },
  {
    q: "Apakah ada bantuan onboarding?",
    a: "Ya. Kami membantu menyiapkan organisasi, musim pertama, struktur tim, dan migrasi data awal agar operasional dapat dimulai dengan rapi.",
  },
];

export type PricingTier = {
  name: string;
  blurb: string;
  price: string;
  unit: string;
  features: string[];
  cta: string;
  highlighted?: boolean;
  contactSales?: boolean;
};

export const PRICING_TIERS: [PricingTier, PricingTier, PricingTier] = [
  {
    name: "Starter PPIU",
    blurb: "Untuk travel yang mulai memusatkan operasional dan layanan jamaah.",
    price: "Rp589.000",
    unit: "bulan",
    features: [
      "Ruang kerja operator dan portal jamaah",
      "Storefront pada subdomain TawafiqHub",
      "Modul jamaah, kloter, hotel, dan transportasi",
      "Agen, produk, pesanan, pembayaran, dan laporan",
      "Bantuan onboarding dan migrasi data awal",
    ],
    cta: "Mulai dengan Starter",
  },
  {
    name: "Growth Enterprise",
    blurb: "Untuk travel yang membutuhkan identitas digital dan kendali lebih luas.",
    price: "Rp789.000",
    unit: "bulan",
    features: [
      "Semua fitur Starter PPIU",
      "Domain travel sendiri dan HTTPS otomatis",
      "Storefront sesuai identitas travel",
      "Portal tim, tour leader, agen, dan jamaah",
      "Kontrol akses dan verifikasi dua langkah",
    ],
    cta: "Pilih Growth",
    highlighted: true,
  },
  {
    name: "Custom Enterprise",
    blurb: "Untuk kebutuhan server, integrasi, dan proses kerja yang lebih khusus.",
    price: "Rp2.489.000",
    unit: "bulan",
    features: [
      "Semua fitur Growth Enterprise",
      "Server terpisah untuk organisasi Anda",
      "Penyesuaian fitur dan integrasi",
      "Kendali data pada lingkungan khusus",
      "Jalur dukungan prioritas",
    ],
    cta: "Diskusikan kebutuhan",
    contactSales: true,
  },
];

export const WA_NUMBER = "6281283031003";
export const WA_NUMBER_DISPLAY = "+62 812-8303-1003";
export const WA_SALES_LINK = `https://wa.me/${WA_NUMBER}?text=${encodeURIComponent(
  "Halo TawafiqHub, saya ingin berdiskusi tentang platform operasional travel",
)}`;
