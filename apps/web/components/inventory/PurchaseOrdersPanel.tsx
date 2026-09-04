"use client";

import { useEffect, useState } from "react";
import { IconPlus, IconTruck, IconX } from "@tabler/icons-react";
import type { InventoryItem, PurchaseOrder, PurchaseOrderItem } from "@hajj-saas/proto-gen/hajj/v1/inventory_pb";
import { inventoryClient } from "@/lib/rpc";

const STATUS_LABEL: Record<string, string> = { DRAFT: "Draft", ORDERED: "Dipesan", PARTIAL: "Sebagian Diterima", RECEIVED: "Diterima", CANCELLED: "Dibatalkan" };
const STATUS_COLOR: Record<string, string> = { DRAFT: "var(--color-warm-400)", ORDERED: "var(--color-gold-800)", PARTIAL: "var(--color-gold-800)", RECEIVED: "var(--color-emerald-800)", CANCELLED: "var(--color-danger-600)" };
const rupiah = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

export default function PurchaseOrdersPanel({ items }: { items: InventoryItem[] }) {
  const [orders, setOrders] = useState<PurchaseOrder[]>([]);
  const [selected, setSelected] = useState<PurchaseOrder>();
  const [lines, setLines] = useState<PurchaseOrderItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ poNumber: "", vendorName: "", notes: "" });
  const [lineForm, setLineForm] = useState({ itemId: "", quantity: "", unitCost: "" });
  const [receiveQty, setReceiveQty] = useState<Record<string, string>>({});

  const refresh = () => {
    setLoading(true);
    inventoryClient.listPurchaseOrders({}).then((r) => setOrders(r.orders)).catch(() => setNotice("Gagal memuat PO.")).finally(() => setLoading(false));
  };
  useEffect(refresh, []);

  const openOrder = async (po: PurchaseOrder) => {
    setSelected(po);
    try {
      const r = await inventoryClient.listPurchaseOrderItems({ poId: po.id });
      setLines(r.items);
    } catch { setNotice("Gagal memuat item PO."); }
  };

  const createOrder = async () => {
    if (!form.poNumber.trim()) { setNotice("Nomor PO wajib diisi."); return; }
    setNotice("");
    try {
      await inventoryClient.createPurchaseOrder({ poNumber: form.poNumber.trim(), vendorName: form.vendorName.trim(), notes: form.notes.trim() });
      setForm({ poNumber: "", vendorName: "", notes: "" });
      setCreating(false);
      refresh();
    } catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal membuat PO."); }
  };

  const addLine = async () => {
    if (!selected || !lineForm.itemId || !Number(lineForm.quantity)) { setNotice("Pilih item dan jumlah."); return; }
    setNotice("");
    try {
      await inventoryClient.addPurchaseOrderItem({
        poId: selected.id, itemId: lineForm.itemId, quantityOrdered: Number(lineForm.quantity),
        unitCostIdr: BigInt(Math.round(Number(lineForm.unitCost)) || 0),
      });
      setLineForm({ itemId: "", quantity: "", unitCost: "" });
      void openOrder(selected);
    } catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menambah item PO."); }
  };

  const receive = async (line: PurchaseOrderItem) => {
    const qty = Number(receiveQty[line.id] || "0");
    if (!qty || qty <= 0) { setNotice("Jumlah terima harus lebih dari 0."); return; }
    setNotice("");
    try {
      await inventoryClient.receivePurchaseOrderItem({ poItemId: line.id, quantity: qty });
      setReceiveQty((c) => ({ ...c, [line.id]: "" }));
      if (selected) void openOrder(selected);
      refresh();
    } catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal mencatat penerimaan."); }
  };

  const cancelOrder = async (po: PurchaseOrder) => {
    if (!window.confirm(`Batalkan PO ${po.poNumber}?`)) return;
    try { await inventoryClient.updatePurchaseOrderStatus({ poId: po.id, status: "CANCELLED" }); refresh(); if (selected?.id === po.id) setSelected(undefined); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal membatalkan PO."); }
  };

  return (
    <section style={card}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 8 }}>
        <h2 style={{ margin: 0, fontSize: 16 }}>Purchase Order</h2>
        <button type="button" onClick={() => setCreating(true)} style={ghostBtn}><IconPlus size={14} /> PO Baru</button>
      </div>
      {notice && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{notice}</p>}

      {loading ? null : orders.length ? (
        <div style={{ display: "grid", gap: 8, marginTop: 12 }}>
          {orders.map((po) => (
            <div key={po.id} style={row}>
              <button type="button" onClick={() => void openOrder(po)} style={rowButton}>
                <strong>{po.poNumber}</strong>
                <span style={{ fontSize: 12, color: "var(--color-warm-500)" }}>{po.vendorName || "Tanpa vendor"}</span>
              </button>
              <span style={{ ...statusBadge, color: STATUS_COLOR[po.status] }}>{STATUS_LABEL[po.status] ?? po.status}</span>
              {po.status !== "RECEIVED" && po.status !== "CANCELLED" && (
                <button type="button" onClick={() => void cancelOrder(po)} style={dangerGhost}>Batalkan</button>
              )}
            </div>
          ))}
        </div>
      ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13, marginTop: 12 }}>Belum ada PO.</p>}

      {creating && (
        <div style={inlineForm}>
          <input placeholder="Nomor PO" value={form.poNumber} onChange={(e) => setForm({ ...form, poNumber: e.target.value })} style={input} />
          <input placeholder="Vendor" value={form.vendorName} onChange={(e) => setForm({ ...form, vendorName: e.target.value })} style={input} />
          <input placeholder="Catatan (opsional)" value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} style={input} />
          <div style={{ display: "flex", gap: 8 }}>
            <button type="button" onClick={() => void createOrder()} style={primary}>Buat</button>
            <button type="button" onClick={() => setCreating(false)} style={ghostBtn}>Batal</button>
          </div>
        </div>
      )}

      {selected && (
        <div style={detailOverlay}>
          <div style={detailCard}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
              <div>
                <p style={{ margin: 0, fontSize: 11, color: "var(--color-gold-800)", fontWeight: 700 }}>PURCHASE ORDER</p>
                <h3 style={{ margin: "2px 0 0" }}>{selected.poNumber}</h3>
              </div>
              <button onClick={() => setSelected(undefined)} style={closeBtn} aria-label="Tutup"><IconX size={16} /></button>
            </div>

            <div style={{ display: "grid", gap: 6, marginTop: 12 }}>
              {lines.map((line) => (
                <div key={line.id} style={lineRow}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <strong>{line.itemName}</strong>
                    <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{line.quantityReceived}/{line.quantityOrdered} {line.unit} · {rupiah(line.unitCostIdr)}/{line.unit}</p>
                  </div>
                  {line.quantityReceived < line.quantityOrdered && (
                    <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                      <input type="number" min={1} max={line.quantityOrdered - line.quantityReceived} placeholder="qty" value={receiveQty[line.id] || ""}
                        onChange={(e) => setReceiveQty((c) => ({ ...c, [line.id]: e.target.value }))} style={{ ...input, width: 70, minHeight: 34 }} />
                      <button type="button" onClick={() => void receive(line)} style={ghostBtn}><IconTruck size={13} /> Terima</button>
                    </div>
                  )}
                </div>
              ))}
              {!lines.length && <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada item di PO ini.</p>}
            </div>

            {selected.status !== "CANCELLED" && (
              <div style={inlineForm}>
                <select value={lineForm.itemId} onChange={(e) => setLineForm({ ...lineForm, itemId: e.target.value })} style={input}>
                  <option value="">Pilih item</option>
                  {items.map((i) => <option key={i.id} value={i.id}>{i.sku} — {i.name}</option>)}
                </select>
                <input type="number" placeholder="Jumlah pesan" value={lineForm.quantity} onChange={(e) => setLineForm({ ...lineForm, quantity: e.target.value })} style={input} />
                <input type="number" placeholder="Harga satuan (Rp)" value={lineForm.unitCost} onChange={(e) => setLineForm({ ...lineForm, unitCost: e.target.value })} style={input} />
                <button type="button" onClick={() => void addLine()} style={primary}>Tambah Item</button>
              </div>
            )}
          </div>
        </div>
      )}
    </section>
  );
}

const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginTop: 16 };
const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "8px 10px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8 };
const rowButton: React.CSSProperties = { flex: 1, display: "grid", gap: 2, textAlign: "start", background: "none", border: 0, color: "inherit", cursor: "pointer" };
const statusBadge: React.CSSProperties = { fontSize: 11, fontWeight: 700 };
const ghostBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 6 };
const dangerGhost: React.CSSProperties = { ...ghostBtn, color: "var(--color-danger-600)" };
const inlineForm: React.CSSProperties = { display: "grid", gap: 8, marginTop: 12, padding: 12, background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8 };
const input: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const primary: React.CSSProperties = { minHeight: 38, border: 0, borderRadius: 8, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, padding: "0 14px" };
const detailOverlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "grid", placeItems: "center", background: "rgba(15,23,42,.48)" };
const detailCard: React.CSSProperties = { width: "min(560px,92vw)", maxHeight: "85vh", overflowY: "auto", background: "#fff", borderRadius: 12, padding: 20 };
const closeBtn: React.CSSProperties = { width: 30, height: 30, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const lineRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "8px 10px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 8, fontSize: 13 };
