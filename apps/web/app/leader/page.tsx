"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { IconClipboardCheck, IconMessageCircle, IconUsersGroup, IconWifiOff } from "@tabler/icons-react";
import { LeaderGroup } from "@hajj-saas/proto-gen/hajj/v1/groupleader_pb";
import { groupLeaderClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { useRegisterShellServiceWorker } from "@/lib/register-sw";

export default function LeaderGroupsPage() {
  useRegisterShellServiceWorker();
  const router = useRouter();
  const [groups, setGroups] = useState<LeaderGroup[]>([]);
  const [fromCache, setFromCache] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    cachedFetch("leader-groups", () => groupLeaderClient.listMyGroups({}))
      .then((result) => {
        if (result.data) setGroups(result.data.groups);
        setFromCache(result.fromCache);
        setLoaded(true);
      })
      .catch(() => setError("Gagal memuat data rombongan Anda."));
  }, []);

  // A leader with exactly one group has nothing useful to do on this list
  // screen — skip straight to the roster instead of an empty-feeling landing page.
  useEffect(() => {
    if (loaded && groups.length === 1 && groups[0]) router.replace(`/leader/${groups[0].id}`);
  }, [loaded, groups, router]);

  if (loaded && groups.length === 1) return null;

  return (
    <main style={page}>
      <p style={eyebrow}>KETUA ROMBONGAN</p>
      <h1 style={title}>Rombongan Saya</h1>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan data tersimpan — Anda sedang offline</p>}
      {error && <p style={{ color: "var(--color-danger-600)" }}>{error}</p>}
      {loaded && !groups.length && !error && (
        <section style={empty}>
          <IconUsersGroup size={44} color="var(--color-warm-400)" />
          <p style={{ color: "var(--color-warm-500)" }}>Anda belum ditugaskan sebagai ketua rombongan mana pun. Hubungi operator Anda untuk penugasan.</p>
        </section>
      )}
      <div style={list}>
        {groups.map((group) => (
          <article key={group.id} style={card}>
            <Link href={`/leader/${group.id}`} style={{ color: "inherit" }}>
              <h2 style={{ margin: 0, fontSize: 18 }}>{group.name}</h2>
              <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{group.pilgrimCount}/{group.capacity} jamaah</p>
              <div style={barTrack}><div style={{ ...barFill, width: `${group.capacity ? Math.min(100, (group.pilgrimCount / group.capacity) * 100) : 0}%` }} /></div>
            </Link>
            <div style={quickActions}>
              <Link href={`/leader/${group.id}/check-in`} style={quickAction}><IconClipboardCheck size={16} />Check-In</Link>
              <Link href={`/leader/${group.id}/chat`} style={quickAction}><IconMessageCircle size={16} />Chat</Link>
            </div>
          </article>
        ))}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 30, margin: "0 0 16px" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 16 };
const list: React.CSSProperties = { display: "grid", gap: 12 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16, display: "grid", gap: 10, color: "inherit" };
const barTrack: React.CSSProperties = { height: 6, borderRadius: 6, overflow: "hidden", background: "var(--color-emerald-100)" };
const barFill: React.CSSProperties = { height: "100%", background: "var(--color-emerald-900)" };
const quickActions: React.CSSProperties = { display: "flex", gap: 8, borderTop: "1px solid var(--color-cream-300)", paddingTop: 10 };
const quickAction: React.CSSProperties = { flex: 1, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 6, minHeight: 36, borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 600 };
const empty: React.CSSProperties = { display: "grid", justifyItems: "center", gap: 12, padding: "48px 16px", textAlign: "center", border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
