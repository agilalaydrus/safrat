"use client";
/* eslint-disable @next/next/no-img-element */

import { useCallback, useEffect, useState } from "react";
import {
  IconArrowUpRight,
  IconDeviceFloppy,
  IconEye,
  IconPhoto,
  IconPlus,
  IconRocket,
  IconTrash,
  IconUpload,
} from "@tabler/icons-react";
import { StorefrontAssetKind } from "@hajj-saas/proto-gen/hajj/v1/operator_pb";
import { operatorClient } from "@/lib/rpc";
import { buildTenantLink } from "@/lib/tenant-link";
import { uploadStorefrontImage } from "@/lib/storefront-upload";
import { DEFAULT_STOREFRONT_THEME } from "@/components/storefront/TenantStorefront";
import type { StorefrontArticle, StorefrontContent, StorefrontPackage, StorefrontPublicPackage, StorefrontSeason, StorefrontTheme } from "@/components/storefront/TenantStorefront";

type Tab = "brand" | "packages" | "gallery" | "trust" | "publishing";

const EMPTY_CONTENT: StorefrontContent = { brandColor: "#059669", theme: { ...DEFAULT_STOREFRONT_THEME }, packages: [], publicPackages: [], gallery: [], testimonials: [], faqs: [], news: [], blogPosts: [] };

