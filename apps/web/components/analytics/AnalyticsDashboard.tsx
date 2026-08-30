"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle, IconHeartbeat, IconMoonStars, IconSos } from "@tabler/icons-react";
import { SeasonAnalytics } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { seasonClient } from "@/lib/rpc";

const money = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

export default function AnalyticsDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [analytics, setAnalytics] = useState<SeasonAnalytics>();
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);

  useEffect(() => {
    if (!seasonId) return;
    setLoading(true);
    seasonClient.getSeasonAnalytics({ seasonId }).then(setAnalytics).catch(() => setNotice("Gagal memuat data analitik.")).finally(() => setLoading(false));
  }, [seasonId]);

  const total = analytics ? Number(analytics.totalPilgrims) : 0;
  const pct = (n: number) => Math.round((n / Math.max(total, 1)) * 100);

  return (
    <main style={page}>
      <header style={header}>
        <div><p style={eyebrow}>OPERASIONAL / ANALITIK</p><h1 style={title}>Dashboard Analitik</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{total} jamaah · {analytics?.orderCount ?? 0} pesanan · {seasons.find((season) => season.id === seasonId)?.name ?? "Pilih musim"}</p></div>
        <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
          {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>)}
        </select>
      </header>
      <div className="gold-divider" />
      {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
      {loading && <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p>}

      {!loading && analytics && (
        <>
          {(analytics.activeSosCount > 0 || analytics.healthBeratCount > 0 || analytics.unassignedGroupCount > 0 || analytics.unassignedKloterCount > 0) && (
            <div style={alertBar}>
              {analytics.activeSosCount > 0 && <span style={{ ...alertPill, background: "var(--color-danger-100)", color: "var(--color-danger-600)" }}><IconSos size={15} />{analytics.activeSosCount} SOS aktif</span>}
              {analytics.healthBeratCount > 0 && <span style={{ ...alertPill, background: "#fdece1", color: "#c2410c" }}><IconHeartbeat size={15} />{analytics.healthBeratCount} laporan kesehatan BERAT</span>}
              {analytics.unassignedGroupCount > 0 && <span style={{ ...alertPill, background: "var(--color-gold-50)", color: "var(--color-gold-800)" }}><IconAlertTriangle size={15} />{analytics.unassignedGroupCount} jamaah belum punya grup</span>}
              {analytics.unassignedKloterCount > 0 && <span style={{ ...alertPill, background: "var(--color-gold-50)", color: "var(--color-gold-800)" }}><IconAlertTriangle size={15} />{analytics.unassignedKloterCount} jamaah belum punya kloter</span>}
            </div>
          )}

          <div style={grid}>
            {[
              { label: "Total Jamaah", value: analytics.totalPilgrims.toString(), color: "var(--color-emerald-900)" },
              { label: "Lunas", value: analytics.paidCount.toString(), color: "var(--color-emerald-700)" },
              { label: "DP", value: analytics.dpCount.toString(), color: "var(--color-gold-800)" },
              { label: "Belum Bayar", value: analytics.unpaidCount.toString(), color: "var(--color-danger-600)" },
              { label: "Total Pendapatan", value: money(analytics.totalRevenueIdr), color: "var(--color-emerald-900)" },
              { label: "Total Pesanan", value: analytics.orderCount.toString(), color: "var(--color-warm-700)" },
              { label: "Kursi Roda", value: analytics.wheelchairCount.toString(), color: "var(--color-warm-700)" },
              { label: "Kamar Dialokasikan", value: analytics.roomsAllocated.toString(), color: "var(--color-warm-700)" },
            ].map((card) => (
              <div key={card.label} style={statCard}>
                <p style={statLabel}>{card.label}</p>
                <p style={{ ...statValue, color: card.color }}>{card.value}</p>
              </div>
            ))}
          </div>

          <div style={twoCol}>
            <div style={progressCard}>
              <h3 style={cardTitle}>Progres Operasional</h3>
              <ProgressRow label="Pembayaran Lunas" pct={pct(Number(analytics.paidCount))} color="var(--color-emerald-700)" />
              <ProgressRow label="Dokumen Lengkap" pct={pct(Number(analytics.docsComplete))} color="var(--color-gold-500)" />
              <ProgressRow label="Check-in Hotel" pct={pct(Number(analytics.checkedInCount))} color="var(--color-emerald-800)" />
              {analytics.ritualCompletionPct >= 0 && <ProgressRow label={<span><IconMoonStars size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} />Progres Ibadah (seluruh grup)</span>} pct={Math.round(analytics.ritualCompletionPct * 100)} color="var(--color-emerald-900)" />}
            </div>

            <div style={progressCard}>
              <h3 style={cardTitle}>Kesiapan Data Jamaah</h3>
              <p style={{ margin: "0 0 4px", fontSize: 13, color: "var(--color-warm-600)" }}>{analytics.unassignedGroupCount} dari {analytics.totalPilgrims} jamaah belum punya grup</p>
              <p style={{ margin: "0 0 4px", fontSize: 13, color: "var(--color-warm-600)" }}>{analytics.unassignedKloterCount} dari {analytics.totalPilgrims} jamaah belum punya kloter</p>
              <p style={{ margin: "0 0 4px", fontSize: 13, color: "var(--color-warm-600)" }}>{analytics.openHealthReportsCount} laporan kesehatan belum ditangani</p>
              <p style={{ margin: 0, fontSize: 13, color: "var(--color-warm-600)" }}>{analytics.activeSosCount} SOS masih aktif</p>
            </div>
          </div>

          {analytics.paymentTimeline.length > 0 && (
            <section style={{ ...progressCard, marginTop: 20 }}>
              <h3 style={cardTitle}>Tren Pendaftaran &amp; Pembayaran per Bulan</h3>
              <div style={{ display: "grid", gap: 10 }}>
                {analytics.paymentTimeline.map((m) => {
                  const monthTotal = Number(m.paidCount) + Number(m.dpCount) + Number(m.unpaidCount);
                  return (
                    <div key={m.month}>
                      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12, color: "var(--color-warm-500)", marginBottom: 4 }}>
                        <span>{m.month}</span><span>{monthTotal} jamaah mendaftar</span>
                      </div>
                      <div style={{ display: "flex", height: 10, borderRadius: 4, overflow: "hidden", background: "var(--color-cream-300)" }}>
                        {Number(m.paidCount) > 0 && <div style={{ width: `${(Number(m.paidCount) / Math.max(monthTotal, 1)) * 100}%`, background: "var(--color-emerald-700)" }} />}
                        {Number(m.dpCount) > 0 && <div style={{ width: `${(Number(m.dpCount) / Math.max(monthTotal, 1)) * 100}%`, background: "var(--color-gold-500)" }} />}
                        {Number(m.unpaidCount) > 0 && <div style={{ width: `${(Number(m.unpaidCount) / Math.max(monthTotal, 1)) * 100}%`, background: "var(--color-danger-600)" }} />}
                      </div>
                    </div>
                  );
                })}
              </div>
              <div style={{ display: "flex", gap: 16, marginTop: 12, fontSize: 12, color: "var(--color-warm-500)" }}>
                <Legend color="var(--color-emerald-700)" label="Lunas" />
                <Legend color="var(--color-gold-500)" label="DP" />
                <Legend color="var(--color-danger-600)" label="Belum Bayar" />
              </div>
            </section>
          )}

          <div style={twoCol}>
            {analytics.kloterFill.length > 0 && (
              <section style={progressCard}>
                <h3 style={cardTitle}>Tingkat Isi Kloter</h3>
                <div style={{ display: "grid", gap: 10 }}>
                  {analytics.kloterFill.map((k) => {
                    const fillPct = k.capacity > 0 ? Math.round((k.pilgrimCount / k.capacity) * 100) : 0;
                    return (
                      <div key={k.kloterCode}>
                        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, marginBottom: 4 }}>
                          <span>{k.kloterCode}</span><span style={{ fontWeight: 700 }}>{k.pilgrimCount}/{k.capacity || "-"}</span>
                        </div>
                        <div style={{ height: 8, background: "var(--color-cream-300)", borderRadius: 4, overflow: "hidden" }}>
                          <div style={{ width: `${Math.min(fillPct, 100)}%`, height: "100%", background: fillPct >= 100 ? "var(--color-danger-600)" : "var(--color-emerald-800)" }} />
                        </div>
                      </div>
                    );
                  })}
                </div>
              </section>
            )}

            {analytics.hotelOccupancy.length > 0 && (
              <section style={progressCard}>
                <h3 style={cardTitle}>Okupansi Hotel</h3>
                <div style={{ display: "grid", gap: 10 }}>
                  {analytics.hotelOccupancy.map((h) => {
                    const occPct = h.capacity > 0 ? Math.round((h.allocated / h.capacity) * 100) : 0;
                    return (
                      <div key={h.hotelName}>
                        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, marginBottom: 4 }}>
                          <span>{h.hotelName} <span style={{ color: "var(--color-warm-400)" }}>({h.city})</span></span><span style={{ fontWeight: 700 }}>{h.allocated}/{h.capacity || "-"}</span>
                        </div>
                        <div style={{ height: 8, background: "var(--color-cream-300)", borderRadius: 4, overflow: "hidden" }}>
                          <div style={{ width: `${Math.min(occPct, 100)}%`, height: "100%", background: "var(--color-gold-500)" }} />
                        </div>
                      </div>
                    );
                  })}
                </div>
              </section>
            )}
          </div>

          {analytics.agentStats.length > 0 && (
            <section style={{ ...progressCard, marginTop: 20 }}>
              <h3 style={cardTitle}>Kinerja Tour Leader / Agen Referral</h3>
              <div style={{ overflowX: "auto" }}>
                <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
                  <thead><tr>{["Nama", "Jamaah Dirujuk", "Tingkat Komisi"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
                  <tbody>
                    {[...analytics.agentStats].sort((a, b) => Number(b.pilgrimCount) - Number(a.pilgrimCount)).map((a) => (
                      <tr key={a.agentName} style={{ borderTop: "1px solid var(--color-cream-300)" }}>
                        <td style={td}>{a.agentName}</td>
                        <td style={td}>{a.pilgrimCount}</td>
                        <td style={td}>{a.commissionRate.toFixed(1)}%</td>
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

function ProgressRow({ label, pct, color }: { label: React.ReactNode; pct: number; color: string }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, marginBottom: 4 }}>
        <span>{label}</span><span style={{ fontWeight: 700, color }}>{pct}%</span>
      </div>
      <div style={{ height: 8, background: "var(--color-cream-300)", borderRadius: 4, overflow: "hidden" }}>
        <div style={{ width: `${Math.min(Math.max(pct, 0), 100)}%`, height: "100%", background: color, borderRadius: 4 }} />
      </div>
    </div>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}><span style={{ width: 10, height: 10, borderRadius: 2, background: color, display: "inline-block" }} />{label}</span>;
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "8px 0 32px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,40px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const alertBar: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap", margin: "16px 0" };
const alertPill: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, padding: "8px 14px", borderRadius: 99, fontSize: 13, fontWeight: 700 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: 16, margin: "24px 0 20px" };
const statCard: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "20px 18px" };
const statLabel: React.CSSProperties = { margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 };
const statValue: React.CSSProperties = { margin: 0, fontSize: 24, fontWeight: 700 };
const twoCol: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(340px,1fr))", gap: 16, marginTop: 20 };
const progressCard: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 };
const cardTitle: React.CSSProperties = { margin: "0 0 20px", fontSize: 16, fontWeight: 700 };
const th: React.CSSProperties = { textAlign: "left", padding: "10px 12px", fontSize: 11, color: "var(--color-warm-400)", background: "var(--color-cream-100)" };
const td: React.CSSProperties = { padding: "10px 12px", color: "var(--color-warm-700)" };
