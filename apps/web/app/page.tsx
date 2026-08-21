"use client";

import Link from "next/link";
import {
  IconArrowRight,
  IconBed,
  IconBuildingHospital,
  IconBus,
  IconDeviceMobile,
  IconMessages,
  IconMoonStars,
  IconShieldCheck,
  IconShoppingCart,
  IconSos,
  IconUserDollar,
  IconUsers,
  IconUsersGroup,
} from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";

const CAPABILITY_BADGES = ["Jamaah & Rombongan", "Kloter & Musim Hijriah", "Akomodasi", "Transportasi", "SOS Real-time", "Produk Digital", "Agen & Komisi"];

const PAIN_POINTS = [
  {
    icon: IconMessages,
    problem: "Data Tersebar di Excel & Grup WhatsApp",
    solution: "Satu database. Staf operator, ketua rombongan, sampai jamaah melihat data yang sama secara real-time, dari perangkat masing-masing.",
  },
  {
    icon: IconSos,
    problem: "SOS Jamaah Telat Direspons",
    solution: "Jamaah kirim SOS satu tombol. Kalau tidak direspons dalam 10 menit, otomatis dieskalasi ke seluruh koordinator operator, bukan menunggu WA dibaca.",
  },
  {
    icon: IconBed,
    problem: "Kamar & Kursi Bus Sering Bentrok",
    solution: "Alokasi kamar dan kursi tervalidasi otomatis: kapasitas, gender, aturan mahram. Substitusi jamaah tidak menghapus riwayat penempatan.",
  },
];

const FEATURES = [
  { icon: IconUsers, title: "Jamaah & Rombongan", desc: "Daftar satu per satu atau impor CSV massal. Substitusi jamaah otomatis tanpa kehilangan riwayat kamar dan kursi." },
  { icon: IconMoonStars, title: "Kloter & Musim Hijriah", desc: "Kelola musim per periode Hijriah: Rajab, Ramadhan, Syawal, Dzulqa'dah, sampai musim Haji itu sendiri." },
  { icon: IconBuildingHospital, title: "Akomodasi", desc: "Alokasikan kamar hotel dengan validasi kapasitas dan aturan gender/mahram otomatis, per musim." },
  { icon: IconBus, title: "Transportasi", desc: "Jadwalkan pergerakan armada, tetapkan kursi per jamaah, pantau manifest kendaraan secara real-time." },
  { icon: IconSos, title: "SOS Darurat", desc: "Satu tombol untuk jamaah, eskalasi otomatis ke koordinator kalau belum direspons. Bukan fitur tempelan." },
  { icon: IconShoppingCart, title: "Produk Digital & Pembayaran", desc: "Jual paket roaming dan layanan tambahan lewat checkout terintegrasi, baik langsung oleh jamaah maupun dicatat manual oleh admin." },
  { icon: IconUserDollar, title: "Agen & Komisi", desc: "Lacak agen rujukan, hitung komisi otomatis dari tiap transaksi, sampai pencairan lewat dompet digital mereka sendiri." },
  { icon: IconDeviceMobile, title: "Aplikasi Ketua Rombongan & Jamaah", desc: "PWA ringan untuk chat, check-in, dan SOS, langsung dari browser tanpa perlu install dari toko aplikasi." },
];

