"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Timestamp } from "@bufbuild/protobuf";
import { IconBus, IconPlane, IconPlus, IconTemplate, IconTrain } from "@tabler/icons-react";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { seasonClient, transportClient } from "@/lib/rpc";
import MovementFormDialog from "./MovementFormDialog";

type MovementStep = { name: string; origin: string; destination: string; mode: "BUS" | "FLIGHT" | "TRAIN"; dayOffset: number; hour: number };

// Indonesian Hajj quotas are split into two gelombang (waves) by Kemenag, and
// which city a jamaah lands in first depends entirely on which wave they're
// in — this isn't a minor detail, it changes the whole itinerary shape:
// Gelombang I flies straight into Madinah and does ziarah there BEFORE the
// Arafah–Muzdalifah–Mina (Armuzna) rituals, then departs from Jeddah after
// Armuzna. Gelombang II flies into Jeddah/Makkah, does Armuzna first, then
// does Madinah ziarah AFTER and departs from Madinah's own airport — not
// Jeddah. Day offsets are compressed but ordered to match how Kemenag's 2025
// published schedule actually spaced these legs (~9 days Madinah ziarah
// before the Makkah transfer for gelombang I; ~2 weeks between the Mina
// return and the Makkah→Madinah move for gelombang II; ~41 days average
// total stay either way).
const HAJJ_GELOMBANG_1: MovementStep[] = [
  { name: "Kedatangan CGK → Madinah (AMM)", origin: "CGK", destination: "AMM", mode: "FLIGHT", dayOffset: 0, hour: 8 },
  { name: "Transfer Bandara Madinah → Hotel Madinah", origin: "Bandara AMM", destination: "Hotel Madinah", mode: "BUS", dayOffset: 0, hour: 12 },
  { name: "Transfer Madinah → Makkah", origin: "Madinah", destination: "Makkah", mode: "BUS", dayOffset: 9, hour: 9 },
  { name: "Makkah → Mina (Tarwiyah)", origin: "Makkah", destination: "Mina", mode: "BUS", dayOffset: 30, hour: 8 },
  { name: "Mina → Arafah (Wukuf)", origin: "Mina", destination: "Arafah", mode: "BUS", dayOffset: 31, hour: 6 },
  { name: "Arafah → Muzdalifah (Mabit)", origin: "Arafah", destination: "Muzdalifah", mode: "BUS", dayOffset: 31, hour: 18 },
  { name: "Muzdalifah → Mina (Tasyrik)", origin: "Muzdalifah", destination: "Mina", mode: "BUS", dayOffset: 32, hour: 7 },
  { name: "Mina → Makkah", origin: "Mina", destination: "Makkah", mode: "BUS", dayOffset: 35, hour: 10 },
  { name: "Transfer Hotel Makkah → Bandara Jeddah", origin: "Makkah", destination: "Bandara JED", mode: "BUS", dayOffset: 40, hour: 6 },
  { name: "Keberangkatan Jeddah (JED) → CGK", origin: "JED", destination: "CGK", mode: "FLIGHT", dayOffset: 40, hour: 10 },
];

const HAJJ_GELOMBANG_2: MovementStep[] = [
  { name: "Kedatangan CGK → Jeddah (JED)", origin: "CGK", destination: "JED", mode: "FLIGHT", dayOffset: 0, hour: 8 },
  { name: "Transfer Bandara Jeddah → Hotel Makkah", origin: "Bandara JED", destination: "Hotel Makkah", mode: "BUS", dayOffset: 0, hour: 12 },
  { name: "Makkah → Mina (Tarwiyah)", origin: "Makkah", destination: "Mina", mode: "BUS", dayOffset: 20, hour: 8 },
  { name: "Mina → Arafah (Wukuf)", origin: "Mina", destination: "Arafah", mode: "BUS", dayOffset: 21, hour: 6 },
  { name: "Arafah → Muzdalifah (Mabit)", origin: "Arafah", destination: "Muzdalifah", mode: "BUS", dayOffset: 21, hour: 18 },
  { name: "Muzdalifah → Mina (Tasyrik)", origin: "Muzdalifah", destination: "Mina", mode: "BUS", dayOffset: 22, hour: 7 },
  { name: "Mina → Makkah", origin: "Mina", destination: "Makkah", mode: "BUS", dayOffset: 25, hour: 10 },
  { name: "Transfer Makkah → Madinah", origin: "Makkah", destination: "Madinah", mode: "BUS", dayOffset: 39, hour: 9 },
  { name: "Keberangkatan Madinah (AMM) → CGK", origin: "AMM", destination: "CGK", mode: "FLIGHT", dayOffset: 47, hour: 10 },
];

