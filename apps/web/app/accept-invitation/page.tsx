"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { authClient } from "@/lib/auth-client";
import { identityClient } from "@/lib/rpc";
import { invalidateMyAccessCache } from "@/lib/access-cache";

function AcceptInvitationInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const invitationId = searchParams.get("id") ?? "";
  const { data: session, isPending } = authClient.useSession();
  const [orgName, setOrgName] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [accepting, setAccepting] = useState(false);

  useEffect(() => {
    if (!invitationId || isPending || !session?.user) return;
    authClient.organization
      .getInvitation({ query: { id: invitationId } })
      .then((result) => {
        if (result.error || !result.data) {
          setError("Undangan ini tidak valid atau sudah kedaluwarsa.");
          return;
        }
        setOrgName(result.data.organizationName);
      })
      .catch(() => setError("Gagal memuat detail undangan."));
  }, [invitationId, isPending, session?.user]);

  async function accept() {
    setAccepting(true);
    setError(null);
    try {
      const result = await authClient.organization.acceptInvitation({ invitationId });
      if (result.error) {
        setError(result.error.message ?? "Gagal menerima undangan.");
        return;
      }
      if (result.data?.invitation.organizationId) {
        await authClient.organization.setActive({ organizationId: result.data.invitation.organizationId });
      }
      // Accepting an org invitation happens entirely in Next.js/Better
      // Auth — the Go API has no visibility into it, so its own
      // GetMyAccess cache (internal/repository/identity.go) still holds
      // whatever it saw the last time this user was checked (very likely
      // "not an org member" from PublicOnly's redirect check on the way to
      // this page, moments before this accept existed). Without an
      // explicit invalidate, /dashboard's RequireAccess guard would read
      // that stale result and bounce them right back out to /onboarding.
      invalidateMyAccessCache();
      await identityClient.invalidateMyAccess({}).catch(() => {});
      router.replace("/dashboard");
    } catch {
      setError("Gagal menerima undangan. Silakan coba lagi.");
    } finally {
      setAccepting(false);
    }
  }

  if (!invitationId) {
    return <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>Tautan undangan tidak valid.</p>;
  }

  if (isPending) {
    return <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Memuat...</p>;
  }

  if (!session?.user) {
    return (
      <div style={{ display: "grid", gap: 14, textAlign: "center" }}>
        <p style={{ fontSize: 13, color: "var(--color-warm-500)" }}>Buat akun atau masuk terlebih dahulu, lalu buka kembali tautan undangan ini dari email Anda.</p>
        <Link href="/sign-up" style={primaryLink}>Buat Akun</Link>
        <Link href="/sign-in" style={secondaryLink}>Sudah punya akun? Masuk</Link>
      </div>
    );
  }

  return (
    <div style={{ display: "grid", gap: 16, textAlign: "center" }}>
      {orgName ? (
        <p style={{ fontSize: 14 }}>Anda diundang untuk bergabung dengan <strong>{orgName}</strong> sebagai Ketua Rombongan / Muttawwif.</p>
      ) : (
        <p style={{ fontSize: 13, color: "var(--color-warm-400)" }}>Memuat detail undangan...</p>
      )}
      {error && <p role="alert" style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
      <button onClick={() => void accept()} disabled={accepting || !!error} style={submitStyle}>{accepting ? "Memproses..." : "Terima Undangan"}</button>
    </div>
  );
}

export default function AcceptInvitationPage() {
  return (
    <main style={page}>
      <div style={card}>
        <div style={brand}>
          <Link href="/" aria-label="Safrat home" style={logo}>Safrat</Link>
        </div>
        <div className="gold-divider" />
        <h2 style={heading}>Undangan Bergabung</h2>
        <Suspense fallback={<p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Memuat...</p>}>
          <AcceptInvitationInner />
        </Suspense>
      </div>
    </main>
  );
}

const page: React.CSSProperties = { minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", background: "var(--color-cream-100)", padding: "24px" };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 14, padding: "36px 32px", width: "100%", maxWidth: 420 };
const brand: React.CSSProperties = { textAlign: "center", marginBottom: 4 };
const logo: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontSize: 32, fontWeight: 700, color: "var(--color-emerald-900)" };
const heading: React.CSSProperties = { fontSize: 20, fontWeight: 500, marginTop: 16, marginBottom: 20, textAlign: "center" };
const submitStyle: React.CSSProperties = { width: "100%", minHeight: 44, background: "var(--color-gold-500)", color: "var(--color-warm-900)", border: "none", borderRadius: 8, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "'Plus Jakarta Sans',sans-serif" };
const primaryLink: React.CSSProperties = { display: "block", width: "100%", minHeight: 44, lineHeight: "44px", background: "var(--color-gold-500)", color: "var(--color-warm-900)", borderRadius: 8, fontSize: 14, fontWeight: 700, textDecoration: "none" };
const secondaryLink: React.CSSProperties = { fontSize: 13, color: "var(--color-emerald-900)", fontWeight: 600 };
