"use client";
import { FormEvent, useEffect, useRef, useState } from "react";
import { IconCheck } from "@tabler/icons-react";
import { agentClient } from "@/lib/rpc";

export default function ApplyAsAgentForm({ operatorId, referredByCode }: { operatorId: string; referredByCode: string }) {
  const [form, setForm] = useState({ name: "", phone: "", email: "" });
  const [error, setError] = useState("");
  const errorRef = useRef<HTMLParagraphElement>(null);

  useEffect(() => {
    if (error) errorRef.current?.focus();
  }, [error]);
  const [submitted, setSubmitted] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (!form.name.trim()) { setError("Nama wajib diisi."); return; }
    setSubmitting(true);
    try {
      await agentClient.applyAsAgent({ operatorId, name: form.name.trim(), phone: form.phone.trim(), email: form.email.trim(), referredByCode });
      setSubmitted(true);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal mengirim pendaftaran Anda.");
    } finally {
      setSubmitting(false);
    }
  }

  if (submitted) {
    return <main style={page}><div style={card}><IconCheck size={40} color="var(--color-emerald-900)" /><h1 style={title}>Pendaftaran diterima</h1><p style={{ color: "var(--color-warm-500)" }}>Operator akan meninjau pendaftaran Anda dan mengaktifkan akun Tour Leader Anda.</p></div></main>;
  }

  return (
    <main style={page}>
      <div style={card}>
        <p style={eyebrow}>DAFTAR SEBAGAI AGEN RUJUKAN</p>
        <h1 style={title}>Daftar sebagai Tour Leader</h1>
        {referredByCode && <p style={{ color: "var(--color-warm-500)" }}>Dirujuk dengan kode <b>{referredByCode}</b></p>}
        <form onSubmit={submit} style={{ display: "grid", gap: 16, marginTop: 16 }}>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Nama lengkap</span><input required autoComplete="name" autoCapitalize="words" className="safrat-input" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Telepon</span><input type="tel" inputMode="tel" required autoComplete="tel" placeholder="08xxxxxxxxxx" className="safrat-input" value={form.phone} onChange={(e) => setForm((f) => ({ ...f, phone: e.target.value }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Email</span><input type="email" inputMode="email" required autoComplete="email" autoCapitalize="none" autoCorrect="off" spellCheck={false} className="safrat-input" value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} style={input} /></label>
          {error && (
            // Announced and focusable for the same reason as the registration
            // form: a message rendered silently above a submit button is a
            // message a phone user never sees.
            <p ref={errorRef} tabIndex={-1} role="alert" style={errStyle}>{error}</p>
          )}
          <button disabled={submitting} style={primary}>{submitting ? "Mengirim..." : "Kirim pendaftaran"}</button>
        </form>
      </div>
    </main>
  );
}

const page: React.CSSProperties = { minHeight: "100vh", display: "grid", placeItems: "center", padding: 24, background: "var(--color-cream-100)" };
const card: React.CSSProperties = { width: "min(440px,100%)", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 16, padding: 32, textAlign: "start", display: "grid", gap: 6 };
const eyebrow: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const title: React.CSSProperties = { margin: "4px 0 8px", fontSize: 28, fontFamily: "'Playfair Display', serif", color: "var(--color-emerald-900)" };
const label: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const errStyle: React.CSSProperties = { margin: 0, padding: 10, borderRadius: 8, background: "#ffe4e6", color: "var(--color-danger-600)" };
