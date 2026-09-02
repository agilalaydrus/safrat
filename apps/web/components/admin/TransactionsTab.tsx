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

export default function TransactionsTab({ onOpenSupplierLog }: { onOpenSupplierLog: (orderId: string) => void }) {
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
  const [reviewing, setReviewing] = useState<PlatformTransaction | null>(null);
  const [notice, setNotice] = useState("");

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
      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}

      {reviewing && (
        <ReviewDialog
          transaction={reviewing}
          onClose={() => setReviewing(null)}
          onDone={(message) => { setReviewing(null); setNotice(message); load(); }}
        />
      )}

      {loading ? <p style={muted}>Memuat...</p> : transactions.length === 0 ? (
        <p style={muted}>
          {needsAttention ? "Tidak ada transaksi yang perlu ditindak." : "Belum ada transaksi."}
        </p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Struk", "Travel / Jamaah", "Produk", "Nilai", "Pembayaran", "Pengiriman", "Dibuat", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
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
                    <td style={td}>
                      {/* The queue could be read and not worked: a delivery
                          nothing could determine sat here permanently with the
                          money already taken. Only shown where a decision is
                          actually open. */}
                      {(transaction.fulfilmentStatus === "NEEDS_REVIEW" || transaction.fulfilmentStatus === "FAILED") && (
                        <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                          <button style={ghost} onClick={() => setReviewing(transaction)}>Tinjau</button>
                          {transaction.supplierName && (
                            <button style={ghost} onClick={() => onOpenSupplierLog(transaction.orderId)}>Log supplier</button>
                          )}
                        </div>
                      )}
                    </td>
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

// Deciding an unreadable delivery. Two outcomes and nothing in between, because
// "probably fine" is the state being resolved rather than an answer to it.
function ReviewDialog({ transaction, onClose, onDone }: {
  transaction: PlatformTransaction;
  onClose: () => void;
  onDone: (message: string) => void;
}) {
  const [status, setStatus] = useState<"DELIVERED" | "FAILED">("FAILED");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      const result = await platformClient.resolveFulfilment({
        orderId: transaction.orderId, status, note: note.trim(),
      });
      onDone(result.refunded
        ? `Ditandai gagal dan dana ${money(result.refundedIdr)} dikembalikan.`
        : "Ditandai sudah sampai.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan keputusan.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div role="dialog" aria-label="Tinjau pengiriman" style={dialog}>
      <strong>Tinjau pengiriman — {transaction.receiptNumber || transaction.orderId.slice(0, 8)}</strong>
      <p style={{ ...muted, margin: 0 }}>
        {transaction.productName} · {money(transaction.amountIdr)}
        {transaction.fulfilmentError ? ` · ${transaction.fulfilmentError}` : ""}
      </p>

      <label style={label}>
        Keputusan
        <select value={status} onChange={(e) => setStatus(e.target.value as "DELIVERED" | "FAILED")} style={input}>
          <option value="FAILED">Gagal — dana dikembalikan ke jamaah</option>
          <option value="DELIVERED">Sudah sampai — tidak ada pengembalian</option>
        </select>
      </label>

      <label style={label}>
        Alasan
        <textarea
          value={note}
          onChange={(e) => setNote(e.target.value)}
          style={{ ...input, minHeight: 72, padding: 10 }}
          placeholder="Mis. dicek ke supplier, transaksi tidak ditemukan"
        />
      </label>
      <small style={{ color: "var(--color-warm-500)", fontSize: 12 }}>
        Wajib diisi. Keputusan ini tidak dikonfirmasi oleh apa pun di luar sistem, jadi alasannya
        adalah satu-satunya jejak pertanggungjawabannya.
      </small>

      {error && <p style={{ color: "var(--color-danger-600)", margin: 0, fontSize: 13 }}>{error}</p>}

      <div style={{ display: "flex", gap: 8 }}>
        <button style={primary} disabled={busy || !note.trim()} onClick={submit}>
          {busy ? "Menyimpan…" : status === "FAILED" ? "Tandai gagal & kembalikan dana" : "Tandai sudah sampai"}
        </button>
        <button style={ghost} onClick={onClose} disabled={busy}>Batal</button>
      </div>
    </div>
  );
}

const dialog: React.CSSProperties = { display: "grid", gap: 12, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderLeft: "3px solid var(--color-danger-600)", borderRadius: 10, maxWidth: 560 };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontFamily: "inherit", fontSize: 14 };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, justifySelf: "start" };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", fontSize: 13 };