export default function OperatorProfilePanel() {
  const [content, setContent] = useState<StorefrontContent>(EMPTY_CONTENT);
  const [seasons, setSeasons] = useState<StorefrontSeason[]>([]);
  const [operator, setOperator] = useState({ name: "", country: "", email: "", licenseNumber: "", slug: "" });
  const [draftRevision, setDraftRevision] = useState(0n);
  const [publishedRevision, setPublishedRevision] = useState(0n);
  const [publishedAt, setPublishedAt] = useState("");
  const [tab, setTab] = useState<Tab>("brand");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true); setError("");
    try {
      const [editor, current] = await Promise.all([operatorClient.getMyStorefront({}), operatorClient.getMyOperator({})]);
      setContent(toDraftContent(editor.content));
      setSeasons(editor.activeSeasons.map((season) => ({ id: season.id, name: season.name, slug: season.slug, type: season.type, startDate: season.startDate?.toDate().toISOString(), endDate: season.endDate?.toDate().toISOString(), pilgrimCount: season.pilgrimCount })));
      setDraftRevision(editor.draftRevision);
      setPublishedRevision(editor.publishedRevision);
      setPublishedAt(editor.publishedAt?.toDate().toLocaleString("id-ID") ?? "");
      setOperator({ name: current.name, country: current.country, email: current.email, licenseNumber: current.licenseNumber, slug: current.slug });
    } catch (cause) { setError(message(cause, "CMS storefront gagal dimuat.")); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const saveDraft = async () => {
    setBusy(true); setError(""); setNotice("");
    try {
      validateContent(content);
      const editor = await operatorClient.saveMyStorefrontDraft({ content: content as never, expectedRevision: draftRevision });
      setDraftRevision(editor.draftRevision);
      setPublishedRevision(editor.publishedRevision);
      setPublishedAt(editor.publishedAt?.toDate().toLocaleString("id-ID") ?? "");
      setNotice("Draft tersimpan. Halaman publik belum berubah.");
      return editor.draftRevision;
    } catch (cause) {
      setError(conflictMessage(cause));
      return null;
    } finally { setBusy(false); }
  };

  const preview = async () => {
    const previewWindow = window.open("", "_blank");
    const revision = await saveDraft();
    if (revision !== null && previewWindow) {
      previewWindow.opener = null;
      previewWindow.location.href = "/storefront-preview";
    } else {
      previewWindow?.close();
      if (!previewWindow) setError("Browser memblokir tab preview. Izinkan pop-up untuk situs ini lalu coba lagi.");
    }
  };

  const publish = async () => {
    setBusy(true); setError(""); setNotice("");
    try {
      validateContent(content);
      const saved = await operatorClient.saveMyStorefrontDraft({ content: content as never, expectedRevision: draftRevision });
      const editor = await operatorClient.publishMyStorefront({ expectedRevision: saved.draftRevision });
      setDraftRevision(editor.draftRevision);
      setPublishedRevision(editor.publishedRevision);
      setPublishedAt(editor.publishedAt?.toDate().toLocaleString("id-ID") ?? "");
      setNotice("Perubahan berhasil dipublikasikan.");
    } catch (cause) { setError(conflictMessage(cause)); }
    finally { setBusy(false); }
  };

  const saveOperator = async () => {
    setBusy(true); setError(""); setNotice("");
    try {
      await operatorClient.updateOperator({ name: operator.name.trim(), country: operator.country.trim().toUpperCase(), email: operator.email.trim(), licenseNumber: operator.licenseNumber.trim() });
      setNotice("Data legal operator diperbarui.");
    } catch (cause) { setError(message(cause, "Data operator gagal disimpan.")); }
    finally { setBusy(false); }
  };

  if (loading) return <div style={skeleton}>Memuat CMS storefront...</div>;
  const publicURL = operator.slug ? buildTenantLink(operator.slug, "/") : "";
  const dirty = draftRevision !== publishedRevision;

  return <div style={{ display: "grid", gap: 18 }}>
    <header style={cmsHeader}>
      <div><p style={eyebrow}>STOREFRONT CMS</p><h2 style={{ margin: 0, fontSize: 24 }}>Landing page travel</h2><p style={muted}>Simpan sebagai draft, tinjau halaman sebenarnya, lalu publikasikan saat siap.</p></div>
      <div style={statusBox}><strong style={{ color: dirty ? "var(--color-gold-800)" : "var(--color-emerald-900)" }}>{dirty ? "Ada draft baru" : "Sudah sinkron"}</strong><span>Draft {draftRevision.toString()} · Live {publishedRevision.toString()}</span>{publishedAt && <span>Terbit {publishedAt}</span>}</div>
    </header>

    {(notice || error) && <p role={error ? "alert" : "status"} style={{ ...alert, color: error ? "var(--color-danger-600)" : "var(--color-emerald-900)" }}>{error || notice}</p>}

    <nav style={tabs} aria-label="Bagian CMS">
      <TabButton active={tab === "brand"} onClick={() => setTab("brand")}>Brand &amp; Hero</TabButton>
      <TabButton active={tab === "packages"} onClick={() => setTab("packages")}>Paket</TabButton>
      <TabButton active={tab === "gallery"} onClick={() => setTab("gallery")}>Galeri</TabButton>
      <TabButton active={tab === "trust"} onClick={() => setTab("trust")}>Testimonial &amp; FAQ</TabButton>
      <TabButton active={tab === "publishing"} onClick={() => setTab("publishing")}>Profil, SEO &amp; Konten</TabButton>
    </nav>

    {tab === "brand" && <div style={{ display: "grid", gap: 16 }}>
      <Section title="Identitas publik" description="Nama display, logo, warna, dan kontak yang tampil di storefront.">
        <Field label="Nama brand"><input value={content.displayName ?? ""} onChange={(event) => patch(setContent, { displayName: event.target.value })} maxLength={120} style={input} /></Field>
        <ImageField label="Logo travel" value={content.logoUrl ?? ""} kind={StorefrontAssetKind.LOGO} hint="PNG/JPG akan dikonversi ke WebP. Maksimal sumber 20 MB." onChange={(logoUrl) => patch(setContent, { logoUrl })} />
        <Field label="Tentang travel"><textarea value={content.description ?? ""} onChange={(event) => patch(setContent, { description: event.target.value })} maxLength={1200} rows={5} style={textarea} /></Field>
        <div style={twoColumns}><Field label="WhatsApp CS"><input value={content.whatsappNumber ?? ""} onChange={(event) => patch(setContent, { whatsappNumber: event.target.value })} maxLength={40} style={input} /></Field><Field label="WhatsApp manajer kemitraan"><input value={content.managerWhatsapp ?? ""} onChange={(event) => patch(setContent, { managerWhatsapp: event.target.value })} maxLength={40} style={input} /></Field></div>
        <Field label="Website"><input type="url" value={content.website ?? ""} onChange={(event) => patch(setContent, { website: event.target.value })} style={input} /></Field>
        <div style={twoColumns}><Field label="Alamat kantor"><input value={content.address ?? ""} onChange={(event) => patch(setContent, { address: event.target.value })} maxLength={300} style={input} /></Field><Field label="Kota"><input value={content.city ?? ""} onChange={(event) => patch(setContent, { city: event.target.value })} maxLength={120} style={input} /></Field></div>
        <div style={twoColumns}><Field label="Email kontak publik"><input type="email" value={content.contactEmail ?? ""} onChange={(event) => patch(setContent, { contactEmail: event.target.value })} maxLength={320} style={input} /></Field><Field label="Tahun berdiri"><input type="number" min={0} max={2100} value={content.foundedYear || ""} onChange={(event) => patch(setContent, { foundedYear: Number(event.target.value) || undefined })} style={input} /></Field></div>
        <Field label="Google Maps URL"><input type="url" value={content.mapUrl ?? ""} onChange={(event) => patch(setContent, { mapUrl: event.target.value })} maxLength={2048} style={input} placeholder="https://maps.google.com/..." /></Field>
        <Field label="Badge kepercayaan (satu per baris)"><textarea value={(content.trustBadges ?? []).join("\n")} onChange={(event) => patch(setContent, { trustBadges: lines(event.target.value).slice(0, 8) })} rows={3} style={textarea} placeholder="Hotel dekat pelataran\nManasik sesuai sunnah" /></Field>
      </Section>
      <Section title="Tema warna" description="Atur warna berdasarkan fungsi. Judul, isi, dan teks sekunder tetap independen untuk mode terang dan gelap.">
        <div style={themeToolbar}>
          <div><strong>Palet semantik</strong><p style={muted}>Aksen dipakai pada tombol dan tautan. Warna teks tidak disamaratakan.</p></div>
          <button type="button" style={secondaryButton} onClick={() => patch(setContent, { brandColor: DEFAULT_STOREFRONT_THEME.accentColor, theme: { ...DEFAULT_STOREFRONT_THEME } })}>Pulihkan tema bawaan</button>
        </div>
        <div style={twoColumns}>
          <ThemeColorField label="Aksen utama" value={content.theme?.accentColor} fallback={content.brandColor || DEFAULT_STOREFRONT_THEME.accentColor} onChange={(accentColor) => patch(setContent, { brandColor: accentColor, theme: { ...content.theme, accentColor } })} />
          <ThemeColorField label="Aksen sekunder" value={content.theme?.secondaryColor} fallback={DEFAULT_STOREFRONT_THEME.secondaryColor} onChange={(secondaryColor) => patchTheme(setContent, content, { secondaryColor })} />
          <ThemeColorField label="Judul hero" value={content.theme?.heroHeadingColor} fallback={DEFAULT_STOREFRONT_THEME.heroHeadingColor} onChange={(heroHeadingColor) => patchTheme(setContent, content, { heroHeadingColor })} />
          <ThemeColorField label="Deskripsi hero" value={content.theme?.heroBodyColor} fallback={DEFAULT_STOREFRONT_THEME.heroBodyColor} onChange={(heroBodyColor) => patchTheme(setContent, content, { heroBodyColor })} />
        </div>
        <fieldset style={themeFieldset}><legend style={themeLegend}>Mode terang</legend><div style={themeGrid}>
          <ThemeColorField label="Latar halaman" value={content.theme?.lightBackgroundColor} fallback={DEFAULT_STOREFRONT_THEME.lightBackgroundColor} onChange={(lightBackgroundColor) => patchTheme(setContent, content, { lightBackgroundColor })} />
          <ThemeColorField label="Permukaan kartu" value={content.theme?.lightSurfaceColor} fallback={DEFAULT_STOREFRONT_THEME.lightSurfaceColor} onChange={(lightSurfaceColor) => patchTheme(setContent, content, { lightSurfaceColor })} />
          <ThemeColorField label="Teks judul" value={content.theme?.lightHeadingColor} fallback={DEFAULT_STOREFRONT_THEME.lightHeadingColor} onChange={(lightHeadingColor) => patchTheme(setContent, content, { lightHeadingColor })} />
          <ThemeColorField label="Teks isi" value={content.theme?.lightBodyColor} fallback={DEFAULT_STOREFRONT_THEME.lightBodyColor} onChange={(lightBodyColor) => patchTheme(setContent, content, { lightBodyColor })} />
          <ThemeColorField label="Teks sekunder" value={content.theme?.lightMutedColor} fallback={DEFAULT_STOREFRONT_THEME.lightMutedColor} onChange={(lightMutedColor) => patchTheme(setContent, content, { lightMutedColor })} />
        </div></fieldset>
        <fieldset style={themeFieldset}><legend style={themeLegend}>Mode gelap</legend><div style={themeGrid}>
          <ThemeColorField label="Latar halaman" value={content.theme?.darkBackgroundColor} fallback={DEFAULT_STOREFRONT_THEME.darkBackgroundColor} onChange={(darkBackgroundColor) => patchTheme(setContent, content, { darkBackgroundColor })} />
          <ThemeColorField label="Permukaan kartu" value={content.theme?.darkSurfaceColor} fallback={DEFAULT_STOREFRONT_THEME.darkSurfaceColor} onChange={(darkSurfaceColor) => patchTheme(setContent, content, { darkSurfaceColor })} />
          <ThemeColorField label="Teks judul" value={content.theme?.darkHeadingColor} fallback={DEFAULT_STOREFRONT_THEME.darkHeadingColor} onChange={(darkHeadingColor) => patchTheme(setContent, content, { darkHeadingColor })} />
          <ThemeColorField label="Teks isi" value={content.theme?.darkBodyColor} fallback={DEFAULT_STOREFRONT_THEME.darkBodyColor} onChange={(darkBodyColor) => patchTheme(setContent, content, { darkBodyColor })} />
          <ThemeColorField label="Teks sekunder" value={content.theme?.darkMutedColor} fallback={DEFAULT_STOREFRONT_THEME.darkMutedColor} onChange={(darkMutedColor) => patchTheme(setContent, content, { darkMutedColor })} />
        </div></fieldset>
      </Section>
      <Section title="Hero" description="Pesan utama dan foto yang pertama kali dilihat calon jamaah.">
        <Field label="Label hero"><input value={content.heroEyebrow ?? ""} onChange={(event) => patch(setContent, { heroEyebrow: event.target.value })} maxLength={80} style={input} /></Field>
        <Field label="Judul utama"><textarea value={content.heroTitle ?? ""} onChange={(event) => patch(setContent, { heroTitle: event.target.value })} maxLength={120} rows={2} style={textarea} /></Field>
        <Field label="Deskripsi hero"><textarea value={content.heroSubtitle ?? ""} onChange={(event) => patch(setContent, { heroSubtitle: event.target.value })} maxLength={240} rows={3} style={textarea} /></Field>
        <ImageField label="Foto hero" value={content.heroImageUrl ?? ""} kind={StorefrontAssetKind.HERO} hint="Gunakan foto lanskap 16:9 dengan fokus utama di area tengah. Otomatis dikecilkan maksimal 2400 px dan dikompresi WebP." onChange={(heroImageUrl) => patch(setContent, { heroImageUrl })} />
      </Section>
      <Section title="Data legal operator" description="Data ini dipakai sistem utama dan disimpan terpisah dari draft storefront.">
        <div style={twoColumns}><Field label="Nama badan usaha"><input value={operator.name} onChange={(event) => setOperator({ ...operator, name: event.target.value })} style={input} /></Field><Field label="Nomor izin PPIU/PIHK"><input value={operator.licenseNumber} onChange={(event) => setOperator({ ...operator, licenseNumber: event.target.value })} style={input} /></Field></div>
        <div style={twoColumns}><Field label="Email"><input type="email" value={operator.email} onChange={(event) => setOperator({ ...operator, email: event.target.value })} style={input} /></Field><Field label="Kode negara"><input value={operator.country} onChange={(event) => setOperator({ ...operator, country: event.target.value })} maxLength={2} style={input} /></Field></div>
        <button type="button" onClick={saveOperator} disabled={busy} style={secondaryButton}><IconDeviceFloppy size={17} /> Simpan Data Operator</button>
      </Section>
    </div>}

    {tab === "packages" && <div style={{ display: "grid", gap: 16 }}><Section title="Paket publik" description="Paket ini menjadi fokus utama landing page. Musim operasional hanya dipakai sebagai referensi pendaftaran.">
      {(content.publicPackages ?? []).map((item, index) => <PublicPackageEditor key={item.id || index} value={item} seasons={seasons} onChange={(value) => setContent({ ...content, publicPackages: content.publicPackages?.map((entry, itemIndex) => itemIndex === index ? value : entry) })} onRemove={() => setContent({ ...content, publicPackages: content.publicPackages?.filter((_, itemIndex) => itemIndex !== index) })} />)}
      {(content.publicPackages?.length ?? 0) < 30 && <Add onClick={() => setContent({ ...content, publicPackages: [...(content.publicPackages ?? []), { id: crypto.randomUUID(), title: "", category: "Umrah", summary: "", facilities: [] }] })}>Tambah Paket Publik</Add>}
    </Section><Section title="Konten paket lama" description="Fallback dari musim aktif agar data operasional yang sudah ada tetap tampil. Paket publik di atas akan tampil lebih dahulu.">
      {seasons.length === 0 ? <Empty text="Belum ada musim aktif. Buat musim terlebih dahulu agar paket muncul di CMS." /> : seasons.map((season) => <PackageEditor key={season.id} season={season} value={(content.packages ?? []).find((item) => item.seasonId === season.id)} onChange={(value) => setContent({ ...content, packages: upsertPackage(content.packages ?? [], value) })} />)}
    </Section></div>}

    {tab === "gallery" && <Section title="Galeri perjalanan" description="Maksimal 12 foto. Alt text wajib agar galeri tetap aksesibel.">
      <ImageField label="Tambah foto" value="" kind={StorefrontAssetKind.GALLERY} hint="Foto baru ditambahkan setelah upload selesai." onChange={(imageUrl) => setContent({ ...content, gallery: [...(content.gallery ?? []), { imageUrl, altText: "", caption: "" }] })} />
      {(content.gallery ?? []).length === 0 ? <Empty text="Belum ada foto galeri." /> : <div style={mediaGrid}>{content.gallery?.map((item, index) => <article key={`${item.imageUrl}-${index}`} style={mediaCard}><img src={item.imageUrl} alt="" style={thumbnail} /><Field label="Alt text"><input value={item.altText} onChange={(event) => updateGallery(setContent, content, index, { altText: event.target.value })} maxLength={160} style={input} /></Field><Field label="Caption"><input value={item.caption ?? ""} onChange={(event) => updateGallery(setContent, content, index, { caption: event.target.value })} maxLength={160} style={input} /></Field><Remove onClick={() => setContent({ ...content, gallery: content.gallery?.filter((_, itemIndex) => itemIndex !== index) })} /></article>)}</div>}
    </Section>}

    {tab === "trust" && <div style={{ display: "grid", gap: 16 }}>
      <Section title="Testimonial" description="Maksimal 6 testimonial. Gunakan kutipan singkat yang dapat diverifikasi.">
        {(content.testimonials ?? []).map((item, index) => <div key={index} style={listEditor}><Field label="Kutipan"><textarea value={item.quote} onChange={(event) => updateList(setContent, content, "testimonials", index, { quote: event.target.value })} maxLength={360} rows={3} style={textarea} /></Field><div style={twoColumns}><Field label="Nama jamaah"><input value={item.name} onChange={(event) => updateList(setContent, content, "testimonials", index, { name: event.target.value })} style={input} /></Field><Field label="Keterangan"><input value={item.role ?? ""} onChange={(event) => updateList(setContent, content, "testimonials", index, { role: event.target.value })} placeholder="Jamaah Umrah Ramadhan" style={input} /></Field></div><Remove onClick={() => setContent({ ...content, testimonials: content.testimonials?.filter((_, itemIndex) => itemIndex !== index) })} /></div>)}
        {(content.testimonials?.length ?? 0) < 6 && <Add onClick={() => setContent({ ...content, testimonials: [...(content.testimonials ?? []), { quote: "", name: "", role: "" }] })}>Tambah Testimonial</Add>}
      </Section>
      <Section title="FAQ" description="Maksimal 10 pertanyaan yang paling sering ditanyakan calon jamaah.">
        {(content.faqs ?? []).map((item, index) => <div key={index} style={listEditor}><Field label="Pertanyaan"><input value={item.question} onChange={(event) => updateList(setContent, content, "faqs", index, { question: event.target.value })} maxLength={180} style={input} /></Field><Field label="Jawaban"><textarea value={item.answer} onChange={(event) => updateList(setContent, content, "faqs", index, { answer: event.target.value })} maxLength={600} rows={4} style={textarea} /></Field><Remove onClick={() => setContent({ ...content, faqs: content.faqs?.filter((_, itemIndex) => itemIndex !== index) })} /></div>)}
        {(content.faqs?.length ?? 0) < 10 && <Add onClick={() => setContent({ ...content, faqs: [...(content.faqs ?? []), { question: "", answer: "" }] })}>Tambah FAQ</Add>}
      </Section>
    </div>}

    {tab === "publishing" && <div style={{ display: "grid", gap: 16 }}>
      <Section title="Profil perusahaan" description="Cerita yang lebih lengkap untuk halaman Tentang Kami.">
        <Field label="Judul tentang kami"><input value={content.aboutTitle ?? ""} onChange={(event) => patch(setContent, { aboutTitle: event.target.value })} maxLength={120} style={input} placeholder={`Tentang ${operator.name || "travel kami"}`} /></Field>
        <Field label="Isi tentang kami"><textarea value={content.aboutBody ?? ""} onChange={(event) => patch(setContent, { aboutBody: event.target.value })} maxLength={4000} rows={8} style={textarea} /></Field>
        <Field label="Judul section agen / tour leader"><input value={content.agentTitle ?? ""} onChange={(event) => patch(setContent, { agentTitle: event.target.value })} maxLength={120} style={input} placeholder="Bergabung sebagai agen perjalanan" /></Field>
        <Field label="Deskripsi pendaftaran agen"><textarea value={content.agentDescription ?? ""} onChange={(event) => patch(setContent, { agentDescription: event.target.value })} maxLength={600} rows={4} style={textarea} /></Field>
        <label style={{ display: "flex", alignItems: "center", gap: 9, color: "var(--color-warm-700)", fontSize: 13, fontWeight: 700 }}><input type="checkbox" checked={content.agentApplicationsEnabled ?? false} onChange={(event) => patch(setContent, { agentApplicationsEnabled: event.target.checked })} /> Tampilkan section pendaftaran agen di landing page</label>
      </Section>
      <Section title="SEO & social sharing" description="Metadata ini dipakai mesin pencari dan saat tautan dibagikan.">
        <Field label="SEO title"><input value={content.seoTitle ?? ""} onChange={(event) => patch(setContent, { seoTitle: event.target.value })} maxLength={70} style={input} /></Field>
        <Field label="SEO description"><textarea value={content.seoDescription ?? ""} onChange={(event) => patch(setContent, { seoDescription: event.target.value })} maxLength={160} rows={3} style={textarea} /></Field>
        <ImageField label="Open Graph image" value={content.ogImageUrl ?? ""} kind={StorefrontAssetKind.HERO} hint="Gunakan gambar lanskap 1200×630; otomatis dikonversi WebP." onChange={(ogImageUrl) => patch(setContent, { ogImageUrl })} />
      </Section>
      <ArticleEditor title="Berita" value={content.news ?? []} onChange={(news) => setContent({ ...content, news })} />
      <ArticleEditor title="Blog SEO" value={content.blogPosts ?? []} onChange={(blogPosts) => setContent({ ...content, blogPosts })} />
    </div>}

    <footer style={actions}>
      <div>{publicURL && <a href={publicURL} target="_blank" rel="noreferrer" style={publicLink}>Buka halaman live <IconArrowUpRight size={16} /></a>}</div>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 9 }}><button type="button" onClick={saveDraft} disabled={busy} style={secondaryButton}><IconDeviceFloppy size={17} /> Simpan Draft</button><button type="button" onClick={preview} disabled={busy} style={secondaryButton}><IconEye size={17} /> Preview</button><button type="button" onClick={publish} disabled={busy} style={primaryButton}><IconRocket size={17} /> {busy ? "Memproses..." : "Publikasikan"}</button></div>
    </footer>
  </div>;
}

