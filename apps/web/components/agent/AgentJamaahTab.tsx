"use client";

import { useEffect, useMemo, useState } from "react";
import { agentClient } from "@/lib/rpc";
import { AgentPilgrim } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";

const payBadge: Record<string, { label: string; color: string; bg: string }> = {
  PAID: { label: "Lunas", color: "#065f46", bg: "#d1fae5" },
  DP: { label: "DP", color: "#92400e", bg: "#fef3c7" },
  UNPAID: { label: "Belum Bayar", color: "#991b1b", bg: "#fee2e2" },
};

export default function AgentJamaahTab() {
  const [pilgrims, setPilgrims] = useState<AgentPilgrim[]>([]);
  const [seasonFilter, setSeasonFilter] = useState("ALL");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    agentClient.listMyPilgrims({}).then((r) => setPilgrims(r.pilgrims)).finally(() => setLoading(false));
  }, []);

  const seasons = useMemo(() => ["ALL", ...Array.from(new Set(pilgrims.map((p) => p.seasonName)))], [pilgrims]);
  const filtered = seasonFilter === "ALL" ? pilgrims : pilgrims.filter((p) => p.seasonName === seasonFilter);

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat data...</p>;

  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <p style={{ margin: 0, fontWeight: 600 }}>{filtered.length} jamaah</p>
        <select value={seasonFilter} onChange={(e) => setSeasonFilter(e.target.value)}
          style={{ padding: "8px 12px", borderRadius: 8, border: "1px solid var(--color-cream-500)", background: "var(--color-cream-200)", fontSize: 13 }}>
          {seasons.map((s) => <option key={s} value={s}>{s === "ALL" ? "Semua Musim" : s}</option>)}
        </select>
      </div>
      {filtered.length === 0 && <p style={{ color: "var(--color-warm-400)", fontSize: 14 }}>Belum ada jamaah referral.</p>}
      {filtered.length > 0 && <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, overflow: "hidden", overflowX: "auto" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13, minWidth: 560 }}>
          <thead>
            <tr style={{ borderBottom: "1px solid var(--color-cream-400)", background: "var(--color-cream-100)" }}>
              {["Nama", "Musim", "Pembayaran", "Dokumen", "Status"].map((h) => (
                <th key={h} style={{ textAlign: "left", padding: "10px 14px", fontWeight: 700, color: "var(--color-warm-600)" }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((p) => {
              const badge = payBadge[p.paymentStatus] ?? { label: p.paymentStatus, color: "#374151", bg: "#f3f4f6" };
              return (
                <tr key={p.id} style={{ borderBottom: "1px solid var(--color-cream-300)" }}>
                  <td style={{ padding: "12px 14px", fontWeight: 600 }}>{p.fullName}</td>
                  <td style={{ padding: "12px 14px", color: "var(--color-warm-500)" }}>{p.seasonName}</td>
                  <td style={{ padding: "12px 14px" }}>
                    <span style={{ padding: "3px 10px", borderRadius: 20, fontSize: 12, fontWeight: 700, color: badge.color, background: badge.bg }}>{badge.label}</span>
                  </td>
                  <td style={{ padding: "12px 14px" }}>{p.docsComplete ? "Lengkap" : "Belum lengkap"}</td>
                  <td style={{ padding: "12px 14px", color: p.pilgrimStatus === "CANCELLED" ? "var(--color-danger-600)" : "var(--color-emerald-700)", fontWeight: 600 }}>
                    {p.pilgrimStatus === "CANCELLED" ? "Dibatalkan" : "Aktif"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>}
    </div>
  );
}
