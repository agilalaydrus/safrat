"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle, IconArrowsExchange, IconEdit, IconPlus, IconTrash } from "@tabler/icons-react";
import type { GetInventorySummaryResponse, InventoryItem } from "@hajj-saas/proto-gen/hajj/v1/inventory_pb";
import { inventoryClient } from "@/lib/rpc";
import ItemFormDialog from "./ItemFormDialog";
import StockAdjustDialog from "./StockAdjustDialog";
import PurchaseOrdersPanel from "./PurchaseOrdersPanel";

const rupiah = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

export default function InventoryDashboard() {
  const [items, setItems] = useState<InventoryItem[]>([]);
  const [summary, setSummary] = useState<GetInventorySummaryResponse>();
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [formItem, setFormItem] = useState<InventoryItem | undefined>();
  const [formOpen, setFormOpen] = useState(false);
  const [adjustItem, setAdjustItem] = useState<InventoryItem | undefined>();

  const refresh = () => {
    setLoading(true);
    Promise.all([
      inventoryClient.listInventoryItems({}).then((r) => setItems(r.items)),
      inventoryClient.getInventorySummary({}).then(setSummary),
    ]).catch(() => setNotice("Gagal memuat data inventaris.")).finally(() => setLoading(false));
  };
  useEffect(refresh, []);

  const belowMinimum = items.filter((i) => i.stock < i.minStock);

  const removeItem = async (item: InventoryItem) => {
    if (!window.confirm(`Hapus item ${item.name}? Riwayat stok item ini juga akan hilang.`)) return;
    try { await inventoryClient.deleteInventoryItem({ itemId: item.id }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menghapus item — mungkin masih dipakai di PO."); }
  };

  return (
    <main style={page}>
      <header>
        <p style={eyebrow}>GUDANG</p>
        <h1 style={{ margin: 0, fontSize: 32 }}>Inventaris</h1>
        <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Stok koper, ihram, seragam, dan atribut rombongan.</p>
      </header>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      {summary && (
        <div style={kpiGrid}>
          <div style={kpiCard}><span style={kpiLabel}>Nilai Persediaan</span><strong style={kpiValue}>{rupiah(summary.valuationIdr)}</strong></div>
          <div style={kpiCard}><span style={kpiLabel}>Item di Bawah Minimum</span><strong style={{ ...kpiValue, color: summary.belowMinimumCount > 0 ? "var(--color-danger-600)" : undefined }}>{summary.belowMinimumCount}</strong></div>
          <div style={kpiCard}><span style={kpiLabel}>PO Berjalan</span><strong style={kpiValue}>{summary.openPurchaseOrders}</strong></div>
          <div style={kpiCard}>
            <span style={kpiLabel}>Perputaran Stok (90 hari)</span>
            <strong style={kpiValue}>{summary.stockTurnoverRatio.toFixed(2)}×</strong>
            <span style={{ fontSize: 11, color: "var(--color-warm-400)" }}>keluar ÷ stok saat ini — perkiraan, bukan angka akuntansi baku</span>
          </div>
        </div>
      )}

      {belowMinimum.length > 0 && (
        <section style={{ ...card, borderLeft: "3px solid var(--color-danger-600)" }}>
          <h2 style={sectionTitle}><IconAlertTriangle size={18} color="var(--color-danger-600)" />Pusat Tindakan Gudang</h2>
          <div style={{ display: "grid", gap: 6 }}>
            {belowMinimum.map((item) => (
              <div key={item.id} style={actionRow}>
                <span><strong>{item.name}</strong> — stok {item.stock} {item.unit}, minimum {item.minStock}</span>
                <button type="button" onClick={() => setAdjustItem(item)} style={ghostBtn}>Restock</button>
              </div>
            ))}
          </div>
        </section>
      )}

      <section style={card}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h2 style={sectionTitle}>Item Inventaris ({items.length})</h2>
          <button type="button" onClick={() => { setFormItem(undefined); setFormOpen(true); }} style={primaryBtn}><IconPlus size={14} /> Item Baru</button>
        </div>
        {loading ? null : items.length ? (
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead><tr>{["SKU", "Nama", "Stok", "Min/Maks", "Harga", "Rak", "Vendor", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id} style={{ ...tr, background: item.stock < item.minStock ? "var(--color-danger-50, #fef2f2)" : undefined }}>
                    <td style={{ ...td, fontFamily: "ui-monospace,monospace" }}>{item.sku}</td>
                    <td style={td}>{item.name}{item.perPilgrimTracked && <span style={miniBadge}>{item.perPilgrimQty}/jamaah</span>}</td>
                    <td style={td}>{item.stock} {item.unit}</td>
                    <td style={td}>{item.minStock}/{item.maxStock || "–"}</td>
                    <td style={td}>{rupiah(item.unitCostIdr)}</td>
                    <td style={td}>{item.rak || "–"}</td>
                    <td style={td}>{item.vendorName || "–"}</td>
                    <td style={{ ...td, display: "flex", gap: 4, justifyContent: "flex-end" }}>
                      <button type="button" onClick={() => setAdjustItem(item)} style={iconBtn} aria-label="Sesuaikan stok"><IconArrowsExchange size={14} /></button>
                      <button type="button" onClick={() => { setFormItem(item); setFormOpen(true); }} style={iconBtn} aria-label="Ubah"><IconEdit size={14} /></button>
                      <button type="button" onClick={() => void removeItem(item)} style={iconBtnDanger} aria-label="Hapus"><IconTrash size={14} /></button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada item. Tambahkan item pertama.</p>}
      </section>

      <PurchaseOrdersPanel items={items} />

      <ItemFormDialog open={formOpen} item={formItem} onClose={() => setFormOpen(false)} onSaved={refresh} />
      <StockAdjustDialog open={!!adjustItem} item={adjustItem} onClose={() => setAdjustItem(undefined)} onSaved={refresh} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const kpiGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(180px,1fr))", gap: 12 };
const kpiCard: React.CSSProperties = { display: "grid", gap: 4, padding: 16, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-gold-500)", borderRadius: 10 };
const kpiLabel: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-400)", textTransform: "uppercase", letterSpacing: ".06em" };
const kpiValue: React.CSSProperties = { fontSize: 22, fontWeight: 700 };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: "0 0 12px", fontSize: 16, display: "flex", alignItems: "center", gap: 8 };
const actionRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "8px 10px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, fontSize: 13 };
const ghostBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600 };
const primaryBtn: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 720, marginTop: 12 };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "10px 12px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "10px 12px", fontSize: 13 };
const miniBadge: React.CSSProperties = { marginLeft: 6, padding: "2px 6px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 10, fontWeight: 700 };
const iconBtn: React.CSSProperties = { width: 28, height: 28, border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-warm-500)", display: "grid", placeItems: "center" };
const iconBtnDanger: React.CSSProperties = { ...iconBtn, color: "var(--color-danger-600)" };
