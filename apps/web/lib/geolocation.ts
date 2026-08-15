"use client";

import { useEffect } from "react";
import { pilgrimAppClient } from "@/lib/rpc";

const PING_INTERVAL_MS = 5 * 60 * 1000;

function getPosition(timeoutMs: number): Promise<GeolocationPosition | null> {
  if (typeof window === "undefined" || !("geolocation" in navigator)) return Promise.resolve(null);
  return new Promise((resolve) => {
    navigator.geolocation.getCurrentPosition(
      (position) => resolve(position),
      () => resolve(null),
      { enableHighAccuracy: true, timeout: timeoutMs, maximumAge: 60_000 },
    );
  });
}

/** Pings the pilgrim's GPS position to the backend every 5 minutes, so an
 * SOS alert always has a recent location to fall back on even if the device
 * can't get a fresh fix at the exact moment SOS is pressed. Silently does
 * nothing if geolocation is unsupported or permission is denied — SOS itself
 * still works without it. */
export function useLocationPing(appAccessCode: string | undefined) {
  useEffect(() => {
    if (!appAccessCode) return;
    let cancelled = false;

    async function ping() {
      const position = await getPosition(8000);
      if (!position || cancelled) return;
      pilgrimAppClient
        .updateMyLocation({ appAccessCode, lat: position.coords.latitude, lng: position.coords.longitude })
        .catch(() => {
          // Best-effort — a missed ping just means the next one (or the SOS
          // page's own fix attempt) carries the location instead.
        });
    }

    void ping();
    const interval = window.setInterval(ping, PING_INTERVAL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [appAccessCode]);
}

/** One-shot, short-timeout GPS fix for the SOS button itself — fresher than
 * the periodic ping if it succeeds in time, but SOS must never wait long for it. */
export async function getFreshLocation(): Promise<{ lat: number; lng: number } | null> {
  const position = await getPosition(4000);
  return position ? { lat: position.coords.latitude, lng: position.coords.longitude } : null;
}
