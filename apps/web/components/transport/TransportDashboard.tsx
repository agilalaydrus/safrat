"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { IconBus, IconPlus } from "@tabler/icons-react";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { seasonClient, transportClient } from "@/lib/rpc";
import MovementFormDialog from "./MovementFormDialog";

export default function TransportDashboard() {
  const [season, setSeason] = useState("");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [moves, setMoves] = useState<Movement[]>([]);
  const [open, setOpen] = useState(false);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeason(response.seasons.find((item) => item.isActive)?.id ?? response.seasons[0]?.id ?? "");
    });
  }, []);

  const refresh = () => {
    if (!season) return;
    transportClient.listMovements({ seasonId: season })
      .then((response) => setMoves(response.movements))
      .catch(() => setNotice("Unable to load movements."));
  };

  useEffect(refresh, [season]);

  const groups = useMemo(() => Object.entries(moves.reduce<Record<string, Movement[]>>((result, movement) => {
    const date = movement.scheduledAt?.toDate().toLocaleDateString() ?? "Unscheduled";
    (result[date] ??= []).push(movement);
    return result;
  }, {})), [moves]);

  return (
    <main style={page}>
      <header style={header}>
        <div>
          <p style={eyebrow}>OPERATIONS / TRANSPORT</p>
          <h1 style={title}>Transport</h1>
          <p style={subtitle}>Coordinate movements, vehicles, and passenger manifests.</p>
        </div>
        <div style={actions}>
          <select value={season} onChange={(event) => setSeason(event.target.value)} style={input}>
            {seasons.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
          <button disabled={!season} onClick={() => setOpen(true)} style={emerald}><IconPlus size={18} />Add Movement</button>
        </div>
      </header>
      <div className="gold-divider" />
      {notice && <p role="status" style={noticeStyle}>{notice}</p>}
      {groups.length ? groups.map(([day, items]) => (
        <section key={day}>
          <h2>{day}</h2>
          <div style={grid}>
            {items.map((movement) => (
              <Link key={movement.id} href={`/dashboard/transport/${movement.id}`} style={card}>
                <div>
                  <strong>{movement.name}</strong>
                  <p>{movement.origin} → {movement.destination}</p>
                  <small>{movement.scheduledAt?.toDate().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</small>
                </div>
                <span style={badge(movement.status)}>{movement.status}</span>
                <div style={bar}><div style={{ ...fill, width: `${movement.totalCapacity ? (movement.assignedCount / movement.totalCapacity) * 100 : 0}%` }} /></div>
                <small>{movement.assignedCount}/{movement.totalCapacity} seats · {movement.vehicleCount} vehicles</small>
              </Link>
            ))}
          </div>
        </section>
      )) : (
        <section style={empty}><IconBus size={48} /><h2>No movements scheduled</h2><button onClick={() => setOpen(true)} style={gold}>Add Movement</button></section>
      )}
      <MovementFormDialog open={open} seasonId={season} onClose={() => setOpen(false)} onSaved={() => { setNotice("Movement added"); refresh(); }} />
    </main>
  );
}

function badge(status: string): React.CSSProperties {
  const map: Record<string, [string, string]> = {
    arrived: ["var(--color-emerald-50)", "var(--color-emerald-900)"],
    departed: ["var(--color-cream-200)", "var(--color-warm-500)"],
    cancelled: ["#fdf0f0", "var(--color-danger-600)"],
  };
  const [bg, color] = map[status] ?? ["var(--color-cream-300)", "var(--color-warm-500)"];
  return { justifySelf: "start", padding: "5px 10px", borderRadius: 99, background: bg, color, textTransform: "capitalize", fontSize: 12 };
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, flexWrap: "wrap" };
const actions: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", margin: 0 };
const subtitle: React.CSSProperties = { color: "var(--color-warm-500)" };
const input: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-emerald-900)", color: "var(--color-cream-100)", fontWeight: 700 };
const gold: React.CSSProperties = { ...emerald, background: "var(--color-gold-500)", color: "var(--color-warm-900)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(270px,1fr))", gap: 16 };
const card: React.CSSProperties = { display: "grid", gap: 12, background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, color: "var(--color-warm-900)", textDecoration: "none" };
const bar: React.CSSProperties = { height: 8, borderRadius: 8, overflow: "hidden", background: "var(--color-emerald-200)" };
const fill: React.CSSProperties = { height: "100%", background: "var(--color-emerald-900)" };
const empty: React.CSSProperties = { minHeight: 300, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12, background: "var(--color-cream-100)" };
const noticeStyle: React.CSSProperties = { color: "var(--color-gold-800)" };
