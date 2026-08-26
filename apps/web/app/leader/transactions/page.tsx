"use client";

import { useEffect, useState } from "react";
import { IconArrowDownRight, IconArrowUpRight, IconClockHour4, IconAdjustments, IconReceipt } from "@tabler/icons-react";
import { WalletTransaction, WalletTransactionType } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { agentClient } from "@/lib/rpc";

const money = (n: bigint | number) => `Rp${Number(n).toLocaleString("id-ID")}`;
const day = (d?: Date) => d?.toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) ?? "";

// Each kind is presented with its own direction and colour, because the whole
// point of this page is that a balance change can be accounted for. A reversal
// that looked like an ordinary credit would defeat it.
type KindStyle = { label: string; sign: string; color: string; Icon: typeof IconArrowUpRight };

const ADJUSTMENT: KindStyle = { label: "Penyesuaian", sign: "", color: "var(--color-warm-700)", Icon: IconAdjustments };

const KIND: Record<number, KindStyle> = {
  [WalletTransactionType.CREDIT]: { label: "Komisi masuk", sign: "+", color: "var(--color-emerald-800)", Icon: IconArrowUpRight },
  [WalletTransactionType.REVERSAL]: { label: "Komisi ditarik", sign: "−", color: "var(--color-danger-600)", Icon: IconArrowDownRight },
  [WalletTransactionType.DEBIT]: { label: "Pencairan", sign: "−", color: "var(--color-warm-700)", Icon: IconArrowDownRight },
  [WalletTransactionType.PENDING_REQUEST]: { label: "Menunggu pencairan", sign: "", color: "#b45309", Icon: IconClockHour4 },
  [WalletTransactionType.ADJUSTMENT]: ADJUSTMENT,
};

export default function LeaderTransactionsPage() {
  const [transactions, setTransactions] = useState<WalletTransaction[]>([]);
  const [balance, setBalance] = useState(0n);
  const [earned, setEarned] = useState(0n);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    agentClient.getMyWallet({})
      .then((wallet) => { setTransactions(wallet.transactions); setBalance(wallet.balanceIdr); setEarned(wallet.totalEarnedIdr); })
      .finally(() => setLoading(false));
  }, []);

  return (
    <main style={page}>
      <p style={eyebrow}>RIWAYAT TRANSAKSI</p>
      <h1 style={title}>Transaksi Komisi</h1>
      <div style={summary}>
        <div style={card}><small style={label}>Total Komisi</small><strong style={value}>{money(earned)}</strong></div>
        <div style={card}><small style={label}>Saldo</small><strong style={value}>{money(balance)}</strong></div>
      </div>

      {loading ? (
        <p style={{ color: "var(--color-warm-500)" }}>Memuat transaksi...</p>
      ) : transactions.length === 0 ? (
        <section style={empty}>
          <IconReceipt size={44} color="var(--color-warm-400)" />
          <p style={{ margin: 0, fontWeight: 600 }}>Belum ada transaksi</p>
          <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 14 }}>Komisi dari jamaah referral Anda akan tercatat di sini.</p>
        </section>
      ) : (
        <ul style={list}>
          {transactions.map((transaction) => {
            const kind = KIND[transaction.type] ?? ADJUSTMENT;
            return (
              <li key={`${transaction.id}-${transaction.type}`} style={row}>
                <span style={{ ...iconWrap, color: kind.color }}><kind.Icon size={18} /></span>
                <div style={{ minWidth: 0, flex: 1 }}>
                  <strong style={{ display: "block" }}>{transaction.description}</strong>
                  <small style={{ color: "var(--color-warm-500)" }}>
                    {kind.label} · {day(transaction.createdAt?.toDate())}
                  </small>
                </div>
                <strong style={{ color: kind.color, whiteSpace: "nowrap" }}>
                  {kind.sign}{money(transaction.amountIdr)}
                </strong>
              </li>
            );
          })}
        </ul>
      )}
    </main>
  );
}

const page: React.CSSProperties = { padding: "24px 18px 96px", maxWidth: 640, margin: "0 auto" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 30, fontWeight: 500, margin: "0 0 16px" };
const summary: React.CSSProperties = { display: "grid", gap: 12, gridTemplateColumns: "repeat(auto-fit,minmax(150px,1fr))", margin: "0 0 22px" };
const card: React.CSSProperties = { display: "grid", gap: 4, padding: 16, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-emerald-800)", borderRadius: 10 };
const label: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 11, letterSpacing: ".06em" };
const value: React.CSSProperties = { fontSize: 20, fontWeight: 700 };
const list: React.CSSProperties = { listStyle: "none", margin: 0, padding: 0, display: "grid", gap: 10 };
const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, padding: 14, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const iconWrap: React.CSSProperties = { display: "grid", placeItems: "center", width: 34, height: 34, borderRadius: 99, background: "var(--color-cream-100)", flexShrink: 0 };
const empty: React.CSSProperties = { display: "grid", placeItems: "center", gap: 8, padding: "48px 20px", border: "1px dashed var(--color-cream-400)", borderRadius: 12, textAlign: "center" };
