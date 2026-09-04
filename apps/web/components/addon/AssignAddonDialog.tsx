"use client";

import { FormEvent, useEffect, useState } from "react";
import { IconX } from "@tabler/icons-react";
import type { AddonItem } from "@hajj-saas/proto-gen/hajj/v1/addon_pb";
import { addonClient } from "@/lib/rpc";

type Props = {
  open: boolean;
  items: AddonItem[];
  pilgrims: { id: string; fullName: string }[];
  onClose: () => void;
  onSaved: () => void;
};

export default function AssignAddonDialog({ open, items, pilgrims, onClose, onSaved }: Props) {
  const [form, setForm] = useState({ pilgrimId: "", pilgrimQuery: "", addonItemId: "", quantity: "1", notes: "" });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError("");
    setForm({ pilgrimId: "", pilgrimQuery: "", addonItemId: items[0]?.id ?? "", quantity: "1", notes: "" });
  }, [open, items]);

  if (!open) return null;

  const update = (key: keyof typeof form, value: string) => setForm((c) => ({ ...c, [key]: value }));

  const matches = form.pilgrimQuery.trim()
    ? pilgrims.filter((p) => p.fullName.toLowerCase().includes(form.pilgrimQuery.trim().toLowerCase())).slice(0, 8)
    : [];

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    const item = items.find((i) => i.id === form.addonItemId);
    const quantity = Number(form.quantity);
    if (!form.pilgrimId) { setError("Pilih jamaah terlebih dahulu."); return; }
    if (!item) { setError("Pilih layanan terlebih dahulu."); return; }
    if (!Number.isFinite(quantity) || quantity < 1) { setError("Kuantitas minimal 1."); return; }
    setSaving(true);
    try {
      await addonClient.assignPilgrimAddon({
        pilgrimId: form.pilgrimId, addonItemId: item.id, quantity, unitPriceIdr: item.unitPriceIdr, notes: form.notes.trim(),
      });
      onSaved();
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menambahkan layanan.");
    } finally {
      setSaving(false);
    }
  }

  const selectedPilgrim = pilgrims.find((p) => p.id === form.pilgrimId);

  return (
    <div style={overlay}>
      <aside style={sheet}>
        <div style={head}>
          <h2 style={{ margin: 0, fontSize: 18 }}>Tambahkan Layanan</h2>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <form onSubmit={submit} style={body}>
          <label style={label}>
            <span>Jamaah</span>
            {selectedPilgrim ? (
              <div style={{ ...input, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <span>{selectedPilgrim.fullName}</span>
                <button type="button" onClick={() => update("pilgrimId", "")} style={{ border: 0, background: "transparent", color: "var(--color-warm-400)", fontSize: 12 }}>Ganti</button>
              </div>
            ) : (
              <>
                <input style={input} placeholder="Cari nama jamaah..." value={form.pilgrimQuery} onChange={(e) => update("pilgrimQuery", e.target.value)} />
                {matches.length > 0 && (
                  <div style={suggestBox}>
                    {matches.map((p) => (
                      <button key={p.id} type="button" style={suggestItem} onClick={() => { update("pilgrimId", p.id); update("pilgrimQuery", ""); }}>
                        {p.fullName}
                      </button>
                    ))}
                  </div>
                )}
              </>
            )}
          </label>
          <label style={label}>
            <span>Layanan</span>
            <select style={input} value={form.addonItemId} onChange={(e) => update("addonItemId", e.target.value)}>
              {items.map((i) => <option key={i.id} value={i.id}>{i.name}</option>)}
            </select>
          </label>
          <label style={label}><span>Kuantitas</span><input type="number" min={1} style={input} value={form.quantity} onChange={(e) => update("quantity", e.target.value)} /></label>
          <label style={label}><span>Catatan (opsional)</span><input style={input} value={form.notes} onChange={(e) => update("notes", e.target.value)} /></label>
          {error && <p style={{ margin: 0, color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          <button disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan"}</button>
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
const label: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 6, fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", position: "relative" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const suggestBox: React.CSSProperties = { position: "absolute", top: "100%", left: 0, right: 0, marginTop: 4, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 8, boxShadow: "0 4px 12px rgba(0,0,0,.08)", zIndex: 1, maxHeight: 200, overflowY: "auto" };
const suggestItem: React.CSSProperties = { display: "block", width: "100%", textAlign: "left", padding: "8px 10px", border: 0, background: "transparent", font: "inherit", fontWeight: 400, cursor: "pointer" };
const primary: React.CSSProperties = { minHeight: 46, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
