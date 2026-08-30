"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle, IconPlus, IconTrash } from "@tabler/icons-react";
import { VendorPayment, MonthlyProjectionEntry, CashFlowSummary } from "@hajj-saas/proto-gen/hajj/v1/cashflow_pb";
import { cashFlowClient, seasonClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

const CATEGORY_LABEL: Record<string, string> = { HOTEL: "Hotel", TRANSPORT: "Transportasi", CATERING: "Katering", VISA: "Visa", INSURANCE: "Asuransi", OTHER: "Lainnya" };
const STATUS_LABEL: Record<string, string> = { PENDING: "Menunggu", PAID: "Lunas", OVERDUE: "Terlambat", CANCELLED: "Dibatalkan" };
const STATUS_COLOR: Record<string, string> = { PENDING: "var(--color-gold-800)", PAID: "var(--color-emerald-800)", OVERDUE: "var(--color-danger-600)", CANCELLED: "var(--color-warm-400)" };

function formatIDR(value: bigint | number | string): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(value));
}

export default function CashFlowDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [summary, setSummary] = useState<CashFlowSummary>();
  const [months, setMonths] = useState<MonthlyProjectionEntry[]>([]);
  const [payments, setPayments] = useState<VendorPayment[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState({ vendorName: "", category: "HOTEL", amount: "", dueDate: "", description: "" });
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    setLoading(true);
    Promise.all([
      cashFlowClient.getCashFlowSummary({ seasonId }),
      cashFlowClient.getMonthlyProjection({ seasonId }),
      cashFlowClient.listVendorPayments({ seasonId }),
    ]).then(([summaryResponse, projectionResponse, paymentsResponse]) => {
      setSummary(summaryResponse);
      setMonths(projectionResponse.months);
      setPayments(paymentsResponse.payments);
    }).catch(() => setNotice("Gagal memuat data cash flow.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";
  const netPosition = summary ? Number(summary.netPositionIdr) : 0;
  const dueNext30 = summary ? Number(summary.dueNext30DaysIdr) : 0;
  const isDangerZone = summary ? netPosition < dueNext30 : false;
  const maxObligation = Math.max(1, ...months.map((m) => Number(m.vendorObligationsIdr)));

  const addPayment = async () => {
    if (!form.vendorName.trim() || !form.amount || !form.dueDate) { setNotice("Nama vendor, jumlah, dan tanggal jatuh tempo wajib diisi."); return; }
    setSaving(true);
    setNotice("");
    try {
      await cashFlowClient.createVendorPayment({
        seasonId, vendorName: form.vendorName.trim(), category: form.category,
        amountIdr: BigInt(Math.round(Number(form.amount))), dueDate: form.dueDate, description: form.description,
      });
      setForm({ vendorName: "", category: "HOTEL", amount: "", dueDate: "", description: "" });
      setFormOpen(false);
      refresh();
    } catch (error) {
      setNotice(`Gagal menyimpan: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSaving(false);
    }
  };

  const markPaid = async (payment: VendorPayment) => {
    try {
      await cashFlowClient.updateVendorPaymentStatus({ id: payment.id, status: "PAID" });
      refresh();
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  const remove = async (payment: VendorPayment) => {
    if (!window.confirm(`Hapus pembayaran ke ${payment.vendorName}?`)) return;
    try {
      await cashFlowClient.deleteVendorPayment({ id: payment.id });
      refresh();
    } catch (error) {
      setNotice(`Gagal menghapus: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / CASH FLOW</p><h1 style={title}>Cash Flow</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{payments.length} pembayaran vendor · {activeName}</p></div>
      <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
        <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
          {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
        </select>
        <RoleGate require={["owner", "admin"]}>
          <button onClick={() => setFormOpen((v) => !v)} style={emerald}><IconPlus size={18} />Tambah Pembayaran Vendor</button>
        </RoleGate>
      </div>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : summary && <>
      <section style={statGrid}>
        <div style={statCard}><span style={statLabel}>Total Terkumpul</span><strong style={{ ...statValue, color: "var(--color-emerald-900)" }}>{formatIDR(summary.totalCollectedIdr)}</strong></div>
        <div style={statCard}><span style={statLabel}>Total Komitmen Vendor</span><strong style={statValue}>{formatIDR(summary.totalCommittedIdr)}</strong></div>
        <div style={statCard}><span style={statLabel}>Posisi Bersih</span><strong style={{ ...statValue, color: netPosition >= 0 ? "var(--color-emerald-900)" : "var(--color-danger-600)" }}>{formatIDR(netPosition)}</strong></div>
        <div style={statCard}><span style={statLabel}>Jatuh Tempo 30 Hari</span><strong style={{ ...statValue, color: "var(--color-gold-800)" }}>{formatIDR(dueNext30)}</strong></div>
      </section>

      {isDangerZone && <div style={dangerBanner}>
        <IconAlertTriangle size={20} style={{ flexShrink: 0 }} />
        <span>Dana tidak cukup untuk pembayaran vendor dalam 30 hari ke depan. Defisit: <strong>{formatIDR(dueNext30 - netPosition)}</strong></span>
      </div>}

      {months.length > 0 && <section style={card}>
        <h2 style={{ margin: "0 0 16px" }}>Proyeksi Bulanan</h2>
        <div style={{ display: "flex", alignItems: "flex-end", gap: 12, height: 160, overflowX: "auto", padding: "0 4px" }}>
          {months.map((m) => {
            const value = Number(m.vendorObligationsIdr);
            const heightPct = Math.max(4, (value / maxObligation) * 100);
            return <div key={m.month} style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 6, minWidth: 64 }}>
              <span style={{ fontSize: 11, color: "var(--color-warm-500)" }}>{formatIDR(value)}</span>
              <div style={{ width: 36, height: `${heightPct}%`, minHeight: 6, background: "var(--color-gold-500)", borderRadius: 4 }} />
              <span style={{ fontSize: 11, color: "var(--color-warm-400)" }}>{m.month}</span>
            </div>;
          })}
        </div>
      </section>}

      {formOpen && <section style={card}>
        <h2 style={{ margin: "0 0 12px" }}>Pembayaran Vendor Baru</h2>
        <div style={formGrid}>
          <label style={field}>Nama Vendor<input value={form.vendorName} onChange={(e) => setForm((f) => ({ ...f, vendorName: e.target.value }))} style={input} /></label>
          <label style={field}>Kategori
            <select value={form.category} onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))} style={input}>
              {Object.entries(CATEGORY_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>
          <label style={field}>Jumlah (Rp)<input type="number" min={0} value={form.amount} onChange={(e) => setForm((f) => ({ ...f, amount: e.target.value }))} style={input} /></label>
          <label style={field}>Tanggal Jatuh Tempo<input type="date" value={form.dueDate} onChange={(e) => setForm((f) => ({ ...f, dueDate: e.target.value }))} style={input} /></label>
        </div>
        <label style={{ ...field, marginTop: 12 }}>Deskripsi<textarea value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} rows={2} style={{ ...input, minHeight: 60, resize: "vertical" }} /></label>
        <button disabled={saving} onClick={addPayment} style={{ ...emerald, marginTop: 12 }}>{saving ? "Menyimpan..." : "Simpan Pembayaran"}</button>
      </section>}

      <section style={card}>
        <h2 style={{ margin: "0 0 12px" }}>Daftar Pembayaran Vendor</h2>
        {payments.length ? <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Vendor", "Kategori", "Jumlah", "Jatuh Tempo", "Status", "Aksi"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {payments.map((payment) => <tr key={payment.id} style={tr}>
                <td style={td}><strong>{payment.vendorName}</strong>{payment.description && <span style={{ display: "block", color: "var(--color-warm-400)", fontSize: 12 }}>{payment.description}</span>}</td>
                <td style={td}>{CATEGORY_LABEL[payment.category] ?? payment.category}</td>
                <td style={td}>{formatIDR(payment.amountIdr)}</td>
                <td style={td}>{payment.dueDate}</td>
                <td style={td}><span style={{ color: STATUS_COLOR[payment.status] ?? "var(--color-warm-500)", fontWeight: 700, fontSize: 12 }}>{STATUS_LABEL[payment.status] ?? payment.status}</span></td>
                <td style={td}>
                  <RoleGate require={["owner", "admin"]}>
                    <div style={{ display: "flex", gap: 8 }}>
                      {payment.status !== "PAID" && payment.status !== "CANCELLED" && <button onClick={() => markPaid(payment)} style={markPaidBtn}>Tandai Lunas</button>}
                      <button onClick={() => remove(payment)} aria-label={`Hapus ${payment.vendorName}`} style={deleteBtn}><IconTrash size={14} /></button>
                    </div>
                  </RoleGate>
                </td>
              </tr>)}
            </tbody>
          </table>
        </div> : <p style={{ color: "var(--color-warm-500)" }}>Belum ada pembayaran vendor untuk musim ini.</p>}
      </section>
    </>}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12, margin: "20px 0" };
const statCard: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 12, background: "white", padding: "16px 20px", display: "grid", gap: 6 };
const statLabel: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-500)", textTransform: "uppercase", letterSpacing: ".05em" };
const statValue: React.CSSProperties = { fontSize: 22, fontWeight: 700 };
const dangerBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, background: "var(--color-danger-100)", border: "1px solid var(--color-danger-600)", borderRadius: 10, padding: "14px 18px", marginBottom: 20, color: "var(--color-danger-600)", fontWeight: 600, fontSize: 14 };
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginBottom: 20 };
const formGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 700 };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "12px 16px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "12px 16px", fontSize: 14 };
const markPaidBtn: React.CSSProperties = { minHeight: 32, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 12px", fontSize: 12 };
const deleteBtn: React.CSSProperties = { minHeight: 32, minWidth: 32, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
