"use client";

import { useEffect, useState } from "react";
import { operatorClient } from "@/lib/rpc";

const DISMISS_KEY = "profile_banner_dismissed";

// Shown once on the dashboard after onboarding completes (is_profile_complete
// === true), until the operator dismisses it. Nudges them to share their
// public /p/{slug} page with prospective jamaah.
export default function ProfileShareBanner() {
  const [slug, setSlug] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [dismissed, setDismissed] = useState(true);

  useEffect(() => {
    if (typeof window !== "undefined" && window.localStorage.getItem(DISMISS_KEY) === "1") return;
    operatorClient
      .getMyOperator({})
      .then((operator) => {
        if (operator.isProfileComplete && operator.slug) {
          setSlug(operator.slug);
          setDismissed(false);
        }
      })
      .catch(() => {});
  }, []);

  if (dismissed || !slug) return null;

  const url = `${process.env.NEXT_PUBLIC_APP_URL ?? window.location.origin}/p/${slug}`;

  const dismiss = () => {
    window.localStorage.setItem(DISMISS_KEY, "1");
    setDismissed(true);
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* ignore */
    }
  };

  return (
    <div style={wrap}>
      <span style={{ fontSize: 14, color: "var(--color-emerald-900)" }}>
        🎉 Profil publik Anda sudah aktif di <strong>/p/{slug}</strong> — bagikan ke calon jamaah!
      </span>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <a href={`/p/${slug}`} target="_blank" rel="noreferrer" style={ghost}>Lihat Profil</a>
        <button onClick={copy} style={solid}>{copied ? "Tersalin ✓" : "Salin Link"}</button>
        <button onClick={dismiss} aria-label="Tutup" style={close}>✕</button>
      </div>
    </div>
  );
}

const wrap: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 12, alignItems: "center", justifyContent: "space-between", background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", borderRadius: 12, padding: "14px 18px", marginBottom: 20 };
const ghost: React.CSSProperties = { border: "1px solid var(--color-emerald-200)", borderRadius: 8, background: "white", color: "var(--color-emerald-900)", fontWeight: 700, fontSize: 13, padding: "8px 14px", cursor: "pointer" };
const solid: React.CSSProperties = { border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, fontSize: 13, padding: "8px 14px", cursor: "pointer" };
const close: React.CSSProperties = { border: 0, background: "transparent", color: "var(--color-warm-400)", fontSize: 16, cursor: "pointer", padding: "4px 8px" };
