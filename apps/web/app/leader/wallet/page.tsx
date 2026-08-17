"use client";

import { useEffect, useState } from "react";
import { IconArrowDownRight, IconArrowUpRight, IconClockHour4 } from "@tabler/icons-react";
import { AgentWallet, WalletTransaction, WalletTransactionType } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { agentClient } from "@/lib/rpc";

const rupiah = (n: number) => `Rp${n.toLocaleString("id-ID")}`;
const parseDigits = (s: string) => Number(s.replace(/[^0-9]/g, "")) || 0;

export default function LeaderWalletPage() {
  const [wallet, setWallet] = useState<AgentWallet>();
  const [loading, setLoading] = useState(true);
  const [requesting, setRequesting] = useState(false);
  const [amountText, setAmountText] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = () => agentClient.getMyWallet({}).then((w) => { setWallet(w); setLoading(false); }).catch(() => setLoading(false));

  useEffect(() => { void load(); }, []);

  const available = Number(wallet?.availableIdr ?? 0);
  const amount = parseDigits(amountText);

  async function submitRequest() {
    setError("");
    if (amount <= 0) { setError("Jumlah harus lebih dari Rp0."); return; }
    if (amount > available) { setError(`Jumlah melebihi saldo tersedia (${rupiah(available)}).`); return; }
    setRequesting(true);
    try {
      await agentClient.requestAgentPayout({ amountIdr: BigInt(amount), note: note.trim() });
      setAmountText("");
      setNote("");
      setNotice("Permintaan pencairan terkirim — menunggu persetujuan operator.");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengajukan pencairan.");
    } finally {
      setRequesting(false);
    }
  }

  if (loading) return <main style={page}><p style={{ color: "var(--color-warm-500)" }}>Memuat dompet...</p></main>;
  if (!wallet) return <main style={page}><p style={{ color: "var(--color-warm-500)" }}>Dompet tidak ditemukan.</p></main>;

  return (
    <main style={page}>
      <p style={eyebrow}>DOMPET KOMISI</p>

      <div style={balanceCard}>
        <span style={{ fontSize: 13, opacity: .85 }}>Saldo tersedia</span>
        <strong style={balanceNumber}>{rupiah(available)}</strong>
        {wallet.pendingRequestedIdr > 0n && <span style={{ fontSize: 12, opacity: .85 }}>{rupiah(Number(wallet.pendingRequestedIdr))} sedang diajukan, belum termasuk di atas</span>}
        <div style={statRow}>
          <div><span style={statLabel}>Total diperoleh</span><b style={statValue}>{rupiah(Number(wallet.totalEarnedIdr))}</b></div>
          <div><span style={statLabel}>Total dicairkan</span><b style={statValue}>{rupiah(Number(wallet.totalWithdrawnIdr))}</b></div>
        </div>
      </div>

      <section style={requestCard}>
        <h2 style={{ margin: "0 0 12px", fontSize: 16 }}>Ajukan Pencairan</h2>
        <label style={{ display: "grid", gap: 6 }}>
          <span style={lab}>Jumlah</span>
          <div style={amountWrap}>
            <span style={amountPrefix}>Rp</span>
            <input inputMode="numeric" value={amountText ? Number(amountText).toLocaleString("id-ID") : ""} onChange={(e) => setAmountText(String(parseDigits(e.target.value)))} placeholder="0" style={{ ...input, paddingInlineStart: 40 }} />
          </div>
          <button type="button" onClick={() => setAmountText(String(available))} style={chip} disabled={available <= 0}>Ajukan semua ({rupiah(available)})</button>
        </label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
          <span style={lab}>Catatan (opsional)</span>
          <input value={note} onChange={(e) => setNote(e.target.value)} placeholder="mis. No. rekening tujuan" style={input} />
        </label>
        {error && <p style={errBox}>{error}</p>}
        {notice && <p style={successBox}>{notice}</p>}
        <button onClick={() => void submitRequest()} disabled={requesting || available <= 0} style={primary}>
          {requesting ? "Mengirim..." : available <= 0 ? "Tidak ada saldo tersedia" : `Ajukan Pencairan ${amount > 0 ? rupiah(amount) : ""}`}
        </button>
      </section>

      <section>
        <h2 style={{ fontSize: 13, fontWeight: 700, letterSpacing: ".08em", color: "var(--color-warm-400)", margin: "24px 0 10px" }}>RIWAYAT TRANSAKSI</h2>
        {!wallet.transactions.length ? (
          <p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>Belum ada transaksi.</p>
        ) : (
          <div style={{ display: "grid", gap: 8 }}>
            {wallet.transactions.map((tx) => <TransactionRow key={tx.id} tx={tx} />)}
          </div>
        )}
      </section>
    </main>
  );
}

function TransactionRow({ tx }: { tx: WalletTransaction }) {
  const isCredit = tx.type === WalletTransactionType.CREDIT;
  const isPending = tx.type === WalletTransactionType.PENDING_REQUEST;
  const Icon = isPending ? IconClockHour4 : isCredit ? IconArrowDownRight : IconArrowUpRight;
  const color = isPending ? "var(--color-gold-800)" : isCredit ? "var(--color-emerald-900)" : "var(--color-danger-600)";
  const sign = isPending ? "" : isCredit ? "+" : "-";
  return (
    <div style={txRow}>
      <span style={{ ...txIcon, color, background: isPending ? "var(--color-gold-50)" : isCredit ? "var(--color-emerald-50)" : "#fdf0f0" }}><Icon size={18} /></span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <p style={{ margin: 0, fontWeight: 600, fontSize: 14 }}>{tx.description || (isCredit ? "Komisi order" : "Pencairan")}</p>
        <p style={{ margin: 0, color: "var(--color-warm-400)", fontSize: 12 }}>{tx.createdAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}</p>
      </div>
      <b style={{ color, fontSize: 14, whiteSpace: "nowrap" }}>{sign}{rupiah(Number(tx.amountIdr))}</b>
    </div>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 12px" };
const balanceCard: React.CSSProperties = { display: "grid", gap: 4, padding: 22, borderRadius: 16, background: "linear-gradient(135deg,var(--color-emerald-900),var(--color-emerald-800))", color: "#fff" };
const balanceNumber: React.CSSProperties = { fontSize: 32, fontWeight: 700, margin: "2px 0 4px" };
const statRow: React.CSSProperties = { display: "flex", gap: 20, marginTop: 14, paddingTop: 14, borderTop: "1px solid rgba(255,255,255,.2)" };
const statLabel: React.CSSProperties = { display: "block", fontSize: 11, opacity: .8 };
const statValue: React.CSSProperties = { fontSize: 14 };
const requestCard: React.CSSProperties = { marginTop: 16, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12 };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 46, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const amountWrap: React.CSSProperties = { position: "relative" };
const amountPrefix: React.CSSProperties = { position: "absolute", insetInlineStart: 14, top: 0, height: 46, display: "flex", alignItems: "center", color: "var(--color-warm-500)", fontWeight: 600, pointerEvents: "none" };
const chip: React.CSSProperties = { justifySelf: "start", minHeight: 30, border: "1px solid var(--color-cream-400)", borderRadius: 999, padding: "0 12px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600, marginTop: 4 };
const primary: React.CSSProperties = { width: "100%", minHeight: 48, marginTop: 16, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700 };
const errBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "#fdf0f0", color: "var(--color-danger-600)", fontSize: 13 };
const successBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 13 };
const txRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, padding: "10px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10 };
const txIcon: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", display: "grid", placeItems: "center", flexShrink: 0 };
