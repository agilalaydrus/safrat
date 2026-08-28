"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { TwoFactorChallenge } from "@/components/auth/TwoFactorChallenge";
import { invalidateMyAccessCache } from "@/lib/access-cache";
import { authClient } from "@/lib/auth-client";
import { resolveLandingPath } from "@/lib/post-login";

export default function TwoFactorChallengePage() {
  const router = useRouter();

  async function completeSignIn() {
    const session = await authClient.getSession({ fetchOptions: { cache: "no-store" } });
    if (!session.data?.user) throw new Error("Sesi tidak dapat dikonfirmasi.");
    invalidateMyAccessCache();
    router.replace(await resolveLandingPath());
  }

  return (
    <main style={page}>
      <section style={card}>
        <div style={brand}>
          <Link href="/" aria-label="Tawafiq Hub home" style={logo}>Tawafiq Hub</Link>
          <p style={tagline}>Konfirmasi keamanan akun Anda</p>
        </div>
        <div className="gold-divider" />
        <p style={notice}>
          Login Google berhasil. Selesaikan verifikasi TawafiqHub sebelum mengakses aplikasi.
        </p>
        <TwoFactorChallenge onVerified={completeSignIn} />
      </section>
    </main>
  );
}

const page: React.CSSProperties = {
  minHeight: "100vh",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  background: "var(--color-cream-100)",
  padding: 24,
};
const card: React.CSSProperties = {
  background: "#fff",
  border: "1px solid var(--color-cream-400)",
  borderRadius: 14,
  padding: "36px 32px",
  width: "100%",
  maxWidth: 420,
  display: "grid",
  gap: 18,
};
const brand: React.CSSProperties = { textAlign: "center" };
const logo: React.CSSProperties = {
  fontFamily: "'Playfair Display',serif",
  fontSize: 32,
  fontWeight: 700,
  color: "var(--color-emerald-900)",
};
const tagline: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-400)", margin: "3px 0 0" };
const notice: React.CSSProperties = { fontSize: 13, color: "var(--color-warm-500)", margin: 0, lineHeight: 1.6 };