export default function LandingPage() {
  const { data: session, isPending } = authClient.useSession();
  const isAuthenticated = Boolean(session?.user);
  const dashboardHref = "/dashboard";

  return (
    <div style={page}>
      <nav style={nav}>
        <Link href="/" aria-label="Tawafiq Hub home" style={navLogo}>Tawafiq Hub</Link>
        <div style={navLinks}>
          {isAuthenticated ? (
            <>
              <span style={signedIn}>{session?.user?.email}</span>
              <Link href={dashboardHref} style={navCta}>Dashboard</Link>
            </>
          ) : !isPending && (
            <>
              <Link href="/sign-in" style={navLink}>Masuk</Link>
              <Link href="/sign-up" style={navCta}>Buat akun</Link>
            </>
          )}
        </div>
      </nav>

      <section style={hero}>
        <IslamicPattern id="safrat-star-hero" opacity={0.16} />
        <div style={heroGlow} />
        <div style={heroInner}>
          <p style={eyebrow}>UNTUK PPIU &amp; PENYELENGGARA HAJI KHUSUS</p>
          <h1 style={heroTitle}>Kelola Jamaah,<br />Bukan Spreadsheet.</h1>
          <p style={heroSub}>
            Dari pendaftaran jamaah, alokasi kamar dan armada, sampai laporan akhir musim, semua dalam satu dashboard
            yang benar-benar dipakai tim Anda tiap hari, bukan cuma waktu demo.
          </p>
          <div style={ctaRow}>
            {isAuthenticated ? (
              <Link href={dashboardHref} style={heroCta}>Buka dashboard<IconArrowRight size={18} /></Link>
            ) : !isPending && (
              <>
                <Link href="/sign-up" style={heroCta}>Mulai Kelola Musim Anda<IconArrowRight size={18} /></Link>
                <Link href="/sign-in" style={heroOutline}>Masuk ke dashboard</Link>
              </>
            )}
          </div>
          <div style={badgeRow}>
            {CAPABILITY_BADGES.map((label) => <span key={label} style={badge}>{label}</span>)}
          </div>
        </div>
      </section>

      <section style={painSection}>
        <div style={sectionInner}>
          <p className="section-eyebrow" style={{ textAlign: "center" }}>Kenapa Tawafiq Hub Dibuat</p>
          <h2 style={{ ...sectionTitle, textAlign: "center" }}>Yang Bikin Musim Kemarin Pusing</h2>
          <div style={painGrid}>
            {PAIN_POINTS.map(({ icon: Icon, problem, solution }) => (
              <div key={problem} style={painCard}>
                <div style={painIcon}><Icon size={22} /></div>
                <h3 style={painProblem}>{problem}</h3>
                <p style={painSolution}>{solution}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section style={section}>
        <div style={sectionInner}>
          <p className="section-eyebrow" style={{ textAlign: "center" }}>Fitur Utama</p>
          <h2 style={{ ...sectionTitle, textAlign: "center" }}>Semua Modul yang Operator Beneran Butuh</h2>
          <p style={{ ...sectionSub, textAlign: "center", maxWidth: 520, margin: "0 auto 40px" }}>
            Bukan daftar fitur generik. Ini modul yang dipakai dari pendaftaran jamaah pertama sampai jamaah pulang.
          </p>
          <div style={grid}>
            {FEATURES.map(({ icon: Icon, title, desc }) => (
              <div key={title} style={featureCard}>
                <div style={featureIcon}><Icon size={22} color="#fff" /></div>
                <h3 style={featureTitle}>{title}</h3>
                <p style={featureDesc}>{desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section style={trustSection}>
        <div style={trustInner}>
          <IconShieldCheck size={28} color="var(--color-gold-500)" />
          <p style={trustText}>
            Dibangun mengikuti istilah dan alur kerja yang sama dengan yang dipakai PPIU/PIHK sehari-hari: Rombongan, Kloter, Muttawwif. Bukan istilah SaaS generik yang harus diterjemahkan dulu ke operasional Anda.
          </p>
        </div>
      </section>

      <section style={banner}>
        <IslamicPattern id="safrat-star-banner" opacity={0.08} />
        <div style={bannerInner}>
          <p className="section-eyebrow" style={{ color: "var(--color-gold-400)" }}>
            {isAuthenticated ? "Selamat datang kembali" : "Siap Musim Berikutnya?"}
          </p>
          <h2 style={bannerTitle}>Musim Berikutnya,<br />Kelola Lebih Rapi.</h2>
          <p style={bannerText}>Buat musim pertama Anda dalam hitungan menit, gratis untuk mulai.</p>
          <Link href={isAuthenticated ? dashboardHref : "/sign-up"} style={heroCta}>
            {isAuthenticated ? "Buka dashboard" : "Buat Musim Pertama Anda"}<IconArrowRight size={18} />
          </Link>
        </div>
      </section>

      <footer style={footer}>
        <Link href="/" aria-label="Tawafiq Hub home" style={navLogo}>Tawafiq Hub</Link>
        <p style={footerTag}><IconUsersGroup size={14} style={{ verticalAlign: "-2px", marginRight: 6 }} />Platform operator Haji &amp; Umrah terpadu</p>
        <p style={{ fontSize: 12, color: "var(--color-warm-400)", marginTop: 12 }}>© 2026 Tawafiq Hub. Hak cipta dilindungi.</p>
      </footer>
    </div>
  );
}

/** Subtle eight-point star tessellation: a restrained nod to Islamic geometric
 * pattern work, not a literal ornament. Two overlapping rotated squares per
 * tile, stroked at low opacity so it reads as texture, not decoration. */
function IslamicPattern({ id, opacity }: { id: string; opacity: number }) {
  return (
    <svg aria-hidden style={{ position: "absolute", inset: 0, width: "100%", height: "100%", opacity, pointerEvents: "none" }}>
      <defs>
        <pattern id={id} width="56" height="56" patternUnits="userSpaceOnUse">
          <g stroke="var(--color-gold-400)" strokeWidth="1" fill="none">
            <rect x="8" y="8" width="40" height="40" />
            <rect x="8" y="8" width="40" height="40" transform="rotate(45 28 28)" />
          </g>
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill={`url(#${id})`} />
    </svg>
  );
}

const page: React.CSSProperties = { fontFamily: "'Plus Jakarta Sans', sans-serif", background: "var(--color-cream-100)", overflowX: "hidden" };

const nav: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "16px 32px", position: "sticky", top: 0, background: "var(--color-emerald-900)", zIndex: 20, borderBottom: "2px solid var(--color-gold-500)" };
const navLogo: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: 22, fontWeight: 700, color: "var(--color-cream-100)" };
const navLinks: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, minHeight: 34 };
const navLink: React.CSSProperties = { fontSize: 13, fontWeight: 500, color: "rgba(253,249,240,.8)" };
const navCta: React.CSSProperties = { fontSize: 13, fontWeight: 700, background: "linear-gradient(135deg, var(--color-gold-400), var(--color-gold-600))", color: "var(--color-warm-900)", padding: "8px 18px", borderRadius: 7 };
const signedIn: React.CSSProperties = { maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12, color: "rgba(253,249,240,.65)" };

const hero: React.CSSProperties = {
  position: "relative",
  textAlign: "center",
  padding: "104px 32px 84px",
  background: "linear-gradient(175deg, var(--color-cream-100) 0%, var(--color-cream-200) 55%, var(--color-gold-50) 100%)",
  overflow: "hidden",
};
const heroGlow: React.CSSProperties = { position: "absolute", insetInlineEnd: "-10%", top: "-15%", width: 420, height: 420, borderRadius: "50%", background: "radial-gradient(circle, rgba(212,184,106,.20) 0%, transparent 70%)", pointerEvents: "none" };
const heroInner: React.CSSProperties = { position: "relative", maxWidth: 720, margin: "0 auto" };
const eyebrow: React.CSSProperties = { fontSize: 11, fontWeight: 700, letterSpacing: ".14em", color: "var(--color-gold-600)", marginBottom: 18 };
const heroTitle: React.CSSProperties = { fontFamily: "'Fraunces', 'Playfair Display', serif", fontOpticalSizing: "auto", fontSize: "clamp(38px,7vw,66px)", fontWeight: 600, color: "var(--color-emerald-900)", lineHeight: 1.1, marginBottom: 22, letterSpacing: "-.01em" };
const heroSub: React.CSSProperties = { fontSize: 17, color: "var(--color-warm-500)", maxWidth: 540, margin: "0 auto 34px", lineHeight: 1.7 };
const ctaRow: React.CSSProperties = { display: "flex", gap: 12, justifyContent: "center", flexWrap: "wrap", minHeight: 48 };
const heroCta: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, background: "linear-gradient(135deg, var(--color-gold-400), var(--color-gold-600))", color: "var(--color-warm-900)", fontWeight: 700, fontSize: 15, padding: "14px 26px", borderRadius: 9, boxShadow: "0 6px 16px -4px rgba(138,104,32,.35)" };
const heroOutline: React.CSSProperties = { display: "inline-flex", alignItems: "center", fontWeight: 700, fontSize: 15, padding: "14px 26px", borderRadius: 9, background: "transparent", color: "var(--color-emerald-900)", border: "1.5px solid var(--color-emerald-800)" };
const badgeRow: React.CSSProperties = { display: "flex", flexWrap: "wrap", justifyContent: "center", gap: 8, marginTop: 40 };
const badge: React.CSSProperties = { fontSize: 12, fontWeight: 600, color: "var(--color-emerald-900)", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 999, padding: "6px 14px" };

const painSection: React.CSSProperties = { padding: "80px 32px", background: "var(--color-cream-100)" };
const painGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(260px,1fr))", gap: 20, marginTop: 40 };
const painCard: React.CSSProperties = { background: "#fff", borderRadius: 16, padding: "28px 24px", borderInlineStart: "3px solid var(--color-gold-500)", boxShadow: "0 1px 2px rgba(26,20,16,.04)" };
const painIcon: React.CSSProperties = { width: 44, height: 44, borderRadius: 12, display: "grid", placeItems: "center", background: "var(--color-gold-50)", color: "var(--color-gold-800)", marginBottom: 16 };
const painProblem: React.CSSProperties = { fontFamily: "'Fraunces', serif", fontSize: 18, fontWeight: 600, color: "var(--color-emerald-900)", marginBottom: 10 };
const painSolution: React.CSSProperties = { fontSize: 14, color: "var(--color-warm-500)", lineHeight: 1.65 };

const section: React.CSSProperties = { padding: "88px 32px", background: "var(--color-cream-200)" };
const sectionInner: React.CSSProperties = { maxWidth: 1040, margin: "0 auto" };
const sectionTitle: React.CSSProperties = { fontFamily: "'Fraunces', serif", fontSize: "clamp(26px,4vw,34px)", fontWeight: 600, color: "var(--color-emerald-900)", marginBottom: 8 };
const sectionSub: React.CSSProperties = { fontSize: 15, color: "var(--color-warm-400)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(230px,1fr))", gap: 18 };
const featureCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 16, padding: "26px 22px", transition: "transform .15s, box-shadow .15s" };
const featureIcon: React.CSSProperties = { width: 44, height: 44, borderRadius: 12, display: "grid", placeItems: "center", background: "linear-gradient(135deg, var(--color-emerald-700), var(--color-emerald-900))", marginBottom: 16 };
const featureTitle: React.CSSProperties = { fontFamily: "'Fraunces', serif", fontSize: 17, fontWeight: 600, color: "var(--color-emerald-900)", marginBottom: 8 };
const featureDesc: React.CSSProperties = { fontSize: 13.5, color: "var(--color-warm-400)", lineHeight: 1.6 };

const trustSection: React.CSSProperties = { padding: "48px 32px", background: "var(--color-cream-100)", borderTop: "1px solid var(--color-cream-300)", borderBottom: "1px solid var(--color-cream-300)" };
const trustInner: React.CSSProperties = { maxWidth: 720, margin: "0 auto", display: "flex", gap: 16, alignItems: "flex-start" };
const trustText: React.CSSProperties = { fontSize: 14.5, color: "var(--color-warm-500)", lineHeight: 1.7 };

const banner: React.CSSProperties = { position: "relative", background: "linear-gradient(135deg, var(--color-emerald-900) 0%, #061f14 100%)", textAlign: "center", padding: "84px 32px", overflow: "hidden" };
const bannerInner: React.CSSProperties = { position: "relative" };
const bannerTitle: React.CSSProperties = { fontFamily: "'Fraunces', serif", color: "var(--color-cream-100)", fontSize: "clamp(28px,5vw,40px)", fontWeight: 600, margin: "10px 0 14px", lineHeight: 1.15 };
const bannerText: React.CSSProperties = { fontSize: 15, color: "rgba(253,249,240,.72)", maxWidth: 460, margin: "0 auto 30px" };

const footer: React.CSSProperties = { textAlign: "center", padding: "40px 32px 32px", borderTop: "1px solid var(--color-cream-300)" };
const footerTag: React.CSSProperties = { fontSize: 12.5, color: "var(--color-warm-400)", marginTop: 10 };
