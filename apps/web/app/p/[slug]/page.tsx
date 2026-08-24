import type { CSSProperties } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import {
  IconArrowRight,
  IconBrandWhatsapp,
  IconBuildingStore,
  IconCalendarEvent,
  IconCertificate,
  IconCheck,
  IconExternalLink,
  IconMapPin,
  IconShieldCheck,
} from "@tabler/icons-react";
import { ThemeProvider } from "@/components/landing/ThemeProvider";
import TenantThemeToggle from "@/components/storefront/TenantThemeToggle";
import { buildTenantLinkFromBase } from "@/lib/tenant-link";

type PublicSeason = {
  id: string;
  name: string;
  slug: string;
  type: string;
  startDate?: string;
  endDate?: string;
  pilgrimCount?: number;
};

type PublicProfile = {
  operatorId: string;
  name: string;
  slug: string;
  logoUrl?: string;
  description?: string;
  whatsappNumber?: string;
  website?: string;
  address?: string;
  city?: string;
  licenseNumber?: string;
  country?: string;
  brandColor?: string;
  heroEyebrow?: string;
  heroTitle?: string;
  heroSubtitle?: string;
  heroImageUrl?: string;
  activeSeasons?: PublicSeason[];
};

const DEFAULT_BRAND_COLOR = "#059669";
const DEFAULT_HERO_IMAGE = "/images/tenant-umrah-hero.webp";
const HEX_COLOR = /^#[0-9a-f]{6}$/i;

async function getProfile(slug: string): Promise<PublicProfile | null> {
  try {
    const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/hajj.v1.OperatorService/GetPublicProfile`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
      body: JSON.stringify({ slug }),
      next: { revalidate: 30 },
    });
    if (!response.ok) return null;
    return (await response.json()) as PublicProfile;
  } catch {
    return null;
  }
}

function waLink(raw: string): string {
  const digits = raw.replace(/\D/g, "");
  const normalized = digits.startsWith("0") ? `62${digits.slice(1)}` : digits;
  return `https://wa.me/${normalized}`;
}

function safeWebLink(raw?: string): string | null {
  if (!raw) return null;
  try {
    const url = new URL(raw);
    return url.protocol === "https:" || url.protocol === "http:" ? url.toString() : null;
  } catch {
    return null;
  }
}

function safeImageLink(raw?: string): string {
  if (!raw) return DEFAULT_HERO_IMAGE;
  if (raw.startsWith("/")) return raw;
  return safeWebLink(raw) ?? DEFAULT_HERO_IMAGE;
}

function safeOptionalImageLink(raw?: string): string | null {
  if (!raw) return null;
  if (raw.startsWith("/")) return raw;
  return safeWebLink(raw);
}

function formatMonthYear(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleDateString("id-ID", { month: "short", year: "numeric" });
}

function readableText(hex: string): string {
  const linear = (channel: number) => (channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4);
  const red = linear(Number.parseInt(hex.slice(1, 3), 16) / 255);
  const green = linear(Number.parseInt(hex.slice(3, 5), 16) / 255);
  const blue = linear(Number.parseInt(hex.slice(5, 7), 16) / 255);
  const luminance = red * 0.2126 + green * 0.7152 + blue * 0.0722;
  return luminance > 0.179 ? "#0f172a" : "#f8fafc";
}

const SEASON_LABEL: Record<string, string> = {
  HAJJ: "Haji",
  UMRAH_REGULER: "Umrah Reguler",
  UMRAH_RAJAB: "Umrah Rajab",
  UMRAH_RAMADHAN: "Umrah Ramadhan",
  UMRAH_SYAWAL: "Umrah Syawal",
  UMRAH_DZULQAIDAH: "Umrah Dzulqaidah",
};

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) return { title: "Travel tidak ditemukan" };
  return {
    title: `${profile.name} | Travel Umrah & Haji`,
    description: profile.heroSubtitle || profile.description || `Paket Umrah dan Haji dari ${profile.name}.`,
    alternates: process.env.NEXT_PUBLIC_APP_URL
      ? { canonical: buildTenantLinkFromBase(profile.slug, "/", process.env.NEXT_PUBLIC_APP_URL) }
      : undefined,
  };
}

