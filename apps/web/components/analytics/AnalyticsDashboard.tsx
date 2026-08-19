"use client";

import { useEffect, useState } from "react";
import { SeasonAnalytics } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { seasonClient } from "@/lib/rpc";

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

  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";
  const total = analytics ? Number(analytics.totalPilgrims) : 0;
  const paidPct = analytics ? Math.round((Number(analytics.paidCount) / Math.max(total, 1)) * 100) : 0;
  const docsPct = analytics ? Math.round((Number(analytics.docsComplete) / Math.max(total, 1)) * 100) : 0;
  const checkedInPct = analytics ? Math.round((Number(analytics.checkedInCount) / Math.max(total, 1)) * 100) : 0;

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / ANALITIK</p><h1 style={title}>Dashboard Analitik</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>Metrik operasional per musim.</p></div>
      <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
        {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
      </select>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    {loading ? <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p> : analytics && <>
      <div style={grid}>
        {[
          { label: "Total Jamaah", value: analytics.totalPilgrims.toString(), color: "var(--color-emerald-900)" },
          { label: "Lunas", value: analytics.paidCount.toString(), color: "var(--color-emerald-700)" },
          { label: "DP", value: analytics.dpCount.toString(), color: "var(--color-gold-800)" },
          { label: "Belum Bayar", value: analytics.unpaidCount.toString(), color: "var(--color-danger-600)" },
          { label: "Dokumen Lengkap", value: analytics.docsComplete.toString(), color: "var(--color-emerald-700)" },
          { label: "Check-in Hotel", value: analytics.checkedInCount.toString(), color: "var(--color-emerald-800)" },
          { label: "Kamar Dialokasikan", value: analytics.roomsAllocated.toString(), color: "var(--color-warm-700)" },
          { label: "Kursi Ditugaskan", value: analytics.seatsAssigned.toString(), color: "var(--color-warm-700)" },
        ].map((card) => <div key={card.label} style={statCard}>
          <p style={statLabel}>{card.label}</p>
          <p style={{ ...statValue, color: card.color }}>{card.value}</p>
        </div>)}
      </div>

      <div style={progressCard}>
        <h3 style={{ margin: "0 0 20px", fontSize: 16, fontWeight: 700 }}>Progres Operasional</h3>
        <ProgressRow label="Pembayaran Lunas" pct={paidPct} color="var(--color-emerald-700)" />
        <ProgressRow label="Dokumen Lengkap" pct={docsPct} color="var(--color-gold-500)" />
        <ProgressRow label="Check-in Hotel" pct={checkedInPct} color="var(--color-emerald-800)" />
      </div>
    </>}
  </main>;
}

function ProgressRow({ label, pct, color }: { label: string; pct: number; color: string }) {
  return <div style={{ marginBottom: 16 }}>
    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, marginBottom: 4 }}>
      <span>{label}</span><span style={{ fontWeight: 700, color }}>{pct}%</span>
    </div>
    <div style={{ height: 8, background: "var(--color-cream-300)", borderRadius: 4, overflow: "hidden" }}>
      <div style={{ width: `${pct}%`, height: "100%", background: color, borderRadius: 4 }} />
    </div>
  </div>;
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: 16, margin: "24px 0 32px" };
const statCard: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: "20px 18px" };
const statLabel: React.CSSProperties = { margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 };
const statValue: React.CSSProperties = { margin: 0, fontSize: 28, fontWeight: 700 };
const progressCard: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24 };
