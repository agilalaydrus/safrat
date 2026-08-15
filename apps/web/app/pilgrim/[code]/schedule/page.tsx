"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { IconArrowRight, IconWifiOff } from "@tabler/icons-react";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { pilgrimAppClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";

export default function PilgrimSchedulePage() {
  const { code } = useParams<{ code: string }>();
  const [movements, setMovements] = useState<Movement[]>([]);
  const [fromCache, setFromCache] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    cachedFetch(`pilgrim-schedule:${code}`, () => pilgrimAppClient.listMySchedule({ appAccessCode: code })).then((result) => {
      if (result.data) setMovements(result.data.movements);
      setFromCache(result.fromCache);
      setLoaded(true);
    });
  }, [code]);

  return (
    <main style={page}>
      <p style={eyebrow}>PERJALANAN ANDA</p>
      <h1 style={title}>Jadwal</h1>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan jadwal tersimpan — Anda sedang offline</p>}
      {loaded && !movements.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada jadwal perjalanan mendatang.</p>}
      <div style={list}>
        {movements.map((movement) => (
          <article key={movement.id} style={card}>
            <p style={date}>{movement.scheduledAt?.toDate().toLocaleDateString("id-ID", { weekday: "short", day: "2-digit", month: "short" })}</p>
            <h2 style={{ margin: "4px 0 8px", fontSize: 18 }}>{movement.name}</h2>
            <p style={route}>{movement.origin}<IconArrowRight size={14} />{movement.destination}</p>
            <p style={time}>{movement.scheduledAt?.toDate().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</p>
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
const date: React.CSSProperties = { margin: 0, fontSize: 11, fontWeight: 700, color: "var(--color-warm-400)", textTransform: "uppercase" };
const route: React.CSSProperties = { margin: 0, display: "flex", alignItems: "center", gap: 8, color: "var(--color-warm-700)" };
const time: React.CSSProperties = { margin: "6px 0 0", fontWeight: 700, color: "var(--color-gold-800)" };
