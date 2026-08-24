"use client";

import { useEffect, useState } from "react";
import { operatorClient } from "@/lib/rpc";
import { buildTenantLink } from "@/lib/tenant-link";

export default function OperatorProfilePanel() {
  const [name, setName] = useState("");
  const [country, setCountry] = useState("");
  const [email, setEmail] = useState("");
  const [licenseNumber, setLicenseNumber] = useState("");
  // Public profile fields
  const [logoUrl, setLogoUrl] = useState("");
  const [description, setDescription] = useState("");
  const [whatsappNumber, setWhatsappNumber] = useState("");
  const [website, setWebsite] = useState("");
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [slug, setSlug] = useState("");
  const [brandColor, setBrandColor] = useState("#059669");
  const [heroEyebrow, setHeroEyebrow] = useState("");
  const [heroTitle, setHeroTitle] = useState("");
  const [heroSubtitle, setHeroSubtitle] = useState("");
  const [heroImageUrl, setHeroImageUrl] = useState("");

  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    operatorClient
      .getMyOperator({})
      .then((operator) => {
        setName(operator.name);
        setCountry(operator.country);
        setEmail(operator.email);
        setLicenseNumber(operator.licenseNumber);
        setLogoUrl(operator.logoUrl);
        setDescription(operator.description);
        setWhatsappNumber(operator.whatsappNumber);
        setWebsite(operator.website);
        setAddress(operator.address);
        setCity(operator.city);
        setSlug(operator.slug);
        setBrandColor(operator.brandColor || "#059669");
        setHeroEyebrow(operator.heroEyebrow);
        setHeroTitle(operator.heroTitle);
        setHeroSubtitle(operator.heroSubtitle);
        setHeroImageUrl(operator.heroImageUrl);
        setLoaded(true);
      })
      .catch(() => setNotice("Gagal memuat profil operator."));
  }, []);

  const publicUrl = slug
    ? buildTenantLink(slug, "/")
    : "";

  const copyLink = async () => {
    if (!publicUrl) return;
    try {
      await navigator.clipboard.writeText(publicUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setNotice("Gagal menyalin link.");
    }
  };

  const save = async () => {
    if (!name.trim()) {
      setNotice("Nama operator wajib diisi.");
      return;
    }
    if (country && country.trim().length !== 2) {
      setNotice("Kode negara harus 2 huruf, mis. ID.");
      return;
    }
    if (!/^#[0-9a-f]{6}$/i.test(brandColor)) {
      setNotice("Warna brand harus menggunakan format hex 6 digit, mis. #059669.");
      return;
    }
    const invalidURL = [logoUrl, website, heroImageUrl].find((value) => value.trim() && !isHTTPURL(value));
    if (invalidURL) {
      setNotice("Logo, website, dan foto hero harus menggunakan URL http atau https yang valid.");
      return;
    }
    setSaving(true);
    setNotice("");
    try {
      await operatorClient.updateOperator({ name: name.trim(), country: country.trim().toUpperCase(), email: email.trim(), licenseNumber: licenseNumber.trim() });
      await operatorClient.updateMyProfile({
        logoUrl: logoUrl.trim(),
        description: description.trim(),
        whatsappNumber: whatsappNumber.trim(),
        website: website.trim(),
        address: address.trim(),
        city: city.trim(),
        brandColor,
        heroEyebrow: heroEyebrow.trim(),
        heroTitle: heroTitle.trim(),
        heroSubtitle: heroSubtitle.trim(),
        heroImageUrl: heroImageUrl.trim(),
      });
      setNotice("Profil operator diperbarui.");
    } catch (error) {
      setNotice(`Gagal menyimpan: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSaving(false);
    }
  };

  if (!loaded) return <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p>;

  return (
    <div style={{ display: "grid", gap: 20 }}>
      {notice && <p role="status" style={{ color: "var(--color-gold-800)", margin: 0 }}>{notice}</p>}

      {/* Share link */}
      {publicUrl && (
        <section style={{ ...card, background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)" }}>
          <span style={{ fontSize: 13, fontWeight: 700, color: "var(--color-emerald-900)" }}>Bagikan profil publik Anda:</span>
          <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
            <code style={{ flex: 1, minWidth: 220, padding: "10px 12px", background: "white", border: "1px solid var(--color-emerald-200)", borderRadius: 8, fontSize: 13, color: "var(--color-warm-900)", overflowX: "auto" }}>{publicUrl}</code>
            <button onClick={copyLink} style={{ ...emerald, minHeight: 40, padding: "0 16px" }}>{copied ? "Tersalin ✓" : "Salin Link"}</button>
            <a href={publicUrl} target="_blank" rel="noreferrer" style={{ ...ghost, minHeight: 40, padding: "0 16px", display: "inline-flex", alignItems: "center" }}>Lihat</a>
          </div>
        </section>
      )}

      {/* Basic operator info */}
      <section style={card}>
        <h3 style={sectionTitle}>Informasi Dasar</h3>
        <label style={field}>Nama Operator<input value={name} onChange={(e) => setName(e.target.value)} style={input} /></label>
        <label style={field}>Kode Negara (2 huruf)<input value={country} onChange={(e) => setCountry(e.target.value)} maxLength={2} placeholder="ID" style={input} /></label>
        <label style={field}>Email<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} style={input} /></label>
        <label style={field}>Nomor Izin Usaha (PPIU/PIHK)<input value={licenseNumber} onChange={(e) => setLicenseNumber(e.target.value)} style={input} /></label>
      </section>

      {/* Public storefront */}
      <section style={card}>
        <div>
          <h3 style={sectionTitle}>Landing Page Travel</h3>
          <p style={sectionDescription}>Atur identitas dan konten yang tampil di subdomain publik travel Anda.</p>
        </div>
        <label style={field}>Logo URL<input type="url" value={logoUrl} onChange={(e) => setLogoUrl(e.target.value)} placeholder="https://... (opsional)" style={input} /></label>
        <label style={field}>
          Warna brand
          <div style={colorRow}>
            <input aria-label="Pilih warna brand" type="color" value={brandColor} onChange={(e) => setBrandColor(e.target.value)} style={colorPicker} />
            <input value={brandColor} onChange={(e) => setBrandColor(e.target.value)} maxLength={7} placeholder="#059669" style={{ ...input, flex: 1 }} />
          </div>
          <span style={hint}>Warna ini dipakai konsisten untuk tombol, aksen, dan area CTA.</span>
        </label>
        <label style={field}>
          Label hero
          <input value={heroEyebrow} onChange={(e) => setHeroEyebrow(e.target.value)} maxLength={80} placeholder="Pendamping perjalanan Umrah dan Haji" style={input} />
          <span style={hint}>{heroEyebrow.length}/80</span>
        </label>
        <label style={field}>
          Judul utama
          <textarea value={heroTitle} onChange={(e) => setHeroTitle(e.target.value)} maxLength={120} rows={2} placeholder={`Perjalanan ibadah yang tenang bersama ${name || "travel Anda"}`} style={{ ...input, minHeight: 70, resize: "vertical" }} />
          <span style={hint}>{heroTitle.length}/120</span>
        </label>
        <label style={field}>
          Deskripsi hero
          <textarea value={heroSubtitle} onChange={(e) => setHeroSubtitle(e.target.value)} maxLength={240} rows={3} placeholder="Jelaskan manfaat utama dan bentuk pendampingan travel Anda." style={{ ...input, minHeight: 80, resize: "vertical" }} />
          <span style={hint}>{heroSubtitle.length}/240</span>
        </label>
        <label style={field}>
          Foto hero URL
          <input type="url" value={heroImageUrl} onChange={(e) => setHeroImageUrl(e.target.value)} placeholder="https://... (kosongkan untuk foto bawaan)" style={input} />
          <span style={hint}>Gunakan foto vertikal 4:5, minimal 1120 x 1400 px, tanpa teks di dalam gambar.</span>
        </label>

        <div style={{ ...preview, borderColor: brandColor }}>
          <span style={{ ...previewMark, background: brandColor, color: readableColor(brandColor) }}>{name.slice(0, 2).toUpperCase() || "TR"}</span>
          <div style={{ minWidth: 0 }}>
            <strong style={{ display: "block", color: "var(--color-warm-900)" }}>{heroTitle || `Perjalanan ibadah bersama ${name || "travel Anda"}`}</strong>
            <span style={{ display: "block", marginTop: 3, color: "var(--color-warm-400)", fontSize: 12 }}>Preview singkat brand storefront</span>
          </div>
        </div>
      </section>

      <section style={card}>
        <h3 style={sectionTitle}>Profil dan Kontak Publik</h3>
        <label style={field}>
          Tentang travel
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} maxLength={500} rows={3} placeholder="Ceritakan tentang travel Anda..." style={{ ...input, minHeight: 80, resize: "vertical" }} />
          <span style={{ fontSize: 12, color: "var(--color-warm-400)" }}>{description.length}/500</span>
        </label>
        <label style={field}>Nomor WhatsApp CS<input value={whatsappNumber} onChange={(e) => setWhatsappNumber(e.target.value)} placeholder="+62 812-xxxx-xxxx" style={input} /></label>
        <label style={field}>Website<input type="url" value={website} onChange={(e) => setWebsite(e.target.value)} placeholder="https://..." style={input} /></label>
        <label style={field}>Alamat Kantor<input value={address} onChange={(e) => setAddress(e.target.value)} style={input} /></label>
        <label style={field}>Kota<input value={city} onChange={(e) => setCity(e.target.value)} placeholder="Jakarta" style={input} /></label>
      </section>

      <button disabled={saving} onClick={save} style={{ ...emerald, justifySelf: "start" }}>{saving ? "Menyimpan..." : "Simpan Perubahan"}</button>
    </div>
  );
}

const card: React.CSSProperties = { display: "grid", gap: 14, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: "0 0 2px", fontSize: 15, fontWeight: 700, color: "var(--color-warm-900)" };
const sectionDescription: React.CSSProperties = { margin: "4px 0 0", fontSize: 12, color: "var(--color-warm-400)" };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const hint: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-400)" };
const colorRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10 };
const colorPicker: React.CSSProperties = { width: 52, minHeight: 44, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: 4, background: "white" };
const preview: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, border: "1px solid", borderRadius: 12, padding: 14, background: "white" };
const previewMark: React.CSSProperties = { display: "grid", width: 42, height: 42, flexShrink: 0, placeItems: "center", borderRadius: 10, fontSize: 12, fontWeight: 800 };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", cursor: "pointer" };
const ghost: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-emerald-200)", borderRadius: 8, background: "white", color: "var(--color-emerald-900)", fontWeight: 700, padding: "0 18px", cursor: "pointer" };

function isHTTPURL(value: string) {
  try {
    const url = new URL(value.trim());
    return url.protocol === "https:" || url.protocol === "http:";
  } catch {
    return false;
  }
}

function readableColor(hex: string) {
  if (!/^#[0-9a-f]{6}$/i.test(hex)) return "#f8fafc";
  const linear = (channel: number) => (channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4);
  const red = linear(Number.parseInt(hex.slice(1, 3), 16) / 255);
  const green = linear(Number.parseInt(hex.slice(3, 5), 16) / 255);
  const blue = linear(Number.parseInt(hex.slice(5, 7), 16) / 255);
  const luminance = red * 0.2126 + green * 0.7152 + blue * 0.0722;
  return luminance > 0.179 ? "#0f172a" : "#f8fafc";
}
