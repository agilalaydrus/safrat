"use client";

import { useEffect, useState } from "react";
import { IconWifiOff } from "@tabler/icons-react";
import { Broadcast } from "@hajj-saas/proto-gen/hajj/v1/broadcast_pb";
import { pilgrimAppClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { usePilgrimCode } from "@/lib/pilgrim-context";

export default function PilgrimAnnouncementsPage() {
  const code = usePilgrimCode();
  const [broadcasts, setBroadcasts] = useState<Broadcast[]>([]);
  const [fromCache, setFromCache] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (!code) return;
    cachedFetch(`pilgrim-broadcasts:${code}`, () => pilgrimAppClient.listMyBroadcasts({ appAccessCode: code })).then((result) => {
      if (result.data) setBroadcasts(result.data.broadcasts);
      setFromCache(result.fromCache);
      setLoaded(true);
    });
  }, [code]);

  return (
    <main style={page}>
      <p style={eyebrow}>INFORMASI</p>
      <h1 style={title}>Pengumuman</h1>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan pengumuman tersimpan — Anda sedang offline</p>}
      {loaded && !broadcasts.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada pengumuman dari operator Anda.</p>}
      <div style={list}>
        {broadcasts.map((broadcast) => (
          <article key={broadcast.id} style={card}>
            <h2 style={{ margin: "0 0 6px", fontSize: 17 }}>{broadcast.title}</h2>
            <p style={{ margin: 0, whiteSpace: "pre-wrap", color: "var(--color-warm-700)" }}>{broadcast.body}</p>
            <p style={date}>{broadcast.createdAt?.toDate().toLocaleString("id-ID") ?? ""}</p>
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
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16 };
const date: React.CSSProperties = { margin: "10px 0 0", fontSize: 11, fontWeight: 700, color: "var(--color-warm-400)", textTransform: "uppercase" };
