"use client";
import { useEffect, useState } from "react";
import { IconBell, IconBellOff, IconCheck, IconSos } from "@tabler/icons-react";
import { SOSAlert } from "@hajj-saas/proto-gen/hajj/v1/sos_pb";
import { notificationClient, sosClient } from "@/lib/rpc";
import { requestPushToken } from "@/lib/firebase";

export default function SOSDashboard() {
  const [alerts, setAlerts] = useState<SOSAlert[]>([]);
  const [notice, setNotice] = useState("");
  const [pushEnabled, setPushEnabled] = useState(false);
  const [resolvingId, setResolvingId] = useState("");

  const refresh = () => sosClient.listActiveSOSAlerts({}).then((r) => setAlerts(r.alerts)).catch(() => setNotice("Unable to load SOS alerts."));

  useEffect(() => {
    refresh();
    const interval = window.setInterval(refresh, 10000);
    return () => window.clearInterval(interval);
  }, []);

  async function enablePush() {
    const token = await requestPushToken();
    if (!token) { setNotice("Push notifications aren't available (check Firebase config or browser permission)."); return; }
    try {
      await notificationClient.registerPushSubscription({ fcmToken: token });
      setPushEnabled(true);
      setNotice("Push notifications enabled for SOS alerts.");
    } catch {
      setNotice("Unable to register for push notifications.");
    }
  }

  async function acknowledge(id: string) {
    try { await sosClient.acknowledgeSOSAlert({ sosAlertId: id }); void refresh(); } catch { setNotice("Unable to acknowledge alert."); }
  }
  async function resolve(id: string) {
    setResolvingId(id);
    try { await sosClient.resolveSOSAlert({ sosAlertId: id, notes: "" }); void refresh(); } catch { setNotice("Unable to resolve alert."); } finally { setResolvingId(""); }
  }

  const active = alerts.filter((a) => a.status !== "RESOLVED");

  return (
    <main style={page}>
      <header style={head}>
        <div><p style={ey}>OPERATIONS / SOS</p><h1 style={title}>SOS Alerts</h1></div>
        <button style={pushEnabled ? enabledButton : ghostButton} onClick={enablePush} disabled={pushEnabled}>{pushEnabled ? <IconBell size={18} /> : <IconBellOff size={18} />}{pushEnabled ? "Push enabled" : "Enable push alerts"}</button>
      </header>
      <div className="gold-divider" />
      {notice && <p style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
      {!active.length ? (
        <section style={empty}><IconSos size={48} color="var(--color-warm-400)" /><h2 style={{ margin: 0 }}>No active SOS alerts</h2></section>
      ) : (
        <div style={list}>
          {active.map((alert) => (
            <article key={alert.id} style={{ ...card, ...(alert.status === "ESCALATED" ? escalatedCard : {}) }}>
              <div style={row}>
                <div>
                  <strong style={{ fontSize: 18 }}>{alert.pilgrimName}</strong>
                  <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{alert.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
                </div>
                <span style={badge(alert.status)}>{alert.status}</span>
              </div>
              <div style={actions}>
                {alert.status !== "ACKNOWLEDGED" && alert.status !== "ESCALATED" && <button style={ackButton} onClick={() => acknowledge(alert.id)}><IconCheck size={16} />Acknowledge</button>}
                {alert.status === "ESCALATED" && <button style={ackButton} onClick={() => acknowledge(alert.id)}><IconCheck size={16} />Acknowledge (escalated)</button>}
                <button disabled={resolvingId === alert.id} style={resolveButton} onClick={() => resolve(alert.id)}>Mark resolved</button>
              </div>
            </article>
          ))}
        </div>
      )}
    </main>
  );
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
const ackButton: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-emerald-900)", color: "#fff", display: "inline-flex", alignItems: "center", gap: 6, fontWeight: 700 };
const resolveButton: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)" };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
function badge(status: string): React.CSSProperties {
  const map: Record<string, [string, string]> = { ACTIVE: ["var(--color-gold-50)", "var(--color-gold-800)"], ACKNOWLEDGED: ["var(--color-emerald-50)", "var(--color-emerald-900)"], ESCALATED: ["var(--color-danger-100)", "var(--color-danger-600)"] };
  const [bg, color] = map[status] ?? ["var(--color-cream-300)", "var(--color-warm-500)"];
  return { padding: "5px 10px", borderRadius: 99, background: bg, color, fontSize: 12, fontWeight: 700 };
}
