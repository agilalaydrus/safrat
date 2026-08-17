"use client";

import { useEffect, useState } from "react";
import { IconCheck, IconClockHour4, IconSos, IconWifiOff } from "@tabler/icons-react";
import { SOSAlert } from "@hajj-saas/proto-gen/hajj/v1/sos_pb";
import { sosClient } from "@/lib/rpc";
import { enqueueAction, useOfflineQueueFlush } from "@/lib/offline";
import { getFreshLocation } from "@/lib/geolocation";
import { usePilgrimCode } from "@/lib/pilgrim-context";
import { enableSosAlarm, usePilgrimSOSAckNotify } from "@/lib/sos-alarm";

const SOS_QUEUE_KIND = "sos-alert";
const HISTORY_KEY = "safrat:sos-sent-log";

type SentEntry = { at: string; delivered: boolean };

const STATUS_LABEL: Record<string, string> = { ACTIVE: "Menunggu konfirmasi", ACKNOWLEDGED: "Ketua rombongan menuju lokasi Anda", ESCALATED: "Diteruskan ke petugas operator", RESOLVED: "Selesai" };

export default function PilgrimSOSPage() {
  const code = usePilgrimCode();
  const [confirming, setConfirming] = useState(false);
  const [status, setStatus] = useState<"idle" | "sent" | "queued">("idle");
  const [history, setHistory] = useState<SentEntry[]>([]);
  const [serverAlerts, setServerAlerts] = useState<SOSAlert[]>([]);
  const [notifEnabled, setNotifEnabled] = useState(false);

  useEffect(() => {
    if (typeof window !== "undefined" && "Notification" in window) setNotifEnabled(Notification.permission === "granted");
  }, []);

  useOfflineQueueFlush(SOS_QUEUE_KIND, async (payload) => {
    const { appAccessCode, lat, lng } = payload as { appAccessCode: string; lat?: number; lng?: number };
    await sosClient.createSOSAlert({ appAccessCode, lat, lng });
    markDelivered();
  });

  // Polls this pilgrim's own alert history so the app can show a live status
  // change — most importantly, a leader acknowledging ("sedang menuju lokasi
  // Anda") without the pilgrim needing to do anything.
  useEffect(() => {
    if (!code) return;
    let cancelled = false;
    function poll() {
      sosClient.listMyPilgrimSOSAlerts({ appAccessCode: code }).then((response) => {
        if (!cancelled) setServerAlerts(response.alerts);
      }).catch(() => {});
    }
    poll();
    const interval = window.setInterval(poll, 8000);
    return () => { cancelled = true; window.clearInterval(interval); };
  }, [code]);

  usePilgrimSOSAckNotify(serverAlerts);
  const latestActive = serverAlerts.find((alert) => alert.status !== "RESOLVED");

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(HISTORY_KEY);
      if (raw) setHistory(JSON.parse(raw));
    } catch {
      // ignore corrupt local history
    }
  }, []);

  function saveHistory(next: SentEntry[]) {
    setHistory(next);
    window.localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
  }

  function markDelivered() {
    setHistory((current) => {
      const next = current.map((entry, index) => (index === current.length - 1 ? { ...entry, delivered: true } : entry));
      window.localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
      return next;
    });
  }

  async function send() {
    if (!code) return;
    setConfirming(false);
    const entry: SentEntry = { at: new Date().toISOString(), delivered: false };
    saveHistory([...history, entry]);
    // Best-effort fresh GPS fix — a short timeout so SOS never waits long on
    // it; falls back server-side to the pilgrim's last periodic location ping.
    const location = await getFreshLocation();
    try {
      await sosClient.createSOSAlert({ appAccessCode: code, lat: location?.lat, lng: location?.lng });
      markDelivered();
      setStatus("sent");
    } catch {
      enqueueAction(SOS_QUEUE_KIND, { appAccessCode: code, lat: location?.lat, lng: location?.lng });
      setStatus("queued");
    }
  }

  return (
    <main style={page}>
      {!notifEnabled && status === "idle" && !confirming && (
        <button onClick={() => void enableSosAlarm().then(setNotifEnabled)} style={notifButton}>Aktifkan notifikasi status SOS</button>
      )}
      {latestActive && status === "idle" && !confirming && (
        <div style={{ ...resultBox, ...(latestActive.status === "ACKNOWLEDGED" ? ackBox : {}) }}>
          {latestActive.status === "ACKNOWLEDGED" ? <IconCheck size={40} color="var(--color-emerald-800)" /> : <IconClockHour4 size={40} color="var(--color-gold-800)" />}
          <p style={{ fontWeight: 700, fontSize: 16 }}>{STATUS_LABEL[latestActive.status] ?? latestActive.status}</p>
          {latestActive.status === "ACKNOWLEDGED" && <p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>Tetap di tempat jika aman untuk melakukannya.</p>}
        </div>
      )}
      {!latestActive && status === "idle" && !confirming && (
        <button onClick={() => setConfirming(true)} style={bigButton}>
          <IconSos size={64} />
          <span style={{ fontSize: 20, fontWeight: 700 }}>SOS</span>
          <span style={{ fontSize: 13, opacity: 0.85 }}>Tekan untuk minta bantuan darurat</span>
        </button>
      )}
      {confirming && (
        <div style={confirmBox}>
          <p style={{ fontSize: 18, fontWeight: 700, margin: "0 0 4px" }}>Kirim sinyal SOS?</p>
          <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Ketua rombongan dan petugas akan segera diberi tahu.</p>
          <button onClick={send} style={confirmButton}>Ya, kirim SOS sekarang</button>
          <button onClick={() => setConfirming(false)} style={cancelButton}>Batal</button>
        </div>
      )}
      {status === "sent" && (
        <div style={resultBox}>
          <IconCheck size={48} color="var(--color-emerald-800)" />
          <p style={{ fontWeight: 700, fontSize: 18 }}>Sinyal SOS terkirim</p>
          <p style={{ color: "var(--color-warm-500)" }}>Bantuan sedang menuju lokasi Anda. Tetap di tempat jika aman untuk melakukannya.</p>
          <button onClick={() => setStatus("idle")} style={cancelButton}>Kembali</button>
        </div>
      )}
      {status === "queued" && (
        <div style={resultBox}>
          <IconWifiOff size={48} color="var(--color-gold-800)" />
          <p style={{ fontWeight: 700, fontSize: 18 }}>SOS tersimpan — tidak ada koneksi</p>
          <p style={{ color: "var(--color-warm-500)" }}>Akan otomatis terkirim begitu Anda kembali online.</p>
          <button onClick={() => setStatus("idle")} style={cancelButton}>Kembali</button>
        </div>
      )}
      {serverAlerts.length > 0 && status === "idle" && !confirming && (
        <section style={historySection}>
          <p style={{ fontSize: 12, fontWeight: 700, color: "var(--color-warm-400)", letterSpacing: ".05em" }}>RIWAYAT SOS ANDA</p>
          {serverAlerts.slice(0, 5).map((alert) => (
            <p key={alert.id} style={historyRow}>{alert.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })} — {STATUS_LABEL[alert.status] ?? alert.status}</p>
          ))}
        </section>
      )}
      {serverAlerts.length === 0 && history.length > 0 && status === "idle" && !confirming && (
        <section style={historySection}>
          <p style={{ fontSize: 12, fontWeight: 700, color: "var(--color-warm-400)", letterSpacing: ".05em" }}>RIWAYAT SOS ANDA</p>
          {history.slice(-5).reverse().map((entry) => (
            <p key={entry.at} style={historyRow}>{new Date(entry.at).toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })} — {entry.delivered ? "Terkirim" : "Menunggu"}</p>
          ))}
        </section>
      )}
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px", minHeight: "calc(100dvh - 84px)", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 16 };
const bigButton: React.CSSProperties = { width: 220, height: 220, borderRadius: "50%", border: 0, background: "var(--color-danger-600)", color: "#fff", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 8, boxShadow: "0 12px 32px rgba(220,38,38,.35)" };
const confirmBox: React.CSSProperties = { width: "100%", textAlign: "center", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 16, padding: 28 };
const confirmButton: React.CSSProperties = { width: "100%", minHeight: 52, border: 0, borderRadius: 10, background: "var(--color-danger-600)", color: "#fff", fontWeight: 700, fontSize: 16, marginBottom: 10 };
const cancelButton: React.CSSProperties = { width: "100%", minHeight: 48, border: "1px solid var(--color-cream-500)", borderRadius: 10, background: "transparent", color: "var(--color-warm-500)" };
const resultBox: React.CSSProperties = { width: "100%", textAlign: "center", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 16, padding: 28, display: "grid", gap: 10, justifyItems: "center" };
const ackBox: React.CSSProperties = { background: "var(--color-emerald-50)", borderColor: "var(--color-emerald-200)" };
const notifButton: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", fontSize: 13 };
const historySection: React.CSSProperties = { width: "100%", marginTop: 12 };
const historyRow: React.CSSProperties = { margin: "6px 0", fontSize: 13, color: "var(--color-warm-500)" };