function PackageEditor({ season, value, onChange }: { season: StorefrontSeason; value?: StorefrontPackage; onChange: (value: StorefrontPackage) => void }) {
  const item = value ?? { seasonId: season.id, facilities: [], itinerary: [] };
  return <article style={packageCard}><div><strong>{season.name}</strong><p style={muted}>{season.slug}</p></div><ImageField label="Foto paket" value={item.imageUrl ?? ""} kind={StorefrontAssetKind.PACKAGE} hint="Foto lanskap disarankan." onChange={(imageUrl) => onChange({ ...item, imageUrl })} /><div style={twoColumns}><Field label="Label harga"><input value={item.priceLabel ?? ""} onChange={(event) => onChange({ ...item, priceLabel: event.target.value })} placeholder="Mulai Rp 28 juta" maxLength={80} style={input} /></Field><Field label="Ringkasan"><input value={item.summary ?? ""} onChange={(event) => onChange({ ...item, summary: event.target.value })} maxLength={300} style={input} /></Field></div><Field label="Fasilitas (satu per baris, maksimal 12)"><textarea value={(item.facilities ?? []).join("\n")} onChange={(event) => onChange({ ...item, facilities: lines(event.target.value).slice(0, 12) })} rows={5} style={textarea} /></Field><Field label="Itinerary (format: Judul | Deskripsi, satu per baris)"><textarea value={(item.itinerary ?? []).map((entry) => `${entry.title}${entry.description ? ` | ${entry.description}` : ""}`).join("\n")} onChange={(event) => onChange({ ...item, itinerary: lines(event.target.value).slice(0, 20).map((line) => { const parts = line.split("|"); const title = parts.shift() ?? ""; return { title: title.trim(), description: parts.join("|").trim() }; }) })} rows={6} style={textarea} /></Field></article>;
}

