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

      {/* Public profile */}
      <section style={card}>
        <h3 style={sectionTitle}>Profil Publik</h3>
        <label style={field}>Logo URL<input value={logoUrl} onChange={(e) => setLogoUrl(e.target.value)} placeholder="https://... (opsional)" style={input} /></label>
        <label style={field}>
          Deskripsi
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} maxLength={500} rows={3} placeholder="Ceritakan tentang travel Anda..." style={{ ...input, minHeight: 80, resize: "vertical" }} />
          <span style={{ fontSize: 12, color: "var(--color-warm-400)" }}>{description.length}/500</span>
        </label>
        <label style={field}>Nomor WhatsApp CS<input value={whatsappNumber} onChange={(e) => setWhatsappNumber(e.target.value)} placeholder="+62 812-xxxx-xxxx" style={input} /></label>
        <label style={field}>Website<input value={website} onChange={(e) => setWebsite(e.target.value)} placeholder="https://..." style={input} /></label>
        <label style={field}>Alamat Kantor<input value={address} onChange={(e) => setAddress(e.target.value)} style={input} /></label>
        <label style={field}>Kota<input value={city} onChange={(e) => setCity(e.target.value)} placeholder="Jakarta" style={input} /></label>
      </section>

      <button disabled={saving} onClick={save} style={{ ...emerald, justifySelf: "start" }}>{saving ? "Menyimpan..." : "Simpan Perubahan"}</button>
    </div>
  );
}

const card: React.CSSProperties = { display: "grid", gap: 14, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: "0 0 2px", fontSize: 15, fontWeight: 700, color: "var(--color-warm-900)" };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", cursor: "pointer" };
const ghost: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-emerald-200)", borderRadius: 8, background: "white", color: "var(--color-emerald-900)", fontWeight: 700, padding: "0 18px", cursor: "pointer" };
