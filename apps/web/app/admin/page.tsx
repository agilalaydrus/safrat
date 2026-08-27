"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { IconBuildingStore, IconCurrencyDollar, IconShieldLock, IconAlertTriangle, IconTruckDelivery, IconReceipt2, IconUsers, IconId } from "@tabler/icons-react";
import { PlatformOperator, PlatformProduct } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";
import SupplierTab from "@/components/admin/SupplierTab";
import TransactionsTab from "@/components/admin/TransactionsTab";
import AccountsTab from "@/components/admin/AccountsTab";
import IdentityTab from "@/components/admin/IdentityTab";

const rupiah = (n: bigint) => `Rp${Number(n).toLocaleString("id-ID")}`;

export default function PlatformAdminPage() {
  // Four states, not two. "Still asking" must not look like "refused", or the
  // panel flashes an access error at the admin it belongs to on every load —
  // and a failed call must not look like one either. Conflating the two sent me
  // hunting a permissions bug that was never there, so the error now says what
  // actually happened.
  const [access, setAccess] = useState<"checking" | "granted" | "enrol" | "denied" | "error">("checking");
  const [failure, setFailure] = useState("");
  const [tab, setTab] = useState<"transactions" | "operators" | "costs" | "suppliers" | "accounts" | "identity">("transactions");

  useEffect(() => {
    platformClient.amIPlatformAdmin({})
      .then((r) => {
        if (!r.isPlatformAdmin) { setAccess("denied"); return; }
        // Granted, but platform access requires a second factor — this
        // identity can read every tenant's data, so it must not rest on a
        // password alone. Told apart from a refusal, because the fix is
        // different: enrol, rather than ask for access.
        setAccess(r.twoFactorEnabled ? "granted" : "enrol");
      })
      .catch((err: unknown) => {
        setFailure(err instanceof Error ? err.message : String(err));
        setAccess("error");
      });
  }, []);

  if (access === "checking") {
    return <main style={page}><p style={{ color: "var(--color-warm-500)" }}>Memeriksa akses...</p></main>;
  }
  if (access === "error") {
    return (
      <main style={page}>
        <section style={locked}>
          <IconAlertTriangle size={44} color="var(--color-danger-600)" />
          <h1 style={{ margin: 0, fontSize: 24, fontWeight: 500 }}>Gagal memeriksa akses</h1>
          <p style={{ color: "var(--color-warm-500)", margin: 0, maxWidth: 520 }}>
            Ini bukan penolakan akses — permintaannya sendiri yang gagal.
          </p>
          <code style={failureBox}>{failure}</code>
        </section>
      </main>
    );
  }
  if (access === "enrol") {
    return (
      <main style={page}>
        <section style={locked}>
          <IconShieldLock size={44} color="#b45309" />
          <h1 style={{ margin: 0, fontSize: 24, fontWeight: 500 }}>Aktifkan verifikasi dua langkah</h1>
          <p style={{ color: "var(--color-warm-500)", margin: 0, maxWidth: 460 }}>
            Panel ini membaca data seluruh travel, jadi tidak boleh bergantung pada kata sandi saja.
            Daftarkan aplikasi authenticator Anda terlebih dahulu.
          </p>
          <Link href="/keamanan" style={enrolButton}>Buka pengaturan keamanan</Link>
        </section>
      </main>
    );
  }
  if (access === "denied") {
    // Nothing is rendered for an account without access — the page reports
    // itself as not existing.
    //
    // This hides the panel, it does not protect it: the JavaScript bundle is
    // downloadable by anyone signed in, so the real control is the server
    // refusing every PlatformService call. Worth doing anyway, because a
    // surface nobody knows about is a surface nobody probes.
    notFound();
  }

  return (
    <main style={page}>
      <p style={eyebrow}>TAWAFIQHUB / ADMIN PLATFORM</p>
      <h1 style={title}>Panel Platform</h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>
        Pengaturan lintas travel yang sebelumnya hanya bisa lewat terminal.
      </p>
      <div className="gold-divider" />

      <div style={tabBar}>
        {([["transactions", "Transaksi", IconReceipt2], ["operators", "Travel", IconBuildingStore], ["costs", "Harga Modal", IconCurrencyDollar], ["suppliers", "Supplier", IconTruckDelivery], ["accounts", "Akun", IconUsers], ["identity", "Identitas", IconId]] as const).map(([id, label, Icon]) => (
          <button key={id} onClick={() => setTab(id)} style={tab === id ? tabActive : tabInactive}>
            <Icon size={17} />{label}
          </button>
        ))}
      </div>

      {tab === "transactions" && <TransactionsTab />}
      {tab === "operators" && <OperatorsTab />}
      {tab === "costs" && <SupplierCostsTab />}
      {tab === "suppliers" && <SupplierTab />}
      {tab === "accounts" && <AccountsTab />}
      {tab === "identity" && <IdentityTab />}
    </main>
  );
}

