"use client";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  IconPencil,
  IconPlus,
  IconShoppingCart,
  IconTrash,
} from "@tabler/icons-react";
import { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { productClient, seasonClient } from "@/lib/rpc";
import ProductFormDialog from "./ProductFormDialog";
import { RoleGate } from "@/components/auth/RoleGate";
const money = (n: bigint) =>
  new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(Number(n));
const CATEGORY_LABEL: Record<string, string> = {
  TRAVEL_PACKAGE: "Paket Perjalanan",
  EQUIPMENT: "Perlengkapan Umrah & Haji",
  ROAMING_DATA: "Paket Data Roaming",
  PPOB_CREDIT: "Pulsa & PPOB",
};
const CATEGORY_ORDER = [
  "TRAVEL_PACKAGE",
  "EQUIPMENT",
  "ROAMING_DATA",
  "PPOB_CREDIT",
];
export default function ProductsDashboard() {
  const [seasons, setSeasons] = useState<
      { id: string; name: string; isActive: boolean }[]
    >([]),
    [seasonId, setSeasonId] = useState(""),
    [products, setProducts] = useState<Product[]>([]),
    [open, setOpen] = useState(false),
    [edit, setEdit] = useState<Product | undefined>(),
    [notice, setNotice] = useState("");
  const load = useCallback(async (id = seasonId) => {
    if (!id) return;
    try {
      setProducts(
        (await productClient.listProducts({ seasonId: id })).products,
      );
    } catch {
      setNotice("Gagal memuat daftar produk.");
    }
  }, [seasonId]);
  useEffect(() => {
    seasonClient
      .listSeasons({})
      .then((r) => {
        setSeasons(r.seasons);
        setSeasonId(
          r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "",
        );
      })
      .catch(() => setNotice("Gagal memuat daftar musim."));
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  const active = products.filter((p) => p.isActive);
  const grouped = useMemo(
    () =>
      CATEGORY_ORDER.map((category) => ({
        category,
        items: products.filter(
          (p) => (p.category || "TRAVEL_PACKAGE") === category,
        ),
      })).filter((g) => g.items.length),
    [products],
  );
  return (
    <main style={page}>
      <header style={header}>
        <div>
          <p style={eyebrow}>OPERASIONAL / PRODUK</p>
          <h1 style={title}>Produk & Layanan</h1>
        </div>
        <div style={actions}>
          <select
            value={seasonId}
            onChange={(e) => setSeasonId(e.target.value)}
            style={select}
          >
            {seasons.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
          <button
            style={emerald}
            onClick={() => {
              setEdit(undefined);
              setOpen(true);
            }}
          >
            <IconPlus size={18} />
            Tambah Produk
          </button>
        </div>
      </header>
      <div className="gold-divider" />
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div style={stats}>
        {[
          ["Total Produk", products.length],
          ["Aktif", active.length],
          [
            "Total Nilai Paket Aktif",
            money(active.reduce((sum, p) => sum + p.priceIdr, BigInt(0))),
          ],
        ].map(([l, v]) => (
          <div key={String(l)} style={stat}>
            <small>{l}</small>
            <strong>{v}</strong>
          </div>
        ))}
      </div>
      {grouped.length ? (
        grouped.map(({ category, items }) => (
          <section key={category} style={{ marginBottom: 28 }}>
            <h2 style={categoryTitle}>
              {CATEGORY_LABEL[category] ?? category}
            </h2>
            <div style={{ overflowX: "auto" }}>
              <table style={table}>
                <thead>
                  <tr>
                    {[
                      "Nama",
                      "Jenis",
                      "Harga",
                      "Durasi",
                      "Itinerary",
                      "Status",
                      "",
                    ].map((h) => (
                      <th key={h} style={th}>
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {items.map((p) => (
                    <tr key={p.id} style={tr}>
                      <td style={td}>{p.name}</td>
                      <td style={td}>
                        {p.type ? (
                          <b style={badge}>
                            {p.type === "HAJJ" ? "Haji" : "Umrah"}
                          </b>
                        ) : (
                          <span style={{ color: "var(--color-warm-400)" }}>
                            —
                          </span>
                        )}
                      </td>
                      <td style={td}>{money(p.priceIdr)}</td>
                      <td style={td}>
                        {p.durationDays > 0 ? `${p.durationDays} hari` : "—"}
                      </td>
                      <td style={td}>
                        {p.itineraryDays.length ? (
                          `${p.itineraryDays.length} hari disusun`
                        ) : (
                          <span style={{ color: "var(--color-warm-400)" }}>
                            —
                          </span>
                        )}
                        {p.hotelIds.length > 0 && (
                          <span
                            style={{
                              display: "block",
                              fontSize: 11,
                              color: "var(--color-warm-400)",
                            }}
                          >
                            {p.hotelIds.length} hotel
                          </span>
                        )}
                      </td>
                      <td style={td}>
                        <span style={p.isActive ? activeBadge : inactiveBadge}>
                          {p.isActive ? "Aktif" : "Nonaktif"}
                        </span>
                      </td>
                      <td style={td}>
                        <button
                          style={ghost}
                          onClick={() => {
                            setEdit(p);
                            setOpen(true);
                          }}
                        >
                          <IconPencil size={15} />
                          Ubah
                        </button>
                        <RoleGate require={["owner", "admin"]}>
                          <button
                            style={{
                              ...ghost,
                              color: "var(--color-danger-600)",
                            }}
                            onClick={async () => {
                              if (window.confirm(`Hapus produk ${p.name}?`)) {
                                await productClient.deleteProduct({
                                  productId: p.id,
                                });
                                void load();
                              }
                            }}
                          >
                            <IconTrash size={15} />
                            Hapus
                          </button>
                        </RoleGate>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        ))
      ) : (
        <section style={empty}>
          <IconShoppingCart size={48} color="var(--color-warm-400)" />
          <h2 style={{ margin: 0 }}>Belum ada produk</h2>
          <button style={gold} onClick={() => setOpen(true)}>
            Tambah Produk
          </button>
        </section>
      )}
      <ProductFormDialog
        open={open}
        seasonId={seasonId}
        initial={edit}
        onClose={() => setOpen(false)}
        onSaved={(name) => {
          setNotice(`${name} berhasil disimpan.`);
          void load();
        }}
      />
    </main>
  );
}
const page: React.CSSProperties = {
  maxWidth: 1400,
  margin: "0 auto",
  padding: "32px 24px",
};
const header: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "flex-start",
  gap: 20,
  flexWrap: "wrap",
};
const eyebrow: React.CSSProperties = {
  color: "var(--color-gold-800)",
  fontSize: 11,
  fontWeight: 700,
  letterSpacing: ".08em",
  margin: "4px 0 8px",
};
const title: React.CSSProperties = {
  fontSize: "clamp(32px,5vw,48px)",
  fontWeight: 500,
  margin: 0,
};
const actions: React.CSSProperties = { display: "flex", gap: 10 };
const select: React.CSSProperties = {
  minHeight: 48,
  border: "1px solid var(--color-cream-400)",
  borderRadius: 8,
  padding: "0 12px",
  background: "#fff",
};
const emerald: React.CSSProperties = {
  minHeight: 48,
  border: 0,
  borderRadius: 8,
  padding: "0 18px",
  display: "inline-flex",
  alignItems: "center",
  gap: 8,
  background: "var(--color-emerald-900)",
  color: "var(--color-cream-100)",
  fontWeight: 700,
};
const gold: React.CSSProperties = {
  ...emerald,
  background: "var(--color-gold-500)",
  color: "#fff",
};
const stats: React.CSSProperties = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fit,minmax(190px,1fr))",
  gap: 14,
  margin: "24px 0",
};
const stat: React.CSSProperties = {
  display: "grid",
  gap: 6,
  padding: 18,
  background: "#fff",
  border: "1px solid var(--color-cream-400)",
  borderTop: "2px solid var(--color-gold-500)",
  borderRadius: 10,
};
const categoryTitle: React.CSSProperties = {
  fontSize: 18,
  margin: "0 0 12px",
  color: "var(--color-emerald-900)",
};
const table: React.CSSProperties = {
  width: "100%",
  borderCollapse: "collapse",
  background: "#fff",
};
const th: React.CSSProperties = {
  textAlign: "left",
  padding: 14,
  fontSize: 11,
  color: "var(--color-warm-400)",
  borderBottom: "1px solid var(--color-cream-300)",
};
const tr: React.CSSProperties = {
  borderBottom: "1px solid var(--color-cream-300)",
};
const td: React.CSSProperties = {
  padding: 14,
  color: "var(--color-warm-700)",
  whiteSpace: "nowrap",
};
const badge: React.CSSProperties = {
  padding: "4px 8px",
  borderRadius: 99,
  background: "var(--color-gold-50)",
  color: "var(--color-gold-800)",
  fontSize: 11,
};
const activeBadge: React.CSSProperties = {
  ...badge,
  background: "var(--color-emerald-50)",
  color: "var(--color-emerald-900)",
};
const inactiveBadge: React.CSSProperties = {
  ...badge,
  background: "var(--color-cream-200)",
  color: "var(--color-warm-500)",
};
const ghost: React.CSSProperties = {
  border: 0,
  background: "transparent",
  display: "inline-flex",
  alignItems: "center",
  gap: 4,
  color: "var(--color-warm-500)",
  marginRight: 8,
};
const empty: React.CSSProperties = {
  minHeight: 280,
  display: "grid",
  placeItems: "center",
  alignContent: "center",
  gap: 12,
  border: "1px dashed var(--color-cream-400)",
  borderRadius: 12,
};
