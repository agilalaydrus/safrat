"use client";

import { useEffect, useState } from "react";
import { IconMail, IconPhone, IconShieldCheck, IconUserCheck, IconUsersGroup } from "@tabler/icons-react";
import { Muttawwif } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { Agent } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { groupClient, agentClient } from "@/lib/rpc";
import AgentKycDialog from "@/components/agents/AgentKycDialog";

const KYC_LABEL: Record<string, string> = { UNVERIFIED: "Belum Diisi", PENDING_REVIEW: "Menunggu Verifikasi", VERIFIED: "Terverifikasi", REJECTED: "Ditolak" };
function kycBadge(status: string): React.CSSProperties {
  const map: Record<string, [string, string]> = { PENDING_REVIEW: ["var(--color-gold-50)", "var(--color-gold-800)"], VERIFIED: ["var(--color-emerald-50)", "var(--color-emerald-900)"], REJECTED: ["var(--color-danger-100)", "var(--color-danger-600)"] };
  const [bg, color] = map[status] ?? ["var(--color-cream-200)", "var(--color-warm-500)"];
  return { background: bg, color, borderRadius: 8, padding: "6px 10px" };
}

export default function MuttawwifDashboard() {
  const [muttawwif, setMuttawwif] = useState<Muttawwif[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [kycTarget, setKycTarget] = useState<Agent | undefined>();

  const refresh = () => groupClient.listMuttawwif({}).then((response) => setMuttawwif(response.muttawwif)).catch(() => setNotice("Gagal memuat data Muttawwif.")).finally(() => setLoading(false));

  useEffect(() => {
    refresh();
  }, []);

  async function openKyc(agentId: string) {
    try {
      const agent = await agentClient.getAgent({ agentId });
      setKycTarget(agent);
    } catch {
      setNotice("Gagal memuat data KYC.");
    }
  }

  const totalGroups = muttawwif.reduce((n, m) => n + m.groups.length, 0);
  const totalPilgrims = muttawwif.reduce((n, m) => n + m.groups.reduce((sum, g) => sum + g.pilgrimCount, 0), 0);

  return <main style={page}>
    <header style={header}>
      <p style={eyebrow}>OPERASIONAL / JAMAAH</p>
      <h1 style={title}>Muttawwif</h1>
      <p style={{ color: "var(--color-warm-500)", margin: 0 }}>{muttawwif.length} Muttawwif · {totalGroups} grup · {totalPilgrims} jamaah terkelola</p>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-danger-600)" }}>{notice}</p>}

    {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : muttawwif.length ? <div style={{ display: "grid", gap: 12, marginTop: 16 }}>
      {muttawwif.map((m) => <article key={m.userId} style={card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 12, flexWrap: "wrap", alignItems: "flex-start" }}>
          <div>
            <h3 style={{ margin: "0 0 6px", display: "flex", alignItems: "center", gap: 8 }}><IconUserCheck size={18} color="var(--color-emerald-800)" />{m.name}</h3>
            <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13, display: "flex", gap: 14, flexWrap: "wrap" }}>
              <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}><IconMail size={14} />{m.email}</span>
              {m.phone && <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}><IconPhone size={14} />{m.phone}</span>}
            </p>
          </div>
        </div>
        <div style={groupsRow}>
          {m.groups.map((g) => <span key={g.id} style={groupChip}><IconUsersGroup size={13} />{g.name} · {g.pilgrimCount}/{g.capacity}</span>)}
        </div>
        {m.agentId && <button style={{ ...kycBadge(m.kycStatus), border: 0, marginTop: 10, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, fontWeight: 600 }} onClick={() => openKyc(m.agentId)}><IconShieldCheck size={14} />KYC: {KYC_LABEL[m.kycStatus] ?? "Belum Diisi"}</button>}
      </article>)}
    </div> : <div style={empty}><IconUserCheck size={48} color="var(--color-warm-400)" /><p style={{ color: "var(--color-warm-500)" }}>Belum ada Muttawwif karena belum ada anggota yang ditugaskan sebagai ketua grup. Buka menu Grup, lalu pilih atau undang Muttawwif pada grup yang dituju.</p></div>}
    <AgentKycDialog open={!!kycTarget} agent={kycTarget} onClose={() => setKycTarget(undefined)} onUpdated={(updated) => { setKycTarget(updated); void refresh(); }} />
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { marginTop: 20 };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: "0 0 8px" };
const card: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16, background: "white" };
const groupsRow: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 8, marginTop: 12 };
const groupChip: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, fontWeight: 600, color: "var(--color-emerald-900)", background: "var(--color-emerald-50)", borderRadius: 999, padding: "4px 10px" };
const empty: React.CSSProperties = { padding: "48px 24px", textAlign: "center", display: "grid", justifyItems: "center", gap: 12 };
