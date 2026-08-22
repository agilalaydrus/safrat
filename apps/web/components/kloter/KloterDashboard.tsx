"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { IconPencil, IconPlane, IconPlus, IconTrash } from "@tabler/icons-react";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { kloterClient, seasonClient, transportClient } from "@/lib/rpc";
import KloterFormDialog from "./KloterFormDialog";
import { RoleGate } from "@/components/auth/RoleGate";

const TRIP_LEG_LABEL: Record<string, string> = { DEPARTURE: "Keberangkatan", RETURN: "Kepulangan" };

export default function KloterDashboard() {
  const router = useRouter();
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const [movements, setMovements] = useState<Movement[]>([]);
  const [open, setOpen] = useState(false);
  const [edit, setEdit] = useState<Kloter | undefined>();
  const [notice, setNotice] = useState("");

  const load = async (id = seasonId) => {
    if (!id) return;
    try {
      const [klotersResponse, movementsResponse] = await Promise.all([kloterClient.listKloters({ seasonId: id }), transportClient.listMovements({ seasonId: id })]);
      setKloters(klotersResponse.kloters);
      setMovements(movementsResponse.movements);
    } catch { setNotice("Gagal memuat data kloter."); }
  };

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => { setSeasons(r.seasons); setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? ""); }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);
  useEffect(() => { void load(); }, [seasonId]);

  // The full flight itinerary for a kloter — departure leg(s), any transit,
  // and the return leg(s) — is whatever FLIGHT movements are scoped to it,
  // sorted chronologically. Consecutive legs tagged the same trip_leg is
  // how a transit shows up, without a separate TRANSIT value.
  const flightLegsFor = (kloterId: string) => movements
    .filter((m) => m.kloterId === kloterId && m.mode === "FLIGHT")
    .sort((a, b) => (a.scheduledAt?.toDate().getTime() ?? 0) - (b.scheduledAt?.toDate().getTime() ?? 0));

  async function remove(kloter: Kloter) {
    if (!window.confirm(`Hapus kloter ${kloter.code}? Jamaah akan dilepas dari kloter ini, bukan dihapus.`)) return;
    await kloterClient.deleteKloter({ kloterId: kloter.id });
    void load();
  }

  return (
    <main style={page}>
      <header style={header}>
        <div><p style={eyebrow}>OPERASIONAL / KLOTER</p><h1 style={title}>Kloter Keberangkatan</h1></div>
        <div style={actions}>
          <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
            {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
          <button style={emerald} onClick={() => { setEdit(undefined); setOpen(true); }}><IconPlus size={18} />Tambah Kloter</button>
        </div>
      </header>
      <div className="gold-divider" />
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      {kloters.length ? (
        <div style={grid}>
          {kloters.map((k) => {
            const legs = flightLegsFor(k.id);
            return (
            <article style={{ ...card, cursor: "pointer" }} key={k.id} onClick={() => router.push(`/dashboard/kloter/${k.id}?seasonId=${seasonId}`)}>
              <div style={row}><h2 style={{ margin: 0, fontSize: 18 }}>{k.code}</h2><span style={capBadge}>{k.pilgrimCount}{k.capacity ? `/${k.capacity}` : ""}</span></div>
              <p style={meta}>{k.embarkation || "Embarkasi belum ditentukan"}</p>
              {k.departureDate && <p style={meta}>{k.departureDate.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "long", year: "numeric" })}</p>}
              {legs.length ? (
                <div style={{ display: "grid", gap: 6, marginTop: 4 }}>
                  {legs.map((leg) => (
                    <div key={leg.id} style={legRow}>
                      <span style={legBadge}>{TRIP_LEG_LABEL[leg.tripLeg] ?? (leg.tripLeg || "Penerbangan")}</span>
                      <span style={{ flex: 1, minWidth: 0 }}>
                        <span style={{ display: "flex", alignItems: "center", gap: 4 }}><IconPlane size={13} />{leg.origin} → {leg.destination}</span>
                        <span style={{ display: "block", fontSize: 11, color: "var(--color-warm-400)" }}>{[leg.airline, leg.flightNumber].filter(Boolean).join(" · ") || "Maskapai belum ditentukan"}</span>
                      </span>
                    </div>
                  ))}
                </div>
              ) : (
                <p style={meta}><IconPlane size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />{k.flightNumber || "Belum ada jadwal penerbangan — tambahkan di menu Transportasi"}</p>
              )}
              <div style={row} onClick={(e) => e.stopPropagation()}>
                <button style={ghost} onClick={() => { setEdit(k); setOpen(true); }}><IconPencil size={15} />Ubah</button>
                <RoleGate require={["owner", "admin"]}>
                  <button style={{ ...ghost, color: "var(--color-danger-600)" }} onClick={() => void remove(k)}><IconTrash size={15} />Hapus</button>
                </RoleGate>
              </div>
            </article>
            );
          })}
        </div>
      ) : (
        <section style={empty}>
          <IconPlane size={48} color="var(--color-warm-400)" />
          <h2 style={{ margin: 0 }}>Belum ada kloter</h2>
          <p style={{ margin: 0, color: "var(--color-warm-500)" }}>Buat kloter untuk mengelompokkan jamaah berdasarkan jadwal keberangkatan.</p>
          <button style={gold} onClick={() => setOpen(true)}>Tambah Kloter</button>
        </section>
      )}
      <KloterFormDialog open={open} seasonId={seasonId} initial={edit} onClose={() => setOpen(false)} onSaved={(code) => { setNotice(`Kloter ${code} berhasil disimpan.`); void load(); }} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 20, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const actions: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap" };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "#fff", maxWidth: 220 };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-emerald-900)", color: "var(--color-cream-100)", fontWeight: 700 };
const gold: React.CSSProperties = { ...emerald, background: "var(--color-gold-500)", color: "#fff" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(260px,1fr))", gap: 16 };
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 8 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 };
const capBadge: React.CSSProperties = { padding: "4px 8px", borderRadius: 99, background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontSize: 12 };
const meta: React.CSSProperties = { margin: 0, color: "var(--color-warm-500)", fontSize: 13 };
const legRow: React.CSSProperties = { display: "flex", gap: 8, alignItems: "flex-start", padding: "6px 8px", background: "var(--color-cream-100)", borderRadius: 8, fontSize: 13 };
const legBadge: React.CSSProperties = { flexShrink: 0, padding: "3px 8px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 10, fontWeight: 700, whiteSpace: "nowrap" };
const ghost: React.CSSProperties = { border: 0, background: "transparent", display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-warm-500)", marginRight: 8 };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12, textAlign: "center", padding: 24 };