export default async function OperatorPublicProfile({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const profile = await getProfile(slug);
  if (!profile) notFound();

  const seasons = profile.activeSeasons ?? [];
  const initials = profile.name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0])
    .join("")
    .toUpperCase();
  const brandColor = profile.brandColor && HEX_COLOR.test(profile.brandColor) ? profile.brandColor : DEFAULT_BRAND_COLOR;
  const heroTitle = profile.heroTitle || `Perjalanan ibadah yang tenang bersama ${profile.name}`;
  const heroSubtitle = profile.heroSubtitle || profile.description || "Pilih paket, konsultasikan kebutuhan, dan daftar melalui tim travel yang mendampingi perjalanan Anda.";
  const heroEyebrow = profile.heroEyebrow || "Pendamping perjalanan Umrah dan Haji";
  const heroImage = safeImageLink(profile.heroImageUrl);
  const logoImage = safeOptionalImageLink(profile.logoUrl);
  const website = safeWebLink(profile.website);
  const whatsapp = profile.whatsappNumber ? waLink(profile.whatsappNumber) : null;
  const themeStyle = {
    "--tenant-brand": brandColor,
    "--tenant-brand-text": readableText(brandColor),
  } as CSSProperties;

  return (
    <ThemeProvider>
      <main className="tenant-scope min-h-[100dvh]" style={themeStyle}>
        <header className="tenant-nav">
          <div className="mx-auto flex h-[72px] max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
            <Link href="/" className="flex min-w-0 items-center gap-3" aria-label={`${profile.name} beranda`}>
              {logoImage ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={logoImage} alt={`Logo ${profile.name}`} className="h-11 w-11 rounded-xl border border-slate-200 object-contain dark:border-slate-700" />
              ) : (
                <span className="tenant-logo-fallback">{initials}</span>
              )}
              <span className="min-w-0">
                <span className="block truncate text-base font-extrabold tracking-tight text-slate-950 sm:text-lg dark:text-slate-100">{profile.name}</span>
                <span className="block truncate text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-500 dark:text-slate-400">Travel Umrah &amp; Haji</span>
              </span>
            </Link>

            <nav className="hidden items-center gap-7 text-sm font-semibold text-slate-600 md:flex dark:text-slate-300" aria-label="Navigasi utama">
              <a href="#paket" className="tenant-nav-link">Paket</a>
              <a href="#tentang" className="tenant-nav-link">Tentang</a>
              <a href="#kontak" className="tenant-nav-link">Kontak</a>
            </nav>

            <div className="flex shrink-0 items-center gap-2">
              <TenantThemeToggle />
            </div>
          </div>
        </header>

        <section className="tenant-hero">
          <div className="mx-auto grid max-w-7xl items-center gap-10 px-4 py-12 sm:px-6 md:min-h-[calc(100dvh-72px)] md:grid-cols-[1.02fr_0.98fr] md:py-16 lg:gap-16 lg:px-8">
            <div className="max-w-2xl">
              <p className="tenant-eyebrow">{heroEyebrow}</p>
              <h1 className="mt-5 text-4xl font-black leading-[1.08] tracking-[-0.035em] text-slate-950 sm:text-5xl lg:text-6xl dark:text-slate-100">
                {heroTitle}
              </h1>
              <p className="mt-6 max-w-xl text-base leading-7 text-slate-600 sm:text-lg dark:text-slate-300">{heroSubtitle}</p>
              <div className="mt-8 flex flex-col gap-3 sm:flex-row">
                <a href="#paket" className="tenant-primary-cta">
                  Lihat Paket <IconArrowRight size={18} stroke={1.9} />
                </a>
                {whatsapp && (
                  <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-secondary-cta">
                    <IconBrandWhatsapp size={18} stroke={1.9} /> Konsultasi WhatsApp
                  </a>
                )}
              </div>
            </div>

            <div className="tenant-hero-media">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={heroImage} alt={`Perjalanan Umrah bersama ${profile.name}`} className="h-full w-full object-cover" fetchPriority="high" />
              <div className="tenant-hero-badge">
                <IconShieldCheck size={22} stroke={1.8} />
                <span><strong>Pendampingan terpercaya</strong><small>Dari persiapan hingga kepulangan</small></span>
              </div>
            </div>
          </div>
        </section>

        <section className="tenant-proof" aria-label="Informasi kepercayaan">
          <div className="mx-auto grid max-w-7xl gap-5 px-4 py-7 sm:grid-cols-3 sm:px-6 lg:px-8">
            <div className="tenant-proof-item">
              <IconCertificate size={24} stroke={1.7} />
              <span><strong>{profile.licenseNumber || "Legalitas travel"}</strong><small>{profile.licenseNumber ? "Nomor izin tercatat" : "Informasi dapat dikonfirmasi"}</small></span>
            </div>
            <div className="tenant-proof-item">
              <IconMapPin size={24} stroke={1.7} />
              <span><strong>{profile.city || "Kantor travel"}</strong><small>{profile.address || "Alamat tersedia melalui tim kami"}</small></span>
            </div>
            <div className="tenant-proof-item">
              <IconCheck size={24} stroke={1.7} />
              <span><strong>Pendaftaran langsung</strong><small>Data masuk ke sistem travel</small></span>
            </div>
          </div>
        </section>

        <section id="paket" className="tenant-section scroll-mt-24">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="max-w-2xl">
              <h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Paket perjalanan tersedia</h2>
              <p className="mt-4 text-base leading-7 text-slate-600 dark:text-slate-300">Temukan jadwal yang sesuai, lalu isi formulir pendaftaran langsung untuk paket pilihan Anda.</p>
            </div>

            {seasons.length === 0 ? (
              <div className="tenant-empty mt-10">
                <IconCalendarEvent size={30} stroke={1.6} />
                <h3>Jadwal baru sedang disiapkan</h3>
                <p>Hubungi tim travel untuk mendapatkan informasi keberangkatan berikutnya.</p>
                {whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-secondary-cta">Konsultasi WhatsApp</a>}
              </div>
            ) : (
              <div className="mt-10 grid gap-5 md:grid-cols-2 lg:grid-cols-3">
                {seasons.map((season, index) => (
                  <article key={season.id} className={`tenant-package ${index === 0 && seasons.length > 1 ? "md:col-span-2 lg:col-span-2" : ""}`}>
                    <div className="flex items-start justify-between gap-4">
                      <span className="tenant-season-label">{SEASON_LABEL[season.type] ?? season.type}</span>
                      <IconCalendarEvent size={24} stroke={1.6} className="tenant-brand-ink" />
                    </div>
                    <div className="mt-8">
                      <h3 className="text-xl font-extrabold tracking-tight text-slate-950 dark:text-slate-100">{season.name}</h3>
                      <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
                        {formatMonthYear(season.startDate)}
                        {season.endDate ? ` - ${formatMonthYear(season.endDate)}` : ""}
                      </p>
                      {typeof season.pilgrimCount === "number" && season.pilgrimCount > 0 && (
                        <p className="mt-2 text-xs font-semibold text-slate-500 dark:text-slate-400">{season.pilgrimCount} jamaah telah terdaftar</p>
                      )}
                    </div>
                    <Link href={`/register/${season.slug}`} className="tenant-package-link">
                      Daftar Paket <IconArrowRight size={17} stroke={1.9} />
                    </Link>
                  </article>
                ))}
              </div>
            )}
          </div>
        </section>

        <section id="tentang" className="tenant-about scroll-mt-24">
          <div className="mx-auto grid max-w-7xl gap-10 px-4 py-20 sm:px-6 md:grid-cols-[1.1fr_0.9fr] lg:px-8 lg:py-24">
            <div>
              <h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Mengenal {profile.name}</h2>
              <p className="mt-6 max-w-2xl text-base leading-8 text-slate-600 dark:text-slate-300">
                {profile.description || `${profile.name} membantu jamaah mempersiapkan perjalanan Umrah dan Haji dengan informasi yang jelas serta pendampingan yang mudah dihubungi.`}
              </p>
            </div>
            <div className="tenant-about-facts">
              <div><IconBuildingStore size={22} stroke={1.7} /><span><small>Brand travel</small><strong>{profile.name}</strong></span></div>
              {profile.city && <div><IconMapPin size={22} stroke={1.7} /><span><small>Lokasi kantor</small><strong>{profile.city}</strong></span></div>}
              {profile.licenseNumber && <div><IconCertificate size={22} stroke={1.7} /><span><small>Nomor izin</small><strong>{profile.licenseNumber}</strong></span></div>}
              {website && <a href={website} target="_blank" rel="noreferrer"><IconExternalLink size={22} stroke={1.7} /><span><small>Website resmi</small><strong>Kunjungi website</strong></span></a>}
            </div>
          </div>
        </section>

        {(whatsapp || website) && (
          <section id="kontak" className="scroll-mt-24 px-4 pb-20 sm:px-6 lg:px-8 lg:pb-24">
            <div className="tenant-contact mx-auto max-w-7xl">
              <div>
                <h2 className="text-3xl font-black tracking-tight sm:text-4xl">Siap merencanakan perjalanan?</h2>
                <p className="mt-4 max-w-xl text-base leading-7 opacity-80">Tim {profile.name} siap membantu memilih jadwal dan menjawab kebutuhan perjalanan Anda.</p>
              </div>
              <div className="flex flex-col gap-3 sm:flex-row">
                {whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-contact-cta"><IconBrandWhatsapp size={19} stroke={1.9} /> Konsultasi WhatsApp</a>}
                {website && <a href={website} target="_blank" rel="noreferrer" className="tenant-contact-ghost"><IconExternalLink size={19} stroke={1.9} /> Website</a>}
              </div>
            </div>
          </section>
        )}

        <footer className="border-t border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-950">
          <div className="mx-auto flex max-w-7xl flex-col gap-4 px-4 py-8 text-sm text-slate-500 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8 dark:text-slate-400">
            <p>© {new Date().getFullYear()} {profile.name}. Seluruh hak dilindungi.</p>
            <a href="https://tawafiqhub.id" className="tenant-powered" target="_blank" rel="noreferrer">
              Powered by <strong>TawafiqHub</strong>
            </a>
          </div>
        </footer>
      </main>
    </ThemeProvider>
  );
}
