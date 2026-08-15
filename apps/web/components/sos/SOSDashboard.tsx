"use client";
import { useEffect, useState } from "react";
import { IconBell, IconBellOff, IconCheck, IconMapPin, IconSos } from "@tabler/icons-react";
import { SOSAlert } from "@hajj-saas/proto-gen/hajj/v1/sos_pb";
import { notificationClient, sosClient } from "@/lib/rpc";
import { requestPushToken } from "@/lib/firebase";
import { enableSosAlarm, useSosAlarm } from "@/lib/sos-alarm";

export default function SOSDashboard() {
  const [alerts, setAlerts] = useState<SOSAlert[]>([]);
  const [notice, setNotice] = useState("");
  const [pushEnabled, setPushEnabled] = useState(false);
  const [alarmEnabled, setAlarmEnabled] = useState(false);
  const [resolvingId, setResolvingId] = useState("");

  const refresh = () => sosClient.listActiveSOSAlerts({}).then((r) => setAlerts(r.alerts)).catch(() => setNotice("Gagal memuat notifikasi SOS."));

  useEffect(() => {
    refresh();
    const interval = window.setInterval(refresh, 10000);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => {
    if (typeof window !== "undefined" && "Notification" in window) setAlarmEnabled(Notification.permission === "granted");
  }, []);

  useSosAlarm(alerts, "SOS Aktif");

  async function enableAlarm() {
    const granted = await enableSosAlarm();
    setAlarmEnabled(granted);
    setNotice(granted ? "Notifikasi & suara SOS telah diaktifkan di perangkat ini." : "Izin notifikasi ditolak — aktifkan dari pengaturan browser Anda.");
  }

  async function enablePush() {
    const token = await requestPushToken();
    if (!token) { setNotice("Notifikasi push latar belakang tidak tersedia (periksa konfigurasi Firebase). Notifikasi & suara saat aplikasi terbuka tetap bisa diaktifkan di atas."); return; }
    try {
      await notificationClient.registerPushSubscription({ fcmToken: token });
      setPushEnabled(true);
      setNotice("Notifikasi push latar belakang untuk SOS telah diaktifkan.");
    } catch {
      setNotice("Gagal mendaftarkan notifikasi push.");
    }
  }

  async function acknowledge(id: string) {
    try { await sosClient.acknowledgeSOSAlert({ sosAlertId: id }); void refresh(); } catch { setNotice("Gagal mengonfirmasi notifikasi."); }
  }
  async function resolve(id: string) {
    setResolvingId(id);
    try { await sosClient.resolveSOSAlert({ sosAlertId: id, notes: "" }); void refresh(); } catch { setNotice("Gagal menyelesaikan notifikasi."); } finally { setResolvingId(""); }
  }

  const active = alerts.filter((a) => a.status !== "RESOLVED");

  return (
    <main style={page}>
      <header style={head}>
        <div><p style={ey}>OPERASIONAL / SOS</p><h1 style={title}>Notifikasi SOS</h1></div>
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <button style={alarmEnabled ? enabledButton : ghostButton} onClick={enableAlarm} disabled={alarmEnabled}>{alarmEnabled ? <IconBell size={18} /> : <IconBellOff size={18} />}{alarmEnabled ? "Notifikasi & suara aktif" : "Aktifkan notifikasi & suara"}</button>
          <button style={pushEnabled ? enabledButton : ghostButton} onClick={enablePush} disabled={pushEnabled}>{pushEnabled ? <IconBell size={18} /> : <IconBellOff size={18} />}{pushEnabled ? "Push latar belakang aktif" : "Aktifkan push latar belakang"}</button>
        </div>
      </header>
      <div className="gold-divider" />
      {notice && <p style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
      {!active.length ? (
        <section style={empty}><IconSos size={48} color="var(--color-warm-400)" /><h2 style={{ margin: 0 }}>Tidak ada notifikasi SOS aktif</h2></section>
      ) : (
        <div style={list}>
          {active.map((alert) => (
            <article key={alert.id} style={{ ...card, ...(alert.status === "ESCALATED" ? escalatedCard : {}) }}>
              <div style={row}>
                <div>
                  <strong style={{ fontSize: 18 }}>{alert.pilgrimName}</strong>
                  <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{alert.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
                </div>
                <span style={badge(alert.status)}>{alertStatusLabel(alert.status)}</span>
              </div>
              {alert.hasLocation ? (
                <a href={`https://www.google.com/maps?q=${alert.lat},${alert.lng}`} target="_blank" rel="noreferrer" style={mapLink}><IconMapPin size={15} />Lihat lokasi di peta</a>
              ) : (
                <p style={noLocation}><IconMapPin size={15} />Lokasi tidak tersedia</p>
              )}
              <div style={actions}>
                {alert.status !== "ACKNOWLEDGED" && alert.status !== "ESCALATED" && <button style={ackButton} onClick={() => acknowledge(alert.id)}><IconCheck size={16} />Konfirmasi</button>}
                {alert.status === "ESCALATED" && <button style={ackButton} onClick={() => acknowledge(alert.id)}><IconCheck size={16} />Konfirmasi (dieskalasi)</button>}
                <button disabled={resolvingId === alert.id} style={resolveButton} onClick={() => resolve(alert.id)}>Tandai selesai</button>
              </div>
            </article>
          ))}
        </div>
      )}
    </main>
  );
}

function alertStatusLabel(status: string): string {
  const map: Record<string, string> = { ACTIVE: "Aktif", ACKNOWLEDGED: "Dikonfirmasi", ESCALATED: "Dieskalasi", RESOLVED: "Selesai" };
  return map[status] ?? status;
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px" };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 16, flexWrap: "wrap" };
const ey: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const ghostButton: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 16px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 8 };
const enabledButton: React.CSSProperties = { ...ghostButton, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", border: "1px solid var(--color-emerald-200)" };
const list: React.CSSProperties = { display: "grid", gap: 14 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 18 };
const escalatedCard: React.CSSProperties = { border: "1px solid var(--color-danger-600)", background: "#fdf0f0" };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12 };
const actions: React.CSSProperties = { display: "flex", gap: 10, marginTop: 14 };
const mapLink: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, marginTop: 10, color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 600 };
const noLocation: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, marginTop: 10, color: "var(--color-warm-400)", fontSize: 13 };
const ackButton: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-emerald-900)", color: "#fff", display: "inline-flex", alignItems: "center", gap: 6, fontWeight: 700 };
const resolveButton: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)" };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
function badge(status: string): React.CSSProperties {
  const map: Record<string, [string, string]> = { ACTIVE: ["var(--color-gold-50)", "var(--color-gold-800)"], ACKNOWLEDGED: ["var(--color-emerald-50)", "var(--color-emerald-900)"], ESCALATED: ["var(--color-danger-100)", "var(--color-danger-600)"] };
  const [bg, color] = map[status] ?? ["var(--color-cream-300)", "var(--color-warm-500)"];
  return { padding: "5px 10px", borderRadius: 99, background: bg, color, fontSize: 12, fontWeight: 700 };
}
