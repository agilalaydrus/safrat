"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { IconBuilding, IconGenderFemale, IconGenderMale, IconMessageCircle, IconPlane, IconUserCheck, IconUserPlus, IconUsers, IconWheelchair } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { groupClient, kloterClient, accommodationClient, ritualClient, healthReportClient, seasonClient } from "@/lib/rpc";
import { RitualProgressItem } from "@hajj-saas/proto-gen/hajj/v1/ritual_pb";
import { HealthReport } from "@hajj-saas/proto-gen/hajj/v1/health_report_pb";
import { SeasonType } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import GroupChatPanel from "./GroupChatPanel";
import GroupFormDialog from "./GroupFormDialog";
import PilgrimFormDialog from "../pilgrims/PilgrimFormDialog";

const CITY_LABEL: Record<string, string> = {
  INDONESIA: "Indonesia", TRANSIT: "Transit", MADINAH: "Madinah", MAKKAH: "Makkah",
  ARAFAH: "Arafah", MUZDALIFAH: "Muzdalifah", MINA: "Mina", DEPARTED: "Dalam Perjalanan Pulang",
};

function lastUpdateColor(lastUpdate?: Date): string {
  if (!lastUpdate) return "var(--color-warm-400)";
  const hours = (Date.now() - lastUpdate.getTime()) / 3_600_000;
  if (hours < 2) return "var(--color-emerald-700)";
  if (hours < 6) return "var(--color-gold-800)";
  return "var(--color-danger-600)";
}

const PAYMENT_LABEL: Record<string, { label: string; color: string; bg: string }> = {
  UNPAID: { label: "Belum Bayar", color: "var(--color-danger-600)", bg: "var(--color-danger-100)" },
  DP: { label: "DP", color: "var(--color-gold-800)", bg: "var(--color-gold-50)" },
  PAID: { label: "Lunas", color: "var(--color-emerald-900)", bg: "var(--color-emerald-50)" },
};

function confirmationState(status: string, kycStatus: string, paymentStatus: string): "CANCELLED" | "CONFIRMED" | "PENDING" {
  if (status === "CANCELLED") return "CANCELLED";
  if (kycStatus === "VERIFIED" && paymentStatus !== "UNPAID") return "CONFIRMED";
  return "PENDING";
}

