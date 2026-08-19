"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { IconArrowLeft, IconReplace } from "@tabler/icons-react";
import { Substitution } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient, seasonClient } from "@/lib/rpc";

export default function SubstitutionHistory({ initialSeasonId }: { initialSeasonId?: string }) {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState(initialSeasonId ?? "");
  const [substitutions, setSubstitutions] = useState<Substitution[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      if (!initialSeasonId) setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!seasonId) return;
    setLoading(true);
    pilgrimClient.listSubstitutions({ seasonId }).then((response) => setSubstitutions(response.substitutions)).catch(() => setNotice("Gagal memuat riwayat substitusi.")).finally(() => setLoading(false));
  }, [seasonId]);

  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";

  return <main style={page}>
    <Link href="/dashboard/pilgrims" style={{ color: "var(--color-gold-800)", display: "inline-flex", alignItems: "center", gap: 6 }}><IconArrowLeft size={16} />Kembali ke daftar jamaah</Link>
    <header style={header}>
      <div><p style={eyebrow}>JAMAAH / SUBSTITUSI</p><h1 style={title}>Riwayat Substitusi Jamaah</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{`${substitutions.length} substitusi${activeName ? ` · ${activeName}` : ""}`}</p></div>
      <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
        {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
      </select>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    <section style={{ marginTop: 20 }}>
      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : substitutions.length ? <div style={{ overflowX: "auto" }}>
        <table style={table}>
          <thead><tr>{["Jamaah Asal", "Jamaah Pengganti", "Alasan", "Tanggal"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
          <tbody>
            {substitutions.map((sub) => <tr key={sub.originalId} style={tr}>
              <td style={td}><Link href={`/dashboard/pilgrims/${sub.originalId}`} style={{ color: "var(--color-emerald-900)", fontWeight: 700 }}>{sub.originalName}</Link><span style={{ display: "block", color: "var(--color-warm-400)", fontSize: 12, fontFamily: "ui-monospace, monospace" }}>{sub.originalPassportNumber}</span></td>
              <td style={td}><Link href={`/dashboard/pilgrims/${sub.newId}`} style={{ color: "var(--color-emerald-900)", fontWeight: 700 }}>{sub.newName}</Link></td>
              <td style={{ ...td, color: "var(--color-warm-500)" }}>{sub.reason || "-"}</td>
              <td style={{ ...td, color: "var(--color-warm-400)" }}>{sub.substitutedAt?.toDate().toLocaleDateString("id-ID") ?? "-"}</td>
            </tr>)}
          </tbody>
        </table>
      </div> : <div style={empty}><IconReplace size={48} color="var(--color-warm-400)" /><p style={{ color: "var(--color-warm-500)" }}>Belum ada substitusi jamaah untuk musim ini.</p></div>}
    </section>
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap", marginTop: 20 };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 640 };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "14px 16px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "14px 16px", fontSize: 14 };
const empty: React.CSSProperties = { padding: "64px 24px", textAlign: "center", display: "grid", justifyItems: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