// A real Umrah itinerary's shape doesn't change with trip length — fly into
// Jeddah, bus to Madinah for ziarah, take the Haramain high-speed train to
// Makkah (2h45m — the real, modern way most operators now move pilgrims
// between the two cities, not an all-day bus ride), then bus back to Jeddah
// airport to fly home. madinahNights is the only thing that shifts: where
// the Madinah→Makkah leg falls.
function umrahTemplate(totalDays: number, madinahNights: number): MovementStep[] {
  const lastDay = totalDays - 1;
  return [
    { name: "Kedatangan CGK → Jeddah (JED)", origin: "CGK", destination: "JED", mode: "FLIGHT", dayOffset: 0, hour: 8 },
    { name: "Transfer Bandara Jeddah → Hotel Madinah", origin: "Bandara JED", destination: "Hotel Madinah", mode: "BUS", dayOffset: 0, hour: 12 },
    { name: "Kereta Cepat Haramain Madinah → Makkah", origin: "Stasiun Madinah", destination: "Stasiun Makkah", mode: "TRAIN", dayOffset: madinahNights, hour: 9 },
    { name: "Transfer Hotel Makkah → Bandara Jeddah", origin: "Makkah", destination: "Bandara JED", mode: "BUS", dayOffset: lastDay, hour: 6 },
    { name: "Keberangkatan Jeddah (JED) → CGK", origin: "JED", destination: "CGK", mode: "FLIGHT", dayOffset: lastDay, hour: 10 },
  ];
}

const TEMPLATES: Record<string, { label: string; steps: MovementStep[] }> = {
  hajj1: { label: "Haji Gelombang I – Madinah Dulu (10 jadwal)", steps: HAJJ_GELOMBANG_1 },
  hajj2: { label: "Haji Gelombang II – Makkah Dulu (9 jadwal)", steps: HAJJ_GELOMBANG_2 },
  umrah9: { label: "Umroh 9 Hari (5 jadwal)", steps: umrahTemplate(9, 3) },
  umrah12: { label: "Umroh 12 Hari (5 jadwal)", steps: umrahTemplate(12, 4) },
  umrah17: { label: "Umroh 17 Hari (5 jadwal)", steps: umrahTemplate(17, 6) },
};

