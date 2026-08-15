"use client";

import { useEffect, useRef } from "react";

// SOS needs to alert someone even if Firebase push was never configured (it's
// optional everywhere else in this app) and even if the tab isn't focused.
// This is the guaranteed, in-app layer: a Web Audio beep plus a browser
// Notification, driven purely by polling — no external asset, no push infra.
let audioCtx: AudioContext | undefined;

function getAudioContext(): AudioContext | undefined {
  if (typeof window === "undefined") return undefined;
  const Ctor = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!Ctor) return undefined;
  audioCtx ??= new Ctor();
  return audioCtx;
}

/** Must be called from within a user-gesture handler (a click) so the browser's autoplay policy allows later automatic beeps. */
export function unlockSosAudio() {
  const ctx = getAudioContext();
  if (ctx?.state === "suspended") void ctx.resume();
}

export function playSosAlarm() {
  const ctx = getAudioContext();
  if (!ctx) return;
  try {
    if (ctx.state === "suspended") void ctx.resume();
    const now = ctx.currentTime;
    [0, 0.35, 0.7].forEach((offset) => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "square";
      osc.frequency.value = 880;
      gain.gain.setValueAtTime(0.0001, now + offset);
      gain.gain.exponentialRampToValueAtTime(0.35, now + offset + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, now + offset + 0.3);
      osc.connect(gain).connect(ctx.destination);
      osc.start(now + offset);
      osc.stop(now + offset + 0.32);
    });
  } catch {
    // Autoplay can still be blocked before any user gesture on this page —
    // the Notification (and the on-screen list) still carries the alert.
  }
}

export async function enableSosAlarm(): Promise<boolean> {
  unlockSosAudio();
  if (typeof window === "undefined" || !("Notification" in window)) return false;
  if (Notification.permission === "granted") return true;
  if (Notification.permission === "denied") return false;
  try {
    return (await Notification.requestPermission()) === "granted";
  } catch {
    return false;
  }
}

function notifyBrowser(title: string, body: string) {
  if (typeof window === "undefined" || !("Notification" in window) || Notification.permission !== "granted") return;
  try {
    const notification = new Notification(title, { body, tag: "safrat-sos", requireInteraction: true });
    notification.onclick = () => window.focus();
  } catch {
    // Some browsers throw when a service worker is required for tagged notifications.
  }
}

/** Beeps + shows a browser Notification the moment a new ACTIVE/ESCALATED alert shows up in `alerts` that this hook hasn't seen before. */
export function useSosAlarm(alerts: { id: string; status: string }[], title = "SOS Aktif") {
  const seen = useRef<Set<string>>(new Set());

  useEffect(() => {
    const urgent = alerts.filter((a) => a.status === "ACTIVE" || a.status === "ESCALATED");
    const fresh = urgent.filter((a) => !seen.current.has(a.id));
    for (const alert of urgent) seen.current.add(alert.id);
    if (fresh.length === 0) return;
    playSosAlarm();
    notifyBrowser(title, fresh.length === 1 ? "Seorang jamaah membutuhkan bantuan segera." : `${fresh.length} jamaah membutuhkan bantuan segera.`);
  }, [alerts, title]);
}
