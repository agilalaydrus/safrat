"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { IconBuildingStore, IconCurrencyDollar, IconAlertTriangle, IconTruckDelivery, IconPackage, IconBuildingBank, IconReceipt2, IconUsers, IconId, IconAdjustments, IconPlugConnected, IconCalendarDollar, IconGauge, IconFilter, IconChartBar, IconHeartbeat, IconHistory, IconHeadset, IconSparkles, IconCheck, IconX, IconTrash, IconSpeakerphone } from "@tabler/icons-react";
import { PlatformOperator, PlatformProduct } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";
import CatalogueTab from "@/components/admin/CatalogueTab";
import TransfersTab from "@/components/admin/TransfersTab";
import SupplierTab from "@/components/admin/SupplierTab";
import TransactionsTab from "@/components/admin/TransactionsTab";
import AccountsTab from "@/components/admin/AccountsTab";
import IdentityTab from "@/components/admin/IdentityTab";
import PlanQuotaTab from "@/components/admin/PlanQuotaTab";
import RoutingTab from "@/components/admin/RoutingTab";
import SubscriptionsTab from "@/components/admin/SubscriptionsTab";
import UsageTab from "@/components/admin/UsageTab";
import FunnelTab from "@/components/admin/FunnelTab";
import AnalyticsTab from "@/components/admin/AnalyticsTab";
import HealthTab from "@/components/admin/HealthTab";
import AuditTab from "@/components/admin/AuditTab";
import SupportTab from "@/components/admin/SupportTab";
import AnnouncementsTab from "@/components/admin/AnnouncementsTab";
import PlatformGate from "@/components/admin/PlatformGate";

const rupiah = (n: bigint) => `Rp${Number(n).toLocaleString("id-ID")}`;

