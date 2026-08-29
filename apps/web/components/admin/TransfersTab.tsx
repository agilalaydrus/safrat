"use client";

import { useCallback, useEffect, useState } from "react";
import { IconCheck, IconAlertTriangle } from "@tabler/icons-react";
import { PendingTransfer } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));
const when = (d?: Date) => d?.toLocaleString("id-ID", { day: "2-digit", month: "short" }) ?? "—";

export default function TransfersTab() {
  const [transfers, setTransfers] = useState<PendingTransfer[]>([]);
  const [amount, setAmount] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    platformClient.listPendingTransfers({})
      .then((r) => setTransfers(r.transfers))
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

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <p style={muted}>
        Setiap tagihan transfer punya nominal yang unik sampai rupiah terakhir — itulah yang
        mengenali pembayarannya. Cocokkan angka di mutasi rekening dengan daftar di bawah, lalu
        konfirmasi. Nominal yang dibulatkan tidak akan cocok, dan memang tidak boleh cocok:
        salah mengkredit travel jauh lebih buruk daripada meminta seseorang membaca ulang angkanya.
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
                    <td style={td}>
                      <button style={ghost} disabled={busy} onClick={() => confirm(String(transfer.amountIdr))}>
                        Sudah masuk
                      </button>
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
