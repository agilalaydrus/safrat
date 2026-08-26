"use client";
import { useCallback, useEffect, useState } from "react";
import { IconArrowBackUp, IconX } from "@tabler/icons-react";
import { Order, OrderRefund } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { orderClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) => `Rp${Number(n).toLocaleString("id-ID")}`;

type Props = { order: Order | null; onClose: () => void; onRefunded: (message: string) => void };

export default function RefundOrderDialog({ order, onClose, onRefunded }: Props) {
  const [refunds, setRefunds] = useState<OrderRefund[]>([]);
  const [refundedTotal, setRefundedTotal] = useState(0n);
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  // Minted once per refund the operator is composing, not per click. A
  // double-click or a retry after a dropped response then carries the same
  // key, and the server settles the same refund instead of issuing a second.
  const [idempotencyKey, setIdempotencyKey] = useState("");

  const remaining = order ? order.totalPriceIdr - refundedTotal : 0n;

  const loadHistory = useCallback(async (orderId: string) => {
    try {
      const response = await orderClient.listOrderRefunds({ orderId });
      setRefunds(response.refunds);
      setRefundedTotal(response.totalRefundedIdr);
    } catch {
      setError("Gagal memuat riwayat refund.");
    }
  }, []);

  useEffect(() => {
    if (!order) return;
    setAmount("");
    setReason("");
    setError("");
    setRefunds([]);
    setRefundedTotal(0n);
    setIdempotencyKey(crypto.randomUUID());
    void loadHistory(order.id);
  }, [order, loadHistory]);

  if (!order) return null;

  const submit = async () => {
    const parsed = BigInt(amount.replace(/\D/g, "") || "0");
    if (parsed <= 0n) { setError("Masukkan nominal refund."); return; }
    if (parsed > remaining) { setError(`Nominal melebihi sisa yang dapat dikembalikan (${rupiah(remaining)}).`); return; }
    setSubmitting(true);
    setError("");
    try {
      const response = await orderClient.refundOrder({
        orderId: order.id, amountIdr: parsed, reason: reason.trim(), idempotencyKey,
      });
      onRefunded(response.created
        ? `Refund ${rupiah(parsed)} tercatat. Saldo jamaah kini ${rupiah(response.pilgrimBalanceIdr)}.`
        : "Refund ini sudah tercatat sebelumnya — tidak ada refund kedua yang dibuat.");
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Refund gagal diproses.");
      setSubmitting(false);
    }
  };

  return (
    <div style={backdrop} role="dialog" aria-modal="true" aria-label="Refund pesanan">
      <div style={panel}>
        <header style={head}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 500 }}>Refund Pesanan</h2>
          <button onClick={onClose} style={iconButton} aria-label="Tutup"><IconX size={18} /></button>
        </header>

        <dl style={summary}>
          <div><dt style={dt}>Jamaah</dt><dd style={dd}>{order.pilgrimName}</dd></div>
          <div><dt style={dt}>Produk</dt><dd style={dd}>{order.productName}</dd></div>
          <div><dt style={dt}>Total Dibayar</dt><dd style={dd}>{rupiah(order.totalPriceIdr)}</dd></div>
          <div><dt style={dt}>Sudah Direfund</dt><dd style={dd}>{rupiah(refundedTotal)}</dd></div>
          <div><dt style={dt}>Sisa</dt><dd style={{ ...dd, fontWeight: 700 }}>{rupiah(remaining)}</dd></div>
        </dl>

        {order.agentCommissionIdr > 0n && (
          <p style={note}>
            Komisi agen ditarik kembali secara proporsional — komisi hanya berlaku atas transaksi yang tetap sah.
          </p>
        )}

        <label style={label}>
          Nominal Refund
          <input
            inputMode="numeric" value={amount} placeholder={String(remaining)}
            onChange={(e) => setAmount(e.target.value)} style={input}
          />
        </label>
        <button type="button" style={linkButton} onClick={() => setAmount(String(remaining))}>
          Refund penuh ({rupiah(remaining)})
        </button>

        <label style={label}>
          Alasan
          <input value={reason} maxLength={500} placeholder="Pembatalan jamaah" onChange={(e) => setReason(e.target.value)} style={input} />
        </label>

        {refunds.length > 0 && (
          <section style={{ marginTop: 8 }}>
            <h3 style={{ fontSize: 12, color: "var(--color-warm-400)", margin: "0 0 8px", letterSpacing: ".06em" }}>RIWAYAT REFUND</h3>
            <ul style={history}>
              {refunds.map((refund) => (
                <li key={refund.id} style={historyItem}>
                  <span>{rupiah(refund.amountIdr)}</span>
                  <span style={{ color: "var(--color-warm-500)" }}>
                    {refund.createdAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}
                    {refund.reason ? ` — ${refund.reason}` : ""}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        )}

        {error && <p style={{ color: "var(--color-danger-600)", margin: "8px 0 0" }}>{error}</p>}

        <footer style={foot}>
          <button onClick={onClose} style={ghost} disabled={submitting}>Batal</button>
          <button onClick={submit} style={danger} disabled={submitting || remaining <= 0n}>
            <IconArrowBackUp size={18} />{submitting ? "Memproses..." : "Catat Refund"}
          </button>
        </footer>
      </div>
    </div>
  );
}

const backdrop: React.CSSProperties = { position: "fixed", inset: 0, background: "rgba(20,24,20,.45)", display: "grid", placeItems: "center", padding: 16, zIndex: 50 };
const panel: React.CSSProperties = { width: "min(560px,100%)", maxHeight: "90vh", overflowY: "auto", background: "#fff", borderRadius: 14, padding: 24, display: "grid", gap: 12 };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center" };
const iconButton: React.CSSProperties = { border: 0, background: "transparent", cursor: "pointer", color: "var(--color-warm-500)" };
const summary: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(150px,1fr))", gap: 12, margin: 0, padding: 16, background: "var(--color-cream-100)", borderRadius: 10 };
const dt: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-400)", letterSpacing: ".06em" };
const dd: React.CSSProperties = { margin: "4px 0 0", color: "var(--color-warm-700)" };
const note: React.CSSProperties = { margin: 0, fontSize: 13, color: "var(--color-warm-500)" };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px" };
const linkButton: React.CSSProperties = { justifySelf: "start", border: 0, background: "transparent", padding: 0, color: "var(--color-emerald-900)", fontSize: 12, textDecoration: "underline", cursor: "pointer" };
const history: React.CSSProperties = { listStyle: "none", margin: 0, padding: 0, display: "grid", gap: 6, fontSize: 13 };
const historyItem: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, paddingBottom: 6, borderBottom: "1px solid var(--color-cream-300)" };
const foot: React.CSSProperties = { display: "flex", justifyContent: "flex-end", gap: 10, marginTop: 8 };
const ghost: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 18px", background: "transparent", color: "var(--color-warm-700)" };
const danger: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-danger-600)", color: "#fff", fontWeight: 700 };
