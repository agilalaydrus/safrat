"use client";

import { useEffect, useState } from "react";
import { IconX } from "@tabler/icons-react";
import { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { ManualOrderPaymentMethod } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { productClient, orderClient } from "@/lib/rpc";
import { checkoutErrorMessage } from "@/lib/checkout-error";

type Props = { open: boolean; pilgrimId: string; pilgrimName: string; seasonId: string; onClose: () => void; onSold: () => void };

const METHODS: [ManualOrderPaymentMethod, string][] = [
  [ManualOrderPaymentMethod.XENDIT_LINK, "Kirim Tautan Pembayaran (Xendit)"],
  [ManualOrderPaymentMethod.CASH, "Tunai (sudah diterima)"],
  [ManualOrderPaymentMethod.BANK_TRANSFER, "Transfer Bank (sudah diterima)"],
];

export default function SellPackageDialog({ open, pilgrimId, pilgrimName, seasonId, onClose, onSold }: Props) {
  const [products, setProducts] = useState<Product[]>([]);
  const [productId, setProductId] = useState("");
  const [method, setMethod] = useState<ManualOrderPaymentMethod>(ManualOrderPaymentMethod.XENDIT_LINK);
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [checkoutUrl, setCheckoutUrl] = useState("");
  const [idempotencyKey, setIdempotencyKey] = useState(() => crypto.randomUUID());

  useEffect(() => {
    if (!open || !seasonId) return;
    setProductId(""); setNote(""); setError(""); setCheckoutUrl(""); setMethod(ManualOrderPaymentMethod.XENDIT_LINK); setIdempotencyKey(crypto.randomUUID());
    productClient.listProducts({ seasonId }).then((r) => setProducts(r.products.filter((p) => p.category === "TRAVEL_PACKAGE" && p.isActive))).catch(() => setProducts([]));
  }, [open, seasonId]);

  if (!open) return null;
  const selected = products.find((p) => p.id === productId);

  async function submit() {
    if (!productId) { setError("Pilih paket terlebih dahulu."); return; }
    setSaving(true);
    setError("");
    try {
      const result = await orderClient.createManualOrder({ pilgrimId, productId, quantity: 1, paymentMethod: method, note, idempotencyKey });
      if (result.checkoutUrl) {
        setCheckoutUrl(result.checkoutUrl);
      } else {
        onSold();
        onClose();
      }
    } catch (caught) {
      setError(checkoutErrorMessage(caught, caught instanceof Error ? caught.message : "Gagal membuat pesanan."));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div role="dialog" aria-modal="true" style={overlay}>
      <aside style={sheet}>
        <div style={head}>
          <div><p style={eyebrow}>JUAL PAKET</p><h2 style={{ margin: 0, fontSize: 18 }}>{pilgrimName}</h2></div>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <div style={body}>
          {checkoutUrl ? (
            <div style={{ display: "grid", gap: 12 }}>
              <p style={{ margin: 0, fontSize: 13, color: "var(--color-warm-600)" }}>Tautan pembayaran berhasil dibuat. Bagikan ke jamaah:</p>
              <div style={{ background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 12px", fontSize: 12, wordBreak: "break-all" }}>{checkoutUrl}</div>
              <button onClick={() => { navigator.clipboard.writeText(checkoutUrl).catch(() => {}); }} style={ghostBtn}>Salin Tautan</button>
              <button onClick={() => { onSold(); onClose(); }} style={primary}>Selesai</button>
            </div>
          ) : (
            <div style={{ display: "grid", gap: 14 }}>
              <label style={{ display: "grid", gap: 6 }}>
                <span style={labelStyle}>Pilih Paket</span>
                <select value={productId} onChange={(e) => setProductId(e.target.value)} style={input}>
                  <option value="">— pilih paket —</option>
                  {products.map((p) => <option key={p.id} value={p.id}>{p.name} — Rp{Number(p.priceIdr).toLocaleString("id-ID")}</option>)}
                </select>
                {!products.length && <small style={{ color: "var(--color-warm-400)" }}>Belum ada Paket Perjalanan aktif untuk musim ini.</small>}
              </label>
              {selected && selected.itineraryDays.length > 0 && (
                <div style={{ fontSize: 12, color: "var(--color-warm-500)" }}>{selected.itineraryDays.length} hari itinerary{selected.hotelIds.length > 0 && ` · ${selected.hotelIds.length} hotel`}</div>
              )}
              <label style={{ display: "grid", gap: 6 }}>
                <span style={labelStyle}>Metode Pembayaran</span>
                <select value={method} onChange={(e) => setMethod(Number(e.target.value) as ManualOrderPaymentMethod)} style={input}>
                  {METHODS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </label>
              <label style={{ display: "grid", gap: 6 }}>
                <span style={labelStyle}>Catatan (opsional)</span>
                <input value={note} onChange={(e) => setNote(e.target.value)} style={input} maxLength={500} />
              </label>
              {error && <p style={{ margin: 0, color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
              <button onClick={() => void submit()} disabled={saving || !productId} style={primary}>{saving ? "Memproses..." : "Buat Pesanan"}</button>
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)" };
const sheet: React.CSSProperties = { width: "min(440px,100%)", height: "100vh", background: "#fff", display: "flex", flexDirection: "column" };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const body: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24 };
const closeBtn: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const eyebrow: React.CSSProperties = { margin: "0 0 4px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 12px", font: "inherit", background: "#fff" };
const primary: React.CSSProperties = { minHeight: 46, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const ghostBtn: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-600)", fontWeight: 600 };
