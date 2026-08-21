"use client";

import { useEffect, useMemo, useState } from "react";
import { IconPrinter } from "@tabler/icons-react";
import { Order } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { operatorClient, orderClient, pilgrimClient } from "@/lib/rpc";

const PAYMENT_LABEL: Record<string, string> = { UNPAID: "Belum Bayar", DP: "DP (Uang Muka)", PAID: "Lunas" };

function formatIDR(value: number): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value);
}

export default function PilgrimInvoice({ pilgrimId }: { pilgrimId: string }) {
  const [pilgrim, setPilgrim] = useState<Pilgrim>();
  const [orders, setOrders] = useState<Order[]>([]);
  const [operatorName, setOperatorName] = useState("");

  useEffect(() => {
    pilgrimClient.getPilgrim({ pilgrimId }).then((value) => {
      setPilgrim(value);
      orderClient.listOrders({ seasonId: value.seasonId, limit: 2000, offset: 0 }).then((response) => setOrders(response.orders.filter((o) => o.pilgrimId === pilgrimId)));
    });
    operatorClient.getMyOperator({}).then((value) => setOperatorName(value.name));
  }, [pilgrimId]);

  const total = useMemo(() => orders.filter((o) => o.status === "PAID").reduce((sum, o) => sum + Number(o.totalPriceIdr), 0), [orders]);

  if (!pilgrim) return <main style={page}>Memuat data invoice...</main>;

  return <main style={page}>
    <div className="no-print" style={toolbar}><button onClick={() => window.print()} style={printButton}><IconPrinter size={18} />Cetak / Simpan PDF</button></div>
    <div style={sheet}>
      <header style={header}>
        <div><h1 style={{ margin: 0, fontFamily: "'Playfair Display', serif" }}>{operatorName || "Tawafiq Hub"}</h1><p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Invoice Jamaah</p></div>
        <div style={{ textAlign: "right" }}><p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Tanggal Cetak</p><p style={{ margin: 0, fontWeight: 700 }}>{new Date().toLocaleDateString("id-ID")}</p></div>
      </header>
      <div className="gold-divider" />
      <section style={grid}>
        <div><h3 style={sectionTitle}>Ditagihkan kepada</h3><p style={{ margin: 0, fontWeight: 700 }}>{pilgrim.fullName}</p><p style={{ margin: "2px 0 0", color: "var(--color-warm-500)" }}>{pilgrim.passportNumber}</p><p style={{ margin: "2px 0 0", color: "var(--color-warm-500)" }}>{pilgrim.phone || "-"}</p></div>
        <div><h3 style={sectionTitle}>Status Pembayaran</h3><p style={{ margin: 0, fontWeight: 700, color: pilgrim.paymentStatus === "PAID" ? "var(--color-emerald-900)" : pilgrim.paymentStatus === "DP" ? "var(--color-gold-800)" : "var(--color-danger-600)" }}>{PAYMENT_LABEL[pilgrim.paymentStatus] ?? "Belum Bayar"}</p>{pilgrim.paymentNotes && <p style={{ margin: "2px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{pilgrim.paymentNotes}</p>}</div>
      </section>
      <section style={{ marginTop: 24 }}>
        <table style={table}>
          <thead><tr><th style={th}>Produk</th><th style={th}>Jumlah</th><th style={th}>Harga Satuan</th><th style={th}>Total</th><th style={th}>Status</th></tr></thead>
          <tbody>
            {orders.length ? orders.map((order) => <tr key={order.id}><td style={td}>{order.productName}</td><td style={td}>{order.quantity}</td><td style={td}>{formatIDR(Number(order.unitPriceIdr))}</td><td style={td}>{formatIDR(Number(order.totalPriceIdr))}</td><td style={td}>{order.status}</td></tr>) : <tr><td style={td} colSpan={5}>Belum ada pesanan tercatat untuk jamaah ini.</td></tr>}
          </tbody>
        </table>
      </section>
      <section style={{ marginTop: 20, display: "flex", justifyContent: "flex-end" }}>
        <div style={{ textAlign: "right" }}><p style={{ margin: 0, color: "var(--color-warm-500)" }}>Total Terbayar (Lunas)</p><p style={{ margin: 0, fontSize: 24, fontWeight: 700, color: "var(--color-emerald-900)" }}>{formatIDR(total)}</p></div>
      </section>
    </div>
  </main>;
}

const page: React.CSSProperties = { maxWidth: 800, margin: "0 auto", padding: "32px 24px" };
const toolbar: React.CSSProperties = { display: "flex", justifyContent: "flex-end", marginBottom: 16 };
const printButton: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px", display: "inline-flex", gap: 8, alignItems: "center" };
const sheet: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 32 };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, marginTop: 20 };
const sectionTitle: React.CSSProperties = { margin: "0 0 6px", fontSize: 12, textTransform: "uppercase", letterSpacing: ".06em", color: "var(--color-warm-400)" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse" };
const th: React.CSSProperties = { textAlign: "left", padding: "10px 8px", fontSize: 12, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-400)" };
const td: React.CSSProperties = { padding: "10px 8px", borderBottom: "1px solid var(--color-cream-300)", fontSize: 14 };
