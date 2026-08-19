"use client";

import { useState } from "react";
import { authClient } from "@/lib/auth-client";

/**
 * Shared across /sign-in, /sign-up, and the Pilgrim App's account-linking
 * card. Google upserts the user on first sign-in, so one button covers both
 * sign-in and sign-up — no separate flow needed. callbackURL is a page that
 * resolves the resulting session (PublicOnly on /sign-in already does this
 * for operator/leader; the pilgrim page does its own after landing back).
 */
export function GoogleSignInButton({ callbackURL, label = "Lanjutkan dengan Google" }: { callbackURL: string; label?: string }) {
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  async function start() {
    setWorking(true);
    setError("");
    try {
      const result = await authClient.signIn.social({ provider: "google", callbackURL });
      if (result.error) {
        setError(result.error.message || "Gagal memulai login Google. Silakan coba lagi.");
        setWorking(false);
      }
      // On success, better-auth navigates the browser away to Google's
      // consent screen itself — no further action needed here.
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal memulai login Google. Silakan coba lagi.");
      setWorking(false);
    }
  }
  return (
    <div style={{ display: "grid", gap: 6 }}>
      <button type="button" onClick={() => void start()} disabled={working} style={button}>
        <GoogleMark />
        {working ? "Mengarahkan..." : label}
      </button>
      {error && <p style={errorStyle}>{error}</p>}
    </div>
  );
}

function GoogleMark() {
  return (
    <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
      <path fill="#4285F4" d="M17.64 9.2c0-.64-.06-1.25-.16-1.84H9v3.48h4.84a4.14 4.14 0 0 1-1.8 2.72v2.26h2.9c1.7-1.57 2.7-3.88 2.7-6.62Z" />
      <path fill="#34A853" d="M9 18c2.43 0 4.47-.8 5.96-2.18l-2.9-2.26c-.8.54-1.84.86-3.06.86-2.35 0-4.34-1.59-5.05-3.72H.98v2.33A9 9 0 0 0 9 18Z" />
      <path fill="#FBBC05" d="M3.95 10.7A5.4 5.4 0 0 1 3.67 9c0-.59.1-1.16.28-1.7V4.97H.98A9 9 0 0 0 0 9c0 1.45.35 2.83.98 4.03l2.97-2.33Z" />
      <path fill="#EA4335" d="M9 3.58c1.32 0 2.51.46 3.44 1.35l2.58-2.58C13.46.89 11.43 0 9 0A9 9 0 0 0 .98 4.97l2.97 2.33C4.66 5.17 6.65 3.58 9 3.58Z" />
    </svg>
  );
}

const button: React.CSSProperties = {
  width: "100%",
  minHeight: 44,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  gap: 10,
  border: "1px solid var(--color-cream-500)",
  borderRadius: 8,
  background: "#fff",
  color: "var(--color-warm-700)",
  fontSize: 14,
  fontWeight: 600,
  fontFamily: "'Plus Jakarta Sans',sans-serif",
  cursor: "pointer",
};

const errorStyle: React.CSSProperties = { margin: 0, fontSize: 12, color: "var(--color-danger-600)" };