function PublicPackageEditor({ value, seasons, onChange, onRemove }: { value: StorefrontPublicPackage; seasons: StorefrontSeason[]; onChange: (value: StorefrontPublicPackage) => void; onRemove: () => void }) { return <article style={packageCard}><div style={twoColumns}><Field label="Nama paket"><input value={value.title} onChange={(event) => onChange({ ...value, title: event.target.value })} maxLength={140} style={input} /></Field><Field label="Kategori"><input value={value.category ?? ""} onChange={(event) => onChange({ ...value, category: event.target.value })} maxLength={40} style={input} placeholder="Umrah / Haji" /></Field></div><ImageField label="Foto paket" value={value.imageUrl ?? ""} kind={StorefrontAssetKind.PACKAGE} hint="Foto lanskap disarankan; diproses WebP." onChange={(imageUrl) => onChange({ ...value, imageUrl })} /><div style={twoColumns}><Field label="Harga mulai"><input value={value.priceLabel ?? ""} onChange={(event) => onChange({ ...value, priceLabel: event.target.value })} maxLength={80} style={input} placeholder="Mulai Rp 28 juta" /></Field><Field label="Durasi"><input value={value.durationLabel ?? ""} onChange={(event) => onChange({ ...value, durationLabel: event.target.value })} maxLength={80} style={input} placeholder="9 hari" /></Field></div><Field label="Ringkasan"><textarea value={value.summary ?? ""} onChange={(event) => onChange({ ...value, summary: event.target.value })} maxLength={400} rows={3} style={textarea} /></Field><Field label="Hubungkan ke musim (opsional)"><select value={value.seasonId ?? ""} onChange={(event) => onChange({ ...value, seasonId: event.target.value || undefined, registrationSlug: seasons.find((season) => season.id === event.target.value)?.slug ?? value.registrationSlug })} style={input}><option value="">Tidak dihubungkan</option>{seasons.map((season) => <option key={season.id} value={season.id}>{season.name}</option>)}</select></Field><Field label="Fasilitas (satu per baris)"><textarea value={(value.facilities ?? []).join("\n")} onChange={(event) => onChange({ ...value, facilities: lines(event.target.value).slice(0, 12) })} rows={4} style={textarea} /></Field>{seasons.length > 0 && <div style={{ display: "grid", gap: 12 }}><strong style={{ color: "var(--color-warm-700)", fontSize: 13 }}>Detail musim (opsional)</strong>{seasons.map((season) => <SeasonOptionEditor key={season.id} season={season} value={value.seasons?.find((item) => item.seasonId === season.id)} onChange={(option) => onChange({ ...value, seasons: [...(value.seasons ?? []).filter((item) => item.seasonId !== season.id), option] })} />)}</div>}<Remove onClick={onRemove} /></article>; }

