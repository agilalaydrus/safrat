"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { IconBuilding, IconBus, IconGenderFemale, IconGenderMale, IconPackage, IconPlane, IconUserCheck, IconUsers, IconUsersGroup, IconWheelchair } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { kloterClient, pilgrimClient, transportClient, accommodationClient, groupClient, productClient } from "@/lib/rpc";

const TRIP_LEG_LABEL: Record<string, string> = { DEPARTURE: "Keberangkatan", RETURN: "Kepulangan" };
const MODE_ICON: Record<string, React.ComponentType<{ size?: number }>> = { FLIGHT: IconPlane, BUS: IconBus, TRAIN: IconBus };
const MODE_LABEL: Record<string, string> = { FLIGHT: "Penerbangan", BUS: "Bus", TRAIN: "Kereta" };
const CITY_LABEL: Record<string, string> = {
  INDONESIA: "Indonesia", TRANSIT: "Transit", MADINAH: "Madinah", MAKKAH: "Makkah",
  ARAFAH: "Arafah", MUZDALIFAH: "Muzdalifah", MINA: "Mina", DEPARTED: "Dalam Perjalanan Pulang",
};
const KLOTER_STEPS = [
  ["DRAFT", "Draft"], ["CONFIRMED", "Konfirmasi"], ["DEPARTED", "Catat Keberangkatan"],
  ["IN_SAUDI", "Tiba di Saudi"], ["DEPARTED_SAUDI", "Boarding Pulang"], ["COMPLETED", "Selesai"],
] as const;

function lastUpdateColor(lastUpdate?: Date): string {
  if (!lastUpdate) return "var(--color-warm-400)";
  const hours = (Date.now() - lastUpdate.getTime()) / 3_600_000;
  if (hours < 2) return "var(--color-emerald-700)";
  if (hours < 6) return "var(--color-gold-800)";
  return "var(--color-danger-600)";
}

