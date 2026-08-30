"use client";

import { useEffect, useMemo, useState } from "react";
import { IconCash, IconReceipt2 } from "@tabler/icons-react";
import { Order } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { orderClient, pilgrimClient } from "@/lib/rpc";

function formatIDR(value: number): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value);
}

export default function SeasonFinancialSummary({ seasonId }: { seasonId: string }) {
  const [orders, setOrders] = useState<Order[]>([]);
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!seasonId) return;
    setLoading(true);
    Promise.all([
      orderClient.listOrders({ seasonId, limit: 2000, offset: 0 }),
      pilgrimClient.listPilgrims({ seasonId, limit: 2000, offset: 0 }),
    ]).then(([orderResponse, pilgrimResponse]) => {
      setOrders(orderResponse.orders);
      setPilgrims(pilgrimResponse.pilgrims);
    }).finally(() => setLoading(false));
  }, [seasonId]);

  const summary = useMemo(() => {
    const paidOrders = orders.filter((o) => o.status === "PAID");
    const totalRevenue = paidOrders.reduce((sum, o) => sum + Number(o.totalPriceIdr), 0);
    const totalOperatorMargin = paidOrders.reduce((sum, o) => sum + Number(o.operatorAmountIdr), 0);
    const totalAgentCommission = paidOrders.reduce((sum, o) => sum + Number(o.agentCommissionIdr), 0);
    const pendingOrders = orders.filter((o) => o.status === "PENDING").length;
    const paymentCounts = { UNPAID: 0, DP: 0, PAID: 0 };
    for (const p of pilgrims) { const status = p.paymentStatus || "UNPAID"; if (status in paymentCounts) paymentCounts[status as keyof typeof paymentCounts]++; }
    return { totalRevenue, totalOperatorMargin, totalAgentCommission, paidOrderCount: paidOrders.length, pendingOrders, paymentCounts };
  }, [orders, pilgrims]);

  if (!seasonId) return null;

  return <section style={wrap}>
    <h2 style={{ margin: "0 0 4px", display: "flex", alignItems: "center", gap: 8 }}><IconCash size={22} />Ringkasan Keuangan Musim</h2>
    {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <>
      <div style={grid}>
        <div style={card}><span style={label}>Total Pendapatan</span><strong style={{ ...value, color: "var(--color-emerald-900)" }}>{formatIDR(summary.totalRevenue)}</strong><span style={sub}>{summary.paidOrderCount} pesanan lunas</span></div>
        <div style={card}><span style={label}>Margin Operator</span><strong style={value}>{formatIDR(summary.totalOperatorMargin)}</strong></div>
        <div style={card}><span style={label}>Komisi Agen</span><strong style={value}>{formatIDR(summary.totalAgentCommission)}</strong></div>
        <div style={card}><span style={label}>Pesanan Tertunda</span><strong style={{ ...value, color: "var(--color-gold-800)" }}>{summary.pendingOrders}</strong></div>
      </div>
      <h3 style={{ margin: "20px 0 4px", display: "flex", alignItems: "center", gap: 8, fontSize: 16 }}><IconReceipt2 size={18} />Status Pembayaran Jamaah</h3>
      <div style={grid}>
        <div style={card}><span style={label}>Belum Bayar</span><strong style={{ ...value, color: "var(--color-danger-600)" }}>{summary.paymentCounts.UNPAID} <span className="tw-stat__unit">jamaah</span></strong></div>
        <div style={card}><span style={label}>DP</span><strong style={{ ...value, color: "var(--color-gold-800)" }}>{summary.paymentCounts.DP} <span className="tw-stat__unit">jamaah</span></strong></div>
        <div style={card}><span style={label}>Lunas</span><strong style={{ ...value, color: "var(--color-emerald-900)" }}>{summary.paymentCounts.PAID} <span className="tw-stat__unit">jamaah</span></strong></div>
      </div>
    </>}
  </section>;
}

const wrap: React.CSSProperties = { marginTop: 28, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12, marginTop: 12 };
const card: React.CSSProperties = { border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)", padding: "14px 16px", display: "grid", gap: 4 };
const label: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-500)", textTransform: "uppercase", letterSpacing: ".05em" };
const value: React.CSSProperties = { fontSize: 22, fontWeight: 700 };
const sub: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-400)" };
