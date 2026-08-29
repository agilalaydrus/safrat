"use client";

import { useCallback, useEffect, useState } from "react";
import { IconAlertTriangle } from "@tabler/icons-react";
import { PlatformTransaction } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const money = (n: bigint) => `Rp${Number(n).toLocaleString("id-ID")}`;
const when = (d?: Date) => d?.toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) ?? "—";

const PAYMENT: Record<string, { label: string; tone: string }> = {
  PENDING: { label: "Menunggu Bayar", tone: "#b45309" },
  PAID: { label: "Lunas", tone: "var(--color-emerald-800)" },
  HELD: { label: "Perlu Ditinjau", tone: "#b45309" },
  REFUNDED: { label: "Direfund", tone: "var(--color-danger-600)" },
  EXPIRED: { label: "Kedaluwarsa", tone: "var(--color-warm-400)" },
  FAILED: { label: "Gagal", tone: "var(--color-danger-600)" },
  CANCELLED: { label: "Dibatalkan", tone: "var(--color-warm-400)" },
};

const DELIVERY: Record<string, { label: string; tone: string }> = {
  PENDING: { label: "Antre kirim", tone: "#b45309" },
  SENT: { label: "Terkirim, menunggu jawaban", tone: "#b45309" },
  DELIVERED: { label: "Sampai", tone: "var(--color-emerald-800)" },
  FAILED: { label: "Gagal", tone: "var(--color-danger-600)" },
  NEEDS_REVIEW: { label: "Perlu ditinjau", tone: "var(--color-danger-600)" },
};

export default function TransactionsTab() {
  const [transactions, setTransactions] = useState<PlatformTransaction[]>([]);
  const [needsAttention, setNeedsAttention] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    platformClient.listTransactions({ needsAttention, limit: 100 })
      .then((r) => setTransactions(r.transactions))
      .catch(() => setError("Gagal memuat transaksi."))
      .finally(() => setLoading(false));
  }, [needsAttention]);
  useEffect(() => { load(); }, [load]);

  return (
    <section style={{ display: "grid", gap: 14 }}>
      <p style={muted}>
        Setiap transaksi tercatat sejak dibuat, bukan sejak dibayar — checkout yang ditinggalkan pun tetap ada di sini.
        Dua kolom status karena keduanya pertanyaan berbeda: sudah dibayar, dan sudah sampai.
      </p>
      <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 14 }}>
        <input type="checkbox" checked={needsAttention} onChange={(e) => setNeedsAttention(e.target.checked)} />
        Hanya yang perlu ditindak (checkout mencurigakan, pembayaran tidak cocok, atau sudah dibayar tapi belum sampai)
      </label>

      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {loading ? <p style={muted}>Memuat...</p> : transactions.length === 0 ? (
        <p style={muted}>
          {needsAttention ? "Tidak ada transaksi yang perlu ditindak." : "Belum ada transaksi."}
        </p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Struk", "Travel / Jamaah", "Produk", "Nilai", "Pembayaran", "Pengiriman", "Dibuat"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {transactions.map((transaction) => {
                const payment = PAYMENT[transaction.status] ?? { label: transaction.status, tone: "var(--color-warm-700)" };
                const delivery = transaction.fulfilmentStatus ? DELIVERY[transaction.fulfilmentStatus] : undefined;
                return (
                  <tr key={transaction.orderId} style={tr}>
                    <td style={{ ...td, fontFamily: "monospace", fontSize: 12 }}>{transaction.receiptNumber}</td>
                    <td style={td}>
                      <strong>{transaction.operatorName}</strong>
                      <small style={{ display: "block", color: "var(--color-warm-400)" }}>{transaction.pilgrimName}</small>
                    </td>
                    <td style={td}>{transaction.productName}
                      <small style={{ display: "block", color: "var(--color-warm-400)" }}>{transaction.category}</small>
                    </td>
                    <td style={td}>
                      {money(transaction.amountIdr)}
                      {/* Only shown when it differs — a matching payment needs
                          no second number, a mismatched one is the whole story. */}
                      {transaction.paidAmountIdr > 0n && transaction.paidAmountIdr !== transaction.amountIdr && (
                        <small style={{ display: "block", color: "var(--color-danger-600)" }}>
                          dibayar {money(transaction.paidAmountIdr)}
                        </small>
                      )}
                    </td>
                    <td style={{ ...td, color: payment.tone, fontWeight: 600 }}>
                      {payment.label}
                      {transaction.heldReason && (
                        <small style={{ display: "block", fontWeight: 400, color: "var(--color-warm-500)", whiteSpace: "normal", maxWidth: 220 }}>
                          {transaction.heldReason}
                        </small>
                      )}
                      {transaction.riskLevel === "REVIEW" && transaction.status === "PENDING" && (
                        <small style={{ display: "block", fontWeight: 400, color: "#b45309", whiteSpace: "normal", maxWidth: 220 }}>
                          <IconAlertTriangle size={12} style={{ verticalAlign: "-2px", marginRight: 4 }} />
                          {transaction.riskReason}
                        </small>
                      )}
                    </td>
                    <td style={{ ...td, color: delivery?.tone ?? "var(--color-warm-400)" }}>
                      {delivery ? (
                        <>
                          {(transaction.fulfilmentStatus === "NEEDS_REVIEW" || transaction.fulfilmentStatus === "FAILED") &&
                            <IconAlertTriangle size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} />}
                          {delivery.label}
                          {transaction.supplierName && (
                            <small style={{ display: "block", color: "var(--color-warm-400)" }}>{transaction.supplierName}</small>
                          )}
                          {transaction.fulfilmentError && (
                            <small style={{ display: "block", color: "var(--color-warm-500)", whiteSpace: "normal", maxWidth: 240 }}>
                              {transaction.fulfilmentError}
                            </small>
                          )}
                        </>
                      ) : "—"}
                    </td>
                    <td style={{ ...td, whiteSpace: "nowrap" }}>{when(transaction.createdAt?.toDate())}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: 0 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
