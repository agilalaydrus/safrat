"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { IconMoonStars, IconPlaneDeparture, IconSos, IconWifi, IconWifiOff, IconX } from "@tabler/icons-react";
import { monitoringClient, seasonClient } from "@/lib/rpc";
import { ActionCenter, type ActionCenterItem } from "@/components/ui/ActionCenter";
import { MonitoringSnapshot, GroupMonitoringCard } from "@hajj-saas/proto-gen/hajj/v1/monitoring_pb";

const CITY_ORDER = ["MAKKAH", "MADINAH", "ARAFAH", "MUZDALIFAH", "MINA", "TRANSIT", "INDONESIA", "DEPARTED"] as const;
const CITY_LABEL: Record<string, string> = {
  MAKKAH: "Makkah", MADINAH: "Madinah", ARAFAH: "Arafah", MUZDALIFAH: "Muzdalifah",
  MINA: "Mina", TRANSIT: "Transit", INDONESIA: "Indonesia", DEPARTED: "Dalam Perjalanan Pulang",
};
const STALE_HOURS = 6;

function hoursSince(date?: Date): number {
  if (!date) return Infinity;
  return (Date.now() - date.getTime()) / 3_600_000;
}
function lastUpdateColor(date?: Date): string {
  const h = hoursSince(date);
  if (h === Infinity) return "var(--color-warm-400)";
  if (h < 2) return "var(--color-emerald-700)";
  if (h < STALE_HOURS) return "var(--color-gold-800)";
  return "var(--color-danger-600)";
}

