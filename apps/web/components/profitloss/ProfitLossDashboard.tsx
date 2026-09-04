"use client";

import { useEffect, useState } from "react";
import { IconDownload } from "@tabler/icons-react";
import type { AgentFigure, BranchFigure, PeriodFigure } from "@hajj-saas/proto-gen/hajj/v1/profitloss_pb";
import { profitLossClient } from "@/lib/rpc";
import { MethodologyNote } from "@/components/ui/MethodologyNote";

const rupiah = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));
const pct = (n: number) => `${n.toFixed(1)}%`;
const MONTHS = [3, 5, 12] as const;

function csvCell(value: string): string {
  return /[",\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}

export default function ProfitLossDashboard() {
  const [months, setMonths] = useState<number>(5);
  const [periods, setPeriods] = useState<PeriodFigure[]>([]);
  const [windowTotal, setWindowTotal] = useState<PeriodFigure>();
  const [branches, setBranches] = useState<BranchFigure[]>([]);
  const [agents, setAgents] = useState<AgentFigure[]>([]);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    setLoading(true);
    profitLossClient.getProfitLossReport({ months })
      .then((r) => { setPeriods(r.periods); setWindowTotal(r.windowTotal); setBranches(r.branches); setAgents(r.agents); })
      .catch(() => setNotice("Gagal memuat laporan laba rugi."))
      .finally(() => setLoading(false));
  }, [months]);

  const exportCsv = async () => {
    setExporting(true);
    setNotice("");
    try {
      const header = ["Tanggal Lunas", "Jamaah", "Produk", "Cabang", "Agen", "Kuantitas", "Pendapatan (Rp)", "Biaya Diketahui", "Biaya (Rp)", "Fee Platform (Rp)", "Komisi Agen (Rp)", "Laba Bersih (Rp)"];
      const lines = [header.join(",")];
      // Streamed from the server one row at a time — this loop only ever
      // holds the rows already received, never the whole export at once.
      for await (const row of profitLossClient.streamProfitLossExport({ months })) {
        lines.push([
          row.paidAt?.toDate().toISOString().slice(0, 10) ?? "",
          csvCell(row.pilgrimName), csvCell(row.productName), csvCell(row.branchName), csvCell(row.agentName || "-"),
          String(row.quantity), String(row.revenueIdr), row.costKnown ? "ya" : "tidak", String(row.costIdr),
          String(row.platformAmountIdr), String(row.agentCommissionIdr), String(row.netProfitIdr),
        ].join(","));
      }
      const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url; a.download = `laba-rugi-${months}bulan.csv`;
      document.body.appendChild(a); a.click(); document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal mengekspor laporan.");
    } finally {
      setExporting(false);
    }
  };

  return (
    <main style={page}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end", flexWrap: "wrap", gap: 12 }}>
        <div>
          <p style={eyebrow}>KEUANGAN</p>
          <h1 style={{ margin: 0, fontSize: 28 }}>Laba Rugi</h1>
          <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Seluruh operator, lintas musim — {windowTotal?.periodLabel ?? "..."}</p>
        </div>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <select value={months} onChange={(e) => setMonths(Number(e.target.value))} style={select}>
            {MONTHS.map((m) => <option key={m} value={m}>{m} bulan terakhir</option>)}
          </select>
          <button type="button" onClick={() => void exportCsv()} disabled={exporting} style={primaryBtn}>
            <IconDownload size={14} /> {exporting ? "Mengekspor..." : "Ekspor CSV"}
          </button>
        </div>
      </header>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      {loading ? <p style={{ color: "var(--color-warm-400)" }}>Memuat...</p> : windowTotal && (
        <>
          <section style={statGrid}>
            <div style={statCard}><span style={statLabel}>Pendapatan</span><strong style={statValue}>{rupiah(windowTotal.revenueIdr)}</strong></div>
            <div style={statCard}><span style={statLabel}>Laba Kotor</span><strong style={statValue}>{rupiah(windowTotal.grossProfitIdr)}</strong><span style={statSub}>{pct(windowTotal.grossMarginPct)} margin</span></div>
            <div style={statCard}><span style={statLabel}>Laba Bersih</span><strong style={{ ...statValue, color: "var(--color-emerald-900)" }}>{rupiah(windowTotal.netProfitIdr)}</strong></div>
            <div style={statCard}><span style={statLabel}>Laba per Unit</span><strong style={statValue}>{rupiah(windowTotal.netProfitPerUnitIdr)}</strong><span style={statSub}>{windowTotal.unitCount} unit terjual</span></div>
          </section>

          <MethodologyNote
            summary="Laba bersih = pendapatan − fee platform − komisi agen − biaya pokok yang diketahui. Bukan pendapatan dikurangi biaya saja — itu akan menghitung bagian platform dan agen seolah milik operator."
            points={[
              windowTotal.ordersMissingCost > 0
                ? `${windowTotal.ordersMissingCost} pesanan (${rupiah(windowTotal.revenueMissingCostIdr)} pendapatan) memakai produk tanpa harga pokok tercatat — biayanya tidak ikut dihitung, sehingga laba kotor/bersih di atas sedikit lebih tinggi dari kenyataan untuk pesanan itu.`
                : "Semua produk pada rentang ini memiliki harga pokok tercatat.",
              "Pesanan berstatus REFUNDED tidak dihitung sebagai pendapatan.",
              "Dihitung dari tabel pesanan langsung setiap kali dibuka — tidak ada angka yang disimpan atau di-cache.",
            ]}
          />

          <section style={card}>
            <h2 style={sectionTitle}>Tren Bulanan</h2>
            <div style={{ overflowX: "auto" }}>
              <table style={table}>
                <thead><tr>{["Bulan", "Pendapatan", "Laba Kotor", "Margin", "Laba Bersih", "Unit"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
                <tbody>
                  {periods.map((p, i) => (
                    <tr key={i}>
                      <td style={td}>{p.periodLabel}</td>
                      <td style={td}>{rupiah(p.revenueIdr)}</td>
                      <td style={td}>{rupiah(p.grossProfitIdr)}</td>
                      <td style={td}>{pct(p.grossMarginPct)}</td>
                      <td style={{ ...td, fontWeight: 700, color: "var(--color-emerald-900)" }}>{rupiah(p.netProfitIdr)}</td>
                      <td style={td}>{p.unitCount}</td>
                    </tr>
                  ))}
                  {periods.length === 0 && <tr><td colSpan={6} style={{ ...td, color: "var(--color-warm-400)" }}>Belum ada pesanan lunas pada rentang ini.</td></tr>}
                </tbody>
              </table>
            </div>
          </section>

          <section style={card}>
            <h2 style={sectionTitle}>Per Cabang — Target vs Realisasi</h2>
            <div style={{ overflowX: "auto" }}>
              <table style={table}>
                <thead><tr>{["Cabang", "Pendapatan", "Laba Bersih", "Kontribusi", "Target", "Capaian"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
                <tbody>
                  {branches.map((b) => (
                    <tr key={b.branchId || "pusat"}>
                      <td style={td}>{b.branchName}</td>
                      <td style={td}>{rupiah(b.revenueIdr)}</td>
                      <td style={td}>{rupiah(b.netProfitIdr)}</td>
                      <td style={td}>{pct(b.netProfitContributionPct)}</td>
                      <td style={td}>{b.targetRevenueIdr > 0n ? rupiah(b.targetRevenueIdr) : "—"}</td>
                      <td style={td}>{b.targetRevenueIdr > 0n ? pct(b.targetAchievedPct) : "—"}</td>
                    </tr>
                  ))}
                  {branches.length === 0 && <tr><td colSpan={6} style={{ ...td, color: "var(--color-warm-400)" }}>Belum ada pesanan lunas pada rentang ini.</td></tr>}
                </tbody>
              </table>
            </div>
          </section>

          {agents.length > 0 && (
            <section style={card}>
              <h2 style={sectionTitle}>Per Agen/Perujuk</h2>
              <div style={{ overflowX: "auto" }}>
                <table style={table}>
                  <thead><tr>{["Agen", "Pesanan", "Pendapatan", "Komisi"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
                  <tbody>
                    {agents.map((a) => (
                      <tr key={a.agentId}>
                        <td style={td}>{a.agentName}</td>
                        <td style={td}>{a.orderCount}</td>
                        <td style={td}>{rupiah(a.revenueIdr)}</td>
                        <td style={td}>{rupiah(a.commissionIdr)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        </>
      )}
    </main>
  );
}

const page: React.CSSProperties = { padding: "24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const select: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const primaryBtn: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12 };
const statCard: React.CSSProperties = { border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)", padding: "14px 16px", display: "grid", gap: 4 };
const statLabel: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-500)", textTransform: "uppercase", letterSpacing: ".05em" };
const statValue: React.CSSProperties = { fontSize: 22, fontWeight: 700 };
const statSub: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-400)" };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: "0 0 12px", fontSize: 16 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", fontSize: 13 };
const th: React.CSSProperties = { textAlign: "left", padding: "8px 10px", borderBottom: "2px solid var(--color-cream-300)", color: "var(--color-warm-500)", fontSize: 11, textTransform: "uppercase", letterSpacing: ".04em" };
const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid var(--color-cream-200)" };
