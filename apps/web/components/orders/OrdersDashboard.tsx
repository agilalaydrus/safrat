"use client";
import { useCallback, useEffect, useState } from "react";
import { IconArrowBackUp, IconCopy, IconPlus, IconShoppingCart } from "@tabler/icons-react";
import { Order } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { orderClient, seasonClient } from "@/lib/rpc";
import CreateOrderDialog from "./CreateOrderDialog";
import RefundOrderDialog from "./RefundOrderDialog";

const rupiah = (n: bigint) => `Rp${Number(n).toLocaleString("id-ID")}`;
const STATUS_LABEL: Record<string, string> = { PENDING: "Menunggu Bayar", PAID: "Lunas", EXPIRED: "Kedaluwarsa", FAILED: "Gagal", CANCELLED: "Dibatalkan", REFUNDED: "Direfund", HELD: "Perlu Ditinjau" };
const STATUS_STYLE: Record<string, React.CSSProperties> = {
  PENDING: { background: "var(--color-gold-50)", color: "var(--color-gold-800)" },
  PAID: { background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" },
  EXPIRED: { background: "var(--color-cream-200)", color: "var(--color-warm-500)" },
  FAILED: { background: "#ffe4e6", color: "var(--color-danger-600)" },
  CANCELLED: { background: "var(--color-cream-200)", color: "var(--color-warm-500)" },
  REFUNDED: { background: "#ffe4e6", color: "var(--color-danger-600)" },
  // Amber, not red: money arrived and is waiting on a decision, which is a
  // different thing from a transaction that failed.
  HELD: { background: "var(--color-gold-50)", color: "#b45309" },
};

const pageSize = 20;

export default function OrdersDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [orders, setOrders] = useState<Order[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [notice, setNotice] = useState("");
  const [refunding, setRefunding] = useState<Order | null>(null);

  const load = useCallback(async (id = seasonId) => {
    if (!id) return;
    setLoading(true);
    try {
      const response = await orderClient.listOrders({ seasonId: id, limit: pageSize, offset });
      setOrders(response.orders);
      setTotal(Number(response.totalCount));
    } catch { setNotice("Gagal memuat daftar pesanan."); }
    finally { setLoading(false); }
  }, [offset, seasonId]);

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => { setSeasons(r.seasons); setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? ""); }).catch(() => setNotice("Gagal memuat daftar musim."));
  }, []);
  useEffect(() => { setOffset(0); }, [seasonId]);
  useEffect(() => { void load(); }, [load]);

  // Only the current page is loaded — accurate as long as everything fits
  // on one page (the common case today). Once a season has more than
  // pageSize orders, these two reflect this page only, not the whole
  // season; "Total Pesanan" always uses the real season-wide count.
  const paidOrders = orders.filter((o) => o.status === "PAID");
  const totalRevenue = paidOrders.reduce((sum, o) => sum + o.totalPriceIdr, 0n);
  const pageScoped = total > pageSize;

  return (
    <main style={page}>
      <header style={header}>
        <div><p style={eyebrow}>OPERASIONAL / PESANAN</p><h1 style={title}>Pesanan Produk Digital</h1></div>
        <div style={actions}>
          <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
            {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
          <button style={gold} onClick={() => setOpen(true)} disabled={!seasonId}><IconPlus size={18} />Buat Pesanan</button>
        </div>
      </header>
      <div className="gold-divider" />
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div style={stats}>
        {[["Total Pesanan", total], [pageScoped ? "Lunas (halaman ini)" : "Lunas", paidOrders.length], [pageScoped ? "Pendapatan (halaman ini)" : "Total Pendapatan", rupiah(totalRevenue)]].map(([l, v]) => (
          <div key={String(l)} style={stat}><small>{l}</small><strong>{v}</strong></div>
        ))}
      </div>
      {loading ? (
        <p style={{ color: "var(--color-warm-500)" }}>Memuat pesanan...</p>
      ) : orders.length ? (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Jamaah", "Produk", "Jumlah", "Total", "Status", "Dibuat", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {orders.map((o) => (
                <tr key={o.id} style={tr}>
                  <td style={td}>{o.pilgrimName}</td>
                  <td style={td}>{o.productName}</td>
                  <td style={td}>{o.quantity}</td>
                  <td style={td}>{rupiah(o.totalPriceIdr)}</td>
                  <td style={td}><span style={{ ...badge, ...STATUS_STYLE[o.status] }}>{STATUS_LABEL[o.status] ?? o.status}</span></td>
                  <td style={td}>{o.createdAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}</td>
                  <td style={td}>
                    {o.status === "PENDING" && o.checkoutUrl && <button style={ghost} onClick={() => navigator.clipboard.writeText(o.checkoutUrl)}><IconCopy size={14} />Salin Link</button>}
                    {o.status === "PAID" && <button style={ghost} onClick={() => setRefunding(o)}><IconArrowBackUp size={14} />Refund</button>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <footer style={pagination}>
            <span>{Math.min(offset + 1, total)}–{Math.min(offset + pageSize, total)} dari {total} pesanan</span>
            <div>
              <button onClick={() => setOffset(Math.max(0, offset - pageSize))} disabled={!offset} style={ghost}>Sebelumnya</button>
              <button onClick={() => setOffset(offset + pageSize)} disabled={offset + pageSize >= total} style={ghost}>Berikutnya</button>
            </div>
          </footer>
        </div>
      ) : (
        <section style={empty}>
          <IconShoppingCart size={48} color="var(--color-warm-400)" />
          <h2 style={{ margin: 0 }}>Belum ada pesanan</h2>
          <p style={{ color: "var(--color-warm-500)" }}>Buat pesanan untuk jamaah, atau tunggu jamaah checkout sendiri lewat Pilgrim App.</p>
          <button style={gold} onClick={() => setOpen(true)} disabled={!seasonId}>Buat Pesanan</button>
        </section>
      )}
      <RefundOrderDialog order={refunding} onClose={() => setRefunding(null)} onRefunded={(message) => { setNotice(message); void load(); }} />
      <CreateOrderDialog open={open} seasonId={seasonId} onClose={() => setOpen(false)} onCreated={() => { setNotice("Pesanan berhasil dibuat."); void load(); }} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 20, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const actions: React.CSSProperties = { display: "flex", gap: 10 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "#fff" };
const gold: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const stats: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(190px,1fr))", gap: 14, margin: "24px 0" };
const stat: React.CSSProperties = { display: "grid", gap: 6, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-gold-500)", borderRadius: 10 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 14, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 14, color: "var(--color-warm-700)", whiteSpace: "nowrap" };
const badge: React.CSSProperties = { padding: "4px 8px", borderRadius: 99, fontSize: 11, fontWeight: 700 };
const ghost: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "4px 8px", background: "transparent", display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-emerald-900)", fontSize: 12 };
const pagination: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 16, padding: 16, color: "var(--color-warm-500)", alignItems: "center", flexWrap: "wrap" };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