export default function MonitoringDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [snapshot, setSnapshot] = useState<MonitoringSnapshot>();
  const [snapshotError, setSnapshotError] = useState("");
  const [connected, setConnected] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState<GroupMonitoringCard | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => {
      setSeasons(r.seasons);
      setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "");
    }).catch(() => {});
  }, []);

  const fetchSnapshot = useCallback((sid: string) => {
    if (!sid) return;
    monitoringClient.getSnapshot({ seasonId: sid }).then((value) => {
      setSnapshot(value);
      setSnapshotError("");
    }).catch(() => setSnapshotError("Data lapangan terbaru gagal dimuat. Jangan anggap kondisi aman sebelum koneksi pulih."));
  }, []);

  const debouncedFetch = useCallback((sid: string) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => fetchSnapshot(sid), 250);
  }, [fetchSnapshot]);

  // Real-time stream: connects once per season, auto-reconnects with
  // exponential backoff if the connection drops (proxy timeout, network
  // blip, server restart) — never hot-loops on a persistent failure.
  useEffect(() => {
    if (!seasonId) return;
    setSnapshot(undefined);
    setSnapshotError("");
    fetchSnapshot(seasonId);
    const controller = new AbortController();
    let cancelled = false;
    let retryDelayMs = 2000;

    async function connectStream() {
      while (!cancelled) {
        try {
          const stream = monitoringClient.streamEvents({ seasonId }, { signal: controller.signal });
          for await (const ping of stream) {
            if (cancelled) break;
            setConnected(true);
            retryDelayMs = 2000;
            if (ping.type !== "connected") debouncedFetch(seasonId);
          }
        } catch {
          // AbortError on cleanup, or a genuine network drop — either way
          // fall through to the backoff/retry below unless we're cleaning up.
        }
        if (cancelled) return;
        setConnected(false);
        await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
        retryDelayMs = Math.min(retryDelayMs * 2, 30_000);
      }
    }
    void connectStream();
    return () => {
      cancelled = true;
      controller.abort();
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [seasonId, fetchSnapshot, debouncedFetch]);

  const sosCount = snapshot?.activeSos.length ?? 0;
  const beratReports = snapshot?.openHealthReports.filter((h) => h.severity === "BERAT") ?? [];
  const staleGroups = snapshot?.groups.filter((g) => hoursSince(g.lastUpdate?.toDate()) >= STALE_HOURS) ?? [];
  const leaderlessGroups = snapshot?.groups.filter((g) => !g.leaderName) ?? [];
  // A field screen, so the stake is people rather than money. Ordered by how
  // fast harm compounds: someone who pressed the button is waiting now.
  const actionItems = useMemo<ActionCenterItem[]>(() => {
    const items: ActionCenterItem[] = [];

    if (sosCount > 0) items.push({
      id: "sos",
      title: `${sosCount} SOS aktif`,
      description: "Jamaah menekan tombol darurat dan belum ada yang menandai penanganannya. Setiap menit tanpa tanggapan adalah menit jamaah menunggu sendirian.",
      financialImpact: `${sosCount} jamaah`,
      actionHref: "/dashboard/sos",
      actionLabel: "Tangani SOS",
      tone: "danger",
    });

    if (beratReports.length > 0) items.push({
      id: "health",
      title: `${beratReports.length} laporan kesehatan berat`,
      description: "Kondisi berat bisa memburuk dalam hitungan jam di cuaca Tanah Suci. Pastikan muttawwif sudah mendampingi dan rujukan klinik disiapkan.",
      financialImpact: `${beratReports.length} jamaah`,
      actionHref: "/dashboard/monitoring",
      actionLabel: "Lihat laporan",
      tone: "danger",
    });

    if (leaderlessGroups.length > 0) items.push({
      id: "leaderless",
      title: `${leaderlessGroups.length} grup tanpa muttawwif`,
      description: "Grup yang berjalan tanpa pendamping resmi tidak punya siapa pun di lapangan yang bertanggung jawab saat jamaah terpisah atau sakit.",
      financialImpact: `${leaderlessGroups.length} grup`,
      actionHref: "/dashboard/groups",
      actionLabel: "Tunjuk muttawwif",
      tone: "warning",
    });

    if (staleGroups.length > 0) items.push({
      id: "stale",
      title: `${staleGroups.length} grup belum mengirim kabar >${STALE_HOURS} jam`,
      description: "Tanpa kabar terbaru, posisi dan kondisi grup ini tidak diketahui — dan keluarga di rumah menanyakannya ke kantor, bukan ke lapangan.",
      financialImpact: `${staleGroups.length} grup`,
      actionHref: "/dashboard/communication",
      actionLabel: "Hubungi muttawwif",
      tone: "warning",
    });

    return items;
  }, [sosCount, beratReports.length, leaderlessGroups.length, staleGroups.length]);

  const cityBuckets = CITY_ORDER.map((city) => ({
    city,
    groups: (snapshot?.groups ?? []).filter((g) => (g.currentCity || "INDONESIA") === city),
  })).filter((b) => b.groups.length > 0);

  return (
    <main style={page}>
      <header style={header}>
        <div>
          <p style={eyebrow}>OPERASIONAL / MONITORING</p>
          <h1 style={title}>Monitoring Real-time</h1>
          <p style={{ color: "var(--color-warm-500)", margin: 0 }}>{snapshot?.groups.length ?? 0} grup · {sosCount} SOS aktif · {connected ? "data live" : "menghubungkan"}</p>
        </div>
        <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
          <span style={{ ...connBadge, color: connected ? "var(--color-emerald-800)" : "var(--color-warm-400)" }}>
            {connected ? <IconWifi size={15} /> : <IconWifiOff size={15} />}
            {connected ? "Live" : "Menghubungkan..."}
          </span>
          <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
            {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        </div>
      </header>
      <div className="gold-divider" />

      <ActionCenter
        items={snapshot ? actionItems : undefined}
        error={snapshotError}
        subtitle="Dari data lapangan langsung — diperbarui otomatis selama ada rombongan berjalan"
        cleanTitle="Lapangan aman"
        cleanDescription="Tidak ada SOS terbuka, tidak ada laporan kesehatan berat, dan setiap grup punya muttawwif serta kabar terbaru."
        className="tw-action-center--inline"
      />

      {!snapshot && <p style={{ color: "var(--color-warm-400)" }}>Memuat data monitoring...</p>}

      {snapshot && (
        <>
          <section style={{ marginTop: 8 }}>
            <h2 style={sectionTitle}>Peta Lokasi Grup</h2>
            {cityBuckets.length ? (
              <div style={{ display: "grid", gap: 20 }}>
                {cityBuckets.map(({ city, groups }) => (
                  <div key={city}>
                    <p style={cityLabel}>{CITY_LABEL[city] ?? city} <span style={cityCount}>({groups.length})</span></p>
                    <div style={cardGrid}>
                      {groups.map((g) => (
                        <button key={g.groupId} onClick={() => setSelectedGroup(g)} style={miniCard}>
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                            <strong style={{ fontSize: 14 }}>{g.name}</strong>
                            {g.hasActiveSos && <IconSos size={16} color="var(--color-danger-600)" />}
                          </div>
                          <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{g.leaderName || "Belum ada Muttawwif"}</p>
                          {g.currentActivity && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-600)" }}>{g.currentActivity}</p>}
                          <p style={{ margin: "6px 0 0", fontSize: 11, color: lastUpdateColor(g.lastUpdate?.toDate()) }}>
                            {g.lastUpdate ? `Update ${g.lastUpdate.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}` : "Belum pernah update"}
                          </p>
                          {g.ritualCompletionPct >= 0 && (
                            <div style={ritualBarTrack}><div style={{ ...ritualBarFill, width: `${Math.round(g.ritualCompletionPct * 100)}%` }} /></div>
                          )}
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada grup yang mengirim lokasi untuk musim ini. Pastikan grup dan Muttawwif sudah ditetapkan, lalu minta Muttawwif memperbarui posisi dari aplikasi.</p>}
          </section>

          <section style={{ marginTop: 28 }}>
            <h2 style={sectionTitle}><IconPlaneDeparture size={18} style={{ verticalAlign: "-3px", marginRight: 6 }} />Kepulangan 7 Hari ke Depan</h2>
            {snapshot.returnTimeline.length ? (
              <div style={{ display: "grid", gap: 8 }}>
                {snapshot.returnTimeline.map((t) => {
                  const pct = t.totalPilgrims > 0 ? Math.round((t.readyCount / t.totalPilgrims) * 100) : 0;
                  return (
                    <div key={t.kloterId} style={timelineRow}>
                      <div>
                        <strong>{t.kloterCode}</strong>
                        <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{t.returnAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
                      </div>
                      <div style={{ textAlign: "right", minWidth: 120 }}>
                        <span style={{ fontSize: 13, fontWeight: 700, color: "var(--color-emerald-900)" }}>{t.readyCount}/{t.totalPilgrims} siap</span>
                        <div style={ritualBarTrack}><div style={{ ...ritualBarFill, width: `${pct}%` }} /></div>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Tidak ada kepulangan dalam 7 hari karena belum ada jadwal yang masuk rentang ini. Jadwal kepulangan dapat diperiksa atau diubah melalui menu Transportasi.</p>}
          </section>
        </>
      )}

      {selectedGroup && (
        <div style={drawerOverlay} onClick={() => setSelectedGroup(null)}>
          <aside style={drawer} onClick={(e) => e.stopPropagation()}>
            <div style={drawerHead}>
              <div><p style={eyebrow}>DETAIL GRUP</p><h2 style={{ margin: 0, fontSize: 20 }}>{selectedGroup.name}</h2></div>
              <button onClick={() => setSelectedGroup(null)} style={drawerClose} aria-label="Tutup"><IconX size={18} /></button>
            </div>
            <div style={{ padding: 24, display: "grid", gap: 14 }}>
              <DrawerRow label="Muttawwif" value={selectedGroup.leaderName || "Belum ada"} />
              <DrawerRow label="Lokasi" value={CITY_LABEL[selectedGroup.currentCity] ?? (selectedGroup.currentCity || "Indonesia")} />
              {selectedGroup.currentActivity && <DrawerRow label="Aktivitas" value={selectedGroup.currentActivity} />}
              <DrawerRow label="Jamaah" value={String(selectedGroup.pilgrimCount)} />
              <DrawerRow label="Update Terakhir" value={selectedGroup.lastUpdate ? selectedGroup.lastUpdate.toDate().toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" }) : "Belum pernah"} />
              {selectedGroup.ritualCompletionPct >= 0 && (
                <div>
                  <p style={{ margin: "0 0 4px", fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" }}><IconMoonStars size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />Progres Ibadah</p>
                  <div style={ritualBarTrack}><div style={{ ...ritualBarFill, width: `${Math.round(selectedGroup.ritualCompletionPct * 100)}%` }} /></div>
                  <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{Math.round(selectedGroup.ritualCompletionPct * 100)}% selesai</p>
                </div>
              )}
              <Link href={`/dashboard/groups/${selectedGroup.groupId}?seasonId=${seasonId}`} style={detailLink}>Buka Detail Grup Lengkap →</Link>
            </div>
          </aside>
        </div>
      )}
    </main>
  );
}

function DrawerRow({ label, value }: { label: string; value: string }) {
  return <div><p style={{ margin: 0, fontSize: 11, color: "var(--color-warm-400)", textTransform: "uppercase", letterSpacing: ".06em" }}>{label}</p><p style={{ margin: "2px 0 0", fontSize: 14, fontWeight: 600 }}>{value}</p></div>;
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 20, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: 0 };
const connBadge: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, fontWeight: 700 };
const select: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "#fff" };
const sectionTitle: React.CSSProperties = { fontSize: 17, margin: "0 0 12px" };
const cityLabel: React.CSSProperties = { margin: "0 0 8px", fontSize: 14, fontWeight: 700, color: "var(--color-emerald-900)" };
const cityCount: React.CSSProperties = { fontWeight: 400, color: "var(--color-warm-400)" };
const cardGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(220px,1fr))", gap: 10 };
const miniCard: React.CSSProperties = { textAlign: "left", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, cursor: "pointer", font: "inherit" };
const ritualBarTrack: React.CSSProperties = { height: 5, marginTop: 8, overflow: "hidden", borderRadius: 99, background: "var(--color-cream-300)" };
const ritualBarFill: React.CSSProperties = { height: "100%", background: "var(--color-gold-500)" };
const timelineRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "12px 16px" };
const drawerOverlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 40, background: "rgba(15,23,42,.44)", display: "flex", justifyContent: "flex-end" };
const drawer: React.CSSProperties = { width: "min(420px,100%)", height: "100vh", background: "#fff", overflowY: "auto" };
const drawerHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const drawerClose: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const detailLink: React.CSSProperties = { marginTop: 10, fontSize: 13, fontWeight: 700, color: "var(--color-gold-800)" };
