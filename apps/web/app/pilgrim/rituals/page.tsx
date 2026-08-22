"use client";

import { useEffect, useState } from "react";
import { IconCheck } from "@tabler/icons-react";
import { PilgrimRitualStatus } from "@hajj-saas/proto-gen/hajj/v1/ritual_pb";
import { pilgrimAppClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { usePilgrimCode } from "@/lib/pilgrim-context";

export default function PilgrimRitualsPage() {
  const code = usePilgrimCode();
  const [rituals, setRituals] = useState<PilgrimRitualStatus[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (!code) return;
    cachedFetch(`pilgrim-rituals:${code}`, () => pilgrimAppClient.listMyRituals({ appAccessCode: code })).then((result) => {
      if (result.data) setRituals(result.data.rituals);
      setLoaded(true);
    });
  }, [code]);

  const completed = rituals.filter((r) => r.completed).length;
  const pct = rituals.length ? Math.round((completed / rituals.length) * 100) : 0;
  const pending = rituals.filter((r) => !r.completed);
  const done = rituals.filter((r) => r.completed);

  return (
    <main style={page}>
      <p style={eyebrow}>PERJALANAN IBADAH ANDA</p>
      <h1 style={title}>Ibadah Saya</h1>

      {loaded && rituals.length > 0 && (
        <section style={ringCard}>
          <div style={ring}>
            <svg width="96" height="96" viewBox="0 0 96 96">
              <circle cx="48" cy="48" r="42" fill="none" stroke="var(--color-cream-300)" strokeWidth="8" />
              <circle cx="48" cy="48" r="42" fill="none" stroke="var(--color-gold-500)" strokeWidth="8" strokeDasharray={`${(pct / 100) * 264} 264`} strokeLinecap="round" transform="rotate(-90 48 48)" />
            </svg>
            <span style={ringLabel}>{completed}/{rituals.length}</span>
          </div>
          <p style={{ margin: "8px 0 0", color: "var(--color-warm-500)" }}>{pct}% ritual telah diselesaikan</p>
        </section>
      )}

      {loaded && !rituals.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada template ritual untuk musim Anda.</p>}

      {pending.length > 0 && (
        <section style={{ marginTop: 20 }}>
          <p style={sectionLabel}>Belum Selesai</p>
          <div style={list}>
            {pending.map((r) => (
              <div key={r.ritualId} style={card}>
                <strong>{r.name}</strong>
                {r.description && <p style={{ margin: "2px 0 0", fontSize: 13, color: "var(--color-warm-500)" }}>{r.description}</p>}
              </div>
            ))}
          </div>
        </section>
      )}

      {done.length > 0 && (
        <section style={{ marginTop: 20 }}>
          <p style={sectionLabel}>Selesai</p>
          <div style={list}>
            {done.map((r) => (
              <div key={r.ritualId} style={{ ...card, background: "var(--color-emerald-50)" }}>
                <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  <IconCheck size={16} color="var(--color-emerald-700)" />
                  <strong>{r.name}</strong>
                </div>
                {r.completedByName && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>Diselesaikan oleh Muttawwif {r.completedByName}{r.completedAt && `, ${r.completedAt.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}`}</p>}
              </div>
            ))}
          </div>
        </section>
      )}
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 30, margin: "0 0 16px" };
const ringCard: React.CSSProperties = { display: "grid", justifyItems: "center", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const ring: React.CSSProperties = { position: "relative", width: 96, height: 96, display: "grid", placeItems: "center" };
const ringLabel: React.CSSProperties = { position: "absolute", fontSize: 18, fontWeight: 700 };
const sectionLabel: React.CSSProperties = { margin: "0 0 8px", fontSize: 12, fontWeight: 700, color: "var(--color-warm-400)", textTransform: "uppercase", letterSpacing: ".06em" };
const list: React.CSSProperties = { display: "grid", gap: 8 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: 12 };
