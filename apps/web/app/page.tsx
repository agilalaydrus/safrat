"use client";

import Link from "next/link";
import { authClient } from "@/lib/auth-client";

const FEATURES = [
  { icon: "👥", title: "Manajemen Jamaah", desc: "Daftarkan, impor, dan kelola data jamaah dengan mudah. Substitusi otomatis jika ada perubahan." },
  { icon: "🏨", title: "Akomodasi", desc: "Alokasikan kamar hotel, kelola kapasitas, dan pantau hunian secara real-time." },
  { icon: "🚌", title: "Transportasi & Jadwal", desc: "Jadwalkan pergerakan, tetapkan kursi bus, dan lacak status armada." },
  { icon: "📊", title: "Laporan & Ekspor", desc: "Ekspor manifest jamaah, laporan kamar, dan data operasional dalam satu klik." },
];

export default function LandingPage() {
  const { data: session, isPending } = authClient.useSession();
  const isAuthenticated = Boolean(session?.user);
  const dashboardHref = "/dashboard";

  return <div style={page}>
    <nav style={nav}>
      <Link href="/" aria-label="Safrat home" style={navLogo}>Safrat</Link>
      <div style={navLinks}>
        {isAuthenticated ? <>
          <span style={signedIn}>{session?.user?.email}</span>
          <Link href={dashboardHref} style={navCta}>Dashboard</Link>
        </> : !isPending && <>
          <Link href="/sign-in" style={navLink}>Masuk</Link>
          <Link href="/sign-up" style={navCta}>Buat akun</Link>
        </>}
      </div>
    </nav>
    <section style={hero}>
      <p style={eyebrow}>PLATFORM OPERATOR HAJI & UMRAH</p>
      <h1 style={heroTitle}>Setiap perjalanan<br />dimulai di sini</h1>
      <p style={heroSub}>Safrat membantu operator Haji & Umrah mengelola jamaah, akomodasi, dan transportasi dalam satu platform yang terintegrasi.</p>
      <div style={ctaRow}>
        {isAuthenticated ? <Link href={dashboardHref} style={heroCta}>Buka dashboard</Link> : !isPending && <>
          <Link href="/sign-up" style={heroCta}>Mulai sekarang</Link>
          <Link href="/sign-in" style={heroOutline}>Masuk ke dashboard</Link>
        </>}
      </div>
    </section>
    <section style={section}><div style={sectionInner}>
      <p className="section-eyebrow" style={{ textAlign: "center" }}>Fitur Utama</p>
      <h2 style={{ ...sectionTitle, textAlign: "center" }}>Semua yang Anda butuhkan</h2>
      <p style={{ ...sectionSub, textAlign: "center", maxWidth: 480, margin: "0 auto 40px" }}>Dari pendaftaran jamaah hingga laporan akhir musim, Safrat menyederhanakan seluruh operasional perjalanan ibadah Anda.</p>
      <div style={grid}>{FEATURES.map((feature) => <div key={feature.title} style={featureCard}><div style={featureIcon}>{feature.icon}</div><h3 style={featureTitle}>{feature.title}</h3><p style={featureDesc}>{feature.desc}</p></div>)}</div>
    </div></section>
    <section style={banner}>
      <p className="section-eyebrow" style={{ color: "var(--color-gold-400)" }}>{isAuthenticated ? "Selamat datang kembali" : "Siap memulai?"}</p>
      <h2 style={bannerTitle}>Kelola musim Haji & Umrah Anda</h2>
      <p style={bannerText}>Bergabunglah dengan operator yang mempercayakan pengelolaan jamaah kepada Safrat.</p>
      <Link href={isAuthenticated ? dashboardHref : "/sign-up"} style={heroCta}>{isAuthenticated ? "Buka dashboard" : "Daftar gratis"}</Link>
    </section>
    <footer style={footer}><Link href="/" aria-label="Safrat home" style={navLogo}>Safrat</Link><p style={{ fontSize: 12, color: "var(--color-warm-400)", marginTop: 4 }}>© 2026 Safrat. Hak cipta dilindungi.</p></footer>
  </div>;
}

const page: React.CSSProperties = { fontFamily: "'Plus Jakarta Sans', sans-serif", background: "var(--color-cream-100)" };
const nav: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "16px 32px", position: "sticky", top: 0, background: "rgba(253,249,240,.95)", backdropFilter: "blur(8px)", zIndex: 10, borderBottom: "1px solid var(--color-cream-300)" };
const navLogo: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: 22, fontWeight: 700, color: "var(--color-emerald-900)" };
const navLinks: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, minHeight: 34 };
const navLink: React.CSSProperties = { fontSize: 13, fontWeight: 500, color: "var(--color-warm-500)" };
const navCta: React.CSSProperties = { fontSize: 13, fontWeight: 600, background: "var(--color-emerald-900)", color: "var(--color-cream-100)", padding: "8px 18px", borderRadius: 7 };
const signedIn: React.CSSProperties = { maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12, color: "var(--color-warm-500)" };
const hero: React.CSSProperties = { textAlign: "center", padding: "96px 32px 80px", maxWidth: 700, margin: "0 auto" };
const eyebrow: React.CSSProperties = { fontSize: 10, fontWeight: 700, letterSpacing: ".1em", color: "var(--color-gold-600)", marginBottom: 16 };
const heroTitle: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: "clamp(40px,8vw,68px)", fontWeight: 500, color: "var(--color-emerald-900)", lineHeight: 1.1, marginBottom: 20 };
const heroSub: React.CSSProperties = { fontSize: 17, color: "var(--color-warm-500)", maxWidth: 520, margin: "0 auto 32px", lineHeight: 1.7 };
const ctaRow: React.CSSProperties = { display: "flex", gap: 12, justifyContent: "center", flexWrap: "wrap", minHeight: 48 };
const heroCta: React.CSSProperties = { display: "inline-block", background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700, fontSize: 15, padding: "14px 28px", borderRadius: 8 };
const heroOutline: React.CSSProperties = { ...heroCta, background: "transparent", color: "var(--color-emerald-900)", border: "1.5px solid var(--color-emerald-900)" };
const section: React.CSSProperties = { padding: "80px 32px", background: "var(--color-cream-200)" };
const sectionInner: React.CSSProperties = { maxWidth: 960, margin: "0 auto" };
const sectionTitle: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: 30, fontWeight: 500, color: "var(--color-emerald-900)", marginBottom: 8 };
const sectionSub: React.CSSProperties = { fontSize: 15, color: "var(--color-warm-400)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 16 };
const featureCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "24px 20px" };
const featureIcon: React.CSSProperties = { fontSize: 28, marginBottom: 14 };
const featureTitle: React.CSSProperties = { fontFamily: "'Playfair Display', serif", fontSize: 16, fontWeight: 500, color: "var(--color-emerald-900)", marginBottom: 8 };
const featureDesc: React.CSSProperties = { fontSize: 13, color: "var(--color-warm-400)" };
const banner: React.CSSProperties = { background: "var(--color-emerald-900)", textAlign: "center", padding: "80px 32px" };
const bannerTitle: React.CSSProperties = { fontFamily: "'Playfair Display', serif", color: "#fff", fontSize: 32, fontWeight: 500, margin: "8px 0 12px" };
const bannerText: React.CSSProperties = { fontSize: 15, color: "rgba(255,255,255,.7)", maxWidth: 460, margin: "0 auto 28px" };
const footer: React.CSSProperties = { textAlign: "center", padding: 32, borderTop: "1px solid var(--color-cream-300)" };
