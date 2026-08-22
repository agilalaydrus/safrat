"use client";
import { FormEvent, useEffect, useMemo, useState } from "react";
import { IconBuildingBank, IconCash, IconCopy, IconLink, IconX } from "@tabler/icons-react";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { ManualOrderPaymentMethod } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import { pilgrimClient, productClient, orderClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) => `Rp${Number(n).toLocaleString("id-ID")}`;

const METHODS: { value: ManualOrderPaymentMethod; label: string; hint: string; icon: typeof IconLink }[] = [
  { value: ManualOrderPaymentMethod.XENDIT_LINK, label: "Kirim Link Xendit", hint: "Jamaah bayar sendiri lewat link", icon: IconLink },
  { value: ManualOrderPaymentMethod.CASH, label: "Tunai", hint: "Sudah diterima tunai", icon: IconCash },
  { value: ManualOrderPaymentMethod.BANK_TRANSFER, label: "Transfer Bank", hint: "Sudah diterima transfer", icon: IconBuildingBank },
];

type Props = { open: boolean; seasonId: string; onClose: () => void; onCreated: () => void };

export default function CreateOrderDialog({ open, seasonId, onClose, onCreated }: Props) {
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [pilgrimSearch, setPilgrimSearch] = useState("");
  const [pilgrimId, setPilgrimId] = useState("");
  const [productId, setProductId] = useState("");
  const [quantity, setQuantity] = useState(1);
  const [method, setMethod] = useState<ManualOrderPaymentMethod>(ManualOrderPaymentMethod.XENDIT_LINK);
  const [note, setNote] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<{ checkoutUrl: string; paidDirectly: boolean } | undefined>();

  useEffect(() => {
    if (!open || !seasonId) return;
    setPilgrimSearch(""); setPilgrimId(""); setProductId(""); setQuantity(1);
    setMethod(ManualOrderPaymentMethod.XENDIT_LINK); setNote(""); setErrors({}); setResult(undefined);
    pilgrimClient.listPilgrims({ seasonId, limit: 500, offset: 0 }).then((r) => setPilgrims(r.pilgrims)).catch(() => setPilgrims([]));
    productClient.listProducts({ seasonId }).then((r) => setProducts(r.products.filter((p) => p.isActive))).catch(() => setProducts([]));
  }, [open, seasonId]);

  useEffect(() => {
    const onEsc = (e: KeyboardEvent) => e.key === "Escape" && !saving && onClose();
    if (open) window.addEventListener("keydown", onEsc);
    return () => window.removeEventListener("keydown", onEsc);
  }, [open, saving, onClose]);

  const filteredPilgrims = useMemo(() => {
    const term = pilgrimSearch.trim().toLowerCase();
    if (!term) return pilgrims.slice(0, 20);
    return pilgrims.filter((p) => `${p.fullName} ${p.passportNumber}`.toLowerCase().includes(term)).slice(0, 20);
  }, [pilgrims, pilgrimSearch]);

  const selectedPilgrim = pilgrims.find((p) => p.id === pilgrimId);
  const selectedProduct = products.find((p) => p.id === productId);
  const total = selectedProduct ? selectedProduct.priceIdr * BigInt(quantity) : 0n;

  if (!open) return null;

  async function submit(e: FormEvent) {
    e.preventDefault();
    const errs: Record<string, string> = {};
    if (!pilgrimId) errs.pilgrim = "Pilih jamaah.";
    if (!productId) errs.product = "Pilih produk.";
    if (quantity < 1 || quantity > 20) errs.quantity = "Jumlah harus 1–20.";
    if (Object.keys(errs).length) { setErrors(errs); return; }
    setSaving(true);
    try {
      const response = await orderClient.createManualOrder({ pilgrimId, productId, quantity, paymentMethod: method, note: note.trim() });
      if (method === ManualOrderPaymentMethod.XENDIT_LINK) {
        setResult({ checkoutUrl: response.checkoutUrl, paidDirectly: false });
      } else {
        setResult({ checkoutUrl: "", paidDirectly: true });
      }
      onCreated();
    } catch (err) {
      setErrors({ _form: err instanceof Error ? err.message : "Gagal membuat pesanan." });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={o}>
      <aside style={s}>
        <div style={h}>
          <div><p style={ey}>PRODUK DIGITAL</p><h2 style={{ margin: 0 }}>Buat Pesanan</h2></div>
          <button className="btn-close-sheet" onClick={() => !saving && onClose()} style={x}><IconX size={18} /></button>
        </div>
        <div style={b}>
          {result ? (
            <div style={{ display: "grid", gap: 16, textAlign: "center", padding: "20px 0" }}>
              {result.paidDirectly ? (
                <>
                  <p style={{ fontSize: 40 }}>✅</p>
                  <p style={{ fontWeight: 700, fontSize: 16 }}>Pesanan tercatat sebagai LUNAS</p>
                  <p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>Sudah masuk ke riwayat pesanan dan laporan komisi.</p>
                </>
              ) : (
                <>
                  <p style={{ fontWeight: 700, fontSize: 16 }}>Link pembayaran siap dibagikan</p>
                  <div style={linkBox}>
                    <code style={{ fontSize: 12, wordBreak: "break-all" }}>{result.checkoutUrl}</code>
                    <button type="button" style={ghost} onClick={() => navigator.clipboard.writeText(result.checkoutUrl)}><IconCopy size={15} />Salin</button>
                  </div>
                  <p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>Kirim link ini ke jamaah lewat WhatsApp. Status pesanan akan otomatis berubah menjadi LUNAS setelah dibayar.</p>
                </>
              )}
              <button onClick={onClose} style={primary}>Selesai</button>
            </div>
          ) : (
            <form id="order-form" onSubmit={submit} style={{ display: "grid", gap: 16 }}>
              <p style={sec}>JAMAAH</p>
              <label style={{ display: "grid", gap: 6 }}>
                <span style={lab}>Cari jamaah</span>
                <input className="safrat-input" placeholder="Nama atau nomor paspor" value={selectedPilgrim ? selectedPilgrim.fullName : pilgrimSearch} onChange={(e) => { setPilgrimSearch(e.target.value); setPilgrimId(""); }} style={i} />
                {errors.pilgrim && <small style={{ color: "var(--color-danger-600)" }}>{errors.pilgrim}</small>}
              </label>
              {!selectedPilgrim && pilgrimSearch && (
                <div style={{ display: "grid", gap: 6, maxHeight: 180, overflowY: "auto" }}>
                  {filteredPilgrims.map((p) => (
                    <button key={p.id} type="button" onClick={() => { setPilgrimId(p.id); setPilgrimSearch(""); }} style={candidateButton}>
                      <strong>{p.fullName}</strong><span>{p.passportNumber}</span>
                    </button>
                  ))}
                  {!filteredPilgrims.length && <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13 }}>Tidak ditemukan.</p>}
                </div>
              )}

              <p style={sec}>PRODUK</p>
              <label style={{ display: "grid", gap: 6 }}>
                <span style={lab}>Produk</span>
                <select className="safrat-input" value={productId} onChange={(e) => setProductId(e.target.value)} style={i}>
                  <option value="">Pilih produk</option>
                  {products.map((p) => <option key={p.id} value={p.id}>{p.name} · {rupiah(p.priceIdr)}</option>)}
                </select>
                {errors.product && <small style={{ color: "var(--color-danger-600)" }}>{errors.product}</small>}
              </label>
              <label style={{ display: "grid", gap: 6 }}>
                <span style={lab}>Jumlah</span>
                <input className="safrat-input" type="number" min={1} max={20} value={quantity} onChange={(e) => setQuantity(Number(e.target.value) || 1)} style={i} />
                {errors.quantity && <small style={{ color: "var(--color-danger-600)" }}>{errors.quantity}</small>}
              </label>
              {selectedProduct && <div style={summaryCard}><span style={{ color: "var(--color-warm-500)" }}>Total tagihan</span><b style={{ fontSize: 20 }}>{rupiah(total)}</b></div>}

              <p style={sec}>PEMBAYARAN</p>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 8 }}>
                {METHODS.map(({ value, label, hint, icon: Icon }) => (
                  <button key={value} type="button" onClick={() => setMethod(value)} style={method === value ? methodBtnActive : methodBtn}>
                    <Icon size={20} />
                    <span style={{ fontSize: 12, fontWeight: 700 }}>{label}</span>
                    <span style={{ fontSize: 10, color: "var(--color-warm-400)" }}>{hint}</span>
                  </button>
                ))}
              </div>
              <label style={{ display: "grid", gap: 6 }}>
                <span style={lab}>Catatan (opsional)</span>
                <input className="safrat-input" placeholder="mis. No. referensi transfer" value={note} onChange={(e) => setNote(e.target.value)} style={i} />
              </label>

              {errors._form && <p style={err}>{errors._form}</p>}
            </form>
          )}
        </div>
        {!result && (
          <div style={foot}>
            <button form="order-form" disabled={saving} style={primary}>{saving ? "Memproses..." : "Buat Pesanan"}</button>
          </div>
        )}
      </aside>
    </div>
  );
}

