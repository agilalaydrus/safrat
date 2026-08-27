"use client";

import { useEffect, useState } from "react";
import { IconReceipt, IconWifiOff, IconArrowBackUp, IconExternalLink, IconFileText } from "@tabler/icons-react";
import TransactionReceipt from "@/components/pilgrim/TransactionReceipt";
import { PilgrimTransaction } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_app_pb";
import { pilgrimAppClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { usePilgrimCode } from "@/lib/pilgrim-context";

const money = (n: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));
const day = (d?: Date) => d?.toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) ?? "";

const STATUS: Record<string, { label: string; style: React.CSSProperties }> = {
  PENDING: { label: "Menunggu Bayar", style: { background: "var(--color-gold-50)", color: "#b45309" } },
  PAID: { label: "Lunas", style: { background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" } },
  REFUNDED: { label: "Dana Dikembalikan", style: { background: "#ffe4e6", color: "var(--color-danger-600)" } },
  EXPIRED: { label: "Kedaluwarsa", style: { background: "var(--color-cream-200)", color: "var(--color-warm-500)" } },
  FAILED: { label: "Gagal", style: { background: "#ffe4e6", color: "var(--color-danger-600)" } },
  CANCELLED: { label: "Dibatalkan", style: { background: "var(--color-cream-200)", color: "var(--color-warm-500)" } },
  // The jamaah paid, but the amount did not match the bill. Saying so plainly
  // beats a status that looks like failure — their money is not lost.
  HELD: { label: "Sedang Diperiksa", style: { background: "var(--color-gold-50)", color: "#b45309" } },
};

export default function PilgrimTransactionsPage() {
  const code = usePilgrimCode();
  const [transactions, setTransactions] = useState<PilgrimTransaction[]>([]);
  const [totalPaid, setTotalPaid] = useState(0n);
  const [balance, setBalance] = useState(0n);
  const [fromCache, setFromCache] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [receipt, setReceipt] = useState<PilgrimTransaction | null>(null);

  useEffect(() => {
    if (!code) return;
    cachedFetch(`pilgrim-transactions:${code}`, () => pilgrimAppClient.listMyTransactions({ appAccessCode: code })).then((result) => {
      if (result.data) {
        setTransactions(result.data.transactions);
        setTotalPaid(result.data.totalPaidIdr);
        setBalance(result.data.balanceIdr);
      }
      setFromCache(result.fromCache);
      setLoaded(true);
    });
  }, [code]);

  return (
    <main style={page}>
      <p style={eyebrow}>RIWAYAT TRANSAKSI</p>
      <h1 style={title}>Transaksi Saya</h1>
      {fromCache && (
        <p style={cacheNote}><IconWifiOff size={15} />Data terakhir yang tersimpan — sedang offline.</p>
      )}

      <div style={summary}>
        <div style={summaryCard}>
          <small style={summaryLabel}>Total Dibayar</small>
          <strong style={summaryValue}>{money(totalPaid)}</strong>
        </div>
        {/* Only shown when there is something to show: a zero balance is not a
            fact a jamaah needs on screen, but money the operator is holding is. */}
        {balance > 0n && (
          <div style={{ ...summaryCard, borderTopColor: "var(--color-danger-600)" }}>
            <small style={summaryLabel}>Saldo Dikembalikan</small>
            <strong style={summaryValue}>{money(balance)}</strong>
            <small style={{ color: "var(--color-warm-500)", fontSize: 12 }}>
              Dana ini dipegang travel untuk Anda. Hubungi petugas untuk pencairan.
            </small>
          </div>
        )}
      </div>

      {!loaded ? (
        <p style={{ color: "var(--color-warm-500)" }}>Memuat transaksi...</p>
      ) : transactions.length === 0 ? (
        <section style={empty}>
          <IconReceipt size={44} color="var(--color-warm-400)" />
          <p style={{ margin: 0, fontWeight: 600 }}>Belum ada transaksi</p>
          <p style={{ color: "var(--color-warm-500)", margin: 0, fontSize: 14 }}>
            Pembelian produk dan paket Anda akan tercatat di sini.
          </p>
        </section>
      ) : (
        <ul style={list}>
          {transactions.map((transaction) => {
            const status = STATUS[transaction.status] ?? { label: transaction.status, style: {} };
            return (
              <li key={transaction.orderId} style={card}>
                <div style={cardHead}>
                  <div>
                    <strong style={{ display: "block" }}>{transaction.productName}</strong>
                    <small style={{ color: "var(--color-warm-500)" }}>
                      {transaction.quantity > 1 ? `${transaction.quantity} × · ` : ""}
                      {day(transaction.createdAt?.toDate())}
                    </small>
                  </div>
                  <span style={{ ...badge, ...status.style }}>{status.label}</span>
                </div>

                <div style={amountRow}>
                  <span style={{
                    fontSize: 18, fontWeight: 700,
                    // A refunded amount is struck through rather than removed:
                    // the transaction happened, and hiding it would make the
                    // history disagree with what the jamaah remembers paying.
                    textDecoration: transaction.status === "REFUNDED" ? "line-through" : "none",
                    color: transaction.status === "REFUNDED" ? "var(--color-warm-400)" : "var(--color-warm-700)",
                  }}>
                    {money(transaction.amountIdr)}
                  </span>
                  {transaction.status === "PENDING" && transaction.checkoutUrl && (
                    <a href={transaction.checkoutUrl} style={payLink}>
                      Bayar Sekarang <IconExternalLink size={14} />
                    </a>
                  )}
                </div>

                {/* Every transaction can be shown as a receipt, whatever its
                    state — an expired or refunded one is exactly what somebody
                    needs proof of. */}
                <button style={receiptButton} onClick={() => setReceipt(transaction)}>
                  <IconFileText size={15} />Lihat Struk
                </button>

                {transaction.status === "HELD" && (
                  <p style={heldNote}>
                    Pembayaran Anda sudah diterima, tetapi nominalnya belum cocok dengan tagihan.
                    Petugas travel sedang memeriksanya — dana Anda aman.
                  </p>
                )}

                {transaction.refundedIdr > 0n && (
                  <div style={refundNote}>
                    <IconArrowBackUp size={16} />
                    <div>
                      <strong>{money(transaction.refundedIdr)} dikembalikan</strong>
                      <small style={{ display: "block", color: "var(--color-warm-500)" }}>
                        {day(transaction.refundedAt?.toDate())}
                        {transaction.refundReason ? ` — ${transaction.refundReason}` : ""}
                      </small>
                    </div>
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
      <TransactionReceipt transaction={receipt} onClose={() => setReceipt(null)} />
    </main>
  );
}

const page: React.CSSProperties = { padding: "24px 18px 96px", maxWidth: 640, margin: "0 auto" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 30, fontWeight: 500, margin: "0 0 16px" };
const cacheNote: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 13, margin: "0 0 14px" };
const summary: React.CSSProperties = { display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit,minmax(160px,1fr))", margin: "0 0 22px" };
const summaryCard: React.CSSProperties = { display: "grid", gap: 4, padding: 16, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-emerald-800)", borderRadius: 10 };
const summaryLabel: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 11, letterSpacing: ".06em" };
const summaryValue: React.CSSProperties = { fontSize: 20, fontWeight: 700 };
const list: React.CSSProperties = { listStyle: "none", margin: 0, padding: 0, display: "grid", gap: 12 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16, display: "grid", gap: 10 };
const cardHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, alignItems: "flex-start" };
const badge: React.CSSProperties = { padding: "4px 9px", borderRadius: 99, fontSize: 11, fontWeight: 700, whiteSpace: "nowrap" };
const amountRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap" };
const payLink: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 5, minHeight: 40, padding: "0 14px", borderRadius: 8, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, fontSize: 14, textDecoration: "none" };
const receiptButton: React.CSSProperties = { justifySelf: "start", minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-emerald-900)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 600 };
const heldNote: React.CSSProperties = { margin: 0, padding: 12, background: "var(--color-gold-50)", borderRadius: 8, color: "#b45309", fontSize: 13 };
const refundNote: React.CSSProperties = { display: "flex", gap: 8, alignItems: "flex-start", padding: 12, background: "#fff1f2", borderRadius: 8, color: "var(--color-danger-600)", fontSize: 14 };
const empty: React.CSSProperties = { display: "grid", placeItems: "center", gap: 8, padding: "48px 20px", border: "1px dashed var(--color-cream-400)", borderRadius: 12, textAlign: "center" };
