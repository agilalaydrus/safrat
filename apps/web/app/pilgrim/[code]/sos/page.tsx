"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { IconCheck, IconSos, IconWifiOff } from "@tabler/icons-react";
import { sosClient } from "@/lib/rpc";
import { enqueueAction, useOfflineQueueFlush } from "@/lib/offline";

const SOS_QUEUE_KIND = "sos-alert";
const HISTORY_KEY = "safrat:sos-sent-log";

type SentEntry = { at: string; delivered: boolean };

export default function PilgrimSOSPage() {
  const { code } = useParams<{ code: string }>();
  const [confirming, setConfirming] = useState(false);
  const [status, setStatus] = useState<"idle" | "sent" | "queued">("idle");
  const [history, setHistory] = useState<SentEntry[]>([]);

  useOfflineQueueFlush(SOS_QUEUE_KIND, async (payload) => {
    const { appAccessCode } = payload as { appAccessCode: string };
    await sosClient.createSOSAlert({ appAccessCode });
    markDelivered();
  });

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
    setConfirming(false);
    const entry: SentEntry = { at: new Date().toISOString(), delivered: false };
    saveHistory([...history, entry]);
    try {
      await sosClient.createSOSAlert({ appAccessCode: code });
      markDelivered();
      setStatus("sent");
    } catch {
      enqueueAction(SOS_QUEUE_KIND, { appAccessCode: code });
      setStatus("queued");
    }
  }

  return (
    <main style={page}>
      {status === "idle" && !confirming && (
        <button onClick={() => setConfirming(true)} style={bigButton}>
          <IconSos size={64} />
          <span style={{ fontSize: 20, fontWeight: 700 }}>SOS</span>
          <span style={{ fontSize: 13, opacity: 0.85 }}>Tap for emergency help</span>
        </button>
      )}
      {confirming && (
        <div style={confirmBox}>
          <p style={{ fontSize: 18, fontWeight: 700, margin: "0 0 4px" }}>Send SOS alert?</p>
          <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Your group leader and coordinators will be notified immediately.</p>
          <button onClick={send} style={confirmButton}>Yes, send SOS now</button>
          <button onClick={() => setConfirming(false)} style={cancelButton}>Cancel</button>
        </div>
      )}
      {status === "sent" && (
        <div style={resultBox}>
          <IconCheck size={48} color="var(--color-emerald-800)" />
          <p style={{ fontWeight: 700, fontSize: 18 }}>Alert sent</p>
          <p style={{ color: "var(--color-warm-500)" }}>Help is on the way. Stay where you are if it's safe to do so.</p>
          <button onClick={() => setStatus("idle")} style={cancelButton}>Back</button>
        </div>
      )}
      {status === "queued" && (
        <div style={resultBox}>
          <IconWifiOff size={48} color="var(--color-gold-800)" />
          <p style={{ fontWeight: 700, fontSize: 18 }}>Alert queued — no connection</p>
          <p style={{ color: "var(--color-warm-500)" }}>It will send automatically the moment you're back online.</p>
          <button onClick={() => setStatus("idle")} style={cancelButton}>Back</button>
        </div>
      )}
      {history.length > 0 && status === "idle" && !confirming && (
        <section style={historySection}>
          <p style={{ fontSize: 12, fontWeight: 700, color: "var(--color-warm-400)", letterSpacing: ".05em" }}>YOUR ALERTS</p>
          {history.slice(-5).reverse().map((entry) => (
            <p key={entry.at} style={historyRow}>{new Date(entry.at).toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })} — {entry.delivered ? "Delivered" : "Pending"}</p>
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
const historySection: React.CSSProperties = { width: "100%", marginTop: 12 };
const historyRow: React.CSSProperties = { margin: "6px 0", fontSize: 13, color: "var(--color-warm-500)" };