export default function TransportDashboard() {
  const [season, setSeason] = useState("");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [moves, setMoves] = useState<Movement[]>([]);
  const [open, setOpen] = useState(false);
  const [notice, setNotice] = useState("");
  const [working, setWorking] = useState(false);
  const [templateKey, setTemplateKey] = useState("hajj1");

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
      .catch(() => setNotice("Gagal memuat data jadwal perjalanan."));
  };

  useEffect(refresh, [season]);

  async function useTemplate() {
    if (!season || working) return;
    const template = TEMPLATES[templateKey];
    if (!template) return;
    setWorking(true);
    setNotice("");
    try {
      const start = new Date();
      start.setDate(start.getDate() + 1);
      for (const step of template.steps) {
        const scheduledAt = new Date(start);
        scheduledAt.setDate(start.getDate() + step.dayOffset);
        scheduledAt.setHours(step.hour, 0, 0, 0);
        await transportClient.createMovement({ seasonId: season, name: step.name, origin: step.origin, destination: step.destination, mode: step.mode, scheduledAt: Timestamp.fromDate(scheduledAt) });
      }
      setNotice(`${template.label} berhasil dibuat.`);
      refresh();
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal membuat templat jadwal.");
    } finally {
      setWorking(false);
    }
  }

  const groups = useMemo(() => Object.entries(moves.reduce<Record<string, Movement[]>>((result, movement) => {
    const date = movement.scheduledAt?.toDate().toLocaleDateString("id-ID") ?? "Belum dijadwalkan";
    (result[date] ??= []).push(movement);
    return result;
  }, {})), [moves]);

  return (
    <main style={page}>
      <header style={header}>
        <div>
          <p style={eyebrow}>OPERASIONAL / TRANSPORTASI</p>
          <h1 style={title}>Transportasi</h1>
          <p style={subtitle}>Koordinasikan jadwal perjalanan, kendaraan, dan manifest penumpang.</p>
        </div>
        <div style={actions}>
          <select value={season} onChange={(event) => setSeason(event.target.value)} style={input}>
            {seasons.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
          </select>
          <select value={templateKey} onChange={(event) => setTemplateKey(event.target.value)} style={{ ...input, maxWidth: 220 }}>
            {Object.entries(TEMPLATES).map(([key, tpl]) => <option key={key} value={key}>{tpl.label}</option>)}
          </select>
          <button disabled={!season || working} onClick={() => void useTemplate()} style={ghost}><IconTemplate size={18} />Gunakan Templat</button>
          <button disabled={!season} onClick={() => setOpen(true)} style={emerald}><IconPlus size={18} />Tambah Jadwal</button>
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
                  <strong style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>{modeIcon(movement.mode)}{movement.name}</strong>
                  <p>{movement.origin} → {movement.destination}</p>
                  <small>{movement.scheduledAt?.toDate().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</small>
                </div>
                <span style={badge(movement.status)}>{statusLabel(movement.status)}</span>
                <div style={bar}><div style={{ ...fill, width: `${movement.totalCapacity ? (movement.assignedCount / movement.totalCapacity) * 100 : 0}%` }} /></div>
                <small>{movement.assignedCount}/{movement.totalCapacity} kursi · {movement.vehicleCount} kendaraan</small>
              </Link>
            ))}
          </div>
        </section>
      )) : (
        <section style={empty}><IconBus size={48} /><h2>Belum ada jadwal perjalanan</h2><button onClick={() => setOpen(true)} style={gold}>Tambah Jadwal</button></section>
      )}
      <MovementFormDialog open={open} seasonId={season} onClose={() => setOpen(false)} onSaved={() => { setNotice("Jadwal berhasil ditambahkan"); refresh(); }} />
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

function modeIcon(mode: string) {
  if (mode === "FLIGHT") return <IconPlane size={16} color="var(--color-emerald-800)" />;
  if (mode === "TRAIN") return <IconTrain size={16} color="var(--color-emerald-800)" />;
  return <IconBus size={16} color="var(--color-emerald-800)" />;
}

function statusLabel(status: string): string {
  const map: Record<string, string> = { scheduled: "Terjadwal", arrived: "Tiba", departed: "Berangkat", cancelled: "Dibatalkan" };
  return map[status] ?? status;
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, flexWrap: "wrap" };
const actions: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", margin: 0 };
const subtitle: React.CSSProperties = { color: "var(--color-warm-500)" };
const input: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-emerald-900)", color: "var(--color-cream-100)", fontWeight: 700 };
const ghost: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "transparent", color: "var(--color-warm-700)", fontWeight: 700 };
const gold: React.CSSProperties = { ...emerald, background: "var(--color-gold-500)", color: "var(--color-warm-900)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(270px,1fr))", gap: 16 };
const card: React.CSSProperties = { display: "grid", gap: 12, background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, color: "var(--color-warm-900)", textDecoration: "none" };
const bar: React.CSSProperties = { height: 8, borderRadius: 8, overflow: "hidden", background: "var(--color-emerald-200)" };
const fill: React.CSSProperties = { height: "100%", background: "var(--color-emerald-900)" };
const empty: React.CSSProperties = { minHeight: 300, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12, background: "var(--color-cream-100)" };
const noticeStyle: React.CSSProperties = { color: "var(--color-gold-800)" };
