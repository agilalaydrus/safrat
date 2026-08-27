"use client";
import { IconPrinter, IconX } from "@tabler/icons-react";
import { PilgrimTransaction } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_app_pb";

const money = (n: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));
const stamp = (d?: Date) => d?.toLocaleString("id-ID", { day: "2-digit", month: "long", year: "numeric", hour: "2-digit", minute: "2-digit" }) ?? "—";

const STATUS_LABEL: Record<string, string> = {
  PENDING: "Menunggu Pembayaran", PAID: "LUNAS", REFUNDED: "DANA DIKEMBALIKAN",
  HELD: "Sedang Diperiksa", EXPIRED: "Kedaluwarsa", FAILED: "Gagal", CANCELLED: "Dibatalkan",
};

/**
 * A receipt the person who paid can read and print for themselves.
 *
 * Printing is the browser's own dialog, which is also how it becomes a PDF —
 * "save as PDF" is a destination in that dialog on every desktop platform, and
 * on iOS and Android the share sheet does the same. Generating a PDF
 * server-side would mean a rendering dependency and a second layout to keep in
 * step with this one, for a file the browser already produces from the markup
 * on screen.
 */
export default function TransactionReceipt({ transaction, onClose }: { transaction: PilgrimTransaction | null; onClose: () => void }) {
  if (!transaction) return null;

  const refunded = transaction.refundedIdr > 0n;

  return (
    <div style={backdrop} role="dialog" aria-modal="true" aria-label="Struk transaksi">
      {/* Only this subtree survives printing — see the rules below. Everything
          else on the page, including the app's navigation, is hidden. */}
      <div style={sheet} className="receipt-sheet">
        <header style={head} className="receipt-hide">
          <strong style={{ fontSize: 15 }}>Struk Transaksi</strong>
          <button onClick={onClose} style={iconButton} aria-label="Tutup"><IconX size={18} /></button>
        </header>

        <div style={body}>
          <div style={{ textAlign: "center", display: "grid", gap: 4 }}>
            <strong style={{ fontSize: 17 }}>{transaction.operatorName || "Tawafiq Hub"}</strong>
            <span style={{ fontSize: 12, color: "var(--color-warm-500)" }}>Bukti Transaksi</span>
          </div>

          <div style={divider} />

          <dl style={rows}>
            <Row label="No. Struk" value={transaction.receiptNumber || "—"} strong />
            <Row label="Tanggal" value={stamp(transaction.createdAt?.toDate())} />
            <Row label="Produk" value={transaction.productName} />
            {transaction.quantity > 1 && <Row label="Jumlah" value={`${transaction.quantity}`} />}
            <Row label="Status" value={STATUS_LABEL[transaction.status] ?? transaction.status} strong />
            {transaction.paidAt && <Row label="Dibayar" value={stamp(transaction.paidAt.toDate())} />}
          </dl>

          <div style={divider} />

          <dl style={rows}>
            <Row label="Nilai Transaksi" value={money(transaction.amountIdr)} strong />
            {refunded && (
              <>
                <Row label="Dikembalikan" value={`− ${money(transaction.refundedIdr)}`} />
                <Row label="Tanggal Refund" value={stamp(transaction.refundedAt?.toDate())} />
                {transaction.refundReason && <Row label="Alasan" value={transaction.refundReason} />}
              </>
            )}
          </dl>

          <div style={divider} />

          {/* The number that matters after a refund is what the operator still
              holds, not what was originally charged. */}
          <dl style={rows}>
            <Row
              label="Total Dibayar"
              value={money(refunded ? transaction.amountIdr - transaction.refundedIdr : (transaction.status === "PAID" ? transaction.amountIdr : 0n))}
              strong
            />
          </dl>

          <p style={footNote}>
            Struk ini dihasilkan otomatis oleh sistem dan sah tanpa tanda tangan.
            Simpan nomor struk di atas jika Anda perlu menghubungi petugas.
          </p>
        </div>

        <footer style={foot} className="receipt-hide">
          <button onClick={onClose} style={ghost}>Tutup</button>
          <button onClick={() => window.print()} style={primary}>
            <IconPrinter size={17} />Cetak / Simpan PDF
          </button>
        </footer>
      </div>

      <style jsx global>{`
        @media print {
          /* Hide the whole page, then bring back just the receipt. Printing a
             dialog otherwise carries the app's navigation onto the paper. */
          body * { visibility: hidden; }
          .receipt-sheet, .receipt-sheet * { visibility: visible; }
          .receipt-sheet {
            position: absolute; left: 0; top: 0; width: 100%;
            max-height: none; box-shadow: none; border-radius: 0;
          }
          .receipt-hide { display: none !important; }
        }
      `}</style>
    </div>
  );
}

function Row({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <div style={row}>
      <dt style={{ color: "var(--color-warm-500)", fontSize: 13 }}>{label}</dt>
      <dd style={{ margin: 0, fontSize: 13, fontWeight: strong ? 700 : 400, textAlign: "right" }}>{value}</dd>
    </div>
  );
}

const backdrop: React.CSSProperties = { position: "fixed", inset: 0, background: "rgba(20,24,20,.45)", display: "grid", placeItems: "center", padding: 16, zIndex: 60 };
const sheet: React.CSSProperties = { width: "min(420px,100%)", maxHeight: "92vh", overflowY: "auto", background: "#fff", borderRadius: 12 };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "14px 18px", borderBottom: "1px solid var(--color-cream-300)" };
const body: React.CSSProperties = { display: "grid", gap: 14, padding: 22 };
const divider: React.CSSProperties = { borderTop: "1px dashed var(--color-cream-400)" };
const rows: React.CSSProperties = { display: "grid", gap: 8, margin: 0 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 16, alignItems: "baseline" };
const footNote: React.CSSProperties = { margin: 0, fontSize: 11, color: "var(--color-warm-400)", textAlign: "center", lineHeight: 1.5 };
const foot: React.CSSProperties = { display: "flex", justifyContent: "flex-end", gap: 10, padding: "14px 18px", borderTop: "1px solid var(--color-cream-300)" };
const iconButton: React.CSSProperties = { border: 0, background: "transparent", cursor: "pointer", color: "var(--color-warm-500)" };
const ghost: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 16px", background: "transparent", color: "var(--color-warm-700)" };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 16px", display: "inline-flex", alignItems: "center", gap: 7, background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700 };