export default function KloterDetail({ id }: { id: string }) {
  const searchParams = useSearchParams();
  const seasonId = searchParams.get("seasonId") ?? "";
  const [kloter, setKloter] = useState<Kloter>();
  const [roster, setRoster] = useState<Pilgrim[]>([]);
  const [movements, setMovements] = useState<Movement[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [linkedProducts, setLinkedProducts] = useState<Product[]>([]);
  const [rooms, setRooms] = useState<Record<string, { hotelName: string; roomNumber: string }[]>>({});
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [statusSaving, setStatusSaving] = useState(false);

  const load = () => {
    if (!seasonId) return;
    setLoading(true);
    Promise.all([
      kloterClient.listKloters({ seasonId }).then((r) => setKloter(r.kloters.find((k) => k.id === id))),
      pilgrimClient.listPilgrims({ seasonId, limit: 1000, offset: 0 }).then((r) => setRoster(r.pilgrims.filter((p) => p.kloterId === id))),
      transportClient.listMovements({ seasonId }).then((r) => setMovements(r.movements.filter((m) => m.kloterId === id))),
      groupClient.listGroupsByKloter({ kloterId: id }).then((r) => setGroups(r.groups)),
      productClient.listProducts({ seasonId }).then((r) => setLinkedProducts(r.products.filter((p) => p.defaultKloterId === id))).catch(() => {}),
      accommodationClient.listPilgrimRoomAssignments({ seasonId }).then((r) => {
        const map: Record<string, { hotelName: string; roomNumber: string }[]> = {};
        for (const a of r.assignments) (map[a.pilgrimId] ??= []).push({ hotelName: a.hotelName, roomNumber: a.roomNumber });
        setRooms(map);
      }),
    ]).catch(() => setNotice("Gagal memuat data kloter.")).finally(() => setLoading(false));
  };
  useEffect(load, [id, seasonId]);

  async function advanceStatus(status: string) {
    if (!kloter) return;
    if (!window.confirm(`Ubah status kloter ${kloter.code} menjadi "${KLOTER_STEPS.find(([s]) => s === status)?.[1] ?? status}"? Ini akan memperbarui status perjalanan seluruh jamaah di kloter ini secara otomatis.`)) return;
    setStatusSaving(true);
    try {
      const updated = await kloterClient.updateKloterStatus({ kloterId: kloter.id, status });
      setKloter(updated);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal memperbarui status kloter.");
    } finally {
      setStatusSaving(false);
    }
  }

  // Full itinerary for the kloter across the whole period — every movement
  // (flight, bus, train) scoped to it, not just the departure flight.
  const itinerary = useMemo(
    () => [...movements].sort((a, b) => (a.scheduledAt?.toDate().getTime() ?? 0) - (b.scheduledAt?.toDate().getTime() ?? 0)),
    [movements],
  );

  const hotelBreakdown = useMemo(() => {
    const map = new Map<string, number>();
    for (const p of roster) {
      const pilgrimRooms = rooms[p.id];
      if (!pilgrimRooms?.length) { map.set("Belum Ditempatkan", (map.get("Belum Ditempatkan") ?? 0) + 1); continue; }
      for (const room of pilgrimRooms) map.set(room.hotelName, (map.get(room.hotelName) ?? 0) + 1);
    }
    return Array.from(map.entries()).sort((a, b) => b[1] - a[1]);
  }, [roster, rooms]);

  const wheelchairCount = roster.filter((p) => p.requiresWheelchair).length;

  if (!seasonId) {
    return <main style={page}><p style={{ color: "var(--color-danger-600)" }}>Musim tidak diketahui — kembali ke daftar Kloter dan buka lagi dari sana.</p><Link href="/dashboard/kloter" style={{ color: "var(--color-gold-800)" }}>Kembali ke Kloter</Link></main>;
  }
  if (loading && !kloter) return <main style={page}><p style={{ color: "var(--color-warm-400)" }}>Memuat...</p></main>;
  if (!kloter) return <main style={page}><p style={{ color: "var(--color-danger-600)" }}>Kloter tidak ditemukan.</p><Link href="/dashboard/kloter" style={{ color: "var(--color-gold-800)" }}>Kembali ke Kloter</Link></main>;

  return (
    <main style={page}>
      <Link href="/dashboard/kloter" style={{ color: "var(--color-gold-800)" }}>Kembali ke daftar Kloter</Link>
      <header style={{ marginTop: 20 }}>
        <p style={eyebrow}>DETAIL KLOTER</p>
        <h1 style={{ margin: 0, fontSize: 40 }}>{kloter.code}</h1>
        <p style={{ color: "var(--color-warm-500)", margin: "4px 0 0" }}>{kloter.embarkation || "Embarkasi belum ditentukan"}{kloter.departureDate && ` · Keberangkatan ${kloter.departureDate.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "long", year: "numeric" })}`}</p>
      </header>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={sectionTitle}>Status Kloter</h2>
        <div style={stepper}>
          {KLOTER_STEPS.map(([status, label], i) => {
            const currentIdx = KLOTER_STEPS.findIndex(([s]) => s === kloter.status);
            const isCurrent = status === kloter.status;
            const isPast = i < currentIdx;
            return (
              <span key={status} style={{ ...stepBadge, ...(isCurrent ? stepCurrent : isPast ? stepPast : {}) }}>{label}</span>
            );
          })}
        </div>
        {(() => {
          const currentIdx = KLOTER_STEPS.findIndex(([s]) => s === kloter.status);
          const next = KLOTER_STEPS[currentIdx + 1];
          return (
            <div style={{ display: "flex", gap: 10, marginTop: 12, flexWrap: "wrap" }}>
              {next && <button disabled={statusSaving} onClick={() => void advanceStatus(next[0])} style={primaryBtn}>{statusSaving ? "Menyimpan..." : next[1]}</button>}
              {kloter.status === "CONFIRMED" && <button disabled={statusSaving} onClick={() => void advanceStatus("DRAFT")} style={ghostBtn}>Batalkan Konfirmasi</button>}
            </div>
          );
        })()}
      </section>

      <div style={statGrid}>
        <div style={statCard}><span style={statLabel}>Jamaah</span><strong style={statValue}>{roster.length}{kloter.capacity ? `/${kloter.capacity}` : ""}</strong></div>
        <div style={statCard}><span style={statLabel}>Jadwal Pergerakan</span><strong style={statValue}>{itinerary.length} <span className="tw-stat__unit">segmen</span></strong></div>
        <div style={statCard}><span style={statLabel}>Hotel</span><strong style={statValue}>{hotelBreakdown.filter(([label]) => label !== "Belum Ditempatkan").length}</strong></div>
        <div style={statCard}><span style={statLabel}>Kursi Roda</span><strong style={statValue}>{wheelchairCount} <span className="tw-stat__unit">jamaah</span></strong></div>
      </div>

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={sectionTitle}><IconPlane size={18} color="var(--color-emerald-800)" />Itinerary Lengkap (Keberangkatan, Transit &amp; Kepulangan)</h2>
        {itinerary.length ? <div style={{ display: "grid", gap: 8 }}>
          {itinerary.map((m) => {
            const Icon = MODE_ICON[m.mode] ?? IconBus;
            return (
              <div key={m.id} style={legRow}>
                <Icon size={16} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <strong>{m.name}</strong>
                    <span style={modeBadge}>{MODE_LABEL[m.mode] ?? m.mode}</span>
                    {m.tripLeg && <span style={legBadge}>{TRIP_LEG_LABEL[m.tripLeg] ?? m.tripLeg}</span>}
                  </div>
                  <p style={{ margin: "2px 0 0", fontSize: 13, color: "var(--color-warm-500)" }}>{m.origin} → {m.destination} · {m.scheduledAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" })}</p>
                  {m.mode === "FLIGHT" && (m.airline || m.flightNumber) && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>{[m.airline, m.flightNumber].filter(Boolean).join(" · ")}</p>}
                </div>
                <span style={statusBadge}>{m.status}</span>
              </div>
            );
          })}
        </div> : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jadwal pergerakan untuk kloter ini — tambahkan di menu Transportasi.</p>}
      </section>

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={sectionTitle}><IconUsersGroup size={18} color="var(--color-emerald-800)" />Grup ({groups.length})</h2>
        {groups.length ? <div style={{ display: "grid", gap: 8 }}>
          {groups.map((g) => (
            <Link key={g.id} href={`/dashboard/groups/${g.id}?seasonId=${seasonId}`} style={groupRow}>
              <div>
                <strong>{g.name}</strong>
                <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}><IconUserCheck size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} />{g.leaderName || "Belum ada Muttawwif"}</p>
              </div>
              <div style={{ textAlign: "right" }}>
                <span style={{ fontSize: 13, color: "var(--color-warm-700)" }}>{g.pilgrimCount}/{g.capacity} jamaah</span>
                <p style={{ margin: "2px 0 0", fontSize: 12, color: lastUpdateColor(g.lastUpdate?.toDate()) }}>
                  {CITY_LABEL[g.currentCity] ?? (g.currentCity || "Indonesia")}
                  {g.lastUpdate && ` · ${g.lastUpdate.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}`}
                </p>
              </div>
            </Link>
          ))}
        </div> : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada grup yang ditugaskan ke kloter ini.</p>}
      </section>

      {linkedProducts.length > 0 && (
        <section style={{ ...card, marginTop: 16 }}>
          <h2 style={sectionTitle}><IconPackage size={18} color="var(--color-emerald-800)" />Paket Terkait ({linkedProducts.length})</h2>
          <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>Jamaah yang membeli paket ini otomatis ditempatkan ke kloter ini.</p>
          <div style={{ display: "grid", gap: 10 }}>
            {linkedProducts.map((p) => (
              <div key={p.id} style={{ background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: 12 }}>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <strong>{p.name}</strong>
                  <Link href="/dashboard/products" style={{ fontSize: 12, color: "var(--color-gold-800)" }}>Kelola Produk</Link>
                </div>
                {p.itineraryDays.length > 0 && (
                  <div style={{ display: "grid", gap: 4, marginTop: 8 }}>
                    {p.itineraryDays.map((day) => (
                      <div key={day.dayNumber} style={{ fontSize: 12, color: "var(--color-warm-600)" }}>
                        <span style={legBadge}>Hari {day.dayNumber}</span> {day.title}{day.city && ` — ${day.city}`}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={sectionTitle}><IconBuilding size={18} color="var(--color-emerald-800)" />Distribusi Hotel &amp; Akomodasi</h2>
        {hotelBreakdown.length ? <div style={{ display: "grid", gap: 8 }}>
          {hotelBreakdown.map(([label, count]) => <div key={label} style={breakdownRow}><span>{label}</span><span style={breakdownCount}>{count}</span></div>)}
        </div> : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jamaah.</p>}
      </section>

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={sectionTitle}><IconUsers size={18} color="var(--color-emerald-800)" />Jamaah di Kloter Ini ({roster.length})</h2>
        {roster.length ? <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Nama", "Paspor", "Hotel", "Status"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {roster.map((p) => (
                <tr key={p.id} style={tr}>
                  <td style={td}><Link href={`/dashboard/pilgrims/${p.id}`} style={{ color: "var(--color-emerald-900)", fontWeight: 700 }}>{p.fullName}</Link>{p.requiresWheelchair && <IconWheelchair size={14} color="var(--color-gold-800)" style={{ marginLeft: 6, verticalAlign: "-2px" }} />}</td>
                  <td style={{ ...td, fontFamily: "ui-monospace,monospace", color: "var(--color-warm-500)" }}>{p.gender === Gender.FEMALE ? <IconGenderFemale size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} /> : <IconGenderMale size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} />}{p.passportNumber}</td>
                  <td style={td}>{rooms[p.id]?.length ? <div style={{ display: "grid", gap: 4 }}>{rooms[p.id]!.map((room, i) => <span key={i}>{room.hotelName}<span style={{ display: "block", fontSize: 11, color: "var(--color-warm-400)" }}>Kamar {room.roomNumber}</span></span>)}</div> : <span style={{ color: "var(--color-warm-400)" }}>-</span>}</td>
                  <td style={td}>{p.isSubstituted ? "Digantikan" : "Aktif"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div> : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jamaah di kloter ini.</p>}
      </section>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(140px,1fr))", gap: 12, margin: "20px 0" };
const statCard: React.CSSProperties = { display: "grid", gap: 6, padding: 16, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-gold-500)", borderRadius: 10 };
const statLabel: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-400)", textTransform: "uppercase", letterSpacing: ".06em" };
const statValue: React.CSSProperties = { fontSize: 24, fontWeight: 700 };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: "0 0 12px", fontSize: 16, display: "flex", alignItems: "center", gap: 8 };
const breakdownRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", padding: "8px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, fontSize: 14 };
const breakdownCount: React.CSSProperties = { fontWeight: 700, color: "var(--color-emerald-900)" };
const legRow: React.CSSProperties = { display: "flex", gap: 10, alignItems: "flex-start", padding: "10px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, fontSize: 13 };
const legBadge: React.CSSProperties = { flexShrink: 0, padding: "3px 8px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 10, fontWeight: 700, whiteSpace: "nowrap" };
const modeBadge: React.CSSProperties = { flexShrink: 0, padding: "3px 8px", borderRadius: 99, background: "var(--color-cream-200)", color: "var(--color-warm-500)", fontSize: 10, fontWeight: 700, whiteSpace: "nowrap" };
const statusBadge: React.CSSProperties = { flexShrink: 0, alignSelf: "center", fontSize: 11, color: "var(--color-warm-400)", textTransform: "capitalize" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 640 };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "12px 14px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "12px 14px", fontSize: 14 };
const stepper: React.CSSProperties = { display: "flex", gap: 8, flexWrap: "wrap" };
const stepBadge: React.CSSProperties = { padding: "6px 12px", borderRadius: 99, background: "#fff", border: "1px solid var(--color-cream-400)", color: "var(--color-warm-400)", fontSize: 12, fontWeight: 600 };
const stepCurrent: React.CSSProperties = { background: "var(--color-gold-500)", border: "1px solid var(--color-gold-500)", color: "#fff" };
const stepPast: React.CSSProperties = { background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", color: "var(--color-emerald-900)" };
const primaryBtn: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-900)", color: "#fff", fontWeight: 700 };
const ghostBtn: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 18px", background: "transparent", color: "var(--color-warm-600)", fontWeight: 600 };
const groupRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, textDecoration: "none", color: "inherit" };
