"use client";

import { useCallback, useEffect, useState } from "react";
import { IconArrowRight, IconCoin } from "@tabler/icons-react";
import type { Order, PlanChangeRow, PilgrimCreditRow } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import type { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { orderClient, productClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) => `Rp${Number(n).toLocaleString("id-ID")}`;
const dateOf = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) : "";

/**
 * Pindah paket, and the credit it can leave behind.
 *
 * Scoped to orders that are PAID — the whole idea of comparing what was
 * already paid to what the new package costs only makes sense once money has
 * actually moved. A pending order should be edited directly, not "moved".
 */
export default function PilgrimPlanChangePanel({ pilgrimId, seasonId }: { pilgrimId: string; seasonId: string }) {
  const [orders, setOrders] = useState<Order[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [changes, setChanges] = useState<PlanChangeRow[]>([]);
  const [credits, setCredits] = useState<PilgrimCreditRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  const [movingOrderId, setMovingOrderId] = useState("");
  const [toProductId, setToProductId] = useState("");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveNotice, setSaveNotice] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    setFailure("");
    Promise.all([
      orderClient.listOrdersForPilgrim({ pilgrimId }),
      orderClient.listPlanChanges({ pilgrimId, limit: 10 }),
      orderClient.listPilgrimCredits({ pilgrimId, onlyOpen: false, limit: 20 }),
    ])
      .then(([ordersResponse, changesResponse, creditsResponse]) => {
        setOrders(ordersResponse.orders);
        setChanges(changesResponse.changes);
        setCredits(creditsResponse.credits);
      })
      .catch(() => setFailure("Gagal memuat riwayat paket."))
      .finally(() => setLoading(false));
  }, [pilgrimId]);

  useEffect(load, [load]);

  useEffect(() => {
    productClient.listProducts({ seasonId }).then((response) => setProducts(response.products)).catch(() => undefined);
  }, [seasonId]);

  const paidOrders = orders.filter((order) => order.status === "PAID");
  const movingOrder = paidOrders.find((order) => order.id === movingOrderId);

  const changePlan = async () => {
    if (!movingOrder || !toProductId.trim() || reason.trim().length < 10) return;
    setSaving(true);
    setSaveNotice("");
    try {
      const result = await orderClient.changeOrderProduct({
        orderId: movingOrder.id, toProductId: toProductId.trim(), reason: reason.trim(),
        idempotencyKey: `plan-${movingOrder.id}-${crypto.randomUUID()}`,
      });
      if (result.overpaymentIdr > 0n) {
        setSaveNotice(`Berhasil dipindah. Kelebihan bayar ${rupiah(result.overpaymentIdr)} tercatat sebagai kredit terbuka.`);
      } else if (result.shortfallIdr > 0n) {
        setSaveNotice(`Berhasil dipindah. Masih ada kekurangan bayar ${rupiah(result.shortfallIdr)} yang perlu ditagih.`);
      } else {
        setSaveNotice("Berhasil dipindah. Tidak ada selisih harga.");
      }
      setMovingOrderId("");
      setToProductId("");
      setReason("");
      load();
    } catch (error: unknown) {
      setSaveNotice(error instanceof Error ? error.message : "Gagal memindahkan paket.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <section style={card}><p style={{ margin: 0, color: "var(--color-warm-500)" }}>Memuat…</p></section>;

  const openCredits = credits.filter((credit) => credit.status === "OPEN");

  return (
    <section style={card}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        <h2 style={{ margin: 0 }}>Pindah Paket</h2>
        {openCredits.length > 0 && (
          <span style={{ ...badge, background: "var(--color-warning-600)" }}>
            <IconCoin size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} />
            {openCredits.length} kredit terbuka
          </span>
        )}
      </div>

      {failure && <p style={warning}>{failure}</p>}

      {paidOrders.length === 0 ? (
        <p style={{ margin: 0, fontSize: 13, color: "var(--color-warm-400)" }}>
          Belum ada pesanan lunas yang bisa dipindahkan paketnya.
        </p>
      ) : (
        <>
          <label style={field}>Pesanan yang dipindah
            <select value={movingOrderId} onChange={(event) => setMovingOrderId(event.target.value)} style={input}>
              <option value="">Pilih pesanan lunas…</option>
              {paidOrders.map((order) => (
                <option key={order.id} value={order.id}>{order.productName} — {rupiah(order.totalPriceIdr)}</option>
              ))}
            </select>
          </label>
          {movingOrder && (
            <>
              <label style={field}>Paket tujuan
                <select value={toProductId} onChange={(event) => setToProductId(event.target.value)} style={input}>
                  <option value="">Pilih paket tujuan…</option>
                  {products.filter((product) => product.id !== movingOrder.productId && product.isActive).map((product) => (
                    <option key={product.id} value={product.id}>{product.name} — {rupiah(product.priceIdr)}</option>
                  ))}
                </select>
              </label>
              <label style={field}>Alasan (minimal 10 huruf)
                <textarea value={reason} onChange={(event) => setReason(event.target.value)} rows={2}
                  placeholder="mis. jamaah minta pindah ke paket yang lebih murah" style={{ ...input, minHeight: 64, resize: "vertical" }} />
              </label>
              <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-500)" }}>
                Sudah dibayar untuk pesanan ini: <strong>{rupiah(movingOrder.paidAmountIdr ?? 0n)}</strong>. Kalau paket
                baru lebih murah, selisihnya jadi kredit terbuka — bukan hilang. Kalau lebih mahal, selisihnya
                dilaporkan sebagai kekurangan yang perlu ditagih terpisah.
              </p>
              {saveNotice && <p style={{ margin: 0, fontSize: 13, fontWeight: 600, color: "var(--color-emerald-800)" }}>{saveNotice}</p>}
              <button disabled={saving || !toProductId.trim() || reason.trim().length < 10} onClick={changePlan} style={emerald}>
                {saving ? "Memindahkan…" : "Pindahkan Paket"}
              </button>
            </>
          )}
        </>
      )}

      {credits.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <p style={{ margin: "0 0 8px", fontWeight: 700, fontSize: 14 }}>Kelebihan Bayar</p>
          <div style={{ display: "grid", gap: 8 }}>
            {credits.map((credit) => (
              <div key={credit.id} style={creditRow}>
                <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
                  <strong>{rupiah(credit.amountIdr)}</strong>
                  <span style={{
                    fontSize: 11, fontWeight: 700, color: credit.status === "OPEN" ? "var(--color-warning-700)" : "var(--color-warm-400)",
                  }}>
                    {credit.status === "OPEN" ? "terbuka" : credit.status === "APPLIED" ? "sudah dipakai" : "dikembalikan"}
                  </span>
                </div>
                <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{credit.reason}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {changes.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <p style={{ margin: "0 0 8px", fontWeight: 700, fontSize: 14 }}>Riwayat Pindah Paket</p>
          <div style={{ display: "grid", gap: 8 }}>
            {changes.map((change) => (
              <div key={change.id} style={creditRow}>
                <div style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 600 }}>
                  {change.fromProductName}<IconArrowRight size={13} />{change.toProductName}
                </div>
                <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
                  {dateOf(change.createdAt)} · {change.reason}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 12, alignContent: "start" };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px" };
const badge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "6px 14px" };
const warning: React.CSSProperties = { margin: 0, color: "var(--color-danger-600)", fontWeight: 600, fontSize: 13 };
const creditRow: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 12px" };