function OperatorsTab() {
  const [operators, setOperators] = useState<PlatformOperator[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    platformClient.listOperators({})
      .then((r) => setOperators(r.operators))
      .catch(() => setError("Gagal memuat daftar travel."))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p>;
  if (error) return <p style={{ color: "var(--color-danger-600)" }}>{error}</p>;

  return (
    <div style={{ overflowX: "auto" }}>
      <table style={table}>
        <thead><tr>{["Travel", "Paket", "Status", "Jamaah", "Produk", "Perlu Ditinjau"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
        <tbody>
          {operators.map((operator) => (
            <tr key={operator.id} style={tr}>
              <td style={td}>
                <strong>{operator.name}</strong>
                {operator.slug && <small style={{ display: "block", color: "var(--color-warm-400)" }}>{operator.slug}</small>}
              </td>
              <td style={td}>{operator.plan || "—"}</td>
              <td style={td}>{operator.subscriptionStatus || "—"}</td>
              <td style={td}>{operator.pilgrimCount}</td>
              <td style={td}>{operator.productCount}</td>
              <td style={td}>
                {/* Money that arrived and is waiting on somebody's decision.
                    Worth seeing from here, because it is invisible unless the
                    operator happens to look at their own orders page. */}
                {operator.heldOrderCount > 0
                  ? <span style={alert}><IconAlertTriangle size={13} />{operator.heldOrderCount}</span>
                  : <span style={{ color: "var(--color-warm-400)" }}>—</span>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {operators.length === 0 && <p style={{ color: "var(--color-warm-500)" }}>Belum ada travel terdaftar.</p>}
      {/* The list is bounded server-side. Saying so beats letting an admin
          assume a tenant does not exist because it fell off the end. */}
      {operators.length >= 100 && (
        <p style={{ color: "var(--color-warm-500)", fontSize: 13, marginTop: 12 }}>
          Menampilkan 100 travel pertama — yang punya transaksi perlu ditinjau didahulukan.
        </p>
      )}
    </div>
  );
}

function SupplierCostsTab() {
  const [products, setProducts] = useState<PlatformProduct[]>([]);
  const [includeCosted, setIncludeCosted] = useState(false);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    platformClient.listProductsNeedingCost({ includeCosted })
      .then((r) => setProducts(r.products))
      .catch(() => setError("Gagal memuat produk."))
      .finally(() => setLoading(false));
  }, [includeCosted]);

  useEffect(() => { load(); }, [load]);

  const save = async (product: PlatformProduct) => {
    const raw = (drafts[product.id] ?? "").replace(/\D/g, "");
    if (!raw) { setError("Masukkan harga modal."); return; }
    setSaving(product.id);
    setError("");
    try {
      await platformClient.setProductSupplierCost({ productId: product.id, supplierCostIdr: BigInt(raw) });
      setNotice(`Harga modal ${product.name} disimpan.`);
      setDrafts((d) => ({ ...d, [product.id]: "" }));
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan harga modal.");
    } finally {
      setSaving("");
    }
  };

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 14 }}>
        Produk tanpa harga modal dijual tanpa batas bawah — sistem tidak bisa menolak penjualan di bawah modal.
        Harga yang terbaca langsung dari supplier tidak bisa ditimpa manual.
      </p>
      <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 14 }}>
        <input type="checkbox" checked={includeCosted} onChange={(e) => setIncludeCosted(e.target.checked)} />
        Tampilkan juga yang sudah punya harga modal
      </label>

      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : products.length === 0 ? (
        <p style={{ color: "var(--color-warm-500)" }}>
          {includeCosted ? "Belum ada produk." : "Semua produk sudah punya harga modal."}
        </p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Produk", "Travel", "Harga Jual", "Harga Modal", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id} style={tr}>
                  <td style={td}>
                    <strong>{product.name}</strong>
                    <small style={{ display: "block", color: "var(--color-warm-400)" }}>{product.category}{product.seasonName ? ` · ${product.seasonName}` : ""}</small>
                  </td>
                  <td style={td}>{product.operatorName}</td>
                  <td style={td}>{rupiah(product.priceIdr)}</td>
                  <td style={td}>
                    {product.supplierCostSource === "OBSERVED" ? (
                      <span>
                        {rupiah(product.supplierCostIdr)}
                        <small style={{ display: "block", color: "var(--color-emerald-800)" }}>dari supplier</small>
                      </span>
                    ) : (
                      <input
                        inputMode="numeric"
                        placeholder={product.supplierCostSource === "MANUAL" ? String(product.supplierCostIdr) : "0"}
                        value={drafts[product.id] ?? ""}
                        onChange={(e) => setDrafts((d) => ({ ...d, [product.id]: e.target.value }))}
                        style={costInput}
                      />
                    )}
                  </td>
                  <td style={td}>
                    {product.supplierCostSource !== "OBSERVED" && (
                      <button style={saveButton} disabled={saving === product.id} onClick={() => save(product)}>
                        {saving === product.id ? "..." : "Simpan"}
                      </button>
                    )}
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

const page: React.CSSProperties = { maxWidth: 1100, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: "0 0 4px" };
const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "20px 0 22px", flexWrap: "wrap" };
const tabBase: React.CSSProperties = { minHeight: 44, borderRadius: 8, padding: "0 16px", display: "inline-flex", alignItems: "center", gap: 8, fontWeight: 600, fontSize: 14 };
const tabActive: React.CSSProperties = { ...tabBase, border: 0, background: "var(--color-emerald-800)", color: "#fff" };
const tabInactive: React.CSSProperties = { ...tabBase, border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-700)" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 14, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 14, color: "var(--color-warm-700)", verticalAlign: "top" };
const alert: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 5, padding: "3px 9px", borderRadius: 99, background: "var(--color-gold-50)", color: "#b45309", fontWeight: 700, fontSize: 12 };
const costInput: React.CSSProperties = { minHeight: 40, width: 140, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px" };
const saveButton: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const enrolButton: React.CSSProperties = { minHeight: 48, display: "inline-flex", alignItems: "center", padding: "0 22px", borderRadius: 8, background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, textDecoration: "none" };
const failureBox: React.CSSProperties = { display: "block", maxWidth: 520, padding: 12, background: "var(--color-cream-100)", borderRadius: 8, fontSize: 13, color: "var(--color-danger-600)", overflowWrap: "anywhere" };
const locked: React.CSSProperties = { minHeight: 320, display: "grid", placeItems: "center", alignContent: "center", gap: 12, textAlign: "center", border: "1px dashed var(--color-cream-400)", borderRadius: 12, padding: 32 };