function SeasonOptionEditor({ season, value, onChange }: { season: StorefrontSeason; value?: NonNullable<StorefrontPublicPackage["seasons"]>[number]; onChange: (value: NonNullable<StorefrontPublicPackage["seasons"]>[number]) => void }) { const item = value ?? { seasonId: season.id }; return <div style={{ display: "grid", gap: 8, borderTop: "1px solid var(--color-cream-500)", paddingTop: 12 }}><strong style={{ fontSize: 13 }}>{season.name}</strong><div style={twoColumns}><Field label="Hotel Makkah"><input value={item.hotelMakkah ?? ""} onChange={(event) => onChange({ ...item, hotelMakkah: event.target.value })} maxLength={160} style={input} /></Field><Field label="Hotel Madinah"><input value={item.hotelMadinah ?? ""} onChange={(event) => onChange({ ...item, hotelMadinah: event.target.value })} maxLength={160} style={input} /></Field><Field label="Maskapai direct"><input value={item.airline ?? ""} onChange={(event) => onChange({ ...item, airline: event.target.value })} maxLength={120} style={input} /></Field><Field label="Sisa kursi"><input type="number" min={0} value={item.seatsRemaining ?? ""} onChange={(event) => onChange({ ...item, seatsRemaining: Number(event.target.value) || 0 })} style={input} /></Field></div></div>; }

