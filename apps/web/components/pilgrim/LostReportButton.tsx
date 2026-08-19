"use client";

import { useState } from "react";
import { IconMapPinExclamation, IconCheck } from "@tabler/icons-react";
import { lostReportClient } from "@/lib/rpc";
import { getFreshLocation } from "@/lib/geolocation";

export function LostReportButton({ appAccessCode }: { appAccessCode: string }) {
  const [confirming, setConfirming] = useState(false);
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");

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

  if (sent) {
    return <div style={sentBanner}>
      <IconCheck size={18} color="var(--color-emerald-800)" />
      <span>Laporan terkirim. Ketua rombongan dan petugas telah diberi tahu.</span>
      <button onClick={() => setSent(false)} style={dismissBtn}>Tutup</button>
    </div>;
  }

  return <>
    <button onClick={() => setConfirming(true)} style={fab} aria-label="Saya Tersesat">
      <IconMapPinExclamation size={26} />
    </button>
    {confirming && <div role="dialog" aria-modal="true" style={overlay} onClick={() => !sending && setConfirming(false)}>
      <div style={dialog} onClick={(e) => e.stopPropagation()}>
        <IconMapPinExclamation size={40} color="var(--color-danger-600)" />
        <p style={{ fontWeight: 700, fontSize: 18, margin: "12px 0 4px" }}>Laporkan tersesat?</p>
        <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Lokasi Anda akan dikirim ke ketua rombongan dan petugas operator agar mereka dapat segera menemukan Anda.</p>
        {error && <p style={{ color: "var(--color-danger-600)", fontSize: 13, marginBottom: 10 }}>{error}</p>}
        <button disabled={sending} onClick={send} style={confirmButton}>{sending ? "Mengirim..." : "Ya, saya tersesat"}</button>
        <button disabled={sending} onClick={() => setConfirming(false)} style={cancelButton}>Batal</button>
      </div>
    </div>}
  </>;
}

const fab: React.CSSProperties = { position: "fixed", right: 16, bottom: 96, zIndex: 35, width: 56, height: 56, borderRadius: "50%", border: 0, background: "var(--color-danger-600)", color: "#fff", display: "grid", placeItems: "center", boxShadow: "0 8px 20px rgba(220,38,38,.35)" };
const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "flex-end", justifyContent: "center", background: "rgba(26,20,16,.48)" };
const dialog: React.CSSProperties = { width: "100%", maxWidth: 420, textAlign: "center", background: "#fff", borderRadius: "20px 20px 0 0", padding: 28 };
const confirmButton: React.CSSProperties = { width: "100%", minHeight: 50, border: 0, borderRadius: 10, background: "var(--color-danger-600)", color: "#fff", fontWeight: 700, fontSize: 15, marginBottom: 10 };
const cancelButton: React.CSSProperties = { width: "100%", minHeight: 46, border: "1px solid var(--color-cream-500)", borderRadius: 10, background: "transparent", color: "var(--color-warm-500)" };
const sentBanner: React.CSSProperties = { position: "fixed", left: 16, right: 16, bottom: 88, zIndex: 35, display: "flex", alignItems: "center", gap: 8, background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", borderRadius: 12, padding: "10px 14px", fontSize: 13, color: "var(--color-emerald-900)" };
const dismissBtn: React.CSSProperties = { marginLeft: "auto", minHeight: 28, border: 0, background: "transparent", color: "var(--color-emerald-800)", fontWeight: 700, fontSize: 12 };
