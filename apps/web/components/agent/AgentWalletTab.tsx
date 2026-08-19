"use client";

import { useEffect, useState } from "react";
import { agentClient } from "@/lib/rpc";
import { AgentWallet, WalletTransactionType } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";

const fmt = (n: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

const typeLabel: Record<number, { label: string; color: string }> = {
  [WalletTransactionType.CREDIT]: { label: "Komisi Diterima", color: "var(--color-emerald-700)" },
  [WalletTransactionType.DEBIT]: { label: "Dicairkan", color: "var(--color-danger-600)" },
  [WalletTransactionType.PENDING_REQUEST]: { label: "Menunggu Persetujuan", color: "var(--color-gold-700)" },
};

export default function AgentWalletTab() {
  const [wallet, setWallet] = useState<AgentWallet>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    agentClient.getMyWallet({}).then(setWallet).catch(() => setError(true)).finally(() => setLoading(false));
  }, []);

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat dompet...</p>;
  if (error || !wallet) return <p style={{ color: "var(--color-danger-600)" }}>Gagal memuat data.</p>;

  return (
    <div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(200px,1fr))", gap: 16, marginBottom: 28 }}>
        {[
          { label: "Total Komisi", value: fmt(wallet.totalEarnedIdr), color: "var(--color-warm-800)" },
          { label: "Tersedia", value: fmt(wallet.availableIdr), color: "var(--color-emerald-700)" },
          { label: "Menunggu Pencairan", value: fmt(wallet.pendingRequestedIdr), color: "var(--color-gold-700)" },
        ].map((c) => (
          <div key={c.label} style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "20px 18px" }}>
            <p style={{ margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 }}>{c.label}</p>
            <p style={{ margin: 0, fontSize: 22, fontWeight: 700, color: c.color }}>{c.value}</p>
          </div>
        ))}
      </div>

      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 }}>
        <h3 style={{ margin: "0 0 16px", fontSize: 15, fontWeight: 700 }}>Riwayat Transaksi</h3>
        {wallet.transactions.length === 0 && <p style={{ color: "var(--color-warm-400)", fontSize: 14 }}>Belum ada transaksi.</p>}
        <div style={{ display: "grid", gap: 10 }}>
          {wallet.transactions.map((tx) => {
            const meta = typeLabel[tx.type] ?? { label: "Transaksi", color: "var(--color-warm-600)" };
            const isDebit = tx.type === WalletTransactionType.DEBIT;
            return (
              <div key={tx.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0", borderBottom: "1px solid var(--color-cream-300)" }}>
                <div>
                  <p style={{ margin: 0, fontSize: 13, fontWeight: 600, color: meta.color }}>{meta.label}</p>
                  <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>{tx.description} · {tx.createdAt?.toDate().toLocaleDateString("id-ID")}</p>
                </div>
                <p style={{ margin: 0, fontWeight: 700, fontSize: 14, color: isDebit ? "var(--color-danger-600)" : "var(--color-emerald-700)" }}>
                  {isDebit ? "-" : "+"}{fmt(tx.amountIdr)}
                </p>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
