"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { IconExternalLink, IconRefresh, IconShoppingCart } from "@tabler/icons-react";
import type { Order, PurchaseCatalogueProduct } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { orderClient, seasonClient } from "@/lib/rpc";
import { checkoutErrorMessage } from "@/lib/checkout-error";
import RefundWalletPanel from "@/components/pilgrim/RefundWalletPanel";

const rupiah = (value: bigint) => `Rp${Number(value).toLocaleString("id-ID")}`;
const statusLabel: Record<string, string> = {
  PENDING: "Menunggu bayar", PAID: "Lunas", HELD: "Perlu ditinjau",
  EXPIRED: "Kedaluwarsa", FAILED: "Gagal", CANCELLED: "Dibatalkan", REFUNDED: "Direfund",
};

export default function AgentPurchaseTab() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [products, setProducts] = useState<PurchaseCatalogueProduct[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [productId, setProductId] = useState("");
  const [quantity, setQuantity] = useState(1);
  const [destination, setDestination] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const idempotencyKey = useRef(crypto.randomUUID());

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((season) => season.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Daftar musim tidak dapat dimuat."));
  }, []);

  const load = useCallback(async () => {
    if (!seasonId) return;
    setLoading(true);
    try {
      const [catalogue, history] = await Promise.all([
        orderClient.listMyPurchaseCatalogue({ seasonId }),
        orderClient.listMyOrders({ seasonId, limit: 50, offset: 0 }),
      ]);
      setProducts(catalogue.products);
      setOrders(history.orders);
      setProductId((current) => catalogue.products.some((item) => item.id === current) ? current : (catalogue.products[0]?.id ?? ""));
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Katalog pembelian tidak dapat dimuat.");
    } finally {
      setLoading(false);
    }
  }, [seasonId]);

  useEffect(() => { void load(); }, [load]);

  const selected = products.find((product) => product.id === productId);

  async function buy() {
    if (!selected || destination.trim().length < 3) {
      setNotice("Pilih produk dan isi nomor tujuan terlebih dahulu.");
      return;
    }
    setSaving(true);
    setNotice("");
    try {
      const response = await orderClient.createOrderForSelf({
        productId: selected.id,
        quantity,
        destination: destination.trim(),
        idempotencyKey: idempotencyKey.current,
      });
      idempotencyKey.current = crypto.randomUUID();
      await load();
      if (response.checkoutUrl) {
        window.location.assign(response.checkoutUrl);
      } else {
        setNotice("Transaksi tercatat, tetapi tautan pembayaran belum tersedia.");
      }
    } catch (caught) {
      // Keep the same key after an uncertain failure. Retrying must ask about
      // the same transaction, not mint a second invoice.
      setNotice(checkoutErrorMessage(caught, caught instanceof Error ? caught.message : "Transaksi tidak dapat dibuat."));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section style={{ display: "grid", gap: 24 }}>
      <div style={panel}>
        <div style={panelHead}>
          <div>
            <p style={eyebrow}>BELI UNTUK AKUN SENDIRI</p>
            <h2 style={heading}>Harga khusus agen</h2>
            <p style={muted}>Harga sudah termasuk markup travel dan tidak menghasilkan komisi referral.</p>
          </div>
          <select value={seasonId} onChange={(event) => setSeasonId(event.target.value)} style={input}>
            {seasons.map((season) => <option key={season.id} value={season.id}>{season.name}</option>)}
          </select>
        </div>

        {notice && <p role="status" style={noticeStyle}>{notice}</p>}
        {loading ? <p style={muted}>Memuat katalog...</p> : products.length ? (
          <div style={purchaseGrid}>
            <label style={field}>
              <span style={label}>Produk digital</span>
              <select value={productId} onChange={(event) => setProductId(event.target.value)} style={input}>
                {products.map((product) => (
                  <option key={product.id} value={product.id}>{product.name} — {rupiah(product.unitPriceIdr)}</option>
                ))}
              </select>
            </label>
            <label style={field}>
              <span style={label}>Nomor tujuan</span>
              <input value={destination} onChange={(event) => setDestination(event.target.value)} style={input} placeholder="Contoh: 081234567890" maxLength={100} inputMode="tel" />
            </label>
            <label style={field}>
              <span style={label}>Jumlah</span>
              <input type="number" min={1} max={20} value={quantity} onChange={(event) => setQuantity(Math.max(1, Math.min(20, Number(event.target.value) || 1)))} style={input} />
            </label>
            <button type="button" onClick={() => void buy()} disabled={saving || !selected} style={primary}>
              <IconShoppingCart size={18} />{saving ? "Membuat tagihan..." : `Bayar ${selected ? rupiah(selected.unitPriceIdr * BigInt(quantity)) : ""}`}
            </button>
          </div>
        ) : (
          <p style={muted}>Belum ada produk digital aktif dengan harga agen yang lengkap untuk musim ini.</p>
        )}
      </div>

      <div style={panel}>
        <div style={panelHead}>
          <div><p style={eyebrow}>TRANSAKSI SAYA</p><h2 style={heading}>Riwayat pembelian</h2></div>
          <button type="button" onClick={() => void load()} style={secondary}><IconRefresh size={16} />Muat ulang</button>
        </div>
        {orders.length ? (
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead><tr>{["Nomor", "Produk", "Tujuan", "Total", "Status", ""].map((item) => <th key={item} style={th}>{item}</th>)}</tr></thead>
              <tbody>{orders.map((order) => (
                <tr key={order.id} style={row}>
                  <td style={td}>{order.receiptNumber || "-"}</td>
                  <td style={td}>{order.productName}</td>
                  <td style={td}>{order.destination || "-"}</td>
                  <td style={td}>{rupiah(order.totalPriceIdr)}</td>
                  <td style={td}><span style={badge}>{statusLabel[order.status] ?? order.status}</span></td>
                  <td style={td}>{order.status === "PENDING" && order.checkoutUrl ? <a href={order.checkoutUrl} style={payLink}>Bayar <IconExternalLink size={14} /></a> : null}</td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        ) : <p style={muted}>Belum ada pembelian pada musim ini.</p>}
      </div>

      <RefundWalletPanel agent />
    </section>
  );
}

const panel: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const panelHead: React.CSSProperties = { display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, flexWrap: "wrap", marginBottom: 18 };
const eyebrow: React.CSSProperties = { margin: "0 0 5px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const heading: React.CSSProperties = { margin: 0, fontSize: 21 };
const muted: React.CSSProperties = { margin: "6px 0 0", color: "var(--color-warm-500)", fontSize: 13 };
const noticeStyle: React.CSSProperties = { padding: 12, borderRadius: 8, background: "var(--color-gold-50)", color: "var(--color-warm-700)", fontSize: 13 };
const purchaseGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "minmax(220px,2fr) minmax(180px,1.4fr) 90px auto", gap: 12, alignItems: "end" };
const field: React.CSSProperties = { display: "grid", gap: 6 };
const label: React.CSSProperties = { fontSize: 12, fontWeight: 700, color: "var(--color-warm-600)" };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 11px", background: "#fff", font: "inherit" };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 7, whiteSpace: "nowrap" };
const secondary: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-emerald-900)", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 6 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", fontSize: 13 };
const th: React.CSSProperties = { padding: "10px 12px", textAlign: "left", color: "var(--color-warm-400)", fontSize: 11, borderBottom: "1px solid var(--color-cream-300)" };
const row: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", whiteSpace: "nowrap" };
const badge: React.CSSProperties = { display: "inline-block", padding: "4px 8px", borderRadius: 99, background: "var(--color-cream-200)", fontWeight: 700, fontSize: 11 };
const payLink: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-emerald-900)", fontWeight: 700 };
