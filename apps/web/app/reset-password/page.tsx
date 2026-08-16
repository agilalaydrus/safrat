"use client";

import { FormEvent, Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { authClient } from "@/lib/auth-client";

function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") ?? "";
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    if (!token) {
      setError("Tautan tidak valid atau sudah kedaluwarsa. Minta tautan baru.");
      return;
    }
    setSubmitting(true);
    try {
      const newPassword = String(new FormData(event.currentTarget).get("password") ?? "");
      const result = await authClient.resetPassword({ newPassword, token });
      if (result.error) {
        setError(result.error.message ?? "Tautan tidak valid atau sudah kedaluwarsa. Minta tautan baru.");
        return;
      }
      setDone(true);
      setTimeout(() => router.replace("/sign-in"), 2000);
    } catch {
      setError("Gagal mengatur ulang kata sandi. Silakan coba lagi.");
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return <div style={{ display: "grid", gap: 10, textAlign: "center", padding: "12px 0" }}><p style={{ fontWeight: 700, fontSize: 15 }}>Kata sandi berhasil diubah</p><p style={{ fontSize: 13, color: "var(--color-warm-500)" }}>Mengarahkan ke halaman masuk...</p></div>;
  }

  return (
    <>
      <h2 style={heading}>Atur ulang kata sandi</h2>
      <p style={sub}>Masukkan kata sandi baru Anda.</p>
      <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
        <label style={labelStyle}>Kata Sandi Baru
          <input name="password" type="password" required minLength={8} style={inputStyle} />
        </label>
        {error && <p role="alert" style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
        <button type="submit" disabled={isSubmitting} style={submitStyle}>{isSubmitting ? "Menyimpan..." : "Simpan Kata Sandi Baru"}</button>
      </form>
    </>
  );
}

export default function ResetPasswordPage() {
  return (
    <main style={page}>
      <div style={card}>
        <div style={brand}>
          <Link href="/" aria-label="Safrat home" style={logo}>Safrat</Link>
        </div>
        <div className="gold-divider" />
        <Suspense fallback={<p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Memuat...</p>}>
          <ResetPasswordForm />
        </Suspense>
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
