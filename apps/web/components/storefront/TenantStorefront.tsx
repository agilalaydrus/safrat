"use client";
/* eslint-disable @next/next/no-img-element */
import { useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import Link from "next/link";
import {
  IconArrowRight,
  IconBrandFacebook,
  IconBrandInstagram,
  IconBrandLinkedin,
  IconBrandThreads,
  IconBrandTiktok,
  IconBrandWhatsapp,
  IconBrandX,
  IconBrandYoutube,
  IconBuildingStore,
  IconCalendarEvent,
  IconCertificate,
  IconCheck,
  IconExternalLink,
  IconMapPin,
  IconMenu2,
  IconPackages,
  IconPlayerPlay,
  IconQuote,
  IconShieldCheck,
  IconVolume,
  IconVolumeOff,
  IconX,
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

export type StorefrontTheme = {
  accentColor?: string;
  secondaryColor?: string;
  lightBackgroundColor?: string;
  lightSurfaceColor?: string;
  lightHeadingColor?: string;
  lightBodyColor?: string;
  lightMutedColor?: string;
  darkBackgroundColor?: string;
  darkSurfaceColor?: string;
  darkHeadingColor?: string;
  darkBodyColor?: string;
  darkMutedColor?: string;
  heroHeadingColor?: string;
  heroBodyColor?: string;
};

export type StorefrontSocialLink = {
  platform: "instagram" | "tiktok" | "youtube" | "facebook" | "linkedin" | "threads" | "x" | "whatsapp";
  label: string;
  handle?: string;
  url: string;
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
  theme?: StorefrontTheme;
  socialLinks?: StorefrontSocialLink[];
  socialTitle?: string;
  socialDescription?: string;
  backgroundMusicUrl?: string;
  backgroundMusicTitle?: string;
  backgroundMusicEnabled?: boolean;
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
export const DEFAULT_STOREFRONT_THEME: Required<StorefrontTheme> = {
  accentColor: "#d2a84b",
  secondaryColor: "#07825f",
  lightBackgroundColor: "#f7f8f5",
  lightSurfaceColor: "#ffffff",
  lightHeadingColor: "#142019",
  lightBodyColor: "#46544b",
  lightMutedColor: "#66736b",
  darkBackgroundColor: "#07110d",
  darkSurfaceColor: "#101b16",
  darkHeadingColor: "#f5f1e8",
  darkBodyColor: "#c5cec8",
  darkMutedColor: "#89968e",
  heroHeadingColor: "#fffdf7",
  heroBodyColor: "#d8e0db",
};
const DEFAULT_HERO_IMAGE = "/images/tenant-editorial/makkah_madinah_panoramic_1787650211904.webp";
const DEFAULT_GALLERY = [
  { imageUrl: "/images/tenant-editorial/hero_kaaba_candid_1787645070767.webp", altText: "Jamaah di pelataran Masjidil Haram", caption: "Pendampingan jamaah dimulai dari momen pertama di Tanah Suci." },
  { imageUrl: "/images/tenant-editorial/about_pilgrim_editorial_1787645090421.webp", altText: "Pembimbing ibadah bersama jamaah", caption: "Arahan yang jelas agar ibadah terasa lebih tenang." },
  { imageUrl: "/images/tenant-editorial/gallery_elderly_pilgrim_1787645158498.webp", altText: "Pendampingan jamaah lansia", caption: "Perhatian yang dekat untuk jamaah dan keluarga." },
  { imageUrl: "/images/tenant-editorial/gallery_tasbih_macro_1787645138667.webp", altText: "Tasbih dan suasana ibadah", caption: "Ruang untuk menjaga ritme ibadah dengan khidmat." },
  { imageUrl: "/images/tenant-editorial/blog_nabawi_dusk_1787645116989.webp", altText: "Senja di Masjid Nabawi", caption: "Madinah dalam suasana sore yang teduh." },
  { imageUrl: "/images/tenant-editorial/gallery_communal_dinner_1787645180550.webp", altText: "Makan bersama rombongan", caption: "Kebersamaan rombongan yang dirawat sepanjang perjalanan." },
  { imageUrl: "/images/tenant-editorial/happy_pilgrim_family_1787650258249.webp", altText: "Keluarga jamaah dalam perjalanan ibadah", caption: "Perjalanan yang nyaman memberi ruang bagi keluarga untuk beribadah bersama." },
  { imageUrl: "/images/tenant-editorial/hotel_view_haram_1787650244269.webp", altText: "Pemandangan hotel dekat Masjidil Haram", caption: "Pilihan akomodasi yang membantu jamaah menjaga ritme ibadah." },
  { imageUrl: "/images/tenant-editorial/muthowif_team_natural_1787650228839.webp", altText: "Tim muthawwif mendampingi jamaah", caption: "Tim lapangan siap mendampingi kebutuhan jamaah selama di Tanah Suci." },
];
const DEFAULT_TRUST_BADGES = ["Izin dan dokumen transparan", "Hotel dekat masjid", "Pendampingan manasik", "Layanan WhatsApp responsif"];
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

const DEFAULT_NEWS: StorefrontArticle[] = [
  { id: "default-news-1", title: "Informasi regulasi perjalanan akan diperbarui di sini", slug: "", excerpt: "Travel dapat menerbitkan pengumuman izin, jadwal, dan ketentuan perjalanan dalam ruang berita ini.", body: "", author: "Redaksi travel", publishedAt: "" },
  { id: "default-news-2", title: "Panduan dokumen sebelum berangkat ke Tanah Suci", slug: "", excerpt: "Checklist dokumen yang perlu disiapkan jamaah bersama tim travel sebelum hari keberangkatan.", body: "", author: "Tim layanan jamaah", publishedAt: "" },
  { id: "default-news-3", title: "Jadwal manasik dan layanan jamaah", slug: "", excerpt: "Gunakan ruang ini untuk mengabarkan jadwal manasik, briefing, dan layanan pendampingan rombongan.", body: "", author: "Informasi travel", publishedAt: "" },
];
const DEFAULT_BLOG: StorefrontArticle[] = [
  { id: "default-blog-1", title: "Cara memilih paket Umrah untuk keluarga", slug: "", excerpt: "Pertimbangkan ritme ibadah, jarak hotel, kebutuhan lansia, dan pola pendampingan sebelum memilih paket.", body: "", author: "Catatan perjalanan", publishedAt: "" },
  { id: "default-blog-2", title: "Persiapan fisik agar ibadah lebih nyaman", slug: "", excerpt: "Latihan ringan dan kebiasaan sederhana dapat membantu jamaah menjaga tenaga selama di Makkah dan Madinah.", body: "", author: "Catatan manasik", publishedAt: "" },
  { id: "default-blog-3", title: "Memahami pilihan kamar Quad, Triple, dan Double", slug: "", excerpt: "Kenali perbedaan tipe kamar agar keluarga dapat menyusun anggaran dan kebutuhan perjalanan dengan lebih mudah.", body: "", author: "Panduan jamaah", publishedAt: "" },
];

export default function TenantStorefront({ profile, preview = false }: { profile: StorefrontProfile; preview?: boolean }) {
  const content = profile.content ?? {};
  const name = content.displayName || profile.name;
  const seasons = profile.activeSeasons ?? [];
  const packageContent = new Map((content.packages ?? []).map((item) => [item.seasonId, item]));
  const gallery = content.gallery?.length ? content.gallery : DEFAULT_GALLERY;
  const testimonials = content.testimonials ?? [];
  const faqs = content.faqs ?? [];
  const publicPackages = content.publicPackages ?? [];
  const packageRows: StorefrontPublicPackage[] = publicPackages.length > 0 ? publicPackages : seasons.map((season) => {
    const detail = packageContent.get(season.id);
    return { id: season.id, title: season.name, category: SEASON_LABEL[season.type] ?? season.type, summary: detail?.summary, imageUrl: detail?.imageUrl, priceLabel: detail?.priceLabel, durationLabel: `${formatMonthYear(season.startDate)}${season.endDate ? ` sampai ${formatMonthYear(season.endDate)}` : ""}`, registrationSlug: season.slug, seasonId: season.id, facilities: detail?.facilities, seasons: [{ seasonId: season.id }] };
  });
  const news = content.news?.length ? content.news : DEFAULT_NEWS;
  const blogPosts = content.blogPosts?.length ? content.blogPosts : DEFAULT_BLOG;
  const [editorialTab, setEditorialTab] = useState<"news" | "blog">("news");
  const [lightbox, setLightbox] = useState<{ imageUrl: string; altText: string; caption?: string } | null>(null);
  const [booking, setBooking] = useState<{ packageTitle: string; seasonName?: string; whatsapp: string } | null>(null);
  const [accessOpen, setAccessOpen] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [activeSection, setActiveSection] = useState("beranda");
  const [navSolid, setNavSolid] = useState(false);
  const [musicMuted, setMusicMuted] = useState(false);
  const [musicPlaying, setMusicPlaying] = useState(false);
  const [musicBlocked, setMusicBlocked] = useState(false);
  const heroRef = useRef<HTMLElement>(null);
  const audioRef = useRef<HTMLAudioElement>(null);
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
  const theme = resolveTheme(content.theme, brandColor);
  const themeStyle = {
    "--tenant-brand": theme.accentColor,
    "--tenant-brand-text": readableText(theme.accentColor),
    "--tenant-secondary": theme.secondaryColor,
    "--tenant-light-bg": theme.lightBackgroundColor,
    "--tenant-light-surface": theme.lightSurfaceColor,
    "--tenant-light-heading": theme.lightHeadingColor,
    "--tenant-light-body": theme.lightBodyColor,
    "--tenant-light-muted": theme.lightMutedColor,
    "--tenant-dark-bg": theme.darkBackgroundColor,
    "--tenant-dark-surface": theme.darkSurfaceColor,
    "--tenant-dark-heading": theme.darkHeadingColor,
    "--tenant-dark-body": theme.darkBodyColor,
    "--tenant-dark-muted": theme.darkMutedColor,
    "--tenant-hero-heading": theme.heroHeadingColor,
    "--tenant-hero-body": theme.heroBodyColor,
  } as CSSProperties;
  const configuredTrustBadges = content.trustBadges?.filter(Boolean) ?? [];
  const trustBadges = configuredTrustBadges.length ? configuredTrustBadges : DEFAULT_TRUST_BADGES;
  const socialLinks = (content.socialLinks ?? []).filter((item) => safeWebLink(item.url));
  const musicURL = content.backgroundMusicEnabled ? safeWebLink(content.backgroundMusicUrl) : null;
  const navItems = [
    { id: "beranda", label: "Beranda" },
    { id: "profil", label: "Profil" },
    { id: "paket", label: "Paket" },
    { id: "berita", label: "Artikel" },
    { id: "galeri", label: "Galeri" },
    ...(socialLinks.length ? [{ id: "sosial", label: "Sosial" }] : []),
    { id: "tentang", label: "Tentang" },
    { id: "kontak", label: "Kontak" },
    { id: "agen", label: "Agen" },
    ...(faqs.length ? [{ id: "faq", label: "FAQ" }] : []),
  ];
  const navSectionIDs = navItems.map((item) => item.id).join(",");

  useEffect(() => {
    const hero = heroRef.current;
    if (!hero) return;
    const observer = new IntersectionObserver(([entry]) => { if (entry) setNavSolid(!entry.isIntersecting); }, {
      rootMargin: "-72px 0px -78% 0px",
      threshold: 0,
    });
    observer.observe(hero);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const sections = navSectionIDs.split(",").map((id) => document.getElementById(id)).filter((section): section is HTMLElement => Boolean(section));
    const observer = new IntersectionObserver((entries) => {
      const visible = entries.filter((entry) => entry.isIntersecting).sort((first, second) => second.intersectionRatio - first.intersectionRatio)[0];
      if (visible?.target.id) setActiveSection(visible.target.id);
    }, { rootMargin: "-24% 0px -64% 0px", threshold: [0, 0.1, 0.35] });
    sections.forEach((section) => observer.observe(section));
    return () => observer.disconnect();
  }, [navSectionIDs]);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !musicURL) return;
    const storedMuted = window.localStorage.getItem("tawafiq-storefront-music-muted") === "true";
    audio.muted = storedMuted;
    setMusicMuted(storedMuted);
    let interactionFallback = false;
    const play = async () => {
      try {
        await audio.play();
        setMusicPlaying(true);
        setMusicBlocked(false);
      } catch {
        setMusicPlaying(false);
        setMusicBlocked(true);
        interactionFallback = true;
      }
    };
    const resumeAfterInteraction = () => {
      if (interactionFallback) void play();
    };
    void play();
    document.addEventListener("pointerdown", resumeAfterInteraction, { once: true });
    document.addEventListener("keydown", resumeAfterInteraction, { once: true });
    return () => {
      interactionFallback = false;
      document.removeEventListener("pointerdown", resumeAfterInteraction);
      document.removeEventListener("keydown", resumeAfterInteraction);
      audio.pause();
    };
  }, [musicURL]);

  const toggleMusic = async () => {
    const audio = audioRef.current;
    if (!audio) return;
    if (!musicPlaying) {
      audio.muted = false;
      setMusicMuted(false);
      window.localStorage.setItem("tawafiq-storefront-music-muted", "false");
      try { await audio.play(); setMusicPlaying(true); setMusicBlocked(false); } catch { setMusicBlocked(true); }
      return;
    }
    const nextMuted = !audio.muted;
    audio.muted = nextMuted;
    setMusicMuted(nextMuted);
    window.localStorage.setItem("tawafiq-storefront-music-muted", String(nextMuted));
  };

  const selectSection = (id: string) => {
    setActiveSection(id);
    setMobileNavOpen(false);
  };

  return (
    <ThemeProvider>
      <main className="tenant-scope min-h-[100dvh]" style={themeStyle}>
        {preview && <div className="tenant-preview-ribbon">Preview draft. Belum dilihat publik.</div>}
        {preview && <div className="tenant-live-bar">Live subdomain: <strong>{profile.slug}.tawafiqhub.id</strong></div>}
        <header className={`tenant-nav${navSolid ? " is-scrolled" : " is-transparent"}${mobileNavOpen ? " is-menu-open" : ""}`}>
          <div className="mx-auto flex h-[72px] max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
            <a href="#beranda" className="tenant-nav-brand flex min-w-0 items-center gap-3" aria-label={`${name} beranda`} onClick={() => selectSection("beranda")}>
              {logoImage ? (
                <img src={logoImage} alt={`Logo ${name}`} className="h-11 w-11 rounded-xl border border-slate-200 object-contain dark:border-slate-700" />
              ) : <span className="tenant-logo-fallback">{initials}</span>}
              <span className="min-w-0">
                <span className="block truncate text-base font-extrabold tracking-tight text-slate-950 sm:text-lg dark:text-slate-100">{name}</span>
                <span className="block truncate text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-500 dark:text-slate-400">Travel Umrah &amp; Haji</span>
              </span>
            </a>
            <nav className="tenant-nav-primary hidden items-center lg:flex" aria-label="Navigasi utama">
              {navItems.map((item) => <a key={item.id} href={`#${item.id}`} className={`tenant-nav-link${activeSection === item.id ? " is-active" : ""}`} aria-current={activeSection === item.id ? "location" : undefined} onClick={() => selectSection(item.id)}>{item.label}</a>)}
            </nav>
            <div className="tenant-nav-actions"><a href="/sign-in" className="tenant-login-link">Masuk</a><button type="button" className="tenant-register-button" onClick={() => setAccessOpen(true)}>Daftar</button><TenantThemeToggle /><button type="button" className="tenant-mobile-menu-button lg:hidden" aria-expanded={mobileNavOpen} aria-controls="tenant-mobile-nav" aria-label={mobileNavOpen ? "Tutup navigasi" : "Buka navigasi"} onClick={() => setMobileNavOpen((open) => !open)}>{mobileNavOpen ? <IconX size={20} stroke={1.9} /> : <IconMenu2 size={20} stroke={1.9} />}</button></div>
          </div>
          <nav id="tenant-mobile-nav" className="tenant-mobile-nav lg:hidden" aria-label="Navigasi seluler" hidden={!mobileNavOpen}>{navItems.map((item) => <a key={item.id} href={`#${item.id}`} className={activeSection === item.id ? "is-active" : ""} aria-current={activeSection === item.id ? "location" : undefined} onClick={() => selectSection(item.id)}>{item.label}</a>)}</nav>
        </header>

        <section ref={heroRef} id="beranda" className="tenant-hero scroll-mt-24">
          <img src={heroImage} alt={`Perjalanan Umrah bersama ${name}`} className="tenant-hero-backdrop" fetchPriority="high" />
          <div className="tenant-hero-scrim" aria-hidden="true" />
          <div className="tenant-hero-content mx-auto flex max-w-7xl items-center justify-center px-4 py-14 text-center sm:px-6 lg:px-8">
            <div className="tenant-hero-copy max-w-4xl">
              <p className="tenant-eyebrow">{heroEyebrow}</p>
              <h1 className="mt-5 text-4xl font-black leading-[1.06] tracking-[-0.035em] sm:text-5xl lg:text-6xl">{heroTitle}</h1>
              <p className="mx-auto mt-6 max-w-2xl text-base leading-7 sm:text-lg">{heroSubtitle}</p>
              <div className="mt-8 flex flex-col justify-center gap-3 sm:flex-row">
                <a href="#paket" className="tenant-primary-cta">Lihat Paket <IconArrowRight size={18} stroke={1.9} /></a>
                {whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-hero-secondary"><IconBrandWhatsapp size={18} stroke={1.9} /> Konsultasi WhatsApp</a>}
              </div>
              <div className="tenant-hero-assurance"><IconShieldCheck size={20} stroke={1.8} /><span><strong>Pendampingan terpercaya</strong><small>Dari persiapan hingga kepulangan</small></span></div>
            </div>
          </div>
        </section>

        {(trustBadges.length > 0 || profile.licenseNumber) && <section className="tenant-trust-strip" aria-label="Keunggulan dan legalitas"><div className="mx-auto flex max-w-7xl flex-wrap gap-x-8 gap-y-3 px-4 py-5 sm:px-6 lg:px-8">{profile.licenseNumber && <span><IconCertificate size={18} stroke={1.7} /> Izin PPIU/PIHK {profile.licenseNumber}</span>}{trustBadges.map((badge) => <span key={badge}><IconShieldCheck size={18} stroke={1.7} /> {badge}</span>)}</div></section>}

        <section className="tenant-proof" aria-label="Empat pilar jaminan perjalanan">
          <div className="mx-auto grid max-w-7xl gap-5 px-4 py-7 sm:grid-cols-2 sm:px-6 lg:grid-cols-4 lg:px-8">
            <div className="tenant-proof-item"><IconCertificate size={24} stroke={1.7} /><span><strong>Izin yang jelas</strong><small>{profile.licenseNumber ? `PPIU/PIHK ${profile.licenseNumber}` : "Legalitas dapat dikonfirmasi"}</small></span></div>
            <div className="tenant-proof-item"><IconBuildingStore size={24} stroke={1.7} /><span><strong>Hotel terpilih</strong><small>Dekat masjid sesuai program</small></span></div>
            <div className="tenant-proof-item"><IconCalendarEvent size={24} stroke={1.7} /><span><strong>Jadwal terencana</strong><small>Musim dan keberangkatan transparan</small></span></div>
            <div className="tenant-proof-item"><IconShieldCheck size={24} stroke={1.7} /><span><strong>Pendampingan utuh</strong><small>Dari manasik sampai pulang</small></span></div>
          </div>
        </section>

        <section id="profil" className="tenant-profile scroll-mt-24">
          <div className="mx-auto grid max-w-7xl gap-10 px-4 py-20 sm:px-6 md:grid-cols-[0.82fr_1.18fr] lg:px-8 lg:py-24">
            <div className="tenant-profile-visual"><img src={safeImageLink(heroImage)} alt={`Kantor dan layanan ${name}`} loading="lazy" /><div><span>Sejak {content.foundedYear || "berpengalaman"}</span><strong>{content.tagline || "Melayani perjalanan ibadah dengan amanah"}</strong></div></div>
            <div className="max-w-2xl"><p className="tenant-eyebrow">Profil biro perjalanan</p><h2 className="mt-4 text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Rencana ibadah yang disusun dengan tanggung jawab</h2><p className="mt-6 text-base leading-8 text-slate-600 dark:text-slate-300">{description || `${name} mendampingi jamaah Umrah dan Haji Khusus melalui persiapan yang tertib, informasi yang terbuka, dan tim yang mudah dihubungi.`}</p><div className="tenant-profile-legal mt-8"><div><span>Legalitas PPIU / PIHK</span><strong>{profile.licenseNumber || "Nomor izin tersedia melalui tim travel"}</strong></div><div><span>Alamat kantor</span><strong>{[address, city].filter(Boolean).join(", ") || "Silakan hubungi kami untuk alamat kantor"}</strong></div></div></div>
          </div>
        </section>

        <section id="paket" className="tenant-section scroll-mt-24">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="max-w-2xl"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Paket perjalanan tersedia</h2><p className="mt-4 text-base leading-7 text-slate-600 dark:text-slate-300">Temukan jadwal yang sesuai, lalu isi formulir pendaftaran langsung untuk paket pilihan Anda.</p></div>
            {packageRows.length === 0 ? (
              <div className="tenant-empty mt-10"><IconCalendarEvent size={30} stroke={1.6} /><h3>Jadwal baru sedang disiapkan</h3><p>Hubungi tim travel untuk mendapatkan informasi keberangkatan berikutnya.</p>{whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-secondary-cta">Konsultasi WhatsApp</a>}</div>
            ) : (
              <div className="mt-10 grid gap-5">
                {packageRows.map((item, packageIndex) => {
                  const packageImage = safeOptionalImageLink(item.imageUrl);
                  const href = item.registrationSlug ? `/register/${item.registrationSlug}` : item.seasonId ? `/register/${item.seasonId}` : "#kontak";
                  return <article key={item.id} className="tenant-package-row"><div className="tenant-package-index">{String(packageIndex + 1).padStart(2, "0")}</div>{packageImage && <img src={packageImage} alt={item.title} loading="lazy" className="tenant-package-thumb" />}<div className="min-w-0"><p className="tenant-eyebrow">{item.category || "Paket perjalanan"}</p><h3>{item.title}</h3>{item.summary && <p>{item.summary}</p>}</div><div className="tenant-package-meta"><span>{item.durationLabel || "Konsultasi durasi"}</span><strong>{item.priceLabel || "Hubungi kami"}</strong></div><details className="tenant-package-expand"><summary aria-label={`Detail ${item.title}`}><IconArrowRight size={19} stroke={1.8} /></summary><div>{(item.facilities?.length ?? 0) > 0 && <div className="tenant-package-facilities">{item.facilities?.map((facility) => <span key={facility}><IconCheck size={16} stroke={2} />{facility}</span>)}</div>}{(item.seasons?.length ?? 0) > 0 && <div className="tenant-season-options">{item.seasons?.map((season) => <div key={season.seasonId} className="tenant-season-option"><div><strong>{seasons.find((entry) => entry.id === season.seasonId)?.name || "Musim tersedia"}</strong><span>{season.hotelMakkah || "Hotel Makkah akan dikonfirmasi"} · {season.hotelMadinah || "Hotel Madinah akan dikonfirmasi"}</span>{season.airline && <span>{season.airline}{season.hotelRating ? ` · ${season.hotelRating}` : ""}</span>}<div className="tenant-room-prices"><span><b>Quad</b>{season.quadPrice || "Hubungi kami"}</span><span><b>Triple</b>{season.triplePrice || "Hubungi kami"}</span><span><b>Double</b>{season.doublePrice || "Hubungi kami"}</span></div></div><div><small>{typeof season.seatsRemaining === "number" ? `${season.seatsRemaining} kursi tersisa` : "Kuota hubungi travel"}</small><button type="button" onClick={() => setBooking({ packageTitle: item.title, seasonName: seasons.find((entry) => entry.id === season.seasonId)?.name, whatsapp: managerWhatsapp || "" })} disabled={!managerWhatsapp}>Pesan Kursi</button></div></div>)}</div>}<div className="flex flex-wrap gap-3"><Link href={href} className="tenant-package-link">Lihat pendaftaran <IconArrowRight size={17} stroke={1.9} /></Link>{managerWhatsapp && <button type="button" className="tenant-secondary-cta" onClick={() => setBooking({ packageTitle: item.title, whatsapp: managerWhatsapp })}>Konsultasi</button>}</div></div></details></article>;
                })}
              </div>
            )}
          </div>
        </section>

        {gallery.length > 0 && <section id="galeri" className="tenant-gallery-section scroll-mt-24"><div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Momen perjalanan jamaah</h2><div className="tenant-gallery mt-10">{gallery.map((item, index) => <figure key={`${item.imageUrl}-${index}`} className={index === 0 ? "tenant-gallery-featured" : ""}><button type="button" className="tenant-gallery-button" onClick={() => setLightbox(item)} aria-label={`Buka foto ${item.altText}`}><img src={safeImageLink(item.imageUrl)} alt={item.altText} loading="lazy" /></button>{item.caption && <figcaption>{item.caption}</figcaption>}</figure>)}</div></div></section>}

        <EditorialHub news={news} blogPosts={blogPosts} activeTab={editorialTab} onTabChange={setEditorialTab} />

        {socialLinks.length > 0 && <section id="sosial" className="tenant-social-section scroll-mt-24"><div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><div className="tenant-social-intro"><h2>{content.socialTitle || `Terhubung lebih dekat bersama ${name}`}</h2><p>{content.socialDescription || "Ikuti kabar perjalanan, panduan ibadah, dan dokumentasi jamaah melalui kanal resmi travel kami."}</p></div><div className="tenant-social-grid">{socialLinks.map((item, index) => <a key={`${item.platform}-${item.url}`} href={safeWebLink(item.url) || "#"} target="_blank" rel="noreferrer" className={index < 2 ? "tenant-social-link is-featured" : "tenant-social-link"}><span className="tenant-social-icon"><SocialPlatformIcon platform={item.platform} /></span><span><small>{socialPlatformName(item.platform)}</small><strong>{item.label}</strong>{item.handle && <em>{item.handle}</em>}</span><IconArrowRight className="tenant-social-arrow" size={20} stroke={1.8} /></a>)}</div></div></section>}

        <section id="tentang" className="tenant-about scroll-mt-24"><div className="mx-auto grid max-w-7xl items-center gap-12 px-4 py-20 sm:px-6 md:grid-cols-[0.88fr_1.12fr] lg:px-8 lg:py-24"><div className="tenant-about-image"><img src="/images/tenant-editorial/about_pilgrim_editorial_1787645090421.webp" alt={`Pendampingan jamaah ${name}`} loading="lazy" /><span>Perjalanan yang dirawat, bukan sekadar dijual.</span></div><div><p className="tenant-eyebrow">Tentang kami</p><h2 className="mt-4 text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">{content.aboutTitle || `Mengenal ${name}`}</h2><p className="mt-6 max-w-2xl whitespace-pre-line text-base leading-8 text-slate-600 dark:text-slate-300">{content.aboutBody || description || `${name} membantu jamaah mempersiapkan perjalanan Umrah dan Haji dengan informasi yang jelas serta pendampingan yang mudah dihubungi.`}</p><div className="tenant-about-facts mt-8"><div><IconBuildingStore size={22} stroke={1.7} /><span><small>Brand travel</small><strong>{name}</strong></span></div>{city && <div><IconMapPin size={22} stroke={1.7} /><span><small>Lokasi kantor</small><strong>{city}</strong></span></div>}{profile.licenseNumber && <div><IconCertificate size={22} stroke={1.7} /><span><small>Nomor izin</small><strong>{profile.licenseNumber}</strong></span></div>}{website && <a href={website} target="_blank" rel="noreferrer"><IconExternalLink size={22} stroke={1.7} /><span><small>Website resmi</small><strong>Kunjungi website</strong></span></a>}</div></div></div></section>

        {testimonials.length > 0 && <section className="tenant-testimonials"><div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Cerita dari jamaah</h2><div className="mt-10 grid gap-5 md:grid-cols-2">{testimonials.map((item, index) => <blockquote key={`${item.name}-${index}`} className={index === 0 && testimonials.length > 2 ? "tenant-testimonial-featured" : "tenant-testimonial"}><IconQuote size={26} stroke={1.5} /><p>{item.quote}</p><footer><strong>{item.name}</strong>{item.role && <span>{item.role}</span>}</footer></blockquote>)}</div></div></section>}

        {faqs.length > 0 && <section id="faq" className="tenant-faq scroll-mt-24"><div className="mx-auto max-w-4xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><h2 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Pertanyaan yang sering diajukan</h2><div className="mt-10 grid gap-3">{faqs.map((item, index) => <details key={`${item.question}-${index}`}><summary>{item.question}</summary><p>{item.answer}</p></details>)}</div></div></section>}

        <section id="agen" className="tenant-section scroll-mt-24"><div className="tenant-contact mx-auto max-w-7xl"><div><p className="tenant-eyebrow">Kemitraan perjalanan</p><h2 className="mt-4 text-3xl font-black tracking-tight sm:text-4xl">{content.agentTitle || "Tumbuh bersama sebagai agen atau tour leader"}</h2><p className="mt-4 max-w-xl text-base leading-7 opacity-80">{content.agentDescription || "Bantu lebih banyak keluarga berangkat dengan program kemitraan yang jelas, materi pendampingan, dan tim yang siap menjawab."}</p></div><AgentWhatsAppForm managerWhatsapp={managerWhatsapp} /></div></section>

        <section id="kontak" className="scroll-mt-24 px-4 pb-20 sm:px-6 lg:px-8 lg:pb-24"><div className="tenant-contact mx-auto max-w-7xl"><div><p className="tenant-eyebrow">Hubungi kami</p><h2 className="mt-4 text-3xl font-black tracking-tight sm:text-4xl">Mari menyiapkan perjalanan Anda</h2><p className="mt-4 max-w-xl text-base leading-7 opacity-80">Tim {name} siap membantu memilih jadwal, tipe kamar, dan kebutuhan pendampingan keluarga.</p><div className="tenant-contact-details"><span>{[address, city].filter(Boolean).join(", ") || "Alamat kantor tersedia melalui tim travel"}</span><span>Senin sampai Sabtu, 09.00 sampai 17.00 WIB</span>{content.contactEmail && <a href={`mailto:${content.contactEmail}`}>{content.contactEmail}</a>}</div></div><div className="flex flex-col gap-3 sm:flex-row">{whatsapp ? <a href={whatsapp} target="_blank" rel="noreferrer" className="tenant-contact-cta"><IconBrandWhatsapp size={19} stroke={1.9} /> Konsultasi WhatsApp</a> : <span className="tenant-contact-ghost">Nomor WhatsApp segera tersedia</span>}{website && <a href={website} target="_blank" rel="noreferrer" className="tenant-contact-ghost"><IconExternalLink size={19} stroke={1.9} /> Website</a>}</div></div></section>

        <footer className="tenant-footer"><div className="mx-auto grid max-w-7xl gap-10 px-4 py-12 sm:px-6 md:grid-cols-[1.2fr_0.8fr_1fr] lg:px-8"><div><div className="flex items-center gap-3">{logoImage ? <img src={logoImage} alt={`Logo ${name}`} className="h-12 w-12 rounded-xl object-contain" /> : <span className="tenant-logo-fallback">{initials}</span>}<strong>{name}</strong></div><p>{content.tagline || description || "Pendampingan perjalanan ibadah yang hangat dan terpercaya."}</p></div><div><strong>Navigasi</strong><nav className="mt-4 grid gap-2 text-sm"><a href="#beranda">Beranda</a><a href="#paket">Paket</a>{socialLinks.length > 0 && <a href="#sosial">Sosial Media</a>}<a href="#tentang">Tentang Kami</a><a href="#kontak">Hubungi Kami</a></nav></div><div><strong>Kontak &amp; legalitas</strong><p>{address || city || "Alamat kantor tersedia melalui tim travel."}</p>{content.contactEmail && <a href={`mailto:${content.contactEmail}`}>{content.contactEmail}</a>}{profile.licenseNumber && <p className="mt-2">Izin PPIU/PIHK: {profile.licenseNumber}</p>}{content.mapUrl && <a className="tenant-footer-map" href={content.mapUrl} target="_blank" rel="noreferrer">Buka lokasi di Google Maps <IconExternalLink size={15} /></a>}</div></div><div className="mx-auto flex max-w-7xl flex-col gap-3 border-t border-white/10 px-4 py-5 text-sm sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8"><span>© {new Date().getFullYear()} {name}. All Rights Reserved.</span><a href="https://tawafiqhub.id" className="tenant-powered" target="_blank" rel="noreferrer">Powered by <strong>TawafiqHub</strong></a></div></footer>
        <div className="tenant-floating-tools" aria-label="Akses cepat">{navSolid && <a href="#paket" className="tenant-floating-package" onClick={() => selectSection("paket")}><IconPackages size={20} stroke={1.8} /><span><small>Lihat pilihan</small><strong>Cek Paket</strong></span></a>}{musicURL && <button type="button" className={`tenant-music-control${musicBlocked ? " is-blocked" : ""}`} onClick={() => void toggleMusic()} aria-label={!musicPlaying ? "Putar musik latar" : musicMuted ? "Nyalakan suara musik" : "Bisukan musik"} title={content.backgroundMusicTitle || "Musik latar"}>{!musicPlaying ? <IconPlayerPlay size={20} stroke={1.9} /> : musicMuted ? <IconVolumeOff size={20} stroke={1.9} /> : <IconVolume size={20} stroke={1.9} />}<span>{!musicPlaying ? "Putar musik" : musicMuted ? "Suara mati" : content.backgroundMusicTitle || "Musik latar"}</span></button>}</div>
        {musicURL && <audio ref={audioRef} src={musicURL} loop preload="metadata" onPlay={() => setMusicPlaying(true)} onPause={() => setMusicPlaying(false)} onError={() => { setMusicPlaying(false); setMusicBlocked(true); }} />}
        {lightbox && <div className="tenant-lightbox" role="dialog" aria-modal="true" aria-label={lightbox.altText} onClick={() => setLightbox(null)}><button type="button" onClick={() => setLightbox(null)} aria-label="Tutup foto">×</button><figure onClick={(event) => event.stopPropagation()}><img src={safeImageLink(lightbox.imageUrl)} alt={lightbox.altText} />{lightbox.caption && <figcaption>{lightbox.caption}</figcaption>}</figure></div>}
        {booking && <BookingModal booking={booking} onClose={() => setBooking(null)} />}
        {accessOpen && <ClientAccessPanel name={name} whatsapp={managerWhatsapp} onClose={() => setAccessOpen(false)} />}
      </main>
    </ThemeProvider>
  );
}

function ClientAccessPanel({ name, whatsapp, onClose }: { name: string; whatsapp: string | null; onClose: () => void }) {
  return <div className="tenant-access-backdrop" role="dialog" aria-modal="true" aria-labelledby="tenant-access-title" onClick={onClose}><section className="tenant-access-panel" onClick={(event) => event.stopPropagation()}><button type="button" className="tenant-modal-close tenant-access-close" onClick={onClose} aria-label="Tutup">×</button><p className="tenant-eyebrow">AKSES PORTAL {name}</p><h2 id="tenant-access-title">Pilih kebutuhan Anda</h2><p className="tenant-access-intro">Jalur pendaftaran berbeda untuk setiap peran agar data jamaah dan tim lapangan tetap aman.</p><div className="tenant-access-options"><article><span className="tenant-access-number">01</span><div><h3>Jamaah</h3><p>Daftar melalui paket perjalanan yang tersedia, lalu lengkapi data pendaftaran.</p><a href="#paket" className="tenant-primary-cta" onClick={onClose}>Pilih paket</a></div></article><article><span className="tenant-access-number">02</span><div><h3>Muttawwif</h3><p>Masuk dengan akun yang sudah diundang operator travel Anda.</p><div className="tenant-access-actions"><a href="/sign-in" className="tenant-secondary-cta">Masuk portal</a>{whatsapp && <a href={`${whatsapp}?text=${encodeURIComponent("Assalamu'alaikum, saya ingin meminta akses sebagai Muttawwif.")}`} target="_blank" rel="noreferrer" className="tenant-access-request">Minta undangan</a>}</div></div></article><article><span className="tenant-access-number">03</span><div><h3>Tour Leader</h3><p>Gunakan undangan operator untuk mendapatkan akses perjalanan dan grup jamaah.</p><div className="tenant-access-actions"><a href="/sign-in" className="tenant-secondary-cta">Masuk portal</a>{whatsapp && <a href={`${whatsapp}?text=${encodeURIComponent("Assalamu'alaikum, saya ingin meminta akses sebagai Tour Leader.")}`} target="_blank" rel="noreferrer" className="tenant-access-request">Minta undangan</a>}</div></div></article></div><p className="tenant-access-footer">Sudah punya akun? <a href="/sign-in">Masuk ke TawafiqHub</a></p></section></div>;
}

function EditorialHub({ news, blogPosts, activeTab, onTabChange }: { news: StorefrontArticle[]; blogPosts: StorefrontArticle[]; activeTab: "news" | "blog"; onTabChange: (tab: "news" | "blog") => void }) {
  const articles = activeTab === "news" ? news : blogPosts;
  const first = articles[0];
  return <section id="berita" className="tenant-editorial-hub scroll-mt-24"><div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8 lg:py-24"><div className="flex flex-col gap-8 md:flex-row md:items-end md:justify-between"><div className="max-w-2xl"><p className="tenant-eyebrow">Ruang informasi</p><h2 className="mt-4 text-3xl font-black tracking-tight text-slate-950 sm:text-4xl dark:text-slate-100">Berita dan panduan untuk jamaah</h2><p className="mt-4 text-base leading-7 text-slate-600 dark:text-slate-300">Informasi regulasi dan catatan ibadah disusun ringkas agar mudah dipahami sebelum keberangkatan.</p></div><div className="tenant-editorial-tabs" role="tablist" aria-label="Pilih jenis artikel"><button type="button" role="tab" aria-selected={activeTab === "news"} className={activeTab === "news" ? "is-active" : ""} onClick={() => onTabChange("news")}>Berita regulasi</button><button type="button" role="tab" aria-selected={activeTab === "blog"} className={activeTab === "blog" ? "is-active" : ""} onClick={() => onTabChange("blog")}>Blog ibadah</button></div></div><div className="tenant-editorial-grid mt-10"><div className="tenant-editorial-cards">{articles.slice(0, 6).map((article) => <article key={article.id} className="tenant-article overflow-hidden"><div className="aspect-[16/9] overflow-hidden">{article.coverImageUrl ? <img src={safeImageLink(article.coverImageUrl)} alt={article.altText || article.title} loading="lazy" className="h-full w-full object-cover" /> : <div className="h-full w-full tenant-article-placeholder" aria-hidden="true" />}</div><div className="p-5"><p className="text-xs font-semibold tenant-brand-ink">{article.author || "Catatan travel"}</p><h3 className="mt-3 text-xl font-extrabold tracking-tight text-slate-950 dark:text-slate-100">{article.title}</h3>{article.excerpt && <p className="mt-3 line-clamp-3 text-sm leading-6 text-slate-600 dark:text-slate-300">{article.excerpt}</p>}{article.slug ? <Link href={`/${activeTab === "blog" ? "blog" : "berita"}/${article.slug}`} className="tenant-package-link mt-5">Baca selengkapnya <IconArrowRight size={17} stroke={1.9} /></Link> : <span className="tenant-editorial-placeholder-link">Konten segera diperbarui</span>}</div></article>)}</div>{first && <aside className="tenant-seo-preview"><span className="tenant-seo-label">Google preview</span><p className="tenant-seo-url">https://travel.tawafiqhub.id/{activeTab === "blog" ? "blog" : "berita"}/{first.slug || "panduan-jamaah"}</p><h3>{first.seoTitle || first.title}</h3><p>{first.seoDescription || first.excerpt || "Informasi perjalanan ibadah yang jelas dan mudah dipahami."}</p></aside>}</div></div></section>;
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

function SocialPlatformIcon({ platform }: { platform: StorefrontSocialLink["platform"] }) {
  const props = { size: 24, stroke: 1.8 };
  switch (platform) {
    case "instagram": return <IconBrandInstagram {...props} />;
    case "tiktok": return <IconBrandTiktok {...props} />;
    case "youtube": return <IconBrandYoutube {...props} />;
    case "facebook": return <IconBrandFacebook {...props} />;
    case "linkedin": return <IconBrandLinkedin {...props} />;
    case "threads": return <IconBrandThreads {...props} />;
    case "x": return <IconBrandX {...props} />;
    case "whatsapp": return <IconBrandWhatsapp {...props} />;
  }
}

function socialPlatformName(platform: StorefrontSocialLink["platform"]) {
  const names: Record<StorefrontSocialLink["platform"], string> = {
    instagram: "Instagram", tiktok: "TikTok", youtube: "YouTube", facebook: "Facebook",
    linkedin: "LinkedIn", threads: "Threads", x: "X", whatsapp: "WhatsApp",
  };
  return names[platform];
}

function waLink(raw: string) { const digits = raw.replace(/\D/g, ""); return `https://wa.me/${digits.startsWith("0") ? `62${digits.slice(1)}` : digits}`; }
function safeWebLink(raw?: string): string | null { if (!raw) return null; try { const url = new URL(raw); return url.protocol === "https:" || url.protocol === "http:" ? url.toString() : null; } catch { return null; } }
function safeImageLink(raw?: string): string { if (!raw) return DEFAULT_HERO_IMAGE; if (raw.startsWith("/")) return raw; return safeWebLink(raw) ?? DEFAULT_HERO_IMAGE; }
function safeOptionalImageLink(raw?: string): string | null { if (!raw) return null; if (raw.startsWith("/")) return raw; return safeWebLink(raw); }
function formatMonthYear(iso?: string) { return iso ? new Date(iso).toLocaleDateString("id-ID", { month: "short", year: "numeric" }) : ""; }
function readableText(hex: string) { const linear = (channel: number) => channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4; const luminance = linear(Number.parseInt(hex.slice(1, 3), 16) / 255) * 0.2126 + linear(Number.parseInt(hex.slice(3, 5), 16) / 255) * 0.7152 + linear(Number.parseInt(hex.slice(5, 7), 16) / 255) * 0.0722; return luminance > 0.179 ? "#0f172a" : "#f8fafc"; }
function resolveTheme(value: StorefrontTheme | undefined, legacyBrand: string): Required<StorefrontTheme> {
  const color = (candidate: string | undefined, fallback: string) => candidate && HEX_COLOR.test(candidate) ? candidate : fallback;
  return {
    accentColor: color(value?.accentColor, legacyBrand || DEFAULT_STOREFRONT_THEME.accentColor),
    secondaryColor: color(value?.secondaryColor, DEFAULT_STOREFRONT_THEME.secondaryColor),
    lightBackgroundColor: color(value?.lightBackgroundColor, DEFAULT_STOREFRONT_THEME.lightBackgroundColor),
    lightSurfaceColor: color(value?.lightSurfaceColor, DEFAULT_STOREFRONT_THEME.lightSurfaceColor),
    lightHeadingColor: color(value?.lightHeadingColor, DEFAULT_STOREFRONT_THEME.lightHeadingColor),
    lightBodyColor: color(value?.lightBodyColor, DEFAULT_STOREFRONT_THEME.lightBodyColor),
    lightMutedColor: color(value?.lightMutedColor, DEFAULT_STOREFRONT_THEME.lightMutedColor),
    darkBackgroundColor: color(value?.darkBackgroundColor, DEFAULT_STOREFRONT_THEME.darkBackgroundColor),
    darkSurfaceColor: color(value?.darkSurfaceColor, DEFAULT_STOREFRONT_THEME.darkSurfaceColor),
    darkHeadingColor: color(value?.darkHeadingColor, DEFAULT_STOREFRONT_THEME.darkHeadingColor),
    darkBodyColor: color(value?.darkBodyColor, DEFAULT_STOREFRONT_THEME.darkBodyColor),
    darkMutedColor: color(value?.darkMutedColor, DEFAULT_STOREFRONT_THEME.darkMutedColor),
    heroHeadingColor: color(value?.heroHeadingColor, DEFAULT_STOREFRONT_THEME.heroHeadingColor),
    heroBodyColor: color(value?.heroBodyColor, DEFAULT_STOREFRONT_THEME.heroBodyColor),
  };
}
