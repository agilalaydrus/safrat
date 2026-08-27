"use client";
import { useEffect, useState } from "react";
import { IconAlertTriangle, IconCheck, IconX } from "@tabler/icons-react";
import { HeldOrderResolution, Order } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { orderClient } from "@/lib/rpc";

const rupiah = (n: bigint) => `Rp${Number(n).toLocaleString("id-ID")}`;

type Props = { order: Order | null; onClose: () => void; onResolved: (message: string) => void };

export default function ResolveHeldOrderDialog({ order, onClose, onResolved }: Props) {
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    setNote("");
    setError("");
  }, [order]);

  if (!order) return null;

  const shortfall = order.totalPriceIdr - order.paidAmountIdr;

  const resolve = async (resolution: HeldOrderResolution) => {
    setSubmitting(true);
    setError("");
    try {
      await orderClient.resolveHeldOrder({ orderId: order.id, resolution, note: note.trim() });
      onResolved(resolution === HeldOrderResolution.ACCEPT
        ? "Transaksi diterima dan ditandai lunas."
        : "Transaksi ditolak. Komisi yang sudah terhitung ditarik kembali.");
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyelesaikan transaksi.");
      setSubmitting(false);
    }
  };

  return (
    <div style={backdrop} role="dialog" aria-modal="true" aria-label="Tinjau transaksi">
      <div style={panel}>
        <header style={head}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 500 }}>Tinjau Transaksi</h2>
          <button onClick={onClose} style={iconButton} aria-label="Tutup"><IconX size={18} /></button>
        </header>

        <p style={warning}>
          <IconAlertTriangle size={18} />
          <span>{order.heldReason || "Nominal pembayaran tidak cocok dengan tagihan."}</span>
        </p>

        <dl style={summary}>
          <div><dt style={dt}>Pembeli</dt><dd style={dd}>{order.buyerName || order.pilgrimName}</dd></div>
          <div><dt style={dt}>Produk</dt><dd style={dd}>{order.productName}</dd></div>
          <div><dt style={dt}>Tagihan</dt><dd style={dd}>{rupiah(order.totalPriceIdr)}</dd></div>
          <div><dt style={dt}>Dibayar</dt><dd style={dd}>{rupiah(order.paidAmountIdr)}</dd></div>
          <div>
            <dt style={dt}>{shortfall > 0n ? "Kurang" : "Lebih"}</dt>
            <dd style={{ ...dd, fontWeight: 700 }}>{rupiah(shortfall > 0n ? shortfall : -shortfall)}</dd>
          </div>
        </dl>

        <label style={label}>
          Catatan
          <input value={note} maxLength={500} placeholder="Kekurangan dibayar tunai di kantor"
            onChange={(e) => setNote(e.target.value)} style={input} />
        </label>

        {/* Named plainly, because there is no gateway confirmation behind
            either choice — the audit log is the only record of who decided. */}
        <p style={{ margin: 0, fontSize: 13, color: "var(--color-warm-500)" }}>
          <strong>Terima</strong> berarti Anda menyatakan selisihnya sudah diselesaikan di luar sistem —
          transaksi dianggap lunas penuh dan komisi agen tetap berlaku.
          <br />
          <strong>Tolak</strong> menutup transaksi dan menarik kembali komisi yang sudah terhitung.
          Pengembalian dananya dilakukan di luar sistem.
        </p>

        {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

        <footer style={foot}>
          <button onClick={() => resolve(HeldOrderResolution.REJECT)} style={danger} disabled={submitting}>
            <IconX size={17} />Tolak
          </button>
          <button onClick={() => resolve(HeldOrderResolution.ACCEPT)} style={accept} disabled={submitting}>
            <IconCheck size={17} />{submitting ? "Memproses..." : "Terima"}
          </button>
        </footer>
      </div>
    </div>
  );
}

const backdrop: React.CSSProperties = { position: "fixed", inset: 0, background: "rgba(20,24,20,.45)", display: "grid", placeItems: "center", padding: 16, zIndex: 50 };
const panel: React.CSSProperties = { width: "min(560px,100%)", maxHeight: "90vh", overflowY: "auto", background: "#fff", borderRadius: 14, padding: 24, display: "grid", gap: 14 };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center" };
const iconButton: React.CSSProperties = { border: 0, background: "transparent", cursor: "pointer", color: "var(--color-warm-500)" };
const warning: React.CSSProperties = { display: "flex", gap: 8, alignItems: "flex-start", margin: 0, padding: 12, background: "var(--color-gold-50)", borderRadius: 8, color: "#b45309", fontSize: 14 };
const summary: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(140px,1fr))", gap: 12, margin: 0, padding: 16, background: "var(--color-cream-100)", borderRadius: 10 };
const dt: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-400)", letterSpacing: ".06em" };
const dd: React.CSSProperties = { margin: "4px 0 0", color: "var(--color-warm-700)" };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px" };
const foot: React.CSSProperties = { display: "flex", justifyContent: "flex-end", gap: 10, marginTop: 4 };
const danger: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-danger-600)", borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "transparent", color: "var(--color-danger-600)", fontWeight: 700 };
const accept: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700 };
