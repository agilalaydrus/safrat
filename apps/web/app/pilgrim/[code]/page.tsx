"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { IconBed, IconBuilding, IconBus, IconUsersGroup, IconWheelchair, IconWifiOff } from "@tabler/icons-react";
import { PilgrimAppInfo } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_app_pb";
import { pilgrimAppClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";

export default function PilgrimHomePage() {
  const { code } = useParams<{ code: string }>();
  const [info, setInfo] = useState<PilgrimAppInfo>();
  const [fromCache, setFromCache] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    cachedFetch(`pilgrim-info:${code}`, () => pilgrimAppClient.getMyInfo({ appAccessCode: code }))
      .then((result) => {
        if (result.data) setInfo(result.data);
        setFromCache(result.fromCache);
      })
      .catch(() => setError("We couldn't find your access code. Please check the link and try again."));
  }, [code]);

  if (error) {
    return <main style={page}><p style={errorText}>{error}</p></main>;
  }
  if (!info) {
    return <main style={page}><p style={{ color: "var(--color-warm-400)" }}>Loading...</p></main>;
  }

  const movement = info.nextMovement;
  return (
    <main style={page}>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Showing saved info — you're offline</p>}
      <p style={eyebrow}>ASSALAMUALAIKUM</p>
      <h1 style={title}>{info.fullName}</h1>
      {info.requiresWheelchair && <p style={wheelchairNote}><IconWheelchair size={16} />Wheelchair assistance noted</p>}

      <div style={grid}>
        <div style={card}>
          <IconUsersGroup size={22} color="var(--color-emerald-800)" />
          <p style={cardLabel}>Group</p>
          <p style={cardValue}>{info.groupName || "Not assigned yet"}</p>
        </div>
        <div style={card}>
          <IconBuilding size={22} color="var(--color-emerald-800)" />
          <p style={cardLabel}>Hotel</p>
          <p style={cardValue}>{info.hotelName || "Not assigned yet"}</p>
        </div>
        <div style={card}>
          <IconBed size={22} color="var(--color-emerald-800)" />
          <p style={cardLabel}>Room</p>
          <p style={cardValue}>{info.roomNumber || "Not assigned yet"}</p>
        </div>
      </div>

      {movement && (
        <section style={nextCard}>
          <p style={eyebrow}><IconBus size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />NEXT MOVEMENT</p>
          <h2 style={{ margin: "4px 0 6px", fontSize: 22 }}>{movement.name}</h2>
          <p style={{ margin: 0, color: "var(--color-warm-500)" }}>{movement.origin} → {movement.destination}</p>
          <p style={{ margin: "6px 0 0", fontWeight: 700, color: "var(--color-gold-800)" }}>{movement.scheduledAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
        </section>
      )}
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 30, margin: "0 0 16px" };
const wheelchairNote: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 13, margin: "0 0 16px" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, display: "grid", gap: 4 };
const cardLabel: React.CSSProperties = { margin: 0, fontSize: 11, color: "var(--color-warm-400)" };
const cardValue: React.CSSProperties = { margin: 0, fontSize: 13, fontWeight: 700, color: "var(--color-warm-900)" };
const nextCard: React.CSSProperties = { marginTop: 20, background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", borderRadius: 14, padding: 18 };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 16 };
const errorText: React.CSSProperties = { color: "var(--color-danger-600)" };
