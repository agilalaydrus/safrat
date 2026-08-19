"use client";

import { useEffect, useState } from "react";
import { agentClient } from "@/lib/rpc";
import { AgentWallet, WalletTransaction, WalletTransactionType } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";

const fmt = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

export default function AgentPayoutTab() {
  const [wallet, setWallet] = useState<AgentWallet>();
  const [requests, setRequests] = useState<WalletTransaction[]>([]);
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(true);

  const refresh = () => {
    agentClient.getMyWallet({}).then((w) => {
      setWallet(w);
      setRequests(w.transactions.filter((t) => t.type === WalletTransactionType.PENDING_REQUEST));
    }).finally(() => setLoading(false));
  };

  useEffect(refresh, []);

  const submit = async () => {
    const amountIDR = parseInt(amount.replace(/\D/g, ""), 10);
    if (!amountIDR || amountIDR <= 0) { setNotice("Masukkan jumlah yang valid."); return; }
    if (!note.trim()) { setNotice("Isi keterangan rekening tujuan transfer."); return; }
    setSubmitting(true);
    setNotice("");
    try {
      await agentClient.requestAgentPayout({ amountIdr: BigInt(amountIDR), note: note.trim() });
      setNotice("Permintaan pencairan berhasil dikirim. Operator akan memproses dalam 1-3 hari kerja.");
      setAmount("");
      setNote("");
      refresh();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "Gagal mengirim permintaan.");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p>;

  const inp: React.CSSProperties = { display: "block", width: "100%", marginTop: 6, padding: "10px 12px", fontSize: 14, borderRadius: 8, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-500)", fontFamily: "'Plus Jakarta Sans',sans-serif", outline: "none", boxSizing: "border-box" };
  const lbl: React.CSSProperties = { display: "block", fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };

  return (
    <div style={{ display: "grid", gap: 20, maxWidth: 600 }}>
      <div style={{ background: "var(--color-emerald-900)", borderRadius: 12, padding: "20px 24px", color: "#fff" }}>
        <p style={{ margin: "0 0 4px", fontSize: 12, opacity: 0.7 }}>Dana Tersedia untuk Dicairkan</p>
        <p style={{ margin: 0, fontSize: 28, fontWeight: 700 }}>{wallet ? fmt(wallet.availableIdr) : "-"}</p>
      </div>

      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 }}>
        <h3 style={{ margin: "0 0 16px", fontSize: 15, fontWeight: 700 }}>Ajukan Pencairan Dana</h3>
        <div style={{ display: "grid", gap: 14 }}>
          <label style={lbl}>
            Jumlah (Rp)
            <input value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="Contoh: 500000" style={inp} />
          </label>
          <label style={lbl}>
            Keterangan (nomor rekening / e-wallet)
            <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} placeholder="BCA 1234567890 a.n. Nama Anda" style={{ ...inp, resize: "vertical" }} />
          </label>
          {notice && <p style={{ fontSize: 13, color: notice.includes("berhasil") ? "var(--color-emerald-700)" : "var(--color-danger-600)" }}>{notice}</p>}
          <button onClick={submit} disabled={submitting} style={{ height: 44, background: "var(--color-gold-500)", color: "var(--color-warm-900)", border: "none", borderRadius: 8, fontWeight: 700, fontSize: 14, cursor: "pointer" }}>
            {submitting ? "Mengirim..." : "Ajukan Pencairan"}
          </button>
        </div>
      </div>

      {requests.length > 0 && (
        <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 }}>
          <h3 style={{ margin: "0 0 12px", fontSize: 15, fontWeight: 700 }}>Permintaan Menunggu</h3>
          {requests.map((r) => (
            <div key={r.id} style={{ display: "flex", justifyContent: "space-between", padding: "10px 0", borderBottom: "1px solid var(--color-cream-300)", fontSize: 13 }}>
              <span style={{ color: "var(--color-warm-500)" }}>{r.createdAt?.toDate().toLocaleDateString("id-ID")}</span>
              <span style={{ fontWeight: 700, color: "var(--color-gold-700)" }}>{fmt(r.amountIdr)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
