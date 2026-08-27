"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { IconShieldLock } from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";

/**
 * Sends a signed-in account that has not enrolled a second factor to
 * /keamanan.
 *
 * Two modes, and the difference is deliberate.
 *
 * `enforce` blocks the surface until enrolment is done. That is right for
 * staff: they hold an operator's whole book, and there is nothing time-critical
 * behind the dashboard that cannot wait the minute it takes to scan a code.
 *
 * `prompt` shows a banner and lets the person carry on. That is what the
 * pilgrim app gets, because behind it sits SOS. Standing between a jamaah in
 * distress and the button that summons help — because they have not installed
 * an authenticator app — is not a security improvement, it is a hazard. The
 * owner's instruction was that jamaah use two-factor too, and they will; the
 * judgement here is only about whether the app refuses to open in the meantime.
 */
export function RequireTwoFactor({ mode, children }: { mode: "enforce" | "prompt"; children: React.ReactNode }) {
  const { data: session, isPending } = authClient.useSession();
  const router = useRouter();
  const pathname = usePathname();
  const [dismissed, setDismissed] = useState(false);

  const enrolled = Boolean((session?.user as { twoFactorEnabled?: boolean } | undefined)?.twoFactorEnabled);
  const missing = Boolean(session?.user) && !enrolled;

  useEffect(() => {
    if (mode !== "enforce" || isPending || !missing) return;
    if (pathname?.startsWith("/keamanan")) return;
    router.replace("/keamanan");
  }, [mode, isPending, missing, pathname, router]);

  // Never hold the surface hostage to a session request that is still in
  // flight: a slow network would look like a lockout.
  if (isPending || !missing || mode !== "enforce" || pathname?.startsWith("/keamanan")) {
    return (
      <>
        {mode === "prompt" && missing && !dismissed && (
          <div style={banner}>
            <IconShieldLock size={18} />
            <span style={{ flex: 1, minWidth: 200 }}>
              Amankan akun Anda dengan verifikasi dua langkah.
            </span>
            <Link href="/keamanan" style={bannerAction}>Aktifkan</Link>
            <button onClick={() => setDismissed(true)} style={bannerDismiss} aria-label="Tutup">Nanti</button>
          </div>
        )}
        {children}
      </>
    );
  }

  // Enforced and not yet enrolled: the redirect above is running.
  return (
    <main style={{ display: "grid", placeItems: "center", minHeight: "60vh", padding: 24, textAlign: "center" }}>
      <div style={{ display: "grid", gap: 10, justifyItems: "center" }}>
        <IconShieldLock size={40} color="#b45309" />
        <p style={{ margin: 0, fontWeight: 700 }}>Mengalihkan ke pengaturan keamanan...</p>
        <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 14, maxWidth: 380 }}>
          Akun staf wajib mengaktifkan verifikasi dua langkah sebelum membuka dashboard.
        </p>
      </div>
    </main>
  );
}

const banner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap", padding: "10px 14px", background: "var(--color-gold-50)", color: "#b45309", fontSize: 13 };
const bannerAction: React.CSSProperties = { fontWeight: 700, color: "#b45309", textDecoration: "underline" };
const bannerDismiss: React.CSSProperties = { border: 0, background: "transparent", color: "#b45309", fontSize: 13, opacity: 0.8 };