export default function PlatformAdminPage() {
  const [tab, setTab] = useState<"transactions" | "operators" | "catalogue" | "transfers" | "costs" | "suppliers" | "accounts" | "identity" | "quotas" | "routing" | "subscriptions" | "usage" | "funnel" | "analytics" | "health" | "audit" | "support" | "announcements">("transactions");
  const [supplierLogOrderId, setSupplierLogOrderId] = useState("");

  return (
    <PlatformGate>
      <main style={page}>
      <p style={eyebrow}>TAWAFIQHUB / ADMIN PLATFORM</p>
      <h1 style={title}>Panel Platform</h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>
        Pengaturan lintas travel yang sebelumnya hanya bisa lewat terminal.
      </p>
      <div className="gold-divider" />

      <div style={tabBar}>
        {([["transactions", "Transaksi", IconReceipt2], ["operators", "Travel", IconBuildingStore], ["catalogue", "Katalog", IconPackage], ["transfers", "Transfer", IconBuildingBank], ["costs", "Harga Modal", IconCurrencyDollar], ["suppliers", "Supplier", IconTruckDelivery], ["accounts", "Akun", IconUsers], ["identity", "Identitas", IconId], ["quotas", "Paket & Kuota", IconAdjustments], ["routing", "Routing & Log", IconPlugConnected], ["subscriptions", "Langganan", IconCalendarDollar], ["usage", "Pemakaian", IconGauge], ["funnel", "Corong", IconFilter], ["analytics", "Analitik", IconChartBar], ["health", "Kesehatan", IconHeartbeat], ["audit", "Audit", IconHistory], ["support", "Support", IconHeadset], ["announcements", "Pengumuman", IconSpeakerphone]] as const).map(([id, label, Icon]) => (
          <button key={id} onClick={() => setTab(id)} style={tab === id ? tabActive : tabInactive}>
            <Icon size={17} />{label}
          </button>
        ))}
      </div>

      {tab === "quotas" && <PlanQuotaTab />}
      {tab === "routing" && <RoutingTab initialOrderId={supplierLogOrderId} />}
      {tab === "subscriptions" && <SubscriptionsTab />}
      {tab === "usage" && <UsageTab />}
      {tab === "funnel" && <FunnelTab />}
      {tab === "analytics" && <AnalyticsTab />}
      {tab === "health" && <HealthTab />}
      {tab === "audit" && <AuditTab />}
      {tab === "support" && <SupportTab />}
      {tab === "announcements" && <AnnouncementsTab />}
      {tab === "transactions" && <TransactionsTab onOpenSupplierLog={(orderId) => {
        setSupplierLogOrderId(orderId);
        setTab("routing");
      }} />}
      {tab === "operators" && <OperatorsTab />}
      {tab === "catalogue" && <CatalogueTab />}
      {tab === "transfers" && <TransfersTab />}
      {tab === "costs" && <SupplierCostsTab />}
      {tab === "suppliers" && <SupplierTab />}
      {tab === "accounts" && <AccountsTab />}
      {tab === "identity" && <IdentityTab />}
    </main>
    </PlatformGate>
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

  // D4 (TUGAS-PANEL-SAAS.md, §7.2 DESAIN): "tenant yang mendaftar lalu tidak
  // pernah kembali adalah sinyal paling awal tentang onboarding yang rusak."
  // A plain filter of data already in this same list — no separate fetch.
  const sevenDaysAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
  const newTenants = operators
    .filter((operator) => (operator.createdAt?.toDate().getTime() ?? 0) >= sevenDaysAgo)
    .sort((a, b) => (b.createdAt?.toDate().getTime() ?? 0) - (a.createdAt?.toDate().getTime() ?? 0));

  // D7 (TUGAS-PANEL-SAAS.md, §7.3 DESAIN): tenants whose 90-day deletion
  // window has already opened, or opens within the next 14 days — so this
  // sits on the same screen as the queue that catches problems, not buried
  // in a tab nobody opens unless they already know to look.
  const fourteenDaysFromNow = Date.now() + 14 * 24 * 60 * 60 * 1000;
  const deletionQueue = operators
    .filter((operator) => {
      const at = operator.deletionEligibleAt?.toDate().getTime();
      return at !== undefined && at <= fourteenDaysFromNow;
    })
    .sort((a, b) => (a.deletionEligibleAt?.toDate().getTime() ?? 0) - (b.deletionEligibleAt?.toDate().getTime() ?? 0));

  return (
    <div style={{ display: "grid", gap: 20 }}>
      {deletionQueue.length > 0 && (
        <div className="tw-card" style={{ padding: 16, borderColor: "var(--color-danger-100)" }}>
          <p style={{ margin: "0 0 10px", fontWeight: 700, display: "flex", alignItems: "center", gap: 8, color: "var(--color-danger-600)" }}>
            <IconTrash size={16} />{deletionQueue.length} travel sudah atau akan bisa dihapus permanen
          </p>
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead>
                <tr>{["Travel", "Bisa dihapus mulai", "Status"].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
              </thead>
              <tbody>
                {deletionQueue.map((operator) => {
                  const eligibleAt = operator.deletionEligibleAt?.toDate();
                  const alreadyEligible = Boolean(eligibleAt && eligibleAt.getTime() <= Date.now());
                  return (
                    <tr key={operator.id} style={tr}>
                      <td style={td}><Link href={`/admin/tenant/${operator.id}`} style={tenantLink}>{operator.name}</Link></td>
                      <td style={td}>{eligibleAt?.toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}</td>
                      <td style={td}>
                        {alreadyEligible
                          ? <span style={alert}><IconAlertTriangle size={13} />Sudah bisa dihapus</span>
                          : <span style={{ color: "var(--color-warm-400)" }}>Menunggu masa tenggang</span>}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <p style={{ margin: "10px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>
            Penghapusan tetap memerlukan ekspor data yang sudah siap dan nama travel diketik ulang — buka halaman
            travelnya masing-masing untuk melakukannya.
          </p>
        </div>
      )}

      {newTenants.length > 0 && (
        <div className="tw-card" style={{ padding: 16 }}>
          <p style={{ margin: "0 0 10px", fontWeight: 700, display: "flex", alignItems: "center", gap: 8 }}>
            <IconSparkles size={16} />{newTenants.length} tenant baru 7 hari terakhir
          </p>
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead>
                <tr>{["Travel", "Daftar", "Musim", "Jamaah", "Login lagi"].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
              </thead>
              <tbody>
                {newTenants.map((operator) => (
                  <tr key={operator.id} style={tr}>
                    <td style={td}><Link href={`/admin/tenant/${operator.id}`} style={tenantLink}>{operator.name}</Link></td>
                    <td style={td}>{operator.createdAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short" })}</td>
                    <td style={td}>{completenessMark(operator.seasonCount > 0)}</td>
                    <td style={td}>{completenessMark(operator.pilgrimCount > 0)}</td>
                    <td style={td}>{completenessMark(operator.hasReturnedSinceSignup)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p style={{ margin: "10px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>
            &quot;Login lagi&quot; hanya perkiraan: tidak ada riwayat masuk yang tersimpan (setiap sesi baru menghapus
            sesi sebelumnya), jadi ini melihat apakah sesi yang aktif sekarang jauh lebih baru dari waktu pendaftaran.
          </p>
        </div>
      )}

      <div style={{ overflowX: "auto" }}>
      <table style={table}>
        <thead><tr>{["Travel", "Paket", "Status", "Jamaah", "Produk", "Perlu Ditinjau"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
        <tbody>
          {operators.map((operator) => (
            <tr key={operator.id} style={tr}>
              <td style={td}>
                {/* The name is the way in. Until now this list was the end of
                    the road: everything about one tenant lived in a different
                    tab, filtered by hand. */}
                <Link href={`/admin/tenant/${operator.id}`} style={tenantLink}>{operator.name}</Link>
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
    </div>
  );
}

function completenessMark(done: boolean) {
  return done
    ? <span style={{ color: "var(--color-emerald-800, #065f46)", display: "inline-flex", alignItems: "center" }}><IconCheck size={15} /></span>
    : <span style={{ color: "var(--color-warm-400)", display: "inline-flex", alignItems: "center" }}><IconX size={15} /></span>;
}

function SupplierCostsTab() {
  const [products, setProducts] = useState<PlatformProduct[]>([]);
  const [includeCosted, setIncludeCosted] = useState(false);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [baseDrafts, setBaseDrafts] = useState<Record<string, string>>({});
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

  const saveBase = async (product: PlatformProduct) => {
    const raw = (baseDrafts[product.id] ?? "").replace(/\D/g, "");
    if (!raw) { setError("Masukkan harga dasar."); return; }
    setSaving(product.id);
    setError("");
    try {
      await platformClient.setProductBasePrice({ productId: product.id, basePriceIdr: BigInt(raw) });
      setNotice(`Harga dasar ${product.name} disimpan.`);
      setBaseDrafts((d) => ({ ...d, [product.id]: "" }));
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan harga dasar.");
    } finally {
      setSaving("");
    }
  };

  return (
    <div style={{ display: "grid", gap: 14 }}>
      <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 14 }}>
        Harga modal adalah yang TawafiqHub bayar ke supplier. Harga dasar adalah yang
        TawafiqHub tagih ke travel — travel menambahkan markup sendiri di atasnya dan
        tidak bisa mengubah angka ini. Produk tanpa salah satu dari keduanya tidak bisa dijual.
      </p>
      <label style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 14 }}>
        <input type="checkbox" checked={includeCosted} onChange={(e) => setIncludeCosted(e.target.checked)} />
        Tampilkan juga yang sudah lengkap
      </label>

      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : products.length === 0 ? (
        <p style={{ color: "var(--color-warm-500)" }}>
          {includeCosted ? "Belum ada produk." : "Semua produk sudah punya harga modal dan harga dasar."}
        </p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Produk", "Travel", "Harga Modal", "Harga Dasar", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {products.map((product) => (
                <tr key={product.id} style={tr}>
                  <td style={td}>
                    <strong>{product.name}</strong>
                    <small style={{ display: "block", color: "var(--color-warm-400)" }}>{product.category}{product.seasonName ? ` · ${product.seasonName}` : ""}</small>
                  </td>
                  <td style={td}>{product.operatorName}</td>
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
                    <input
                      inputMode="numeric"
                      placeholder={product.basePriceSet ? String(product.basePriceIdr) : "belum diatur"}
                      value={baseDrafts[product.id] ?? ""}
                      onChange={(e) => setBaseDrafts((d) => ({ ...d, [product.id]: e.target.value }))}
                      style={costInput}
                    />
                    {!product.basePriceSet && (
                      <small style={{ display: "block", color: "#b45309", fontWeight: 700, fontSize: 12 }}>
                        belum bisa dijual
                      </small>
                    )}
                  </td>
                  <td style={td}>
                    <div style={{ display: "grid", gap: 6 }}>
                      {product.supplierCostSource !== "OBSERVED" && (
                        <button style={saveButton} disabled={saving === product.id} onClick={() => save(product)}>
                          {saving === product.id ? "..." : "Simpan modal"}
                        </button>
                      )}
                      <button style={saveButton} disabled={saving === product.id} onClick={() => saveBase(product)}>
                        {saving === product.id ? "..." : "Simpan dasar"}
                      </button>
                    </div>
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

const tenantLink: React.CSSProperties = { color: "var(--color-emerald-900)", fontWeight: 700, textDecoration: "none", borderBottom: "1px solid var(--color-cream-400)" };
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