function ImageField({ label, value, kind, hint, onChange }: { label: string; value: string; kind: StorefrontAssetKind; hint: string; onChange: (url: string) => void }) {
  const [uploading, setUploading] = useState(false); const [info, setInfo] = useState("");
  const upload = async (file?: File) => { if (!file) return; setUploading(true); setInfo(""); try { const result = await uploadStorefrontImage(file, kind); onChange(result.url); setInfo(`WebP ${formatBytes(result.optimizedBytes)} dari ${formatBytes(result.originalBytes)}`); } catch (cause) { setInfo(message(cause, "Upload gagal.")); } finally { setUploading(false); } };
  return <Field label={label}><div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}><label style={{ ...uploadButton, opacity: uploading ? 0.6 : 1 }}><IconUpload size={17} /> {uploading ? "Mengoptimalkan..." : "Pilih Gambar"}<input type="file" accept="image/jpeg,image/png,image/webp,image/avif" disabled={uploading} onChange={(event) => { void upload(event.target.files?.[0]); event.currentTarget.value = ""; }} style={{ display: "none" }} /></label>{value && <button type="button" onClick={() => onChange("")} style={iconButton} aria-label={`Hapus ${label}`}><IconTrash size={17} /></button>}</div>{value && <div style={imagePreview}><img src={value} alt="" style={{ width: "100%", height: "100%", objectFit: "cover" }} /><span>{value}</span></div>}<span style={hintStyle}>{info || hint}</span></Field>;
}

function Section({ title, description, children }: { title: string; description: string; children: React.ReactNode }) { return <section style={section}><div><h3 style={{ margin: 0, fontSize: 17 }}>{title}</h3><p style={muted}>{description}</p></div>{children}</section>; }
function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label style={field}><span>{label}</span>{children}</label>; }
function ThemeColorField({ label, value, fallback, onChange }: { label: string; value?: string; fallback: string; onChange: (value: string) => void }) { const color = validColor(value, fallback); return <Field label={label}><div style={{ display: "flex", gap: 10 }}><input aria-label={`Pilih ${label.toLowerCase()}`} type="color" value={color} onChange={(event) => onChange(event.target.value)} style={colorInput} /><input aria-label={`${label} dalam format hex`} value={value ?? fallback} onChange={(event) => onChange(event.target.value)} maxLength={7} spellCheck={false} style={input} /></div></Field>; }
function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) { return <button type="button" onClick={onClick} style={{ ...tabButton, ...(active ? activeTab : {}) }}>{children}</button>; }
function Add({ onClick, children }: { onClick: () => void; children: React.ReactNode }) { return <button type="button" onClick={onClick} style={secondaryButton}><IconPlus size={17} />{children}</button>; }
function Remove({ onClick }: { onClick: () => void }) { return <button type="button" onClick={onClick} style={removeButton}><IconTrash size={16} /> Hapus</button>; }
function Empty({ text }: { text: string }) { return <div style={empty}><IconPhoto size={24} /><span>{text}</span></div>; }

function ArticleEditor({ title, value, onChange }: { title: string; value: StorefrontArticle[]; onChange: (value: StorefrontArticle[]) => void }) { return <Section title={title} description="Konten tersimpan sebagai draft dan baru terlihat publik setelah dipublikasikan.">{value.map((item, index) => <div key={item.id || index} style={listEditor}><div style={twoColumns}><Field label="Judul"><input value={item.title} onChange={(event) => onChange(value.map((entry, itemIndex) => itemIndex === index ? { ...entry, title: event.target.value } : entry))} maxLength={180} style={input} /></Field><Field label="Slug"><input value={item.slug} onChange={(event) => onChange(value.map((entry, itemIndex) => itemIndex === index ? { ...entry, slug: event.target.value.toLowerCase().replace(/[^a-z0-9-]+/g, "-").replace(/^-|-$/g, "") } : entry))} maxLength={180} style={input} /></Field></div><Field label="Ringkasan"><textarea value={item.excerpt ?? ""} onChange={(event) => onChange(value.map((entry, itemIndex) => itemIndex === index ? { ...entry, excerpt: event.target.value } : entry))} maxLength={320} rows={3} style={textarea} /></Field><Field label="Isi artikel"><textarea value={item.body} onChange={(event) => onChange(value.map((entry, itemIndex) => itemIndex === index ? { ...entry, body: event.target.value } : entry))} maxLength={20000} rows={8} style={textarea} /></Field><Remove onClick={() => onChange(value.filter((_, itemIndex) => itemIndex !== index))} /></div>)}{value.length < 30 && <Add onClick={() => onChange([...value, { id: crypto.randomUUID(), title: "", slug: "", excerpt: "", body: "", author: "" }])}>Tambah {title}</Add>}</Section>; }

