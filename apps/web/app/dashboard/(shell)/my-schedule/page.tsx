"use client";

import { useEffect, useState } from "react";
import { IconCalendarEvent } from "@tabler/icons-react";
import { KloterStaff } from "@hajj-saas/proto-gen/hajj/v1/staff_schedule_pb";
import { staffScheduleClient } from "@/lib/rpc";

const ROLE_LABEL: Record<string, string> = { COORDINATOR: "Koordinator", MEDICAL: "Medis", GUIDE: "Pemandu", ADMIN_SUPPORT: "Dukungan Admin" };

export default function MySchedulePage() {
  const [assignments, setAssignments] = useState<KloterStaff[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    staffScheduleClient.listMyAssignments({}).then((response) => setAssignments(response.assignments)).catch(() => setNotice("Gagal memuat jadwal Anda.")).finally(() => setLoading(false));
  }, []);

  return <main style={page}>
    <header>
      <p style={eyebrow}>TIM SAYA</p>
      <h1 style={title}>Jadwal Saya</h1>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <div style={{ display: "grid", gap: 12, marginTop: 20 }}>
      {assignments.map((item) => <div key={item.id} style={card}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <IconCalendarEvent size={20} color="var(--color-emerald-800)" />
          <div>
            <strong>{item.kloterName}</strong>
            <span style={{ display: "block", fontSize: 13, color: "var(--color-warm-500)" }}>{item.seasonName}</span>
          </div>
        </div>
        <div style={{ marginTop: 10, fontSize: 13, color: "var(--color-warm-500)" }}>
          <span style={badge}>{ROLE_LABEL[item.role] ?? item.role}</span>
          {item.duties && <p style={{ margin: "8px 0 0" }}>{item.duties}</p>}
        </div>
      </div>)}
      {!assignments.length && <p style={{ color: "var(--color-warm-500)" }}>Belum ada penugasan jadwal untuk Anda.</p>}
    </div>}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 800, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,40px)", fontWeight: 500, margin: 0 };
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 18 };
const badge: React.CSSProperties = { display: "inline-block", background: "var(--color-cream-300)", borderRadius: 999, padding: "2px 10px", fontWeight: 600, color: "var(--color-warm-900)" };
