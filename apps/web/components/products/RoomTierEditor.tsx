"use client";

import { useCallback, useEffect, useState } from "react";
import { IconAlertTriangle, IconBed, IconDeviceFloppy } from "@tabler/icons-react";
import type { ProductRoomTier } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { productClient } from "@/lib/rpc";

const money = (n: bigint | number) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

// The ladder as a person reads it: most people to a room first, because that is
// the cheapest and the one most packages lead with.
const TIERS: [string, string, string][] = [
  ["QUAD", "Quad", "4 orang sekamar"],
  ["TRIPLE", "Triple", "3 orang sekamar"],
  ["DOUBLE", "Double", "2 orang sekamar"],
];

interface Draft {
  enabled: boolean;
  delta: string;
  quota: string;
  taken: number;
}

const emptyDraft = (): Draft => ({ enabled: false, delta: "0", quota: "", taken: 0 });

export default function RoomTierEditor({ productId, productName }: { productId: string; productName: string }) {
  const [drafts, setDrafts] = useState<Record<string, Draft>>(() =>
    Object.fromEntries(TIERS.map(([tier]) => [tier, emptyDraft()])));
  const [basePrice, setBasePrice] = useState(0n);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState("");
  const [saved, setSaved] = useState(false);

  const apply = useCallback((tiers: ProductRoomTier[], base: bigint) => {
    setBasePrice(base);
    setDrafts(Object.fromEntries(TIERS.map(([tier]) => {
      const found = tiers.find((row) => row.tier === tier);
      if (!found) return [tier, emptyDraft()];
      return [tier, {
        enabled: found.isActive,
        delta: String(found.priceDeltaIdr),
        // An absent quota is not zero. Blank means no limit; "0" means the tier
        // exists and has nothing left to sell, and the two must not read alike.
        quota: found.seatQuota === undefined ? "" : String(found.seatQuota),
        taken: found.seatsTaken,
      }];
    })));
  }, []);

  useEffect(() => {
    setLoading(true);
    productClient
      .listProductRoomTiers({ productId })
      .then((response) => apply(response.tiers, response.basePriceIdr))
      .catch(() => setFailure("Gagal memuat tier kamar."))
      .finally(() => setLoading(false));
  }, [productId, apply]);

  const save = async () => {
    setSaving(true);
    setFailure("");
    setSaved(false);
    try {
      const response = await productClient.setProductRoomTiers({
        productId,
        tiers: TIERS.filter(([tier]) => drafts[tier]?.enabled).map(([tier]) => ({
          tier,
          priceDeltaIdr: BigInt(drafts[tier]?.delta || "0"),
          // Blank means no limit, and must be sent as absent rather than as a
          // number the server would read as a real cap.
          seatQuota: (drafts[tier]?.quota ?? "").trim() === "" ? undefined : Number(drafts[tier]?.quota),
          isActive: true,
          priceIdr: 0n,
          seatsTaken: 0,
          $typeName: "hajj.v1.ProductRoomTier" as const,
        })),
      });
      // The server recomputes; showing its answer keeps one implementation of
      // the pricing rule.
      apply(response.tiers, response.basePriceIdr);
      setSaved(true);
    } catch (error: unknown) {
      setFailure(error instanceof Error ? error.message : "Gagal menyimpan tier kamar.");
    } finally {
      setSaving(false);
    }
  };

  const update = (tier: string, patch: Partial<Draft>) =>
    setDrafts((current) => ({ ...current, [tier]: { ...(current[tier] ?? emptyDraft()), ...patch } }));

  if (loading) return <p style={muted}>Memuat tier kamar…</p>;

  return (
    <div style={panel}>
      <div style={{ marginBottom: 12 }}>
        <h3 style={{ margin: 0, fontSize: 15, display: "flex", alignItems: "center", gap: 8 }}>
          <IconBed size={17} />Tier kamar — {productName}
        </h3>
        <p style={muted}>
          Harga tier disimpan sebagai <strong>selisih</strong> dari harga paket ({money(basePrice)}), bukan sebagai
          angka tersendiri. Kalau harga paketnya berubah, seluruh tier ikut — tanpa ada yang perlu disamakan ulang.
          Kuota kosong berarti tanpa batas; nol berarti tiernya ada tapi sudah habis.
        </p>
      </div>

      {failure && <p style={errorBox}><IconAlertTriangle size={15} />{failure}</p>}
      {saved && !failure && <p style={okBox}>Tier kamar tersimpan.</p>}

      <div style={{ overflowX: "auto" }}>
        <table style={table}>
          <thead>
            <tr>{["Tier", "Ditawarkan", "Selisih harga", "Harga jadinya", "Kuota kursi", "Terisi"].map((head) => (
              <th key={head} style={th}>{head}</th>
            ))}</tr>
          </thead>
          <tbody>
            {TIERS.map(([tier, label, hint]) => {
              // Index access is typed as possibly missing; the map is built from
              // TIERS so every key exists, and the fallback keeps that true even
              // if the list ever grows a tier the state was not seeded with.
              const draft = drafts[tier] ?? emptyDraft();
              const delta = Number(draft.delta || "0");
              const total = Number(basePrice) + delta;
              const quota = draft.quota.trim() === "" ? undefined : Number(draft.quota);
              const full = quota !== undefined && draft.taken >= quota;
              return (
                <tr key={tier} style={tr}>
                  <td style={td}>
                    <strong>{label}</strong>
                    <small style={{ display: "block", color: "var(--color-warm-400)" }}>{hint}</small>
                  </td>
                  <td style={td}>
                    <input
                      type="checkbox"
                      checked={draft.enabled}
                      onChange={(event) => update(tier, { enabled: event.target.checked })}
                      aria-label={`Tawarkan ${label}`}
                      style={{ width: 18, height: 18 }}
                    />
                  </td>
                  <td style={td}>
                    <input
                      inputMode="numeric"
                      value={draft.delta}
                      disabled={!draft.enabled}
                      onChange={(event) => update(tier, { delta: event.target.value.replace(/[^\d-]/g, "") })}
                      style={numInput}
                      aria-label={`Selisih harga ${label}`}
                    />
                  </td>
                  <td style={{ ...td, fontWeight: 700, color: total < 0 ? "var(--color-danger-600)" : undefined }}>
                    {draft.enabled ? money(total) : "—"}
                    {total < 0 && <small style={{ display: "block", fontWeight: 500 }}>di bawah nol</small>}
                  </td>
                  <td style={td}>
                    <input
                      inputMode="numeric"
                      value={draft.quota}
                      disabled={!draft.enabled}
                      placeholder="tanpa batas"
                      onChange={(event) => update(tier, { quota: event.target.value.replace(/\D/g, "") })}
                      style={numInput}
                      aria-label={`Kuota kursi ${label}`}
                    />
                  </td>
                  <td style={td}>
                    {draft.taken}
                    {full && <small style={{ display: "block", color: "var(--color-warning-700)", fontWeight: 700 }}>penuh</small>}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <button type="button" onClick={save} disabled={saving} style={saveButton}>
        <IconDeviceFloppy size={15} />{saving ? "Menyimpan…" : "Simpan tier kamar"}
      </button>
      <p style={{ ...muted, marginTop: 10 }}>
        Kuota ditegakkan saat pemesanan, bukan hanya ditampilkan di sini: pemesanan yang melewati batas ditolak
        database, jadi kursi terakhir tidak bisa terjual dua kali walau dua orang menekan bayar bersamaan.
      </p>
    </div>
  );
}

const panel: React.CSSProperties = { border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "#fff", padding: 18 };
const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: "6px 0 0", lineHeight: 1.6 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse" };
const th: React.CSSProperties = { textAlign: "left", padding: 10, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 10, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const numInput: React.CSSProperties = { minHeight: 38, width: 130, textAlign: "right", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", fontFamily: "inherit", fontSize: 13 };
const saveButton: React.CSSProperties = { marginTop: 14, minHeight: 42, padding: "0 18px", borderRadius: 8, border: 0, background: "var(--color-emerald-800)", color: "#fff", font: "inherit", fontWeight: 700, fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" };
const errorBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: "0 0 12px", padding: "10px 14px", borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontSize: 13, fontWeight: 600 };
const okBox: React.CSSProperties = { margin: "0 0 12px", padding: "10px 14px", borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)", fontSize: 13, fontWeight: 600 };
