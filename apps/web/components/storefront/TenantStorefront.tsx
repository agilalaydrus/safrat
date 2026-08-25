"use client";
/* eslint-disable @next/next/no-img-element */
import { useState } from "react";
import type { CSSProperties } from "react";
import Link from "next/link";
import {
  IconArrowRight,
  IconBrandWhatsapp,
  IconBuildingStore,
  IconCalendarEvent,
  IconCertificate,
  IconCheck,
  IconExternalLink,
  IconMapPin,
  IconQuote,
  IconShieldCheck,
} from "@tabler/icons-react";
import { ThemeProvider } from "@/components/landing/ThemeProvider";
import TenantThemeToggle from "@/components/storefront/TenantThemeToggle";

export type StorefrontSeason = {
  id: string;
  name: string;
  slug: string;
  type: string;
  startDate?: string;
  endDate?: string;
  pilgrimCount?: number;
};

export type StorefrontPackage = {
  seasonId: string;
  imageUrl?: string;
  summary?: string;
  priceLabel?: string;
  facilities?: string[];
  itinerary?: { title: string; description?: string }[];
};

export type StorefrontPublicPackage = {
  id: string;
  title: string;
  category?: string;
  summary?: string;
  imageUrl?: string;
  priceLabel?: string;
  durationLabel?: string;
  registrationSlug?: string;
  seasonId?: string;
  facilities?: string[];
  seasons?: StorefrontSeasonOption[];
};

export type StorefrontSeasonOption = {
  seasonId: string;
  hotelMakkah?: string;
  hotelMadinah?: string;
  hotelRating?: string;
  airline?: string;
  seatsRemaining?: number;
  quadPrice?: string;
  triplePrice?: string;
  doublePrice?: string;
};

export type StorefrontArticle = {
  id: string;
  title: string;
  slug: string;
  excerpt?: string;
  body: string;
  coverImageUrl?: string;
  altText?: string;
  author?: string;
  publishedAt?: string;
  seoTitle?: string;
  seoDescription?: string;
};

export type StorefrontContent = {
  displayName?: string;
  logoUrl?: string;
  description?: string;
  whatsappNumber?: string;
  website?: string;
  address?: string;
  city?: string;
  brandColor?: string;
  heroEyebrow?: string;
  heroTitle?: string;
  heroSubtitle?: string;
  heroImageUrl?: string;
  packages?: StorefrontPackage[];
  gallery?: { imageUrl: string; altText: string; caption?: string }[];
  testimonials?: { quote: string; name: string; role?: string }[];
  faqs?: { question: string; answer: string }[];
  publicPackages?: StorefrontPublicPackage[];
  news?: StorefrontArticle[];
  blogPosts?: StorefrontArticle[];
  aboutTitle?: string;
  aboutBody?: string;
  seoTitle?: string;
  seoDescription?: string;
  ogImageUrl?: string;
  agentTitle?: string;
  agentDescription?: string;
  agentApplicationsEnabled?: boolean;
  tagline?: string;
  foundedYear?: number;
  contactEmail?: string;
  mapUrl?: string;
  managerWhatsapp?: string;
  trustBadges?: string[];
};

export type StorefrontProfile = {
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
  activeSeasons?: StorefrontSeason[];
  content?: StorefrontContent;
};

const DEFAULT_BRAND_COLOR = "#059669";
const DEFAULT_HERO_IMAGE = "/images/tenant-umrah-hero.webp";
const HEX_COLOR = /^#[0-9a-f]{6}$/i;

const SEASON_LABEL: Record<string, string> = {
  HAJJ: "Haji",
  UMRAH_REGULER: "Umrah Reguler",
  UMRAH_RAJAB: "Umrah Rajab",
  UMRAH_RAMADHAN: "Umrah Ramadhan",
  UMRAH_SYAWAL: "Umrah Syawal",
  UMRAH_DZULQAIDAH: "Umrah Dzulqaidah",
  SEASON_TYPE_HAJJ: "Haji",
  SEASON_TYPE_UMRAH_REGULER: "Umrah Reguler",
  SEASON_TYPE_UMRAH_RAJAB: "Umrah Rajab",
  SEASON_TYPE_UMRAH_RAMADHAN: "Umrah Ramadhan",
  SEASON_TYPE_UMRAH_SYAWAL: "Umrah Syawal",
  SEASON_TYPE_UMRAH_DZULQAIDAH: "Umrah Dzulqaidah",
};

