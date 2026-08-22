"use client";

import { useEffect, useState } from "react";
import { IconBed, IconBus, IconCheck, IconSos } from "@tabler/icons-react";
import { KloterStaff } from "@hajj-saas/proto-gen/hajj/v1/staff_schedule_pb";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { SOSAlert } from "@hajj-saas/proto-gen/hajj/v1/sos_pb";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { CheckIn } from "@hajj-saas/proto-gen/hajj/v1/groupleader_pb";
import { staffScheduleClient, tripClient } from "@/lib/rpc";

const SOS_STATUS_LABEL: Record<string, string> = { ACTIVE: "Aktif", ACKNOWLEDGED: "Dikonfirmasi", ESCALATED: "Dieskalasi" };

export default function AgentTripTab() {
  const [assignments, setAssignments] = useState<KloterStaff[]>([]);
  const [selected, setSelected] = useState<KloterStaff>();
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [alerts, setAlerts] = useState<SOSAlert[]>([]);
  const [movements, setMovements] = useState<Movement[]>([]);
  const [checkIns, setCheckIns] = useState<Record<string, CheckIn[]>>({});
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [working, setWorking] = useState("");

  useEffect(() => {
    staffScheduleClient.listMyAssignments({}).then((r) => {
      setAssignments(r.assignments);
      setSelected(r.assignments[0]);
    }).catch(() => setNotice("Gagal memuat penugasan kloter Anda.")).finally(() => setLoading(false));
  }, []);

  const refreshRoster = () => {
    if (!selected) return;
    setLoading(true);
    Promise.all([
      tripClient.getTripRoster({ kloterId: selected.kloterId }),
      tripClient.listTripSOSAlerts({ kloterId: selected.kloterId }),
      tripClient.listTripMovements({ kloterId: selected.kloterId }),
    ]).then(([rosterRes, alertsRes, movementsRes]) => {
      setPilgrims(rosterRes.pilgrims);
      setAlerts(alertsRes.alerts.filter((a) => a.status !== "RESOLVED"));
      setMovements(movementsRes.movements);
      Promise.all(movementsRes.movements.map((m) => tripClient.listTripCheckIns({ kloterId: selected.kloterId, movementId: m.id }).then((r) => [m.id, r.checkIns] as const))).then((pairs) => {
        setCheckIns(Object.fromEntries(pairs));
      });
    }).catch(() => setNotice("Gagal memuat data perjalanan.")).finally(() => setLoading(false));
  };
  useEffect(refreshRoster, [selected]);

  const toggleHotelCheckIn = async (pilgrim: Pilgrim) => {
    if (!selected) return;
    setWorking(pilgrim.id);
    try {
      const result = await tripClient.setTripHotelCheckIn({ kloterId: selected.kloterId, pilgrimId: pilgrim.id, checkedIn: !pilgrim.hotelCheckedIn });
      setPilgrims((current) => current.map((p) => (p.id === pilgrim.id ? result : p)));
    } catch {
      setNotice("Gagal memperbarui status check-in hotel.");
    } finally {
      setWorking("");
    }
  };

  const acknowledgeAlert = async (alert: SOSAlert) => {
    if (!selected) return;
    setWorking(alert.id);
    try {
      await tripClient.acknowledgeTripSOSAlert({ kloterId: selected.kloterId, sosAlertId: alert.id });
      refreshRoster();
    } catch {
      setNotice("Gagal mengonfirmasi SOS.");
    } finally {
      setWorking("");
    }
  };

  const resolveAlert = async (alert: SOSAlert) => {
    if (!selected) return;
    setWorking(alert.id);
    try {
      await tripClient.resolveTripSOSAlert({ kloterId: selected.kloterId, sosAlertId: alert.id, notes: "" });
      refreshRoster();
    } catch {
      setNotice("Gagal menyelesaikan SOS.");
    } finally {
      setWorking("");
    }
  };

  const checkInMovement = async (movement: Movement, pilgrim: Pilgrim, type: "DEPARTURE" | "ARRIVAL") => {
    if (!selected) return;
    const key = `${movement.id}:${pilgrim.id}`;
    setWorking(key);
    try {
      await tripClient.createTripCheckIn({ kloterId: selected.kloterId, movementId: movement.id, pilgrimId: pilgrim.id, type });
      const r = await tripClient.listTripCheckIns({ kloterId: selected.kloterId, movementId: movement.id });
      setCheckIns((current) => ({ ...current, [movement.id]: r.checkIns }));
    } catch {
      setNotice("Gagal mencatat check-in.");
    } finally {
      setWorking("");
    }
  };

  if (loading && !assignments.length) return <p style={{ color: "var(--color-warm-400)" }}>Memuat...</p>;
  if (!assignments.length) return <p style={{ color: "var(--color-warm-400)" }}>Anda belum ditugaskan ke kloter mana pun.</p>;

  return (
    <div>
      {assignments.length > 1 && (
        <select value={selected?.id} onChange={(e) => setSelected(assignments.find((a) => a.id === e.target.value))} style={select}>
          {assignments.map((a) => <option key={a.id} value={a.id}>{a.kloterName} · {a.seasonName}</option>)}
        </select>
      )}
      {notice && <p style={{ color: "var(--color-gold-800)", fontSize: 13 }}>{notice}</p>}

      {alerts.length > 0 && (
        <section style={{ ...card, border: "1px solid var(--color-danger-600)", marginBottom: 16 }}>
          <h3 style={sectionTitle}><IconSos size={16} color="var(--color-danger-600)" />SOS Aktif</h3>
          <div style={{ display: "grid", gap: 8 }}>
            {alerts.map((alert) => (
              <div key={alert.id} style={alertRow}>
                <div>
                  <strong>{alert.pilgrimName}</strong>
                  <span style={{ display: "block", fontSize: 12, color: "var(--color-warm-500)" }}>{SOS_STATUS_LABEL[alert.status] ?? alert.status}</span>
                </div>
                <div style={{ display: "flex", gap: 6 }}>
                  {alert.status === "ACTIVE" && <button disabled={working === alert.id} onClick={() => acknowledgeAlert(alert)} style={smallBtn}>Konfirmasi</button>}
                  <button disabled={working === alert.id} onClick={() => resolveAlert(alert)} style={smallBtnGhost}>Selesai</button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      <section style={{ ...card, marginBottom: 16 }}>
        <h3 style={sectionTitle}><IconBed size={16} color="var(--color-emerald-800)" />Jamaah & Check-in Hotel ({pilgrims.length})</h3>
        <div style={{ display: "grid", gap: 6 }}>
          {pilgrims.map((p) => (
            <div key={p.id} style={rosterRow}>
              <span>{p.fullName}</span>
              <button disabled={working === p.id} onClick={() => toggleHotelCheckIn(p)} style={p.hotelCheckedIn ? smallBtnActive : smallBtnGhost}>
                {p.hotelCheckedIn ? <IconCheck size={14} /> : null}{p.hotelCheckedIn ? "Sudah check-in" : "Tandai check-in"}
              </button>
            </div>
          ))}
          {!pilgrims.length && <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jamaah di kloter ini.</p>}
        </div>
      </section>

      <section style={card}>
        <h3 style={sectionTitle}><IconBus size={16} color="var(--color-emerald-800)" />Jadwal Pergerakan</h3>
        <div style={{ display: "grid", gap: 16 }}>
          {movements.map((m) => {
            const done = new Set((checkIns[m.id] ?? []).map((c) => c.pilgrimId));
            return (
              <div key={m.id}>
                <p style={{ margin: "0 0 6px", fontWeight: 700 }}>{m.name}</p>
                <p style={{ margin: "0 0 4px", fontSize: 12, color: "var(--color-warm-500)" }}>{m.origin} → {m.destination} · {m.scheduledAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
                {m.mode === "FLIGHT" && (m.airline || m.flightNumber) && <p style={{ margin: "0 0 8px", fontSize: 12, color: "var(--color-warm-500)" }}>{[m.airline, m.flightNumber].filter(Boolean).join(" · ")}{m.tripLeg && ` · ${m.tripLeg === "DEPARTURE" ? "Keberangkatan" : "Kepulangan"}`}</p>}
                <div style={{ display: "grid", gap: 6 }}>
                  {pilgrims.map((p) => (
                    <div key={p.id} style={rosterRow}>
                      <span>{p.fullName}</span>
                      {done.has(p.id) ? <span style={{ fontSize: 12, color: "var(--color-emerald-700)", fontWeight: 600 }}><IconCheck size={14} style={{ verticalAlign: "-2px" }} /> Sudah check-in</span> : (
                        <button disabled={working === `${m.id}:${p.id}`} onClick={() => checkInMovement(m, p, "DEPARTURE")} style={smallBtnGhost}>Check-in</button>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
          {!movements.length && <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada jadwal pergerakan untuk kloter ini.</p>}
        </div>
      </section>
    </div>
  );
}

const select: React.CSSProperties = { minHeight: 44, width: "100%", marginBottom: 16, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 12px", background: "#fff", font: "inherit" };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: "0 0 12px", fontSize: 15, fontWeight: 700 };
const rosterRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "8px 0", borderBottom: "1px solid var(--color-cream-300)", fontSize: 13 };
const alertRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 12px", background: "var(--color-danger-100)", borderRadius: 8 };
const smallBtn: React.CSSProperties = { minHeight: 32, border: 0, borderRadius: 6, padding: "0 10px", background: "var(--color-emerald-900)", color: "#fff", fontSize: 12, fontWeight: 600 };
const smallBtnGhost: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 10px", background: "transparent", color: "var(--color-warm-700)", fontSize: 12, fontWeight: 600 };
const smallBtnActive: React.CSSProperties = { minHeight: 32, border: 0, borderRadius: 6, padding: "0 10px", background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 4 };
