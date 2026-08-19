"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { IconCheck, IconClock } from "@tabler/icons-react";
import { waitlistClient } from "@/lib/rpc";

export default function PublicWaitlistForm({ operatorId, seasonId }: { operatorId: string; seasonId: string }) {
  const router = useRouter();
  const [form, setForm] = useState({ fullName: "", email: "", phone: "" });
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<{ position: number; email: string }>();

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (!form.fullName.trim() || !form.email.trim()) { setError("Nama lengkap dan email wajib diisi."); return; }
    setSubmitting(true);
    try {
      const response = await waitlistClient.joinWaitlist({ operatorId, seasonId, fullName: form.fullName.trim(), email: form.email.trim(), phone: form.phone.trim() });
      if (!response.isFull) {
        router.push(`/register/${operatorId}/${seasonId}`);
        return;
      }
      setResult({ position: response.position, email: form.email.trim() });
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal mendaftar. Coba lagi.");
    } finally {
      setSubmitting(false);
    }
  }

  if (result) {
    return <main style={page}><div style={card}>
      <IconCheck size={40} color="var(--color-emerald-900)" />
      <h1 style={title}>Anda Masuk Daftar Tunggu</h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Posisi antrian Anda: <strong style={{ color: "var(--color-emerald-900)" }}>#{result.position}</strong></p>
      <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Kami akan menghubungi <strong>{result.email}</strong> jika ada slot yang tersedia. Anda memiliki 48 jam untuk mengkonfirmasi setelah slot ditawarkan.</p>
    </div></main>;
  }

  return <main style={page}>
    <div style={card}>
      <p style={eyebrow}>PENDAFTARAN JAMAAH</p>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}><IconClock size={22} color="var(--color-gold-800)" /><h1 style={{ ...title, margin: 0 }}>Daftar Tunggu</h1></div>
      <p style={{ color: "var(--color-warm-400)", fontSize: 13, margin: "8px 0 24px" }}>Musim ini sudah penuh. Daftarkan diri Anda dan kami akan memberitahu saat ada slot tersedia.</p>
      <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
        <label style={label}>Nama Lengkap<input value={form.fullName} onChange={(e) => setForm((f) => ({ ...f, fullName: e.target.value }))} required style={input} placeholder="Nama sesuai paspor" /></label>
        <label style={label}>Email<input type="email" value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} required style={input} placeholder="email@anda.com" /></label>
        <label style={label}>Nomor WhatsApp<input type="tel" value={form.phone} onChange={(e) => setForm((f) => ({ ...f, phone: e.target.value }))} style={input} placeholder="+62 8xx xxxx xxxx" /></label>
        {error && <p style={errStyle}>{error}</p>}
        <button type="submit" disabled={submitting} style={primary}>{submitting ? "Mendaftar..." : "Masuk Daftar Tunggu"}</button>
      </form>
    </div>
  </main>;
}

const page: React.CSSProperties = { minHeight: "100vh", display: "grid", placeItems: "center", padding: 24, background: "var(--color-cream-100)" };
const card: React.CSSProperties = { width: "min(440px,100%)", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 16, padding: 32, textAlign: "start", display: "grid", gap: 6 };
const eyebrow: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const title: React.CSSProperties = { fontSize: 22, fontFamily: "'Playfair Display', serif", color: "var(--color-emerald-900)" };
const label: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", display: "grid", gap: 6 };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700 };
const errStyle: React.CSSProperties = { margin: 0, padding: 10, borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)" };