const o: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)", backdropFilter: "blur(2px)" };
const s: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#fff", borderRadius: "16px 0 0 16px", overflow: "hidden" };
const h: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const b: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24 };
const foot: React.CSSProperties = { padding: "16px 24px", borderTop: "1px solid var(--color-cream-300)" };
const x: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const ey: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const sec: React.CSSProperties = { margin: 0, fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "var(--color-warm-400)", paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)" };
const i: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const primary: React.CSSProperties = { minHeight: 48, width: "100%", border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const err: React.CSSProperties = { margin: 0, padding: 10, borderRadius: 8, background: "#ffe4e6", color: "var(--color-danger-600)" };
const candidateButton: React.CSSProperties = { minHeight: 44, display: "grid", gap: 2, textAlign: "start", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "8px 12px", background: "white", color: "var(--color-emerald-900)" };
const summaryCard: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: 14, background: "var(--color-cream-200)", borderRadius: 10, border: "1px solid var(--color-cream-400)" };
const methodBtn: React.CSSProperties = { minHeight: 72, display: "grid", justifyItems: "center", gap: 4, border: "1.5px solid var(--color-cream-400)", borderRadius: 10, background: "#fff", color: "var(--color-warm-500)", padding: "8px 4px" };
const methodBtnActive: React.CSSProperties = { ...methodBtn, border: "1.5px solid var(--color-gold-500)", background: "var(--color-gold-50)", color: "var(--color-gold-800)" };
const ghost: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "6px 10px", background: "transparent", display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600 };
const linkBox: React.CSSProperties = { display: "grid", gap: 8, padding: 12, background: "var(--color-cream-200)", borderRadius: 10, border: "1px solid var(--color-cream-400)" };