function toDraftContent(value: unknown): StorefrontContent { if (!value || typeof value !== "object") return { ...EMPTY_CONTENT, theme: { ...DEFAULT_STOREFRONT_THEME } }; const raw = value as StorefrontContent; return { displayName: raw.displayName ?? "", logoUrl: raw.logoUrl ?? "", description: raw.description ?? "", whatsappNumber: raw.whatsappNumber ?? "", managerWhatsapp: raw.managerWhatsapp ?? "", contactEmail: raw.contactEmail ?? "", website: raw.website ?? "", address: raw.address ?? "", city: raw.city ?? "", mapUrl: raw.mapUrl ?? "", foundedYear: raw.foundedYear, tagline: raw.tagline ?? "", trustBadges: [...(raw.trustBadges ?? [])], brandColor: raw.brandColor || "#059669", theme: { ...DEFAULT_STOREFRONT_THEME, ...(raw.theme ?? {}), accentColor: raw.theme?.accentColor || raw.brandColor || DEFAULT_STOREFRONT_THEME.accentColor }, heroEyebrow: raw.heroEyebrow ?? "", heroTitle: raw.heroTitle ?? "", heroSubtitle: raw.heroSubtitle ?? "", heroImageUrl: raw.heroImageUrl ?? "", packages: raw.packages?.map((item) => ({ seasonId: item.seasonId, imageUrl: item.imageUrl, summary: item.summary, priceLabel: item.priceLabel, facilities: [...(item.facilities ?? [])], itinerary: item.itinerary?.map((entry) => ({ title: entry.title, description: entry.description })) })) ?? [], publicPackages: raw.publicPackages?.map((item) => ({ ...item, facilities: [...(item.facilities ?? [])], seasons: item.seasons?.map((season) => ({ ...season })) })) ?? [], gallery: raw.gallery?.map((item) => ({ ...item })) ?? [], testimonials: raw.testimonials?.map((item) => ({ ...item })) ?? [], faqs: raw.faqs?.map((item) => ({ ...item })) ?? [], news: raw.news?.map((item) => ({ ...item })) ?? [], blogPosts: raw.blogPosts?.map((item) => ({ ...item })) ?? [], aboutTitle: raw.aboutTitle ?? "", aboutBody: raw.aboutBody ?? "", seoTitle: raw.seoTitle ?? "", seoDescription: raw.seoDescription ?? "", ogImageUrl: raw.ogImageUrl ?? "", agentTitle: raw.agentTitle ?? "", agentDescription: raw.agentDescription ?? "", agentApplicationsEnabled: raw.agentApplicationsEnabled ?? false }; }
function patch(setter: React.Dispatch<React.SetStateAction<StorefrontContent>>, value: Partial<StorefrontContent>) { setter((current) => ({ ...current, ...value })); }
function patchTheme(setter: React.Dispatch<React.SetStateAction<StorefrontContent>>, content: StorefrontContent, value: Partial<StorefrontTheme>) { setter({ ...content, theme: { ...content.theme, ...value } }); }
function upsertPackage(items: StorefrontPackage[], value: StorefrontPackage) { const index = items.findIndex((item) => item.seasonId === value.seasonId); return index < 0 ? [...items, value] : items.map((item, itemIndex) => itemIndex === index ? value : item); }
function updateGallery(setter: React.Dispatch<React.SetStateAction<StorefrontContent>>, content: StorefrontContent, index: number, value: Partial<NonNullable<StorefrontContent["gallery"]>[number]>) { setter({ ...content, gallery: content.gallery?.map((item, itemIndex) => itemIndex === index ? { ...item, ...value } : item) }); }
function updateList<K extends "testimonials" | "faqs">(setter: React.Dispatch<React.SetStateAction<StorefrontContent>>, content: StorefrontContent, key: K, index: number, value: Partial<NonNullable<StorefrontContent[K]>[number]>) { const current = content[key] ?? []; setter({ ...content, [key]: current.map((item, itemIndex) => itemIndex === index ? { ...item, ...value } : item) }); }
function lines(value: string) { return value.split("\n").map((item) => item.trim()).filter(Boolean); }
function validateContent(content: StorefrontContent) { if (!content.displayName?.trim()) throw new Error("Nama brand wajib diisi."); if (!/^#[0-9a-f]{6}$/i.test(content.brandColor ?? "")) throw new Error("Warna aksen utama harus berupa hex 6 digit."); const theme = { ...DEFAULT_STOREFRONT_THEME, ...(content.theme ?? {}) }; for (const value of Object.values(theme)) { if (!/^#[0-9a-f]{6}$/i.test(value)) throw new Error("Semua warna tema harus berupa hex 6 digit, misalnya #142019."); } const contrastPairs: [string, string, string][] = [[theme.lightHeadingColor, theme.lightBackgroundColor, "judul mode terang terhadap latar"], [theme.lightBodyColor, theme.lightBackgroundColor, "isi mode terang terhadap latar"], [theme.lightMutedColor, theme.lightBackgroundColor, "teks sekunder mode terang terhadap latar"], [theme.lightHeadingColor, theme.lightSurfaceColor, "judul mode terang terhadap kartu"], [theme.lightBodyColor, theme.lightSurfaceColor, "isi mode terang terhadap kartu"], [theme.lightMutedColor, theme.lightSurfaceColor, "teks sekunder mode terang terhadap kartu"], [theme.darkHeadingColor, theme.darkBackgroundColor, "judul mode gelap terhadap latar"], [theme.darkBodyColor, theme.darkBackgroundColor, "isi mode gelap terhadap latar"], [theme.darkMutedColor, theme.darkBackgroundColor, "teks sekunder mode gelap terhadap latar"], [theme.darkHeadingColor, theme.darkSurfaceColor, "judul mode gelap terhadap kartu"], [theme.darkBodyColor, theme.darkSurfaceColor, "isi mode gelap terhadap kartu"], [theme.darkMutedColor, theme.darkSurfaceColor, "teks sekunder mode gelap terhadap kartu"], [theme.heroHeadingColor, theme.darkBackgroundColor, "judul hero"], [theme.heroBodyColor, theme.darkBackgroundColor, "deskripsi hero"]]; const lowContrast = contrastPairs.find(([foreground, background]) => contrastRatio(foreground, background) < 4.5); if (lowContrast) throw new Error(`Kontras ${lowContrast[2]} terlalu rendah. Pilih warna dengan rasio minimal 4.5:1.`); const invalidGallery = content.gallery?.find((item) => !item.altText.trim()); if (invalidGallery) throw new Error("Alt text setiap foto galeri wajib diisi."); }
function validColor(value: string | undefined, fallback: string) { return value && /^#[0-9a-f]{6}$/i.test(value) ? value : fallback; }
function contrastRatio(foreground: string, background: string) { const luminance = (hex: string) => { const linear = (index: number) => { const channel = Number.parseInt(hex.slice(index, index + 2), 16) / 255; return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4; }; return linear(1) * 0.2126 + linear(3) * 0.7152 + linear(5) * 0.0722; }; const first = luminance(foreground); const second = luminance(background); return (Math.max(first, second) + 0.05) / (Math.min(first, second) + 0.05); }
function conflictMessage(cause: unknown) { const text = message(cause, "Perubahan gagal disimpan."); return text.toLowerCase().includes("aborted") || text.toLowerCase().includes("conflict") ? "Draft berubah dari tab lain. Muat ulang CMS sebelum melanjutkan agar perubahan tidak tertimpa." : text; }
function message(cause: unknown, fallback: string) { return cause instanceof Error && cause.message ? cause.message : fallback; }
function formatBytes(bytes: number) { return bytes < 1024 * 1024 ? `${Math.round(bytes / 1024)} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`; }

const cmsHeader: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 20, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { margin: "0 0 5px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 800, letterSpacing: "0.12em" };
const muted: React.CSSProperties = { margin: "4px 0 0", color: "var(--color-warm-400)", fontSize: 12 };
const statusBox: React.CSSProperties = { display: "grid", gap: 2, minWidth: 180, border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "10px 12px", background: "var(--color-cream-200)", fontSize: 11, color: "var(--color-warm-400)" };
const alert: React.CSSProperties = { margin: 0, border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "10px 12px", background: "var(--color-cream-200)", fontSize: 13 };
const tabs: React.CSSProperties = { display: "flex", gap: 6, overflowX: "auto", borderBottom: "1px solid var(--color-cream-400)", paddingBottom: 8 };
const tabButton: React.CSSProperties = { minHeight: 38, flexShrink: 0, border: 0, borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-warm-400)", fontWeight: 700, cursor: "pointer" };
const activeTab: React.CSSProperties = { background: "var(--color-emerald-900)", color: "white" };
const section: React.CSSProperties = { display: "grid", gap: 16, border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, background: "var(--color-cream-200)" };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 13, fontWeight: 600 };
const input: React.CSSProperties = { width: "100%", minHeight: 44, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "9px 11px", background: "white", color: "var(--color-warm-900)", font: "inherit", fontWeight: 400 };
const textarea: React.CSSProperties = { ...input, resize: "vertical" };
const colorInput: React.CSSProperties = { width: 54, minHeight: 44, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: 4, background: "white" };
const twoColumns: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: 12 };
const themeToolbar: React.CSSProperties = { display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" };
const themeFieldset: React.CSSProperties = { minWidth: 0, border: "1px solid var(--color-cream-500)", borderRadius: 10, padding: 14 };
const themeLegend: React.CSSProperties = { padding: "0 7px", color: "var(--color-warm-700)", fontSize: 13, fontWeight: 800 };
const themeGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(210px, 1fr))", gap: 12 };
const actions: React.CSSProperties = { position: "sticky", bottom: 12, zIndex: 5, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap", border: "1px solid var(--color-cream-500)", borderRadius: 12, padding: 12, background: "color-mix(in srgb, var(--color-cream-100) 92%, transparent)", boxShadow: "0 14px 34px rgba(15,23,42,.12)", backdropFilter: "blur(12px)" };
const primaryButton: React.CSSProperties = { display: "inline-flex", minHeight: 42, alignItems: "center", gap: 7, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-emerald-900)", color: "white", fontWeight: 800, cursor: "pointer", whiteSpace: "nowrap" };
const secondaryButton: React.CSSProperties = { ...primaryButton, border: "1px solid var(--color-cream-500)", background: "white", color: "var(--color-warm-700)" };
const uploadButton: React.CSSProperties = { ...secondaryButton, cursor: "pointer" };
const iconButton: React.CSSProperties = { display: "grid", width: 42, height: 42, placeItems: "center", border: "1px solid var(--color-cream-500)", borderRadius: 8, background: "white", color: "var(--color-danger-600)", cursor: "pointer" };
const removeButton: React.CSSProperties = { display: "inline-flex", justifySelf: "start", alignItems: "center", gap: 6, border: 0, background: "transparent", color: "var(--color-danger-600)", fontWeight: 700, cursor: "pointer" };
const imagePreview: React.CSSProperties = { display: "grid", gridTemplateColumns: "88px 1fr", alignItems: "center", gap: 10, minWidth: 0, height: 72, overflow: "hidden", border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "white", color: "var(--color-warm-400)", fontSize: 10, wordBreak: "break-all" };
const hintStyle: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 11, fontWeight: 400 };
const packageCard: React.CSSProperties = { display: "grid", gap: 14, borderTop: "1px solid var(--color-cream-500)", paddingTop: 18 };
const mediaGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: 12 };
const mediaCard: React.CSSProperties = { display: "grid", gap: 10, border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: 12, background: "white" };
const thumbnail: React.CSSProperties = { width: "100%", aspectRatio: "16/9", borderRadius: 8, objectFit: "cover", background: "var(--color-cream-300)" };
const listEditor: React.CSSProperties = { display: "grid", gap: 12, borderTop: "1px solid var(--color-cream-500)", paddingTop: 16 };
const empty: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, border: "1px dashed var(--color-cream-500)", borderRadius: 10, padding: 18, color: "var(--color-warm-400)", fontSize: 13 };
const publicLink: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 5, color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 800 };
const skeleton: React.CSSProperties = { borderRadius: 12, padding: 28, background: "var(--color-cream-200)", color: "var(--color-warm-400)" };
