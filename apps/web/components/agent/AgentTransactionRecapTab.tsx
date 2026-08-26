"use client";

import { useEffect, useState } from "react";
import { IconReceipt, IconArrowBackUp } from "@tabler/icons-react";
import { ReferredCustomerRecap } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { agentClient } from "@/lib/rpc";

const money = (n: bigint) => `Rp${Number(n).toLocaleString("id-ID")}`;
const day = (d?: Date) => d?.toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) ?? "—";

export default function AgentTransactionRecapTab() {
  const [customers, setCustomers] = useState<ReferredCustomerRecap[]>([]);
  const [totalPaid, setTotalPaid] = useState(0n);
  const [totalRefunded, setTotalRefunded] = useState(0n);
  const [totalCommission, setTotalCommission] = useState(0n);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    agentClient.listMyReferredTransactions({})
      .then((result) => {
        setCustomers(result.customers);
        setTotalPaid(result.totalPaidIdr);
        setTotalRefunded(result.totalRefundedIdr);
        setTotalCommission(result.totalCommissionIdr);
      })
      .catch(() => setError("Gagal memuat rekap transaksi."))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <p style={{ color: "var(--color-warm-500)" }}>Memuat rekap transaksi...</p>;
  if (error) return <p style={{ color: "var(--color-danger-600)" }}>{error}</p>;

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={stats}>
        <div style={stat}><small style={statLabel}>Total Transaksi Jamaah</small><strong style={statValue}>{money(totalPaid)}</strong></div>
        <div style={stat}><small style={statLabel}>Komisi Anda</small><strong style={statValue}>{money(totalCommission)}</strong></div>
        {/* Surfaced only when it happened — a permanent "Rp0 dikembalikan" is
            noise, but a refund an agent has not noticed is not. */}
        {totalRefunded > 0n && (
          <div style={{ ...stat, borderTopColor: "var(--color-danger-600)" }}>
            <small style={statLabel}>Dikembalikan</small>
            <strong style={statValue}>{money(totalRefunded)}</strong>
          </div>
        )}
      </div>

      <p style={{ color: "var(--color-warm-500)", fontSize: 13, margin: 0 }}>
        Nilai bersih — transaksi yang direfund tidak dihitung, dan komisinya sudah ditarik kembali.
      </p>

      {customers.length === 0 ? (
        <div style={empty}>
          <IconReceipt size={44} color="var(--color-warm-400)" />
          <p style={{ margin: 0, fontWeight: 600 }}>Belum ada transaksi</p>
          <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 14 }}>
            Transaksi jamaah yang mendaftar lewat link referral Anda akan muncul di sini.
          </p>
        </div>
      ) : (
        <ul style={list}>
          {customers.map((customer) => (
            <li key={customer.pilgrimId} style={card}>
              <div style={cardHead}>
                <div>
                  <strong style={{ display: "block" }}>{customer.pilgrimName}</strong>
                  <small style={{ color: "var(--color-warm-500)" }}>
                    {customer.orderCount} transaksi · terakhir {day(customer.lastTransactionAt?.toDate())}
                  </small>
                </div>
                <div style={{ textAlign: "right" }}>
                  <strong style={{ display: "block" }}>{money(customer.totalPaidIdr)}</strong>
                  <small style={{ color: "var(--color-emerald-800)" }}>komisi {money(customer.commissionIdr)}</small>
                </div>
              </div>
              {customer.refundedOrderCount > 0 && (
                <p style={refund}>
                  <IconArrowBackUp size={15} />
                  {customer.refundedOrderCount} transaksi direfund ({money(customer.refundedIdr)}) — komisinya sudah ditarik kembali.
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

const stats: React.CSSProperties = { display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit,minmax(160px,1fr))" };
const stat: React.CSSProperties = { display: "grid", gap: 4, padding: 16, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-emerald-800)", borderRadius: 10 };
const statLabel: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 11, letterSpacing: ".06em" };
const statValue: React.CSSProperties = { fontSize: 19, fontWeight: 700 };
const list: React.CSSProperties = { listStyle: "none", margin: 0, padding: 0, display: "grid", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: 16, display: "grid", gap: 10 };
const cardHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, alignItems: "flex-start", flexWrap: "wrap" };
const refund: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, margin: 0, padding: 10, background: "#fff1f2", borderRadius: 8, color: "var(--color-danger-600)", fontSize: 13 };
const empty: React.CSSProperties = { display: "grid", placeItems: "center", gap: 8, padding: "48px 20px", border: "1px dashed var(--color-cream-400)", borderRadius: 12, textAlign: "center" };
