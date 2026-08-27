"use client";

import { useCallback, useEffect, useState } from "react";
import { IconPlus, IconAlertTriangle } from "@tabler/icons-react";
import { PlatformProduct } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint) =>
  new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(Number(n));

const CATEGORIES = [
  { value: "PPOB_CREDIT", label: "Pulsa & PPOB" },
  { value: "ROAMING_DATA", label: "Paket Data Roaming" },
];

type Draft = {
  productId: string;
  name: string;
  code: string;
  category: string;
  nominalIdr: string;
  basePriceIdr: string;
  description: string;
  isActive: boolean;
};

const emptyDraft = (): Draft => ({
  productId: "",
  name: "",
  code: "",
  category: "PPOB_CREDIT",
  nominalIdr: "",
  basePriceIdr: "",
  description: "",
  isActive: true,
});

export default function CatalogueTab() {
  const [products, setProducts] = useState<PlatformProduct[]>([]);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    platformClient
      .listPlatformCatalogue({})
      .then((r) => setProducts(r.products))
      .catch(() => setError("Gagal memuat katalog."))
      .finally(() => setLoading(false));
  }, []);
  useEffect(() => { load(); }, [load]);

  const save = async () => {
    if (!draft) return;
    setSaving(true);
    setError("");
    try {
      await platformClient.savePlatformProduct({
        productId: draft.productId,
        name: draft.name.trim(),
        code: draft.code.trim().toUpperCase(),
        category: draft.category,
        nominalIdr: BigInt(draft.nominalIdr.replace(/\D/g, "") || "0"),
        basePriceIdr: BigInt(draft.basePriceIdr.replace(/\D/g, "") || "0"),
        description: draft.description.trim(),
        isActive: draft.isActive,
      });
      setNotice(`${draft.name} disimpan.`);
      setDraft(null);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan produk.");
    } finally {
      setSaving(false);
    }
  };

  const incomplete = products.filter((p) => !p.basePriceSet).length;

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <p style={muted}>
        Produk digital dipasok TawafiqHub dan dijual oleh semua travel. Satu baris di
        sini dipakai bersama; masing-masing travel menambahkan markup sendiri di atas
        harga dasar. Travel tidak dapat membuat atau mengubah produk ini.
      </p>

      {incomplete > 0 && (
        <p style={warnBox}>
          <IconAlertTriangle size={17} />
          {incomplete} produk belum punya harga dasar dan belum bisa dijual travel mana pun.
        </p>
      )}

      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {draft ? (
        <div style={card}>
          <div style={grid}>
            <Field label="Nama" value={draft.name} onChange={(v) => setDraft({ ...draft, name: v })} />
            <Field label="Kode" value={draft.code} onChange={(v) => setDraft({ ...draft, code: v })} hint="Dikutip orang, mis. PULSA-TSEL-10K" />
            <label style={label}>
              Kategori
              <select value={draft.category} onChange={(e) => setDraft({ ...draft, category: e.target.value })} style={input}>
                {CATEGORIES.map((c) => <option key={c.value} value={c.value}>{c.label}</option>)}
              </select>
            </label>
            <Field label="Nominal (Rp)" value={draft.nominalIdr} onChange={(v) => setDraft({ ...draft, nominalIdr: v.replace(/\D/g, "") })} hint="Nilai yang diterima pelanggan" />
            <Field label="Harga Dasar (Rp)" value={draft.basePriceIdr} onChange={(v) => setDraft({ ...draft, basePriceIdr: v.replace(/\D/g, "") })} hint="Yang ditagih ke travel" />
          </div>
          <Field label="Deskripsi" value={draft.description} onChange={(v) => setDraft({ ...draft, description: v })} />
          <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 14 }}>
            <input type="checkbox" checked={draft.isActive} onChange={(e) => setDraft({ ...draft, isActive: e.target.checked })} />
            Aktif
          </label>
          <div style={{ display: "flex", gap: 8 }}>
            <button style={primary} onClick={save} disabled={saving || !draft.name.trim() || !draft.code.trim()}>
              {saving ? "Menyimpan…" : "Simpan"}
            </button>
            <button style={ghost} onClick={() => setDraft(null)}>Batal</button>
          </div>
        </div>
      ) : (
        <button style={primary} onClick={() => setDraft(emptyDraft())}>
          <IconPlus size={17} />Tambah Produk Platform
        </button>
      )}

      {loading ? (
        <p style={muted}>Memuat…</p>
      ) : products.length === 0 ? (
        <p style={muted}>Katalog platform masih kosong.</p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead>
              <tr>{["Produk", "Kategori", "Nominal", "Harga Dasar", "Harga Modal", "Status", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
            </thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id} style={tr}>
                  <td style={td}>
                    <strong>{product.name}</strong>
                    <small style={{ display: "block", color: "var(--color-warm-400)" }}>{product.code}</small>
                  </td>
                  <td style={td}>{CATEGORIES.find((c) => c.value === product.category)?.label ?? product.category}</td>
                  <td style={td}>{product.nominalIdr > 0n ? rupiah(product.nominalIdr) : "—"}</td>
                  <td style={td}>
                    {product.basePriceSet ? rupiah(product.basePriceIdr) : <span style={warn}>belum diatur</span>}
                  </td>
                  <td style={td}>
                    {product.supplierCostSource
                      ? <>{rupiah(product.supplierCostIdr)}<small style={{ display: "block", color: "var(--color-warm-400)" }}>{product.supplierCostSource === "OBSERVED" ? "dari supplier" : "manual"}</small></>
                      : <span style={warn}>belum diketahui</span>}
                  </td>
                  <td style={td}>{product.isActive ? "Aktif" : "Nonaktif"}</td>
                  <td style={td}>
                    <button
                      style={ghost}
                      onClick={() => setDraft({
                        productId: product.id,
                        name: product.name,
                        code: product.code,
                        category: product.category,
                        nominalIdr: product.nominalIdr > 0n ? String(product.nominalIdr) : "",
                        basePriceIdr: product.basePriceSet ? String(product.basePriceIdr) : "",
                        description: product.description,
                        isActive: product.isActive,
                      })}
                    >
                      Ubah
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function Field({ label: text, value, onChange, hint }: { label: string; value: string; onChange: (v: string) => void; hint?: string }) {
  return (
    <label style={label}>
      {text}
      <input value={value} onChange={(e) => onChange(e.target.value)} style={input} />
      {hint && <small style={{ color: "var(--color-warm-400)", fontSize: 11 }}>{hint}</small>}
    </label>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const card: React.CSSProperties = { display: "grid", gap: 12, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 12 };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontFamily: "inherit", fontSize: 14 };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7, justifySelf: "start" };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const warn: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, color: "#b45309", fontWeight: 700, fontSize: 12 };
const warnBox: React.CSSProperties = { ...warn, padding: "10px 14px", background: "#fffbeb", border: "1px solid #fde68a", borderRadius: 8, margin: 0 };