export default function TenantStorefront({ profile, preview = false }: { profile: StorefrontProfile; preview?: boolean }) {
  const content = profile.content ?? {};
  const name = content.displayName || profile.name;
  const seasons = profile.activeSeasons ?? [];
  const packageContent = new Map((content.packages ?? []).map((item) => [item.seasonId, item]));
  const gallery = content.gallery ?? [];
  const testimonials = content.testimonials ?? [];
  const faqs = content.faqs ?? [];
  const publicPackages = content.publicPackages ?? [];
  const news = content.news ?? [];
  const blogPosts = content.blogPosts ?? [];
  const [lightbox, setLightbox] = useState<{ imageUrl: string; altText: string; caption?: string } | null>(null);
  const [booking, setBooking] = useState<{ packageTitle: string; seasonName?: string; whatsapp: string } | null>(null);
  const initials = name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase();
  const brandColorCandidate = content.brandColor || profile.brandColor;
  const brandColor = brandColorCandidate && HEX_COLOR.test(brandColorCandidate) ? brandColorCandidate : DEFAULT_BRAND_COLOR;
  const heroTitle = content.heroTitle || profile.heroTitle || `Perjalanan ibadah yang tenang bersama ${name}`;
  const heroSubtitle = content.heroSubtitle || profile.heroSubtitle || content.description || profile.description || "Pilih paket, konsultasikan kebutuhan, dan daftar melalui tim travel yang mendampingi perjalanan Anda.";
  const heroEyebrow = content.heroEyebrow || profile.heroEyebrow || "Pendamping perjalanan Umrah dan Haji";
  const heroImage = safeImageLink(content.heroImageUrl || profile.heroImageUrl);
  const logoImage = safeOptionalImageLink(content.logoUrl || profile.logoUrl);
  const website = safeWebLink(content.website || profile.website);
  const whatsappNumber = content.whatsappNumber || profile.whatsappNumber;
  const whatsapp = whatsappNumber ? waLink(whatsappNumber) : null;
  const managerWhatsapp = content.managerWhatsapp ? waLink(content.managerWhatsapp) : whatsapp;
  const address = content.address || profile.address;
  const city = content.city || profile.city;
  const description = content.description || profile.description;
  const themeStyle = { "--tenant-brand": brandColor, "--tenant-brand-text": readableText(brandColor) } as CSSProperties;
  const trustBadges = content.trustBadges?.filter(Boolean) ?? [];

  return (
    <ThemeProvider>
      <main className="tenant-scope min-h-[100dvh]" style={themeStyle}>
        {preview && <div className="tenant-preview-ribbon">Preview draft. Belum dilihat publik.</div>}
        {preview && <div className="tenant-live-bar">Live subdomain: <strong>{profile.slug}.tawafiqhub.id</strong></div>}
        <header className="tenant-nav">
          <div className="mx-auto flex h-[72px] max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
            <a href="#beranda" className="flex min-w-0 items-center gap-3" aria-label={`${name} beranda`}>
              {logoImage ? (
                <img src={logoImage} alt={`Logo ${name}`} className="h-11 w-11 rounded-xl border border-slate-200 object-contain dark:border-slate-700" />
              ) : <span className="tenant-logo-fallback">{initials}</span>}
              <span className="min-w-0">
                <span className="block truncate text-base font-extrabold tracking-tight text-slate-950 sm:text-lg dark:text-slate-100">{name}</span>
                <span className="block truncate text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-500 dark:text-slate-400">Travel Umrah &amp; Haji</span>
              </span>
            </a>
            <nav className="hidden items-center gap-7 text-sm font-semibold text-slate-600 md:flex dark:text-slate-300" aria-label="Navigasi utama">
              <a href="#paket" className="tenant-nav-link">Paket</a>
              {news.length > 0 && <a href="#berita" className="tenant-nav-link">Berita</a>}
              {blogPosts.length > 0 && <a href="#blog" className="tenant-nav-link">Blog</a>}
              {gallery.length > 0 && <a href="#galeri" className="tenant-nav-link">Galeri</a>}
              <a href="#tentang" className="tenant-nav-link">Tentang</a>
              {content.agentApplicationsEnabled && <a href="#agen" className="tenant-nav-link">Agen</a>}
              {faqs.length > 0 && <a href="#faq" className="tenant-nav-link">FAQ</a>}
            </nav>
            <TenantThemeToggle />
          </div>
        </header>

        <section id="beranda" className="tenant-hero scroll-mt-24">
          <div className="mx-auto grid max-w-7xl items-center gap-10 px-4 py-12 sm:px-6 md:min-h-[calc(100dvh-72px)] md:grid-cols-[1.02fr_0.98fr] md:py-16 lg:gap-16 lg:px-8">
            <div className="max-w-2xl">
              <p className="tenant-eyebrow">{heroEyebrow}</p>
              <h1 className="mt-5 text-4xl font-black leading-[1.08] tracking-[-0.035em] text-slate-950 sm:text-5xl lg:text-6xl dark:text-slate-100">{heroTitle}</h1>
              <p className="mt-6 max-w-xl text-base leading-7 text-slate-600 sm:text-lg dark:text-slate-300">{heroSubtitle}</p>
              <div className="mt-8 flex flex-col gap-3 sm:flex-row">
                <a href="#paket" className="tenant-primary-cta">Lihat Paket <IconArrowRight size={18} stroke={1.9} /></a>
                {whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-secondary-cta"><IconBrandWhatsapp size={18} stroke={1.9} /> Konsultasi WhatsApp</a>}
              </div>
            </div>
            <div className="tenant-hero-media">
              <img src={heroImage} alt={`Perjalanan Umrah bersama ${name}`} className="h-full w-full object-cover" fetchPriority="high" />
              <div className="tenant-hero-badge"><IconShieldCheck size={22} stroke={1.8} /><span><strong>Pendampingan terpercaya</strong><small>Dari persiapan hingga kepulangan</small></span></div>
            </div>
          </div>
        </section>

        {(trustBadges.length > 0 || profile.licenseNumber) && <section className="tenant-trust-strip" aria-label="Keunggulan dan legalitas"><div className="mx-auto flex max-w-7xl flex-wrap gap-x-8 gap-y-3 px-4 py-5 sm:px-6 lg:px-8">{profile.licenseNumber && <span><IconCertificate size={18} stroke={1.7} /> Izin PPIU/PIHK {profile.licenseNumber}</span>}{trustBadges.map((badge) => <span key={badge}><IconShieldCheck size={18} stroke={1.7} /> {badge}</span>)}</div></section>}

        <section className="tenant-proof" aria-label="Informasi kepercayaan">
          <div className="mx-auto grid max-w-7xl gap-5 px-4 py-7 sm:grid-cols-3 sm:px-6 lg:px-8">
            <div className="tenant-proof-item"><IconCertificate size={24} stroke={1.7} /><span><strong>{profile.licenseNumber || "Legalitas travel"}</strong><small>{profile.licenseNumber ? "Nomor izin tercatat" : "Informasi dapat dikonfirmasi"}</small></span></div>
            <div className="tenant-proof-item"><IconMapPin size={24} stroke={1.7} /><span><strong>{city || "Kantor travel"}</strong><small>{address || "Alamat tersedia melalui tim kami"}</small></span></div>
            <div className="tenant-proof-item"><IconCheck size={24} stroke={1.7} /><span><strong>Pendaftaran langsung</strong><small>Data masuk ke sistem travel</small></span></div>
          </div>
        </section>

        <section id="paket" className="tenant-section scroll-mt-24">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="max-w-2xl"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Paket perjalanan tersedia</h2><p className="mt-4 text-base leading-7 text-slate-600 dark:text-slate-300">Temukan jadwal yang sesuai, lalu isi formulir pendaftaran langsung untuk paket pilihan Anda.</p></div>
            {publicPackages.length === 0 && seasons.length === 0 ? (
              <div className="tenant-empty mt-10"><IconCalendarEvent size={30} stroke={1.6} /><h3>Jadwal baru sedang disiapkan</h3><p>Hubungi tim travel untuk mendapatkan informasi keberangkatan berikutnya.</p>{whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-secondary-cta">Konsultasi WhatsApp</a>}</div>
            ) : (
              <div className="mt-10 grid gap-5">
                {publicPackages.map((item) => {
                  const packageImage = safeOptionalImageLink(item.imageUrl);
                  const href = item.registrationSlug ? `/register/${item.registrationSlug}` : item.seasonId ? `/register/${item.seasonId}` : "#kontak";
                  return <article key={item.id} className="tenant-package-row"><div className="tenant-package-index">{String(publicPackages.indexOf(item) + 1).padStart(2, "0")}</div>{packageImage && <img src={packageImage} alt={item.title} loading="lazy" className="tenant-package-thumb" />}<div className="min-w-0"><p className="tenant-eyebrow">{item.category || "Paket perjalanan"}</p><h3>{item.title}</h3>{item.summary && <p>{item.summary}</p>}</div><div className="tenant-package-meta"><span>{item.durationLabel || "Konsultasi durasi"}</span><strong>{item.priceLabel || "Hubungi kami"}</strong></div><details className="tenant-package-expand"><summary aria-label={`Detail ${item.title}`}><IconArrowRight size={19} stroke={1.8} /></summary><div>{(item.facilities?.length ?? 0) > 0 && <div className="tenant-package-facilities">{item.facilities?.map((facility) => <span key={facility}><IconCheck size={16} stroke={2} />{facility}</span>)}</div>}{(item.seasons?.length ?? 0) > 0 && <div className="tenant-season-options">{item.seasons?.map((season) => <div key={season.seasonId} className="tenant-season-option"><div><strong>{seasons.find((entry) => entry.id === season.seasonId)?.name || "Musim tersedia"}</strong><span>{season.hotelMakkah || "Hotel Makkah belum diisi"} · {season.hotelMadinah || "Hotel Madinah belum diisi"}</span>{season.airline && <span>{season.airline}{season.hotelRating ? ` · ${season.hotelRating}` : ""}</span>}</div><div><small>{typeof season.seatsRemaining === "number" ? `${season.seatsRemaining} kursi tersisa` : "Kuota hubungi travel"}</small><button type="button" onClick={() => setBooking({ packageTitle: item.title, seasonName: seasons.find((entry) => entry.id === season.seasonId)?.name, whatsapp: managerWhatsapp || "" })} disabled={!managerWhatsapp}>Pesan Kursi</button></div></div>)}</div>}<div className="flex flex-wrap gap-3"><Link href={href} className="tenant-package-link">Lihat pendaftaran <IconArrowRight size={17} stroke={1.9} /></Link>{managerWhatsapp && <button type="button" className="tenant-secondary-cta" onClick={() => setBooking({ packageTitle: item.title, whatsapp: managerWhatsapp })}>Konsultasi</button>}</div></div></details></article>;
                })}
                {seasons.map((season) => {
                  const detail = packageContent.get(season.id);
                  const packageImage = safeOptionalImageLink(detail?.imageUrl);
                  return (
                    <article key={season.id} className="tenant-package overflow-hidden p-0">
                      {packageImage && <div className="aspect-[16/8] overflow-hidden"><img src={packageImage} alt={`Paket ${season.name}`} className="h-full w-full object-cover" /></div>}
                      <div className="p-6 sm:p-7">
                        <div className="flex items-start justify-between gap-4"><span className="tenant-season-label">{SEASON_LABEL[season.type] ?? season.type}</span>{detail?.priceLabel ? <strong className="tenant-brand-ink text-sm">{detail.priceLabel}</strong> : <IconCalendarEvent size={24} stroke={1.6} className="tenant-brand-ink" />}</div>
                        <h3 className="mt-7 text-xl font-extrabold tracking-tight text-slate-950 dark:text-slate-100">{season.name}</h3>
                        <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">{formatMonthYear(season.startDate)}{season.endDate ? ` - ${formatMonthYear(season.endDate)}` : ""}</p>
                        {detail?.summary && <p className="mt-4 text-sm leading-6 text-slate-600 dark:text-slate-300">{detail.summary}</p>}
                        {(detail?.facilities?.length ?? 0) > 0 && <div className="mt-5 grid gap-2 sm:grid-cols-2">{detail?.facilities?.map((facility) => <span key={facility} className="flex items-start gap-2 text-sm text-slate-600 dark:text-slate-300"><IconCheck size={17} stroke={2} className="tenant-brand-ink mt-0.5 shrink-0" />{facility}</span>)}</div>}
                        {(detail?.itinerary?.length ?? 0) > 0 && <details className="tenant-itinerary mt-5"><summary>Lihat itinerary</summary><div>{detail?.itinerary?.map((item, index) => <div key={`${item.title}-${index}`}><strong>{item.title}</strong>{item.description && <p>{item.description}</p>}</div>)}</div></details>}
                        <Link href={`/register/${season.slug}`} className="tenant-package-link">Daftar Paket <IconArrowRight size={17} stroke={1.9} /></Link>
                      </div>
                    </article>
                  );
                })}
              </div>
            )}
          </div>
        </section>

        {gallery.length > 0 && <section id="galeri" className="tenant-gallery-section scroll-mt-24"><div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Momen perjalanan jamaah</h2><div className="tenant-gallery mt-10">{gallery.map((item, index) => <figure key={`${item.imageUrl}-${index}`} className={index === 0 ? "tenant-gallery-featured" : ""}><button type="button" className="tenant-gallery-button" onClick={() => setLightbox(item)} aria-label={`Buka foto ${item.altText}`}><img src={safeImageLink(item.imageUrl)} alt={item.altText} loading="lazy" /></button>{item.caption && <figcaption>{item.caption}</figcaption>}</figure>)}</div></div></section>}

        {news.length > 0 && <ArticleStrip id="berita" title="Berita terbaru" articles={news} />}
        {blogPosts.length > 0 && <ArticleStrip id="blog" title="Catatan perjalanan" articles={blogPosts} />}

        <section id="tentang" className="tenant-about scroll-mt-24"><div className="mx-auto grid max-w-7xl gap-10 px-4 py-20 sm:px-6 md:grid-cols-[1.1fr_0.9fr] lg:px-8 lg:py-24"><div><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">{content.aboutTitle || `Mengenal ${name}`}</h2><p className="mt-6 max-w-2xl whitespace-pre-line text-base leading-8 text-slate-600 dark:text-slate-300">{content.aboutBody || description || `${name} membantu jamaah mempersiapkan perjalanan Umrah dan Haji dengan informasi yang jelas serta pendampingan yang mudah dihubungi.`}</p></div><div className="tenant-about-facts"><div><IconBuildingStore size={22} stroke={1.7} /><span><small>Brand travel</small><strong>{name}</strong></span></div>{city && <div><IconMapPin size={22} stroke={1.7} /><span><small>Lokasi kantor</small><strong>{city}</strong></span></div>}{profile.licenseNumber && <div><IconCertificate size={22} stroke={1.7} /><span><small>Nomor izin</small><strong>{profile.licenseNumber}</strong></span></div>}{website && <a href={website} target="_blank" rel="noreferrer"><IconExternalLink size={22} stroke={1.7} /><span><small>Website resmi</small><strong>Kunjungi website</strong></span></a>}</div></div></section>

        {testimonials.length > 0 && <section className="tenant-testimonials"><div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Cerita dari jamaah</h2><div className="mt-10 grid gap-5 md:grid-cols-2">{testimonials.map((item, index) => <blockquote key={`${item.name}-${index}`} className={index === 0 && testimonials.length > 2 ? "tenant-testimonial-featured" : "tenant-testimonial"}><IconQuote size={26} stroke={1.5} /><p>{item.quote}</p><footer><strong>{item.name}</strong>{item.role && <span>{item.role}</span>}</footer></blockquote>)}</div></div></section>}

        {faqs.length > 0 && <section id="faq" className="tenant-faq scroll-mt-24"><div className="mx-auto max-w-4xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Pertanyaan yang sering diajukan</h2><div className="mt-10 grid gap-3">{faqs.map((item, index) => <details key={`${item.question}-${index}`}><summary>{item.question}</summary><p>{item.answer}</p></details>)}</div></div></section>}

        {content.agentApplicationsEnabled && <section id="agen" className="tenant-section scroll-mt-24"><div className="tenant-contact mx-auto max-w-7xl"><div><h2 className="text-3xl font-black tracking-tight sm:text-4xl">{content.agentTitle || "Bergabung sebagai agen perjalanan"}</h2><p className="mt-4 max-w-xl text-base leading-7 opacity-80">{content.agentDescription || "Bantu lebih banyak keluarga berangkat dengan pendampingan yang aman dan program kemitraan yang jelas."}</p></div><AgentWhatsAppForm managerWhatsapp={managerWhatsapp} /></div></section>}

        {(whatsapp || website) && <section id="kontak" className="scroll-mt-24 px-4 pb-20 sm:px-6 lg:px-8 lg:pb-24"><div className="tenant-contact mx-auto max-w-7xl"><div><h2 className="text-3xl font-black tracking-tight sm:text-4xl">Siap merencanakan perjalanan?</h2><p className="mt-4 max-w-xl text-base leading-7 opacity-80">Tim {name} siap membantu memilih jadwal dan menjawab kebutuhan perjalanan Anda.</p></div><div className="flex flex-col gap-3 sm:flex-row">{whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-contact-cta"><IconBrandWhatsapp size={19} stroke={1.9} /> Konsultasi WhatsApp</a>}{website && <a href={website} target="_blank" rel="noreferrer" className="tenant-contact-ghost"><IconExternalLink size={19} stroke={1.9} /> Website</a>}</div></div></section>}

        <footer className="tenant-footer"><div className="mx-auto grid max-w-7xl gap-10 px-4 py-12 sm:px-6 md:grid-cols-[1.2fr_0.8fr_1fr] lg:px-8"><div><div className="flex items-center gap-3">{logoImage ? <img src={logoImage} alt={`Logo ${name}`} className="h-12 w-12 rounded-xl object-contain" /> : <span className="tenant-logo-fallback">{initials}</span>}<strong>{name}</strong></div><p>{content.tagline || description || "Pendampingan perjalanan ibadah yang hangat dan terpercaya."}</p></div><div><strong>Navigasi</strong><nav className="mt-4 grid gap-2 text-sm"><a href="#beranda">Beranda</a><a href="#paket">Paket</a><a href="#tentang">Tentang Kami</a><a href="#kontak">Hubungi Kami</a></nav></div><div><strong>Kontak &amp; legalitas</strong><p>{address || city || "Alamat kantor tersedia melalui tim travel."}</p>{content.contactEmail && <a href={`mailto:${content.contactEmail}`}>{content.contactEmail}</a>}{profile.licenseNumber && <p className="mt-2">Izin PPIU/PIHK: {profile.licenseNumber}</p>}{content.mapUrl && <a className="tenant-footer-map" href={content.mapUrl} target="_blank" rel="noreferrer">Buka lokasi di Google Maps <IconExternalLink size={15} /></a>}</div></div><div className="mx-auto flex max-w-7xl flex-col gap-3 border-t border-white/10 px-4 py-5 text-sm sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8"><span>© {new Date().getFullYear()} {name}. All Rights Reserved.</span><a href="https://tawafiqhub.id" className="tenant-powered" target="_blank" rel="noreferrer">Powered by <strong>TawafiqHub</strong></a></div></footer>
        {lightbox && <div className="tenant-lightbox" role="dialog" aria-modal="true" aria-label={lightbox.altText} onClick={() => setLightbox(null)}><button type="button" onClick={() => setLightbox(null)} aria-label="Tutup foto">×</button><figure onClick={(event) => event.stopPropagation()}><img src={safeImageLink(lightbox.imageUrl)} alt={lightbox.altText} />{lightbox.caption && <figcaption>{lightbox.caption}</figcaption>}</figure></div>}
        {booking && <BookingModal booking={booking} onClose={() => setBooking(null)} />}
      </main>
    </ThemeProvider>
  );
}

function ArticleStrip({ id, title, articles }: { id: string; title: string; articles: StorefrontArticle[] }) {
  return <section id={id} className="tenant-section scroll-mt-24"><div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">{title}</h2><div className="mt-10 grid gap-6 md:grid-cols-3">{articles.slice(0, 6).map((article) => <article key={article.id} className="tenant-article overflow-hidden"><div className="aspect-[16/9] overflow-hidden">{article.coverImageUrl ? <img src={safeImageLink(article.coverImageUrl)} alt={article.altText || article.title} loading="lazy" className="h-full w-full object-cover" /> : <div className="h-full w-full tenant-article-placeholder" aria-hidden="true" />}</div><div className="p-5"><p className="text-xs font-semibold uppercase tracking-[0.14em] tenant-brand-ink">{article.author || "Catatan travel"}</p><h3 className="mt-3 text-xl font-extrabold tracking-tight text-slate-950 dark:text-slate-100">{article.title}</h3>{article.excerpt && <p className="mt-3 line-clamp-3 text-sm leading-6 text-slate-600 dark:text-slate-300">{article.excerpt}</p>}<Link href={`/${id === "blog" ? "blog" : "berita"}/${article.slug}`} className="tenant-package-link mt-5">Baca selengkapnya <IconArrowRight size={17} stroke={1.9} /></Link></div></article>)}</div></div></section>;
}

function AgentWhatsAppForm({ managerWhatsapp }: { managerWhatsapp: string | null }) {
  const [role, setRole] = useState("Mitra Agen Syiar");
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const submit = (event: React.FormEvent) => { event.preventDefault(); if (!managerWhatsapp || !name.trim() || !phone.trim()) return; const text = encodeURIComponent(`Assalamu'alaikum, saya ingin bergabung sebagai ${role}.\nNama: ${name.trim()}\nWhatsApp: ${phone.trim()}`); window.open(`${managerWhatsapp}?text=${text}`, "_blank", "noopener,noreferrer"); };
  return <form className="tenant-agent-form" onSubmit={submit}><select value={role} onChange={(event) => setRole(event.target.value)} aria-label="Peran kemitraan"><option>Mitra Agen Syiar</option><option>Tour Leader / Muthowif</option></select><input value={name} onChange={(event) => setName(event.target.value)} placeholder="Nama lengkap" aria-label="Nama lengkap" required /><input value={phone} onChange={(event) => setPhone(event.target.value)} placeholder="Nomor WhatsApp aktif" aria-label="Nomor WhatsApp aktif" inputMode="tel" required /><button type="submit" className="tenant-contact-cta" disabled={!managerWhatsapp}>Kirim ke WhatsApp <IconArrowRight size={18} /></button>{!managerWhatsapp && <small>Nomor manajer kemitraan belum diatur oleh travel.</small>}</form>;
}

function BookingModal({ booking, onClose }: { booking: { packageTitle: string; seasonName?: string; whatsapp: string }; onClose: () => void }) {
  const [room, setRoom] = useState("Quad");
  const [count, setCount] = useState("1");
  const [note, setNote] = useState("");
  const submit = (event: React.FormEvent) => { event.preventDefault(); if (!booking.whatsapp) return; const text = encodeURIComponent(`Assalamu'alaikum, saya ingin reservasi.\nPaket: ${booking.packageTitle}\nMusim: ${booking.seasonName || "Konsultasi"}\nTipe kamar: ${room}\nJumlah jamaah: ${count}\nCatatan: ${note || "-"}`); window.open(`${booking.whatsapp}?text=${text}`, "_blank", "noopener,noreferrer"); onClose(); };
  return <div className="tenant-modal-backdrop" role="dialog" aria-modal="true" aria-label="Reservasi paket" onClick={onClose}><form className="tenant-modal" onSubmit={submit} onClick={(event) => event.stopPropagation()}><button type="button" className="tenant-modal-close" onClick={onClose} aria-label="Tutup">×</button><p className="tenant-eyebrow">RESERVASI</p><h2>{booking.packageTitle}</h2><label>Tipe kamar<select value={room} onChange={(event) => setRoom(event.target.value)}><option>Quad</option><option>Triple</option><option>Double</option></select></label><label>Jumlah jamaah<input type="number" min="1" max="99" value={count} onChange={(event) => setCount(event.target.value)} /></label><label>Catatan khusus<textarea value={note} onChange={(event) => setNote(event.target.value)} placeholder="Lansia, kursi roda, atau kebutuhan lain" rows={3} /></label><button type="submit" className="tenant-primary-cta" disabled={!booking.whatsapp}>Lanjut ke WhatsApp <IconArrowRight size={18} /></button></form></div>;
}

function waLink(raw: string) { const digits = raw.replace(/\D/g, ""); return `https://wa.me/${digits.startsWith("0") ? `62${digits.slice(1)}` : digits}`; }
function safeWebLink(raw?: string): string | null { if (!raw) return null; try { const url = new URL(raw); return url.protocol === "https:" || url.protocol === "http:" ? url.toString() : null; } catch { return null; } }
function safeImageLink(raw?: string): string { if (!raw) return DEFAULT_HERO_IMAGE; if (raw.startsWith("/")) return raw; return safeWebLink(raw) ?? DEFAULT_HERO_IMAGE; }
function safeOptionalImageLink(raw?: string): string | null { if (!raw) return null; if (raw.startsWith("/")) return raw; return safeWebLink(raw); }
function formatMonthYear(iso?: string) { return iso ? new Date(iso).toLocaleDateString("id-ID", { month: "short", year: "numeric" }) : ""; }
function readableText(hex: string) { const linear = (channel: number) => channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4; const luminance = linear(Number.parseInt(hex.slice(1, 3), 16) / 255) * 0.2126 + linear(Number.parseInt(hex.slice(3, 5), 16) / 255) * 0.7152 + linear(Number.parseInt(hex.slice(5, 7), 16) / 255) * 0.0722; return luminance > 0.179 ? "#0f172a" : "#f8fafc"; }
