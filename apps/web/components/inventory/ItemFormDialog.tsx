"use client";

import { FormEvent, useEffect, useState } from "react";
import { IconX } from "@tabler/icons-react";
import type { InventoryItem } from "@hajj-saas/proto-gen/hajj/v1/inventory_pb";
import { inventoryClient } from "@/lib/rpc";

type Props = { open: boolean; item?: InventoryItem; onClose: () => void; onSaved: () => void };

const emptyForm = {
  sku: "", name: "", unit: "pcs", minStock: "", maxStock: "", unitCost: "",
  perPilgrimTracked: false, perPilgrimQty: "", perPilgrimNotes: "",
  moq: "", leadTimeDays: "", vendorName: "", rak: "",
};

export default function ItemFormDialog({ open, item, onClose, onSaved }: Props) {
  const [form, setForm] = useState(emptyForm);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError("");
    setForm(item ? {
      sku: item.sku, name: item.name, unit: item.unit,
      minStock: String(item.minStock), maxStock: String(item.maxStock), unitCost: String(item.unitCostIdr),
      perPilgrimTracked: item.perPilgrimTracked, perPilgrimQty: String(item.perPilgrimQty || ""), perPilgrimNotes: item.perPilgrimNotes,
      moq: String(item.moq), leadTimeDays: String(item.leadTimeDays), vendorName: item.vendorName, rak: item.rak,
    } : emptyForm);
  }, [open, item]);

  if (!open) return null;

  const update = (key: keyof typeof form, value: string | boolean) => setForm((current) => ({ ...current, [key]: value }));

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (!form.sku.trim() || !form.name.trim()) { setError("SKU dan nama wajib diisi."); return; }
    setSaving(true);
    try {
      const payload = {
        name: form.name.trim(), unit: form.unit.trim() || "pcs",
        minStock: Number(form.minStock) || 0, maxStock: Number(form.maxStock) || 0,
        unitCostIdr: BigInt(Math.round(Number(form.unitCost)) || 0),
        perPilgrimTracked: form.perPilgrimTracked, perPilgrimQty: form.perPilgrimTracked ? Number(form.perPilgrimQty) || 0 : 0,
        perPilgrimNotes: form.perPilgrimNotes.trim(), moq: Number(form.moq) || 0, leadTimeDays: Number(form.leadTimeDays) || 0,
        vendorName: form.vendorName.trim(), rak: form.rak.trim(),
      };
      if (item) {
        await inventoryClient.updateInventoryItem({ itemId: item.id, ...payload });
      } else {
        await inventoryClient.createInventoryItem({ sku: form.sku.trim(), ...payload });
      }
      onSaved();
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan item.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={overlay}>
      <aside style={sheet}>
        <div style={head}>
          <h2 style={{ margin: 0, fontSize: 18 }}>{item ? "Ubah Item" : "Item Baru"}</h2>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <form onSubmit={submit} style={body}>
          <div style={grid2}>
            <label style={label}><span>SKU</span><input style={input} value={form.sku} disabled={!!item} onChange={(e) => update("sku", e.target.value.toUpperCase())} /></label>
            <label style={label}><span>Satuan</span><input style={input} value={form.unit} onChange={(e) => update("unit", e.target.value)} placeholder="pcs / set / pasang" /></label>
          </div>
          <label style={label}><span>Nama Item</span><input style={input} value={form.name} onChange={(e) => update("name", e.target.value)} /></label>
          <div style={grid2}>
            <label style={label}><span>Stok Minimum</span><input type="number" style={input} value={form.minStock} onChange={(e) => update("minStock", e.target.value)} /></label>
            <label style={label}><span>Stok Maksimum</span><input type="number" style={input} value={form.maxStock} onChange={(e) => update("maxStock", e.target.value)} /></label>
          </div>
          <div style={grid2}>
            <label style={label}><span>Harga Satuan (Rp)</span><input type="number" style={input} value={form.unitCost} onChange={(e) => update("unitCost", e.target.value)} /></label>
            <label style={label}><span>MOQ (Min. Order)</span><input type="number" style={input} value={form.moq} onChange={(e) => update("moq", e.target.value)} /></label>
          </div>
          <div style={grid2}>
            <label style={label}><span>Lead Time (hari)</span><input type="number" style={input} value={form.leadTimeDays} onChange={(e) => update("leadTimeDays", e.target.value)} /></label>
            <label style={label}><span>Rak / Lokasi</span><input style={input} value={form.rak} onChange={(e) => update("rak", e.target.value)} /></label>
          </div>
          <label style={label}><span>Vendor</span><input style={input} value={form.vendorName} onChange={(e) => update("vendorName", e.target.value)} /></label>

          <label style={{ ...label, flexDirection: "row", alignItems: "center", gap: 8 }}>
            <input type="checkbox" checked={form.perPilgrimTracked} onChange={(e) => update("perPilgrimTracked", e.target.checked)} />
            <span>Diberikan per jamaah</span>
          </label>
          {form.perPilgrimTracked && (
            <div style={grid2}>
              <label style={label}><span>Kebutuhan per Jamaah</span><input type="number" style={input} value={form.perPilgrimQty} onChange={(e) => update("perPilgrimQty", e.target.value)} /></label>
              <label style={label}><span>Catatan</span><input style={input} value={form.perPilgrimNotes} onChange={(e) => update("perPilgrimNotes", e.target.value)} /></label>
            </div>
          )}

          {error && <p style={{ margin: 0, color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          <button disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan Item"}</button>
        </form>
      </aside>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)" };
const sheet: React.CSSProperties = { width: "min(480px,100%)", height: "100vh", background: "#fff", display: "flex", flexDirection: "column" };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const closeBtn: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const body: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24, display: "grid", gap: 14 };
const grid2: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 };
const label: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 6, fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const primary: React.CSSProperties = { minHeight: 46, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
