"use client";

import { useCallback, useEffect, useState } from "react";
import { IconTruckDelivery, IconAlertTriangle, IconCheck } from "@tabler/icons-react";
import { Shipment } from "@hajj-saas/proto-gen/hajj/v1/shipment_pb";
import { shipmentClient } from "@/lib/rpc";

const rupiah = (n: bigint) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

const STATUS_LABEL: Record<string, string> = {
  PENDING: "Menunggu diproses",
  SENT: "Dalam pengiriman",
  DELIVERED: "Sudah diterima",
  FAILED: "Gagal",
};

export default function ShipmentsDashboard() {
  const [shipments, setShipments] = useState<Shipment[]>([]);
  const [includeDelivered, setIncludeDelivered] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    shipmentClient.listShipments({ includeDelivered })
      .then((r) => setShipments(r.shipments))
      .catch(() => setError("Gagal memuat daftar pengiriman."))
      .finally(() => setLoading(false));
  }, [includeDelivered]);
  useEffect(() => { load(); }, [load]);

  const replace = (next: Shipment) => {
    setShipments((current) =>
      // A delivered parcel leaves the working queue, matching what the server
      // returns on the next load — rather than sitting there looking unfinished.
      includeDelivered || next.status !== "DELIVERED"
        ? current.map((s) => (s.orderId === next.orderId ? next : s))
        : current.filter((s) => s.orderId !== next.orderId),
    );
  };

  const waiting = shipments.filter((s) => s.status === "PENDING").length;

  return (
    <section style={{ display: "grid", gap: 16 }}>
      <div>
        <p style={eyebrow}>OPERASIONAL / PENGIRIMAN</p>
        <h1 style={{ margin: 0, fontSize: 22 }}>Pengiriman Perlengkapan</h1>
        <p style={muted}>
          Perlengkapan yang sudah dibayar dan menunggu diserahkan. Barang yang sudah
          berangkat tidak dapat diubah tujuannya — catatan pengiriman adalah bukti,
          bukan formulir.
        </p>
      </div>

      {waiting > 0 && (
        <p style={warnBox}>
          <IconAlertTriangle size={17} />
          {waiting} pesanan sudah dibayar dan belum diproses.
        </p>
      )}

      <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 14 }}>
        <input type="checkbox" checked={includeDelivered} onChange={(e) => setIncludeDelivered(e.target.checked)} />
        Tampilkan juga yang sudah diterima
      </label>

      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {loading ? (
        <p style={muted}>Memuat…</p>
      ) : shipments.length === 0 ? (
        <p style={muted}>Tidak ada pengiriman yang menunggu.</p>
      ) : (
        <div style={{ display: "grid", gap: 12 }}>
          {shipments.map((shipment) => (
            <ShipmentCard
              key={shipment.orderId}
              shipment={shipment}
              onSaved={(next) => { replace(next); }}
              onNotice={setNotice}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function ShipmentCard({ shipment, onSaved, onNotice }: {
  shipment: Shipment;
  onSaved: (next: Shipment) => void;
  onNotice: (message: string) => void;
}) {
  const [method, setMethod] = useState(shipment.deliveryMethod || "SHIP");
  const [recipientName, setRecipientName] = useState(shipment.recipientName);
  const [recipientPhone, setRecipientPhone] = useState(shipment.recipientPhone);
  const [address, setAddress] = useState(shipment.shippingAddress);
  const [courier, setCourier] = useState(shipment.courier);
  const [tracking, setTracking] = useState(shipment.trackingNumber);
  const [handover, setHandover] = useState("");
  const [handoverNote, setHandoverNote] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  const run = async (label: string, action: () => Promise<Shipment>, done: string) => {
    setBusy(label);
    setError("");
    try {
      onSaved(await action());
      onNotice(done);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan.");
    } finally {
      setBusy("");
    }
  };

  const editable = shipment.destinationEditable;
  const delivered = shipment.status === "DELIVERED";

  return (
    <div style={card}>
      <div style={{ display: "flex", gap: 12, flexWrap: "wrap", alignItems: "baseline" }}>
        <strong style={{ fontSize: 15 }}>{shipment.productName}</strong>
        <span style={badge}>{STATUS_LABEL[shipment.status] ?? shipment.status}</span>
        <span style={{ ...muted, margin: 0 }}>
          {shipment.buyerName} · {shipment.quantity}× · {rupiah(shipment.totalPriceIdr)}
          {shipment.receiptNumber ? ` · ${shipment.receiptNumber}` : ""}
        </span>
      </div>

      {error && <p style={{ color: "var(--color-danger-600)", margin: 0, fontSize: 13 }}>{error}</p>}

      {delivered ? (
        <p style={{ margin: 0, display: "flex", gap: 6, alignItems: "center", color: "var(--color-emerald-900)", fontWeight: 700, fontSize: 14 }}>
          <IconCheck size={17} />
          Diterima oleh {shipment.handoverRecipient}
          {shipment.handoverNote ? (
            <span style={{ fontWeight: 400, color: "var(--color-warm-500)" }}>· {shipment.handoverNote}</span>
          ) : null}
        </p>
      ) : (
        <>
          {editable ? (
            <div style={{ display: "grid", gap: 10 }}>
              <div style={grid}>
                <label style={label}>
                  Metode
                  <select value={method} onChange={(e) => setMethod(e.target.value)} style={input}>
                    <option value="SHIP">Dikirim kurir</option>
                    <option value="PICKUP">Diambil sendiri</option>
                  </select>
                </label>
                <Field label="Nama penerima" value={recipientName} onChange={setRecipientName} />
                <Field label="Nomor penerima" value={recipientPhone} onChange={setRecipientPhone} />
              </div>
              {method === "SHIP" && (
                <label style={label}>
                  Alamat lengkap
                  <textarea value={address} onChange={(e) => setAddress(e.target.value)} style={{ ...input, minHeight: 72, padding: 10 }} />
                </label>
              )}
              <button
                style={ghost}
                disabled={busy !== ""}
                onClick={() => run("dest", () =>
                  shipmentClient.saveShipmentDestination({
                    orderId: shipment.orderId, deliveryMethod: method,
                    recipientName, recipientPhone, shippingAddress: method === "SHIP" ? address : "",
                  }), "Tujuan disimpan.")}
              >
                {busy === "dest" ? "Menyimpan…" : "Simpan tujuan"}
              </button>
            </div>
          ) : (
            <p style={{ ...muted, margin: 0 }}>
              {shipment.deliveryMethod === "PICKUP"
                ? `Diambil sendiri oleh ${shipment.recipientName}`
                : `${shipment.courier} · ${shipment.trackingNumber} → ${shipment.shippingAddress}`}
            </p>
          )}

          {shipment.status === "PENDING" && method === "SHIP" && (
            <div style={grid}>
              <Field label="Kurir" value={courier} onChange={setCourier} />
              <Field label="Nomor resi" value={tracking} onChange={setTracking} />
              <button
                style={primary}
                disabled={busy !== "" || !courier.trim() || !tracking.trim()}
                onClick={() => run("sent", () =>
                  shipmentClient.markShipmentSent({ orderId: shipment.orderId, courier, trackingNumber: tracking }),
                  "Ditandai terkirim.")}
              >
                <IconTruckDelivery size={16} />
                {busy === "sent" ? "Menyimpan…" : "Tandai terkirim"}
              </button>
            </div>
          )}

          <div style={grid}>
            <Field label="Diterima oleh" value={handover} onChange={setHandover} hint="Nama orang yang menerima barang" />
            <Field label="Catatan" value={handoverNote} onChange={setHandoverNote} />
            <button
              style={primary}
              disabled={busy !== "" || !handover.trim()}
              onClick={() => run("handover", () =>
                shipmentClient.markShipmentHandedOver({
                  orderId: shipment.orderId, handoverRecipient: handover, handoverNote,
                }), "Serah terima tercatat.")}
            >
              <IconCheck size={16} />
              {busy === "handover" ? "Menyimpan…" : "Catat serah terima"}
            </button>
          </div>
        </>
      )}
    </div>
  );
}

// The hint sits outside the <label> and is attached with aria-describedby.
// Inside it, a wrapping label folds the hint into the field's accessible name —
// so "Diterima oleh" becomes "Diterima oleh Nama orang yang menerima barang",
// which is worse to hear read aloud and impossible to address precisely.
function Field({ label: text, value, onChange, hint }: { label: string; value: string; onChange: (v: string) => void; hint?: string }) {
  const hintId = hint ? `hint-${text.replace(/\s+/g, "-").toLowerCase()}` : undefined;
  return (
    <div style={label}>
      <label style={{ display: "grid", gap: 6 }}>
        {text}
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          style={input}
          aria-describedby={hintId}
        />
      </label>
      {hint ? <small id={hintId} style={{ color: "var(--color-warm-400)", fontSize: 11 }}>{hint}</small> : null}
    </div>
  );
}

const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: "6px 0 0", maxWidth: 620 };
const card: React.CSSProperties = { display: "grid", gap: 12, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(180px,1fr))", gap: 12, alignItems: "end" };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontFamily: "inherit", fontSize: 14 };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7, justifySelf: "start" };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, justifySelf: "start", fontSize: 13 };
const badge: React.CSSProperties = { fontSize: 11, fontWeight: 700, padding: "3px 9px", borderRadius: 999, background: "var(--color-cream-300)", color: "var(--color-warm-700)" };
const warnBox: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, color: "#b45309", fontWeight: 700, fontSize: 13, padding: "10px 14px", background: "#fffbeb", border: "1px solid #fde68a", borderRadius: 8, margin: 0 };
