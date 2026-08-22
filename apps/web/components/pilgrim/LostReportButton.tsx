"use client";

import { useRef, useState } from "react";
import { IconMapPinExclamation, IconCheck } from "@tabler/icons-react";
import { lostReportClient } from "@/lib/rpc";
import { getFreshLocation } from "@/lib/geolocation";

// A single accidental/mischievous tap must never send a real report — the
// dialog's send action requires a sustained press instead of a tap, and the
// hold duration + visible fill-in progress make that unmistakable, unlike
// the old plain confirm button.
const HOLD_MS = 1600;

export function LostReportButton({ appAccessCode }: { appAccessCode: string }) {
  const [confirming, setConfirming] = useState(false);
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const [holding, setHolding] = useState(false);
  const [progress, setProgress] = useState(0);
  const holdTimer = useRef<number | null>(null);
  const holdStart = useRef(0);

  const send = async () => {
    setSending(true);
    setError("");
    try {
      const location = await getFreshLocation();
      await lostReportClient.reportLost({
        appAccessCode, latitude: location?.lat ?? 0, longitude: location?.lng ?? 0, lastKnownLocation: "",
      });
      setSent(true);
      setConfirming(false);
    } catch {
      setError("Gagal mengirim laporan. Periksa koneksi Anda dan coba lagi.");
    } finally {
      setSending(false);
    }
  };

  const clearHold = () => {
    if (holdTimer.current) window.clearInterval(holdTimer.current);
    holdTimer.current = null;
    setHolding(false);
    setProgress(0);
  };

  const startHold = () => {
    if (sending) return;
    setHolding(true);
    holdStart.current = Date.now();
    holdTimer.current = window.setInterval(() => {
      const elapsed = Date.now() - holdStart.current;
      const pct = Math.min(1, elapsed / HOLD_MS);
      setProgress(pct);
      if (pct >= 1) {
        clearHold();
        void send();
      }
    }, 30);
  };

  if (sent) {
    return <div style={sentBanner}>
      <IconCheck size={18} color="var(--color-emerald-800)" />
      <span>Laporan terkirim. Ketua grup dan petugas telah diberi tahu.</span>
      <button onClick={() => setSent(false)} style={dismissBtn}>Tutup</button>
    </div>;
  }

  return <>
    <button onClick={() => setConfirming(true)} style={fab} aria-label="Saya Tersesat">
      <IconMapPinExclamation size={22} />
      <span style={fabLabel}>Tersesat</span>
    </button>
    {confirming && <div role="dialog" aria-modal="true" style={overlay} onClick={() => !sending && !holding && setConfirming(false)}>
      <div style={dialog} onClick={(e) => e.stopPropagation()}>
        <IconMapPinExclamation size={40} color="var(--color-danger-600)" />
        <p style={{ fontWeight: 700, fontSize: 18, margin: "12px 0 4px" }}>Laporkan tersesat?</p>
        <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Lokasi Anda akan dikirim ke ketua grup dan petugas operator agar mereka dapat segera menemukan Anda.</p>
        {error && <p style={{ color: "var(--color-danger-600)", fontSize: 13, marginBottom: 10 }}>{error}</p>}
        <button
          disabled={sending}
          onPointerDown={startHold}
          onPointerUp={clearHold}
          onPointerLeave={clearHold}
          onPointerCancel={clearHold}
          style={{ ...confirmButton, ...holdFillStyle(progress) }}
        >
          <span style={{ position: "relative", zIndex: 1 }}>{sending ? "Mengirim..." : holding ? "Tahan terus..." : "Tahan untuk konfirmasi"}</span>
        </button>
        <button disabled={sending} onClick={() => setConfirming(false)} style={cancelButton}>Batal</button>
      </div>
    </div>}
  </>;
}

function holdFillStyle(progress: number): React.CSSProperties {
  return {
    backgroundImage: `linear-gradient(to right, var(--color-danger-700, #991b1b) ${progress * 100}%, var(--color-danger-600) ${progress * 100}%)`,
  };
}

const fab: React.CSSProperties = { position: "fixed", right: 16, bottom: 96, zIndex: 35, minHeight: 44, borderRadius: 999, border: "1px solid rgba(220,38,38,.35)", background: "#fff", color: "var(--color-danger-600)", display: "inline-flex", alignItems: "center", gap: 6, padding: "0 14px 0 12px", boxShadow: "0 6px 16px rgba(26,20,16,.14)" };
const fabLabel: React.CSSProperties = { fontSize: 13, fontWeight: 700 };
const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "flex-end", justifyContent: "center", background: "rgba(26,20,16,.48)" };
const dialog: React.CSSProperties = { width: "100%", maxWidth: 420, textAlign: "center", background: "#fff", borderRadius: "20px 20px 0 0", padding: 28 };
const confirmButton: React.CSSProperties = { position: "relative", overflow: "hidden", width: "100%", minHeight: 50, border: 0, borderRadius: 10, background: "var(--color-danger-600)", color: "#fff", fontWeight: 700, fontSize: 15, marginBottom: 10, userSelect: "none", touchAction: "none" };
const cancelButton: React.CSSProperties = { width: "100%", minHeight: 46, border: "1px solid var(--color-cream-500)", borderRadius: 10, background: "transparent", color: "var(--color-warm-500)" };
const sentBanner: React.CSSProperties = { position: "fixed", left: 16, right: 16, bottom: 88, zIndex: 35, display: "flex", alignItems: "center", gap: 8, background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", borderRadius: 12, padding: "10px 14px", fontSize: 13, color: "var(--color-emerald-900)" };
const dismissBtn: React.CSSProperties = { marginLeft: "auto", minHeight: 28, border: 0, background: "transparent", color: "var(--color-emerald-800)", fontWeight: 700, fontSize: 12 };
