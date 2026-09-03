"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { IconAlertTriangle, IconShieldLock } from "@tabler/icons-react";
import { platformClient } from "@/lib/rpc";

/**
 * The access gate every platform screen sits behind.
 *
 * Four states, not two. "Still asking" must not look like "refused", or the
 * panel flashes an access error at the admin it belongs to on every load — and
 * a failed call must not look like one either. Conflating the two once sent us
 * hunting a permissions bug that was never there, so the error says what
 * actually happened.
 *
 * This hides the panel, it does not protect it: the JavaScript bundle is
 * downloadable by anyone signed in, so the real control is the server refusing
 * every PlatformService call. Worth doing anyway, because a surface nobody
 * knows about is a surface nobody probes.
 */
export default function PlatformGate({ children }: { children: React.ReactNode }) {
  const [access, setAccess] = useState<"checking" | "granted" | "enrol" | "denied" | "error">("checking");
  const [failure, setFailure] = useState("");

  useEffect(() => {
    platformClient.amIPlatformAdmin({})
      .then((r) => {
        if (!r.isPlatformAdmin) { setAccess("denied"); return; }
        // Granted, but platform access requires a second factor — this
        // identity can read every tenant's data, so it must not rest on a
        // password alone. Told apart from a refusal, because the fix is
        // different: enrol, rather than ask for access.
        setAccess(r.twoFactorEnabled ? "granted" : "enrol");
      })
      .catch((err: unknown) => {
        setFailure(err instanceof Error ? err.message : String(err));
        setAccess("error");
      });
  }, []);

  if (access === "checking") {
    return <main style={page}><p style={{ color: "var(--color-warm-500)" }}>Memeriksa akses...</p></main>;
  }
  if (access === "error") {
    return (
      <main style={page}>
        <section style={locked}>
          <IconAlertTriangle size={44} color="var(--color-danger-600)" />
          <h1 style={{ margin: 0, fontSize: 24, fontWeight: 500 }}>Gagal memeriksa akses</h1>
          <p style={{ color: "var(--color-warm-500)", margin: 0, maxWidth: 520 }}>
            Ini bukan penolakan akses — permintaannya sendiri yang gagal.
          </p>
          <code style={failureBox}>{failure}</code>
        </section>
      </main>
    );
  }
  if (access === "enrol") {
    return (
      <main style={page}>
        <section style={locked}>
          <IconShieldLock size={44} color="#b45309" />
          <h1 style={{ margin: 0, fontSize: 24, fontWeight: 500 }}>Aktifkan verifikasi dua langkah</h1>
          <p style={{ color: "var(--color-warm-500)", margin: 0, maxWidth: 460 }}>
            Panel ini membaca data seluruh travel, jadi tidak boleh bergantung pada kata sandi saja.
            Daftarkan aplikasi authenticator Anda terlebih dahulu.
          </p>
          <Link href="/keamanan" style={enrolButton}>Buka pengaturan keamanan</Link>
        </section>
      </main>
    );
  }
  if (access === "denied") {
    // Nothing is rendered for an account without access — the page reports
    // itself as not existing.
    notFound();
  }

  return <>{children}</>;
}

const page: React.CSSProperties = { maxWidth: 1100, margin: "0 auto", padding: "32px 24px" };
const enrolButton: React.CSSProperties = { minHeight: 48, display: "inline-flex", alignItems: "center", padding: "0 22px", borderRadius: 8, background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, textDecoration: "none" };
const failureBox: React.CSSProperties = { display: "block", maxWidth: 520, padding: 12, background: "var(--color-cream-100)", borderRadius: 8, fontSize: 13, color: "var(--color-danger-600)", overflowWrap: "anywhere" };
const locked: React.CSSProperties = { minHeight: 320, display: "grid", placeItems: "center", alignContent: "center", gap: 12, textAlign: "center", border: "1px dashed var(--color-cream-400)", borderRadius: 12, padding: 32 };
