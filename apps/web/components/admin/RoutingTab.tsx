"use client";

import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconPlugConnected, IconFileSearch } from "@tabler/icons-react";
import type { PlatformProductRoute, PlatformSupplier, SupplierLogEntry } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

const waktu = (d: Date) =>
  d.toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" });

export default function RoutingTab() {
  const [view, setView] = useState<"routes" | "logs">("routes");
  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={switcher}>
        <button onClick={() => setView("routes")} style={view === "routes" ? switchOn : switchOff}>
          <IconPlugConnected size={16} />Routing Produk
        </button>
        <button onClick={() => setView("logs")} style={view === "logs" ? switchOn : switchOff}>
          <IconFileSearch size={16} />Log Supplier
        </button>
      </div>
      {view === "routes" ? <Routes /> : <Logs />}
    </section>
  );
}

function Routes() {
  const [routes, setRoutes] = useState<PlatformProductRoute[]>([]);
  const [suppliers, setSuppliers] = useState<PlatformSupplier[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [editing, setEditing] = useState("");
  const [supplierId, setSupplierId] = useState("");
  const [sku, setSku] = useState("");
  const [active, setActive] = useState(true);
  const [saving, setSaving] = useState(false);

  const refresh = useCallback(() => {
    setLoading(true);
    Promise.all([platformClient.listProductRoutes({}), platformClient.listSuppliers({})])
      .then(([routeResponse, supplierResponse]) => {
        setRoutes(routeResponse.routes);
        setSuppliers(supplierResponse.suppliers);
      })
      .catch(() => setNotice("Gagal memuat routing produk."))
      .finally(() => setLoading(false));
  }, []);
  useEffect(refresh, [refresh]);

  // An empty id means the product has no route at all — the queue this screen
  // exists for, since an order for one comes back "Produk Belum di Atur Routing".
  const unrouted = useMemo(() => routes.filter((r) => r.id === ""), [routes]);
  const inactive = useMemo(() => routes.filter((r) => r.id !== "" && !r.isActive), [routes]);

  function startEditing(route: PlatformProductRoute) {
    setEditing(route.productId);
    setSupplierId(route.supplierId);
    setSku(route.supplierSku);
    setActive(route.id === "" ? true : route.isActive);
    setNotice("");
  }

  async function save(productId: string) {
    setSaving(true);
    try {
      await platformClient.saveProductRoute({ productId, supplierId, supplierSku: sku.trim(), isActive: active });
      setEditing("");
      setNotice("Routing disimpan.");
      refresh();
    } catch {
      setNotice("Gagal menyimpan routing. Periksa supplier dan SKU.");
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <p style={muted}>Memuat routing produk…</p>;

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <div>
        <h2 style={heading}>Routing Produk</h2>
        <p style={muted}>
          {routes.length} produk digital · {unrouted.length} belum dirutekan · {inactive.length} routing nonaktif
        </p>
      </div>

      {notice && <p role="status" style={noticeBox}>{notice}</p>}

      {unrouted.length > 0 && (
        <p style={warnBox}>
          <IconAlertTriangle size={16} />
          {unrouted.length} produk dijual tanpa supplier. Pesanan untuk produk ini gagal dengan
          &ldquo;Produk Belum di Atur Routing&rdquo; — uangnya masuk, barangnya tidak pernah terkirim.
        </p>
      )}

      {routes.length === 0 ? (
        <div style={emptyBox}>
          <p style={{ margin: 0, fontWeight: 700 }}>Belum ada produk digital</p>
          <p style={{ ...muted, marginTop: 6 }}>
            Routing hanya berlaku untuk pulsa dan paket data. Tambahkan produknya lebih dulu di tab Katalog.
          </p>
        </div>
      ) : (
        <table style={table}>
          <caption style={caption}>Lintas seluruh travel · yang belum dirutekan di atas</caption>
          <thead>
            <tr>{["Produk", "Kategori", "Supplier", "SKU supplier", "Status", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
          </thead>
          <tbody>
            {routes.map((route) => {
              const missing = route.id === "";
              return (
                <tr key={route.productId} style={missing ? { ...tr, background: "var(--color-warning-50)" } : tr}>
                  <td style={{ ...td, fontWeight: 700 }}>{route.productName}</td>
                  <td style={td}>{route.category}</td>
                  <td style={td}>
                    {editing === route.productId ? (
                      <select value={supplierId} onChange={(e) => setSupplierId(e.target.value)} style={input}>
                        <option value="">Pilih supplier…</option>
                        {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
                      </select>
                    ) : (
                      route.supplierName || <span style={{ color: "var(--color-warning-700)", fontWeight: 700 }}>Belum diatur</span>
                    )}
                  </td>
                  <td style={td}>
                    {editing === route.productId
                      ? <input value={sku} onChange={(e) => setSku(e.target.value)} style={input} placeholder="Kode produk di sisi supplier" />
                      : route.supplierSku || "—"}
                  </td>
                  <td style={td}>
                    {editing === route.productId ? (
                      <label style={checkRow}>
                        <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
                        Aktif
                      </label>
                    ) : missing ? "—" : route.isActive ? "Aktif" : "Nonaktif"}
                  </td>
                  <td style={td}>
                    {editing === route.productId ? (
                      <div style={{ display: "flex", gap: 6 }}>
                        <button
                          onClick={() => save(route.productId)}
                          disabled={!supplierId || !sku.trim() || saving}
                          style={{ ...primary, minHeight: 38, opacity: supplierId && sku.trim() && !saving ? 1 : 0.5 }}
                        >
                          Simpan
                        </button>
                        <button onClick={() => setEditing("")} style={ghost}>Batal</button>
                      </div>
                    ) : (
                      <button onClick={() => startEditing(route)} style={ghost}>
                        {missing ? "Atur routing" : "Ubah"}
                      </button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </div>
  );
}

function Logs() {
  const [logs, setLogs] = useState<SupplierLogEntry[]>([]);
  const [unmatchedOnly, setUnmatchedOnly] = useState(false);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [open, setOpen] = useState("");

  useEffect(() => {
    setLoading(true);
    platformClient
      .listSupplierLogs({ unmatchedOnly, limit: 200 })
      .then((response) => setLogs(response.logs))
      .catch(() => setNotice("Gagal memuat log supplier."))
      .finally(() => setLoading(false));
  }, [unmatchedOnly]);

  if (loading) return <p style={muted}>Memuat log supplier…</p>;

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", gap: 12, flexWrap: "wrap" }}>
        <div>
          <h2 style={heading}>Log Supplier</h2>
          <p style={muted}>
            {logs.length} kejadian terakhir{unmatchedOnly ? " · hanya yang tidak dikenali aturan" : ""}
          </p>
        </div>
        <label style={checkRow}>
          <input type="checkbox" checked={unmatchedOnly} onChange={(e) => setUnmatchedOnly(e.target.checked)} />
          Hanya yang tidak dikenali aturan
        </label>
      </div>

      {notice && <p role="status" style={noticeBox}>{notice}</p>}

      {logs.length === 0 ? (
        <div style={emptyBox}>
          <p style={{ margin: 0, fontWeight: 700 }}>
            {unmatchedOnly ? "Semua respons dikenali aturan" : "Belum ada log supplier"}
          </p>
          <p style={{ ...muted, marginTop: 6 }}>
            {unmatchedOnly
              ? "Tidak ada supplier yang menjawab dengan bentuk yang belum ada aturannya."
              : "Log terisi saat ada pesanan produk digital yang diteruskan ke supplier."}
          </p>
        </div>
      ) : (
        <table style={table}>
          <caption style={caption}>Lintas seluruh travel · klik baris untuk melihat isi permintaan &amp; respons</caption>
          <thead>
            <tr>{["Waktu", "Supplier", "Arah", "Endpoint", "HTTP", "Hasil", "Referensi", "Biaya"].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
          </thead>
          <tbody>
            {logs.map((log) => (
              <Fragment key={log.id}>
                <tr
                  style={{ ...tr, cursor: "pointer" }}
                  onClick={() => setOpen(open === log.id ? "" : log.id)}
                >
                  <td style={{ ...td, whiteSpace: "nowrap" }}>{log.createdAt ? waktu(log.createdAt.toDate()) : "—"}</td>
                  <td style={{ ...td, fontWeight: 700 }}>{log.supplierName}</td>
                  <td style={td}>{log.direction === "CALLBACK" ? "Balasan" : "Permintaan"}</td>
                  <td style={{ ...td, fontFamily: "ui-monospace, monospace", fontSize: 12 }}>{log.endpoint || "—"}</td>
                  <td style={td}>{log.httpStatus || "—"}</td>
                  <td style={td}>
                    {log.error
                      ? <span style={{ color: "var(--color-danger-600)", fontWeight: 700 }}>{log.outcome || "gagal"}</span>
                      : log.outcome || <span style={{ color: "var(--color-warning-700)", fontWeight: 700 }}>tidak dikenali</span>}
                  </td>
                  <td style={{ ...td, fontFamily: "ui-monospace, monospace", fontSize: 12 }}>{log.supplierReference || "—"}</td>
                  <td style={td}>{log.costIdr > 0n ? rupiah(log.costIdr) : "—"}</td>
                </tr>
                {open === log.id && (
                  <tr style={tr}>
                    <td colSpan={8} style={{ ...td, background: "var(--color-cream-100)" }}>
                      {log.error && <p style={{ margin: "0 0 10px", color: "var(--color-danger-600)" }}>{log.error}</p>}
                      <div style={{ display: "grid", gap: 10, gridTemplateColumns: "repeat(auto-fit,minmax(280px,1fr))" }}>
                        <Body label="Permintaan" value={log.requestBody} />
                        <Body label="Respons" value={log.responseBody} />
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function Body({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p style={{ margin: "0 0 4px", fontSize: 11, color: "var(--color-warm-400)", textTransform: "uppercase", letterSpacing: "0.06em" }}>{label}</p>
      <pre style={pre}>{value || "—"}</pre>
    </div>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const heading: React.CSSProperties = { margin: "0 0 4px", fontSize: 18 };
const input: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", fontFamily: "inherit", fontSize: 13, width: "100%" };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, fontSize: 13 };
const ghost: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const caption: React.CSSProperties = { captionSide: "top", textAlign: "left", padding: "0 0 8px", fontSize: 11, color: "var(--color-warm-400)", letterSpacing: "0.06em", textTransform: "uppercase" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const checkRow: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const noticeBox: React.CSSProperties = { margin: 0, padding: "10px 14px", borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)", fontSize: 13 };
const warnBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: 0, padding: "12px 16px", background: "var(--color-warning-50)", border: "1px solid var(--color-warning-200)", borderRadius: 8, color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const emptyBox: React.CSSProperties = { padding: "20px 18px", border: "1px dashed var(--color-cream-400)", borderRadius: 10, background: "#fff" };
const pre: React.CSSProperties = { margin: 0, padding: 10, background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, fontSize: 12, maxHeight: 220, overflow: "auto", whiteSpace: "pre-wrap", wordBreak: "break-word" };
const switcher: React.CSSProperties = { display: "flex", gap: 8, flexWrap: "wrap" };
const switchOn: React.CSSProperties = { minHeight: 40, borderRadius: 8, border: "1px solid var(--color-emerald-800)", background: "var(--color-emerald-800)", color: "#fff", padding: "0 14px", fontWeight: 700, fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6 };
const switchOff: React.CSSProperties = { minHeight: 40, borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-700)", padding: "0 14px", fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6 };