export default function GroupDetail({ id }: { id: string }) {
  const searchParams = useSearchParams();
  const seasonId = searchParams.get("seasonId") ?? "";
  const [group, setGroup] = useState<Group>();
  const [roster, setRoster] = useState<Pilgrim[]>([]);
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const [rooms, setRooms] = useState<Record<string, { hotelName: string; roomNumber: string }[]>>({});
  const [ritualItems, setRitualItems] = useState<RitualProgressItem[]>([]);
  const [healthReports, setHealthReports] = useState<HealthReport[]>([]);
  const [seasonTypeBucket, setSeasonTypeBucket] = useState<"HAJJ" | "UMRAH">("UMRAH");
  const [seeding, setSeeding] = useState(false);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [chatOpen, setChatOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [addJamaahOpen, setAddJamaahOpen] = useState(false);

  const load = () => {
    if (!seasonId) return;
    setLoading(true);
    Promise.all([
      groupClient.listGroups({ seasonId }).then((r) => setGroup(r.groups.find((g) => g.id === id))),
      groupClient.getGroupRoster({ groupId: id }).then((r) => setRoster(r.pilgrims)),
      kloterClient.listKloters({ seasonId }).then((r) => setKloters(r.kloters)),
      accommodationClient.listPilgrimRoomAssignments({ seasonId }).then((r) => {
        const map: Record<string, { hotelName: string; roomNumber: string }[]> = {};
        for (const a of r.assignments) (map[a.pilgrimId] ??= []).push({ hotelName: a.hotelName, roomNumber: a.roomNumber });
        setRooms(map);
      }),
      ritualClient.getGroupRitualProgress({ groupId: id }).then((r) => setRitualItems(r.items)).catch(() => {}),
      healthReportClient.listHealthReports({}).then((r) => setHealthReports(r.reports.filter((x) => x.groupId === id && !x.resolved))).catch(() => {}),
      seasonClient.listSeasons({}).then((r) => {
        const season = r.seasons.find((s) => s.id === seasonId);
        setSeasonTypeBucket(season?.type === SeasonType.HAJJ ? "HAJJ" : "UMRAH");
      }).catch(() => {}),
    ]).catch(() => setNotice("Gagal memuat data grup.")).finally(() => setLoading(false));
  };
  useEffect(load, [id, seasonId]);

  async function resolveHealthReport(reportId: string) {
    await healthReportClient.resolveHealthReport({ reportId });
    load();
  }

  async function seedRituals() {
    setSeeding(true);
    try {
      await ritualClient.seedDefaultTemplates({ seasonType: seasonTypeBucket });
      load();
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal membuat template ritual.");
    } finally {
      setSeeding(false);
    }
  }

  const kloterCodes = useMemo(() => new Map(kloters.map((kloter) => [kloter.id, kloter.code])), [kloters]);

  const kloterBreakdown = useMemo(() => {
    const map = new Map<string, number>();
    for (const p of roster) {
      const key = (p.kloterId && kloterCodes.get(p.kloterId)) || "Belum Ada Kloter";
      map.set(key, (map.get(key) ?? 0) + 1);
    }
    return Array.from(map.entries()).sort((a, b) => b[1] - a[1]);
  }, [kloterCodes, roster]);

  const hotelBreakdown = useMemo(() => {
    const map = new Map<string, number>();
    for (const p of roster) {
      const pilgrimRooms = rooms[p.id];
      if (!pilgrimRooms?.length) { map.set("Belum Ditempatkan", (map.get("Belum Ditempatkan") ?? 0) + 1); continue; }
      for (const room of pilgrimRooms) map.set(room.hotelName, (map.get(room.hotelName) ?? 0) + 1);
    }
    return Array.from(map.entries()).sort((a, b) => b[1] - a[1]);
  }, [roster, rooms]);

  const paymentBreakdown = useMemo(() => {
    const counts = { UNPAID: 0, DP: 0, PAID: 0 };
    for (const p of roster) { const status = p.paymentStatus || "UNPAID"; if (status in counts) counts[status as keyof typeof counts]++; }
    return counts;
  }, [roster]);

  const confirmedCount = roster.filter((p) => confirmationState(p.status, p.kycStatus, p.paymentStatus) === "CONFIRMED").length;
  const wheelchairCount = roster.filter((p) => p.requiresWheelchair).length;

  if (!seasonId) {
    return <main style={page}><p style={{ color: "var(--color-danger-600)" }}>Musim tidak diketahui — kembali ke daftar Grup dan buka lagi dari sana.</p><Link href="/dashboard/groups" style={{ color: "var(--color-gold-800)" }}>Kembali ke Grup</Link></main>;
  }
  if (loading && !group) return <main style={page}><p style={{ color: "var(--color-warm-400)" }}>Memuat...</p></main>;
  if (!group) return <main style={page}><p style={{ color: "var(--color-danger-600)" }}>Grup tidak ditemukan.</p><Link href="/dashboard/groups" style={{ color: "var(--color-gold-800)" }}>Kembali ke Grup</Link></main>;

  return (
    <main style={page}>
      <Link href="/dashboard/groups" style={{ color: "var(--color-gold-800)" }}>Kembali ke daftar Grup</Link>
      <header style={{ display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", marginTop: 20 }}>
        <div>
          <p style={eyebrow}>DETAIL GRUP</p>
          <h1 style={{ margin: 0, fontSize: 40 }}>{group.name}</h1>
          <p style={{ color: "var(--color-warm-500)", margin: "4px 0 0" }}><IconUserCheck size={15} style={{ verticalAlign: "-2px", marginRight: 6 }} />{group.leaderName || "Belum ada Muttawwif"}</p>
          <p style={{ margin: "4px 0 0", fontSize: 13, color: lastUpdateColor(group.lastUpdate?.toDate()) }}>
            <IconBuilding size={14} style={{ verticalAlign: "-2px", marginRight: 6 }} />
            {CITY_LABEL[group.currentCity] ?? (group.currentCity || "Indonesia")}
            {group.currentActivity && ` · ${group.currentActivity}`}
            {group.lastUpdate && ` · diperbarui ${group.lastUpdate.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}`}
          </p>
        </div>
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <button onClick={() => setAddJamaahOpen(true)} style={ghostBtn}><IconUserPlus size={18} />Tambah Jamaah</button>
          <button onClick={() => setChatOpen(true)} style={ghostBtn}><IconMessageCircle size={18} />Chat</button>
          <button onClick={() => setEditOpen(true)} style={emerald}>Ubah Grup</button>
        </div>
      </header>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      <div style={statGrid}>
        <div style={statCard}><span style={statLabel}>Kapasitas</span><strong style={statValue}>{group.pilgrimCount}/{group.capacity}</strong></div>
        <div style={statCard}><span style={statLabel}>Terkonfirmasi</span><strong style={{ ...statValue, color: "var(--color-emerald-900)" }}>{confirmedCount}</strong></div>
        <div style={statCard}><span style={statLabel}>Belum Bayar</span><strong style={{ ...statValue, color: "var(--color-danger-600)" }}>{paymentBreakdown.UNPAID}</strong></div>
        <div style={statCard}><span style={statLabel}>DP</span><strong style={{ ...statValue, color: "var(--color-gold-800)" }}>{paymentBreakdown.DP}</strong></div>
        <div style={statCard}><span style={statLabel}>Lunas</span><strong style={{ ...statValue, color: "var(--color-emerald-900)" }}>{paymentBreakdown.PAID}</strong></div>
        <div style={statCard}><span style={statLabel}>Kursi Roda</span><strong style={statValue}>{wheelchairCount}</strong></div>
      </div>

      <div style={twoCol}>
        <section style={card}>
          <h2 style={{ margin: "0 0 12px", fontSize: 16, display: "flex", alignItems: "center", gap: 8 }}><IconPlane size={18} color="var(--color-emerald-800)" />Distribusi Kloter</h2>
          {kloterBreakdown.length ? <div style={{ display: "grid", gap: 8 }}>
            {kloterBreakdown.map(([label, count]) => <div key={label} style={breakdownRow}><span>{label}</span><span style={breakdownCount}>{count}</span></div>)}
          </div> : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jamaah.</p>}
        </section>
        <section style={card}>
          <h2 style={{ margin: "0 0 12px", fontSize: 16, display: "flex", alignItems: "center", gap: 8 }}><IconBuilding size={18} color="var(--color-emerald-800)" />Distribusi Hotel</h2>
          {hotelBreakdown.length ? <div style={{ display: "grid", gap: 8 }}>
            {hotelBreakdown.map(([label, count]) => <div key={label} style={breakdownRow}><span>{label}</span><span style={breakdownCount}>{count}</span></div>)}
          </div> : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jamaah.</p>}
        </section>
      </div>

      {(ritualItems.length > 0 || healthReports.length > 0 || roster.length > 0) && (
        <div style={twoCol}>
          {ritualItems.length > 0 ? (
            <section style={card}>
              <h2 style={{ margin: "0 0 12px", fontSize: 16 }}>Progres Ibadah</h2>
              <div style={{ display: "grid", gap: 8 }}>
                {ritualItems.map((item) => (
                  <div key={item.ritualId} style={breakdownRow}>
                    <span>{item.name}</span>
                    <span style={breakdownCount}>{item.completedCount}/{item.totalPilgrims}</span>
                  </div>
                ))}
              </div>
            </section>
          ) : roster.length > 0 ? (
            <section style={card}>
              <h2 style={{ margin: "0 0 12px", fontSize: 16 }}>Progres Ibadah</h2>
              <p style={{ margin: "0 0 10px", fontSize: 13, color: "var(--color-warm-400)" }}>Belum ada template ritual untuk musim {seasonTypeBucket === "HAJJ" ? "Haji" : "Umrah"} ini.</p>
              <button onClick={() => void seedRituals()} disabled={seeding} style={emerald}>{seeding ? "Membuat..." : "Buat Template Default"}</button>
            </section>
          ) : null}
          {healthReports.length > 0 && (
            <section style={card}>
              <h2 style={{ margin: "0 0 12px", fontSize: 16 }}>Laporan Kesehatan Aktif ({healthReports.length})</h2>
              <div style={{ display: "grid", gap: 8 }}>
                {healthReports.map((r) => (
                  <div key={r.id} style={{ ...breakdownRow, alignItems: "flex-start", flexDirection: "column", gap: 4 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", width: "100%" }}>
                      <strong>{r.pilgrimName}</strong>
                      <span style={{ color: r.severity === "BERAT" ? "var(--color-danger-600)" : "var(--color-gold-800)", fontSize: 12, fontWeight: 700 }}>{r.severity}</span>
                    </div>
                    <span style={{ fontSize: 13, color: "var(--color-warm-600)" }}>{r.symptoms}</span>
                    <button onClick={() => void resolveHealthReport(r.id)} style={{ ...ghostBtn, minHeight: 32, padding: "0 10px", fontSize: 12 }}>Tandai Ditangani</button>
                  </div>
                ))}
              </div>
            </section>
          )}
        </div>
      )}

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={{ margin: "0 0 12px", fontSize: 16, display: "flex", alignItems: "center", gap: 8 }}><IconUsers size={18} color="var(--color-emerald-800)" />Anggota ({roster.length})</h2>
        {roster.length ? <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Nama", "Paspor", "Kloter", "Hotel", "Status", "Pembayaran"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {roster.map((p) => {
                const state = confirmationState(p.status, p.kycStatus, p.paymentStatus);
                const paymentMeta = PAYMENT_LABEL[p.paymentStatus] ?? PAYMENT_LABEL.UNPAID!;
                return (
                  <tr key={p.id} style={tr}>
                    <td style={td}><Link href={`/dashboard/pilgrims/${p.id}`} style={{ color: "var(--color-emerald-900)", fontWeight: 700 }}>{p.fullName}</Link>{p.requiresWheelchair && <IconWheelchair size={14} color="var(--color-gold-800)" style={{ marginLeft: 6, verticalAlign: "-2px" }} />}</td>
                    <td style={{ ...td, fontFamily: "ui-monospace,monospace", color: "var(--color-warm-500)" }}>{p.gender === Gender.FEMALE ? <IconGenderFemale size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} /> : <IconGenderMale size={13} style={{ verticalAlign: "-2px", marginRight: 4 }} />}{p.passportNumber}</td>
                    <td style={td}>{(p.kloterId && kloterCodes.get(p.kloterId)) || <span style={{ color: "var(--color-warm-400)" }}>-</span>}</td>
                    <td style={td}>{rooms[p.id]?.length ? <div style={{ display: "grid", gap: 4 }}>{rooms[p.id]!.map((room, i) => <span key={i}>{room.hotelName}<span style={{ display: "block", fontSize: 11, color: "var(--color-warm-400)" }}>Kamar {room.roomNumber}</span></span>)}</div> : <span style={{ color: "var(--color-warm-400)" }}>-</span>}</td>
                    <td style={td}><span style={{ ...badgeBase, color: state === "CANCELLED" ? "var(--color-danger-600)" : state === "CONFIRMED" ? "var(--color-emerald-900)" : "var(--color-gold-800)", background: state === "CANCELLED" ? "var(--color-danger-100)" : state === "CONFIRMED" ? "var(--color-emerald-50)" : "var(--color-gold-50)" }}>{state === "CANCELLED" ? "Dibatalkan" : state === "CONFIRMED" ? "Terkonfirmasi" : "Menunggu"}</span></td>
                    <td style={td}><span style={{ ...badgeBase, color: paymentMeta.color, background: paymentMeta.bg }}>{paymentMeta.label}</span></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div> : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jamaah di grup ini.</p>}
      </section>

      <GroupChatPanel open={chatOpen} groupId={id} groupName={group.name} onClose={() => setChatOpen(false)} />
      <GroupFormDialog open={editOpen} seasonId={seasonId} initial={group} onClose={() => setEditOpen(false)} onSaved={() => { setEditOpen(false); load(); }} />
      <PilgrimFormDialog open={addJamaahOpen} onClose={() => setAddJamaahOpen(false)} seasonId={seasonId} pilgrims={[]} defaultGroupId={id} onSaved={(name) => { setNotice(`${name} berhasil ditambahkan.`); setAddJamaahOpen(false); load(); }} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px" };
const ghostBtn: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, padding: "0 16px", display: "inline-flex", gap: 8, alignItems: "center" };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(140px,1fr))", gap: 12, margin: "20px 0" };
const statCard: React.CSSProperties = { display: "grid", gap: 6, padding: 16, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-gold-500)", borderRadius: 10 };
const statLabel: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-400)", textTransform: "uppercase", letterSpacing: ".06em" };
const statValue: React.CSSProperties = { fontSize: 24, fontWeight: 700 };
const twoCol: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(280px,1fr))", gap: 16 };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const breakdownRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", padding: "8px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, fontSize: 14 };
const breakdownCount: React.CSSProperties = { fontWeight: 700, color: "var(--color-emerald-900)" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 720 };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "12px 14px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "12px 14px", fontSize: 14 };
const badgeBase: React.CSSProperties = { display: "inline-flex", padding: "4px 10px", borderRadius: 99, fontSize: 11, fontWeight: 700 };
