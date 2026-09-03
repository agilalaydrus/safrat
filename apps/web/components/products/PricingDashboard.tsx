"use client";

import { useCallback, useEffect, useState } from "react";
import { IconAlertTriangle, IconBed, IconDeviceFloppy } from "@tabler/icons-react";
import { ProductPricing } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import RoomTierEditor from "./RoomTierEditor";
import { productClient, seasonClient } from "@/lib/rpc";

const money = (n: bigint) =>
  new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(Number(n));

export default function PricingDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [rows, setRows] = useState<ProductPricing[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const load = useCallback(async (id: string) => {
    if (!id) return;
    setLoading(true);
    try {
      setRows((await productClient.listProductPricing({ seasonId: id })).pricing);
      setError("");
    } catch {
      setError("Gagal memuat harga produk.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    seasonClient.listSeasons({})
      .then((r) => {
        setSeasons(r.seasons);
        const first = r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "";
        setSeasonId(first);
        void load(first);
      })
      .catch(() => setError("Gagal memuat daftar musim."));
  }, [load]);

  const unsellable = rows.filter((r) => !r.sellable).length;
  // One package's tiers open at a time. Three ladders side by side is three
  // sets of numbers to confuse, and the decision is per package anyway.
  const [tierProductId, setTierProductId] = useState("");
  const tierProduct = rows.find((row) => row.productId === tierProductId);

  return (
    <section style={{ display: "grid", gap: 16 }}>
      <div>
        <h1 style={{ margin: 0, fontSize: 22 }}>Harga & Markup</h1>
        <p style={muted}>
          Harga dihitung ulang setiap kali halaman ini dibuka, bukan disimpan. Harga
          dasar ditetapkan TawafiqHub dan tidak bisa diubah dari sini — yang Anda atur
          adalah markup di atasnya.
        </p>
      </div>

      <label style={label}>
        Musim
        <select value={seasonId} onChange={(e) => { setSeasonId(e.target.value); void load(e.target.value); }} style={input}>
          {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
      </label>

      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {unsellable > 0 && (
        <p style={warnBox}>
          <IconAlertTriangle size={17} />
          {unsellable} produk belum bisa dijual. Alasannya tertulis di barisnya masing-masing.
        </p>
      )}

      {loading && <p style={muted}>Memuat…</p>}
      {!loading && rows.length === 0 && <p style={muted}>Belum ada produk di musim ini.</p>}

      <div style={{ overflowX: "auto" }}>
        <table style={table}>
          <thead>
            <tr>
              <th style={th}>PRODUK</th>
              <th style={th}>HARGA DASAR</th>
              <th style={th}>MARKUP TRAVEL</th>
              <th style={th}>MARKUP AGEN</th>
              <th style={th}>HARGA AGEN</th>
              <th style={th}>HARGA JAMAAH</th>
              <th style={th}>TIER KAMAR</th>
              <th style={th} />
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <PricingRow
                key={row.productId}
                row={row}
                tiersOpen={row.productId === tierProductId}
                onToggleTiers={() => setTierProductId((current) => (current === row.productId ? "" : row.productId))}
                onSaved={(next) =>
                  setRows((current) => current.map((r) => (r.productId === next.productId ? next : r)))}
              />
            ))}
          </tbody>
        </table>
      </div>

      {tierProduct && (
        <RoomTierEditor key={tierProduct.productId} productId={tierProduct.productId} productName={tierProduct.productName} />
      )}
    </section>
  );
}

function PricingRow({ row, onSaved, tiersOpen, onToggleTiers }: {
  row: ProductPricing;
  onSaved: (next: ProductPricing) => void;
  tiersOpen: boolean;
  onToggleTiers: () => void;
}) {
  const [operatorMarkup, setOperatorMarkup] = useState(String(row.operatorMarkupIdr));
  const [agentMarkup, setAgentMarkup] = useState(String(row.agentMarkupIdr));
  const [saving, setSaving] = useState(false);
  const [rowError, setRowError] = useState("");

  const dirty =
    operatorMarkup !== String(row.operatorMarkupIdr) ||
    agentMarkup !== String(row.agentMarkupIdr) ||
    // An unconfigured product needs a first save even when the numbers already
    // read zero, or the only way to configure it would be to type a value and
    // change it back.
    !row.markupConfigured;

  const save = async () => {
    setSaving(true);
    setRowError("");
    try {
      const result = await productClient.setProductMarkup({
        productId: row.productId,
        operatorMarkupIdr: BigInt(operatorMarkup || "0"),
        agentMarkupIdr: BigInt(agentMarkup || "0"),
      });
      // The server recomputes and returns the prices; showing its answer rather
      // than our own arithmetic keeps one implementation of the pricing rule.
      if (result.pricing) onSaved(result.pricing);
    } catch (err) {
      setRowError(err instanceof Error ? err.message : "Gagal menyimpan markup.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr style={tr}>
      <td style={td}>
        <strong>{row.productName}</strong>
        {row.code && <small style={{ display: "block", color: "var(--color-warm-400)" }}>{row.code}</small>}
        {!row.sellable && (
          <small style={warn}>
            <IconAlertTriangle size={13} />
            {row.unsellableReason}
          </small>
        )}
        {rowError && <small style={{ display: "block", color: "var(--color-danger-600)" }}>{rowError}</small>}
      </td>
      <td style={td}>
        {row.basePriceSet ? money(row.basePriceIdr) : <span style={warn}>belum diatur</span>}
      </td>
      <td style={td}>
        <input inputMode="numeric" value={operatorMarkup} onChange={(e) => setOperatorMarkup(e.target.value.replace(/\D/g, ""))} style={numInput} />
      </td>
      <td style={td}>
        <input inputMode="numeric" value={agentMarkup} onChange={(e) => setAgentMarkup(e.target.value.replace(/\D/g, ""))} style={numInput} />
      </td>
      <td style={td}>{row.sellable ? money(row.agentPriceIdr) : "—"}</td>
      <td style={{ ...td, fontWeight: 700 }}>{row.sellable ? money(row.pilgrimPriceIdr) : "—"}</td>
      <td style={td}>
        <button style={ghost} onClick={onToggleTiers} aria-expanded={tiersOpen}>
          <IconBed size={15} />{tiersOpen ? "Tutup" : "Atur"}
        </button>
      </td>
      <td style={td}>
        <button style={ghost} onClick={save} disabled={saving || !dirty}>
          <IconDeviceFloppy size={15} />
          {saving ? "Menyimpan…" : "Simpan"}
        </button>
      </td>
    </tr>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: "6px 0 0" };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)", maxWidth: 320 };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontFamily: "inherit", fontSize: 14 };
const numInput: React.CSSProperties = { ...input, minHeight: 38, width: 120, textAlign: "right" };
const ghost: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const warn: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, color: "#b45309", fontWeight: 700, fontSize: 12 };
const warnBox: React.CSSProperties = { ...warn, padding: "10px 14px", background: "#fffbeb", border: "1px solid #fde68a", borderRadius: 8, margin: 0 };
