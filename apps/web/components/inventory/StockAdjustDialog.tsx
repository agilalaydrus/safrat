"use client";

import { FormEvent, useEffect, useState } from "react";
import { IconX } from "@tabler/icons-react";
import type { InventoryItem } from "@hajj-saas/proto-gen/hajj/v1/inventory_pb";
import { inventoryClient } from "@/lib/rpc";

type Props = { open: boolean; item?: InventoryItem; onClose: () => void; onSaved: () => void };

const TYPES: [string, string][] = [["IN", "Masuk (restock manual)"], ["OUT", "Keluar (dipakai/dibagikan)"], ["ADJUSTMENT", "Penyesuaian (hasil opname)"]];

export default function StockAdjustDialog({ open, item, onClose, onSaved }: Props) {
  const [movementType, setMovementType] = useState("IN");
  const [quantity, setQuantity] = useState("");
  const [reason, setReason] = useState("");
  const [reference, setReference] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setMovementType("IN"); setQuantity(""); setReason(""); setReference(""); setError("");
  }, [open]);

  if (!open || !item) return null;

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    const qty = Number(quantity);
    if (!qty || qty <= 0) { setError("Jumlah harus lebih dari 0."); return; }
    setSaving(true);
    try {
      await inventoryClient.adjustStock({ itemId: item!.id, movementType, quantity: qty, reason: reason.trim(), reference: reference.trim() });
      onSaved();
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyesuaikan stok.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={overlay}>
      <div style={card}>
        <div style={head}>
          <h2 style={{ margin: 0, fontSize: 16 }}>Sesuaikan Stok — {item.name}</h2>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={16} /></button>
        </div>
        <p style={{ margin: 0, fontSize: 13, color: "var(--color-warm-500)" }}>Stok saat ini: <strong>{item.stock} {item.unit}</strong></p>
        <form onSubmit={submit} style={{ display: "grid", gap: 12, marginTop: 12 }}>
          <label style={label}>
            <span>Jenis</span>
            <select style={input} value={movementType} onChange={(e) => setMovementType(e.target.value)}>
              {TYPES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </select>
          </label>
          <label style={label}><span>Jumlah</span><input type="number" min={1} style={input} value={quantity} onChange={(e) => setQuantity(e.target.value)} /></label>
          <label style={label}><span>Alasan</span><input style={input} value={reason} onChange={(e) => setReason(e.target.value)} placeholder="mis. dibagikan ke kloter SOC-01" /></label>
          <label style={label}><span>Referensi (opsional)</span><input style={input} value={reference} onChange={(e) => setReference(e.target.value)} /></label>
          {error && <p style={{ margin: 0, color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          <button disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan"}</button>
        </form>
      </div>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "grid", placeItems: "center", background: "rgba(15,23,42,.48)" };
const card: React.CSSProperties = { width: "min(400px,92vw)", background: "#fff", borderRadius: 12, padding: 20 };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start" };
const closeBtn: React.CSSProperties = { width: 30, height: 30, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const label: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 6, fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
