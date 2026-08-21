"use client";

import { FormEvent, useState } from "react";
import Link from "next/link";
import { authClient } from "@/lib/auth-client";

export default function ForgotPasswordPage() {
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const email = String(new FormData(event.currentTarget).get("email") ?? "");
      const result = await authClient.requestPasswordReset({ email, redirectTo: "/reset-password" });
      if (result.error) {
        setError(result.error.message ?? "Gagal mengirim tautan reset.");
        return;
      }
      // Always show the same success state whether or not the email exists —
      // confirming/denying account existence here is a user-enumeration leak.
      setSent(true);
    } catch {
      setError("Gagal mengirim tautan reset. Silakan coba lagi.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main style={page}>
      <div style={card}>
        <div style={brand}>
          <Link href="/" aria-label="Tawafiq Hub home" style={logo}>Tawafiq Hub</Link>
        </div>
        <div className="gold-divider" />
        {sent ? (
          <div style={{ display: "grid", gap: 10, textAlign: "center", padding: "12px 0" }}>
            <p style={{ fontWeight: 700, fontSize: 15 }}>Periksa email Anda</p>
            <p style={{ fontSize: 13, color: "var(--color-warm-500)" }}>Jika alamat email tersebut terdaftar, kami telah mengirim tautan untuk mengatur ulang kata sandi Anda.</p>
          </div>
        ) : (
          <>
            <h2 style={heading}>Lupa kata sandi?</h2>
            <p style={sub}>Masukkan email Anda, kami kirim tautan untuk mengatur ulang kata sandi.</p>
            <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
              <label style={labelStyle}>Email
                <input name="email" type="email" required style={inputStyle} />
              </label>
              {error && <p role="alert" style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
              <button type="submit" disabled={isSubmitting} style={submitStyle}>{isSubmitting ? "Mengirim..." : "Kirim Tautan Reset"}</button>
            </form>
          </>
        )}
        <p style={footer}><Link href="/sign-in" style={footerLink}>Kembali ke halaman masuk</Link></p>
      </div>
    </main>
  );
}

const page: React.CSSProperties = { minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", background: "var(--color-cream-100)", padding: "24px" };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 14, padding: "36px 32px", width: "100%", maxWidth: 420 };
const brand: React.CSSProperties = { textAlign: "center", marginBottom: 4 };
const logo: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontSize: 32, fontWeight: 700, color: "var(--color-emerald-900)" };
const heading: React.CSSProperties = { fontSize: 20, fontWeight: 500, marginTop: 16, marginBottom: 4 };
const sub: React.CSSProperties = { fontSize: 13, color: "var(--color-warm-400)", marginBottom: 20 };
const labelStyle: React.CSSProperties = { display: "block", fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const inputStyle: React.CSSProperties = { display: "block", width: "100%", marginTop: 6, padding: "10px 12px", fontSize: 14, borderRadius: 8, background: "var(--color-cream-200)", fontFamily: "'Plus Jakarta Sans',sans-serif", outline: "none", border: "1px solid var(--color-cream-500)" };
const submitStyle: React.CSSProperties = { width: "100%", height: 44, background: "var(--color-gold-500)", color: "var(--color-warm-900)", border: "none", borderRadius: 8, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "'Plus Jakarta Sans',sans-serif", marginTop: 8 };
const footer: React.CSSProperties = { textAlign: "center", fontSize: 13, color: "var(--color-warm-400)", marginTop: 20 };
const footerLink: React.CSSProperties = { color: "var(--color-emerald-900)", fontWeight: 600 };
