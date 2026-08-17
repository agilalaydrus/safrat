"use client";

import { useEffect, useMemo, useState } from "react";
import { IconCheck, IconWifiOff } from "@tabler/icons-react";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { CheckIn } from "@hajj-saas/proto-gen/hajj/v1/groupleader_pb";
import { groupLeaderClient, seasonClient, transportClient } from "@/lib/rpc";
import { cachedFetch, enqueueAction, toDateSafe, useOfflineQueueFlush } from "@/lib/offline";
import { useLeaderGroup } from "@/lib/leader-context";

const QUEUE_KIND = "check-in";
type QueuedCheckIn = { movementId: string; pilgrimId: string; type: "DEPARTURE" | "ARRIVAL" };

export default function LeaderCheckInPage() {
  const { selectedGroupId: groupId } = useLeaderGroup();
  const [movements, setMovements] = useState<Movement[]>([]);
  const [movementId, setMovementId] = useState("");
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [checkIns, setCheckIns] = useState<CheckIn[]>([]);
  const [fromCache, setFromCache] = useState(false);
  const [working, setWorking] = useState("");
  const [notice, setNotice] = useState("");
  const [queuedLocal, setQueuedLocal] = useState<QueuedCheckIn[]>([]);

  useEffect(() => {
    if (!groupId) return;
    (async () => {
      const seasons = await seasonClient.listSeasons({});
      const season = seasons.seasons.find((s) => s.isActive) ?? seasons.seasons[0];
      if (!season) return;
      const result = await cachedFetch(`leader-movements:${season.id}`, () => transportClient.listMovements({ seasonId: season.id }));
      if (result.data) {
        const scheduled = result.data.movements.filter((m) => m.status === "scheduled");
        setMovements(scheduled);
        setMovementId(scheduled[0]?.id ?? "");
      }
    })();
    cachedFetch(`leader-roster:${groupId}`, () => groupLeaderClient.getGroupRoster({ groupId })).then((result) => {
      if (result.data) setPilgrims(result.data.pilgrims);
      setFromCache(result.fromCache);
    });
  }, [groupId]);

  useEffect(() => {
    if (!movementId) return;
    groupLeaderClient.listCheckIns({ movementId }).then((response) => setCheckIns(response.checkIns)).catch(() => setCheckIns([]));
  }, [movementId]);

  useOfflineQueueFlush(QUEUE_KIND, async (payload) => {
    const item = payload as QueuedCheckIn;
    await groupLeaderClient.createCheckIn(item);
    if (item.movementId === movementId) groupLeaderClient.listCheckIns({ movementId }).then((response) => setCheckIns(response.checkIns));
  });

  const status = useMemo(() => {
    const map: Record<string, { departed: boolean; arrived: boolean }> = {};
    for (const pilgrim of pilgrims) map[pilgrim.id] = { departed: false, arrived: false };
    for (const checkIn of checkIns) {
      const entry = map[checkIn.pilgrimId];
      if (!entry) continue;
      if (checkIn.type === "DEPARTURE") entry.departed = true;
      if (checkIn.type === "ARRIVAL") entry.arrived = true;
    }
    for (const item of queuedLocal) {
      const entry = map[item.pilgrimId];
      if (item.movementId !== movementId || !entry) continue;
      if (item.type === "DEPARTURE") entry.departed = true;
      if (item.type === "ARRIVAL") entry.arrived = true;
    }
    return map;
  }, [pilgrims, checkIns, queuedLocal, movementId]);

  async function checkIn(pilgrimId: string, type: "DEPARTURE" | "ARRIVAL") {
    if (!movementId) return;
    setWorking(pilgrimId + type);
    setNotice("");
    try {
      await groupLeaderClient.createCheckIn({ movementId, pilgrimId, type });
      const response = await groupLeaderClient.listCheckIns({ movementId });
      setCheckIns(response.checkIns);
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : "";
      if (/unavailable|network|fetch/i.test(message)) {
        enqueueAction(QUEUE_KIND, { movementId, pilgrimId, type });
        setQueuedLocal((current) => [...current, { movementId, pilgrimId, type }]);
        setNotice("Tidak ada koneksi — check-in disimpan, akan tersinkron otomatis.");
      } else {
        setNotice("Sudah check-in sebelumnya.");
      }
    } finally {
      setWorking("");
    }
  }

  return (
    <main style={page}>
      <p style={eyebrow}>CHECK-IN</p>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menggunakan data tersimpan — Anda sedang offline</p>}
      <select value={movementId} onChange={(event) => setMovementId(event.target.value)} style={select}>
        {!movements.length && <option value="">Belum ada jadwal perjalanan</option>}
        {movements.map((m) => <option key={m.id} value={m.id}>{m.name} — {toDateSafe(m.scheduledAt)?.toLocaleDateString("id-ID", { day: "2-digit", month: "short" })}</option>)}
      </select>
      {notice && <p style={noticeStyle}>{notice}</p>}
      <div style={list}>
        {pilgrims.map((pilgrim) => {
          const state = status[pilgrim.id] ?? { departed: false, arrived: false };
          return (
            <article key={pilgrim.id} style={card}>
              <strong style={{ display: "block", marginBottom: 8 }}>{pilgrim.fullName}</strong>
              <div style={buttonRow}>
                <button disabled={!movementId || state.departed || working === pilgrim.id + "DEPARTURE"} onClick={() => checkIn(pilgrim.id, "DEPARTURE")} style={state.departed ? doneButton : actionButton}>
                  {state.departed && <IconCheck size={16} />}Keberangkatan
                </button>
                <button disabled={!movementId || state.arrived || working === pilgrim.id + "ARRIVAL"} onClick={() => checkIn(pilgrim.id, "ARRIVAL")} style={state.arrived ? doneButton : actionButton}>
                  {state.arrived && <IconCheck size={16} />}Kedatangan
                </button>
              </div>
            </article>
          );
        })}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 0" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 12px" };
const select: React.CSSProperties = { width: "100%", minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "0 12px", background: "#fff", marginBottom: 14, font: "inherit" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 14 };
const noticeStyle: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 13, marginBottom: 12 };
const list: React.CSSProperties = { display: "grid", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14 };
const buttonRow: React.CSSProperties = { display: "flex", gap: 8 };
const actionButton: React.CSSProperties = { flex: 1, minHeight: 44, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 6 };
const doneButton: React.CSSProperties = { ...actionButton, border: "1px solid var(--color-cream-400)", background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" };
