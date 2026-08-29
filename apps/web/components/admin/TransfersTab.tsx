"use client";

import { useCallback, useEffect, useState } from "react";
import { IconCheck, IconAlertTriangle } from "@tabler/icons-react";
import { BankMutation, PendingTransfer } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));
const when = (d?: Date) => d?.toLocaleString("id-ID", { day: "2-digit", month: "short" }) ?? "—";

export default function TransfersTab() {
  const [transfers, setTransfers] = useState<PendingTransfer[]>([]);
  const [mutations, setMutations] = useState<BankMutation[]>([]);
  // Which invoice a credit is being attached to. Settlement happens on the
  // invoice, not on a global amount box — an admin is answering "did this
  // ticket get paid", and the credit is the evidence.
  const [attaching, setAttaching] = useState<{ invoiceId: string; note: string } | null>(null);
  const [amount, setAmount] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    Promise.all([
      platformClient.listPendingTransfers({}),
      platformClient.listBankMutations({ unmatchedOnly: true }),
    ])
      .then(([pending, credits]) => {
        setTransfers(pending.transfers);
        setMutations(credits.mutations);
      })
      .catch(() => setError("Gagal memuat daftar transfer."))
      .finally(() => setLoading(false));
  }, []);
  useEffect(() => { load(); }, [load]);

  const confirm = async (value: string) => {
    const digits = value.replace(/\D/g, "");
    if (!digits) { setError("Masukkan nominal transfer."); return; }
    setBusy(true);
    setError("");
    try {
      const result = await platformClient.confirmBankTransfer({ amountIdr: BigInt(digits) });
      setNotice(`${rupiah(result.amountIdr)} dari ${result.operatorName || "travel"} dikonfirmasi. Langganan diperpanjang.`);
      setAmount("");
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengonfirmasi transfer.");
    } finally {
      setBusy(false);
    }
  };

  const attach = async (mutationId: string) => {
    if (!attaching) return;
    setBusy(true);
    setError("");
    try {
      await platformClient.settleInvoiceWithMutation({
        mutationId, invoiceId: attaching.invoiceId, note: attaching.note.trim(),
      });
      setNotice("Kredit dilampirkan dan tagihan dilunasi.");
      setAttaching(null);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal melampirkan kredit.");
    } finally {
      setBusy(false);
    }
  };

  const ignore = async (mutationId: string) => {
    const note = window.prompt("Kenapa kredit ini bukan pembayaran langganan?");
    if (!note?.trim()) return;
    setBusy(true);
    setError("");
    try {
      await platformClient.ignoreBankMutation({ mutationId, note: note.trim() });
      setNotice("Kredit ditandai bukan pembayaran langganan.");
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menandai kredit.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <p style={muted}>
        Mutasi masuk dicocokkan otomatis dari feed bank: nominal setiap tagihan unik sampai
        rupiah terakhir, jadi satu kredit hanya bisa melunasi satu tagihan. Yang tampil di sini
        adalah sisanya — kredit yang belum ada tagihannya, dan tagihan yang belum ada kreditnya.
        Nominal yang dibulatkan memang tidak akan cocok: salah mengkredit travel jauh lebih buruk
        daripada meminta seseorang membaca ulang angkanya.
      </p>

      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "end" }}>
        <label style={label}>
          Nominal masuk (Rp)
          <input
            inputMode="numeric"
            value={amount}
            onChange={(e) => setAmount(e.target.value.replace(/\D/g, ""))}
            style={{ ...input, width: 200, textAlign: "right" }}
          />
        </label>
        <button style={primary} disabled={busy || !amount} onClick={() => confirm(amount)}>
          <IconCheck size={16} />{busy ? "Memeriksa…" : "Konfirmasi transfer"}
        </button>
      </div>

      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {mutations.length > 0 && (
        <div style={{ display: "grid", gap: 8 }}>
          <strong style={{ fontSize: 14 }}>
            Kredit masuk yang belum ada tagihannya ({mutations.length})
          </strong>
          <p style={{ ...muted, fontSize: 13 }}>
            Uang yang sudah masuk rekening dan belum diakui tagihan mana pun. Lampirkan ke tagihan
            yang benar, atau tandai bukan pembayaran langganan — jangan dibiarkan.
          </p>
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead><tr>{["Waktu", "Nominal", "Sumber", "Keterangan", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
              <tbody>
                {mutations.map((mutation) => (
                  <tr key={mutation.id} style={tr}>
                    <td style={td}>{when(mutation.occurredAt?.toDate())}</td>
                    <td style={{ ...td, fontWeight: 700, whiteSpace: "nowrap" }}>{rupiah(mutation.amountIdr)}</td>
                    <td style={td}>{mutation.source}</td>
                    <td style={{ ...td, maxWidth: 260, whiteSpace: "normal" }}>{mutation.description || "—"}</td>
                    <td style={td}>
                      {attaching ? (
                        <button
                          style={ghost}
                          disabled={busy || !attaching.note.trim()}
                          onClick={() => attach(mutation.id)}
                        >
                          Lampirkan ke tagihan terpilih
                        </button>
                      ) : (
                        <button style={ghost} disabled={busy} onClick={() => ignore(mutation.id)}>
                          Bukan pembayaran
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {attaching && (
        <div style={{ display: "grid", gap: 8, padding: 14, background: "#fffbeb", border: "1px solid #fde68a", borderRadius: 8 }}>
          <strong style={{ fontSize: 14 }}>Melampirkan kredit ke tagihan terpilih</strong>
          <label style={label}>
            Alasan
            <input
              value={attaching.note}
              onChange={(e) => setAttaching({ ...attaching, note: e.target.value })}
              style={input}
              placeholder="Mis. transfer masuk terpotong biaya admin bank"
            />
          </label>
          <small style={{ color: "var(--color-warm-500)", fontSize: 12 }}>
            Wajib diisi. Pencocokan manual tidak dikonfirmasi oleh apa pun di luar sistem.
          </small>
          <button style={ghost} onClick={() => setAttaching(null)}>Batal</button>
        </div>
      )}

      {loading ? <p style={muted}>Memuat…</p> : transfers.length === 0 ? (
        <p style={muted}>Tidak ada tagihan transfer yang menunggu.</p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Travel", "Paket", "Nominal yang ditunggu", "Terbit", "Kedaluwarsa", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {transfers.map((transfer) => {
                const expired = transfer.expiresAt ? transfer.expiresAt.toDate() < new Date() : false;
                return (
                  <tr key={transfer.invoiceId} style={tr}>
                    <td style={td}>{transfer.operatorName}</td>
                    <td style={td}>{transfer.plan}</td>
                    <td style={{ ...td, fontWeight: 700, whiteSpace: "nowrap" }}>{rupiah(transfer.amountIdr)}</td>
                    <td style={td}>{when(transfer.issuedAt?.toDate())}</td>
                    <td style={{ ...td, color: expired ? "#b45309" : undefined }}>
                      {expired && <IconAlertTriangle size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} />}
                      {when(transfer.expiresAt?.toDate())}
                    </td>
                    <td style={{ ...td, display: "flex", gap: 6, flexWrap: "wrap" }}>
                      <button style={ghost} disabled={busy} onClick={() => confirm(String(transfer.amountIdr))}>
                        Sudah masuk
                      </button>
                      {mutations.length > 0 && (
                        <button
                          style={ghost}
                          disabled={busy}
                          onClick={() => setAttaching({ invoiceId: transfer.invoiceId, note: "" })}
                        >
                          Lampirkan kredit
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0, maxWidth: 720 };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontFamily: "inherit", fontSize: 14 };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7 };
const ghost: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", fontSize: 13 };
