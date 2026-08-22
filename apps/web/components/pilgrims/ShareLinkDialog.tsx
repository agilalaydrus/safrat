"use client";
import { useState } from "react";
import { IconCheck, IconCopy, IconExternalLink, IconX } from "@tabler/icons-react";
import { copyToClipboard } from "@/lib/clipboard";

type Props = { title: string; url: string; onClose: () => void };

/**
 * Shows the link as visible, selectable text instead of relying purely on
 * an invisible clipboard write — navigator.clipboard.writeText() silently
 * fails in enough real conditions (non-secure context, iframe, lost focus,
 * denied permission) that "click a button, trust a toast said it worked"
 * was never actually verifiable. This always gives the user something
 * they can see and manually select, with copy-to-clipboard as a bonus on
 * top rather than the only path.
 */
export default function ShareLinkDialog({ title, url, onClose }: Props) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    const ok = await copyToClipboard(url);
    setCopied(ok);
    if (ok) window.setTimeout(() => setCopied(false), 2500);
  }

  return (
    <div style={overlay} onClick={onClose}>
      <div style={dialog} onClick={(e) => e.stopPropagation()}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14 }}>
          <h2 style={{ margin: 0, fontSize: 18 }}>{title}</h2>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <label style={{ display: "grid", gap: 6 }}>
          <span style={{ fontSize: 12, color: "var(--color-warm-500)" }}>Tautan (ketuk untuk pilih semua, lalu salin manual jika tombol Salin tidak berfungsi)</span>
          <input
            readOnly
            value={url}
            onFocus={(e) => e.currentTarget.select()}
            onClick={(e) => e.currentTarget.select()}
            style={urlInput}
          />
        </label>
        <div style={{ display: "flex", gap: 8, marginTop: 14, flexWrap: "wrap" }}>
          <button onClick={copy} style={copied ? successBtn : primaryBtn}>
            {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}{copied ? "Tersalin!" : "Salin Tautan"}
          </button>
          <a href={url} target="_blank" rel="noreferrer" style={ghostBtn}><IconExternalLink size={16} />Buka Tautan</a>
        </div>
      </div>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 50, display: "flex", alignItems: "center", justifyContent: "center", background: "rgba(15,23,42,.48)", padding: 20 };
const dialog: React.CSSProperties = { width: "100%", maxWidth: 480, background: "#fff", borderRadius: 16, padding: 24, boxShadow: "0 20px 60px rgba(15,23,42,.25)" };
const closeBtn: React.CSSProperties = { width: 32, height: 32, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const urlInput: React.CSSProperties = { minHeight: 46, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "var(--color-cream-100)", font: "inherit", fontSize: 13, color: "var(--color-warm-900)" };
const primaryBtn: React.CSSProperties = { minHeight: 46, border: 0, borderRadius: 10, background: "var(--color-emerald-900)", color: "#fff", fontWeight: 700, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8 };
const successBtn: React.CSSProperties = { ...primaryBtn, background: "var(--color-emerald-700)" };
const ghostBtn: React.CSSProperties = { minHeight: 46, border: "1px solid var(--color-emerald-800)", borderRadius: 10, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, textDecoration: "none" };
