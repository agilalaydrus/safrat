"use client";

import { useEffect, useState } from "react";
import { IconCheck, IconChevronDown, IconChevronUp } from "@tabler/icons-react";
import { RitualProgressItem } from "@hajj-saas/proto-gen/hajj/v1/ritual_pb";
import { ritualClient } from "@/lib/rpc";
import { useLeaderGroup } from "@/lib/leader-context";

export default function LeaderRitualsPage() {
  const { selectedGroupId: groupId, loaded } = useLeaderGroup();
  const [items, setItems] = useState<RitualProgressItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState("");
  const [expanded, setExpanded] = useState("");
  const [notice, setNotice] = useState("");

  const refresh = () => {
    if (!groupId) return;
    setLoading(true);
    ritualClient.getGroupRitualProgress({ groupId }).then((r) => setItems(r.items)).catch(() => setNotice("Gagal memuat progres ibadah.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [groupId]);

  async function bulkComplete(ritualId: string) {
    if (!groupId) return;
    if (!window.confirm("Tandai ritual ini selesai untuk SEMUA jamaah di grup Anda?")) return;
    setWorking(ritualId);
    try {
      await ritualClient.bulkCompleteRitual({ groupId, ritualId, notes: "" });
      refresh();
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal menyelesaikan ritual.");
    } finally {
      setWorking("");
    }
  }

  if (!loaded || loading) return <main style={page}><p style={{ color: "var(--color-warm-400)" }}>Memuat...</p></main>;

  return (
    <main style={page}>
      <p style={eyebrow}>IBADAH GRUP</p>
      {notice && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{notice}</p>}
      {!items.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada template ritual untuk musim ini — hubungi operator untuk mengaturnya.</p>}
      <div style={list}>
        {items.map((item) => {
          const done = item.completedCount >= item.totalPilgrims && item.totalPilgrims > 0;
          const isOpen = expanded === item.ritualId;
          return (
            <article key={item.ritualId} style={card}>
              <div style={row}>
                <div>
                  <strong>{item.name}</strong>
                  <p style={{ margin: "2px 0 0", fontSize: 13, color: "var(--color-warm-500)" }}>{item.completedCount} dari {item.totalPilgrims} jamaah selesai</p>
                </div>
                {done ? <span style={doneBadge}><IconCheck size={14} />Selesai</span> : (
                  <button disabled={working === item.ritualId} onClick={() => void bulkComplete(item.ritualId)} style={completeBtn}>
                    {working === item.ritualId ? "..." : "Semua Selesai"}
                  </button>
                )}
              </div>
              <div style={barTrack}><div style={{ ...barFill, width: `${item.totalPilgrims ? (item.completedCount / item.totalPilgrims) * 100 : 0}%` }} /></div>
              {item.incompletePilgrimNames.length > 0 && (
                <>
                  <button onClick={() => setExpanded(isOpen ? "" : item.ritualId)} style={toggleBtn}>
                    {isOpen ? <IconChevronUp size={14} /> : <IconChevronDown size={14} />}
                    {item.incompletePilgrimNames.length} jamaah belum selesai
                  </button>
                  {isOpen && <ul style={nameList}>{item.incompletePilgrimNames.map((n) => <li key={n}>{n}</li>)}</ul>}
                </>
              )}
            </article>
          );
        })}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 0" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 12px" };
const list: React.CSSProperties = { display: "grid", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 10 };
const doneBadge: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, fontWeight: 700, color: "var(--color-emerald-700)" };
const completeBtn: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, padding: "0 12px", background: "var(--color-emerald-900)", color: "#fff", fontSize: 12, fontWeight: 700, flexShrink: 0 };
const barTrack: React.CSSProperties = { height: 6, marginTop: 10, overflow: "hidden", borderRadius: 99, background: "var(--color-cream-300)" };
const barFill: React.CSSProperties = { height: "100%", background: "var(--color-gold-500)" };
const toggleBtn: React.CSSProperties = { display: "flex", alignItems: "center", gap: 4, marginTop: 8, border: 0, background: "transparent", color: "var(--color-warm-500)", fontSize: 12, padding: 0 };
const nameList: React.CSSProperties = { margin: "6px 0 0", paddingLeft: 18, fontSize: 13, color: "var(--color-warm-600)" };
