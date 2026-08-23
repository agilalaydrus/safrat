"use client";

import { useEffect } from "react";

/**
 * Returns the one root-scope service worker used for both offline caching and
 * Firebase Messaging. Development keeps the generated Firebase-only fallback
 * because Serwist is intentionally disabled under Turbopack.
 */
export async function registerTawafiqServiceWorker(): Promise<ServiceWorkerRegistration | undefined> {
  if (!("serviceWorker" in navigator)) return undefined;

  const scriptUrl = process.env.NODE_ENV === "production" ? "/sw.js" : "/firebase-messaging-sw.js";
  return navigator.serviceWorker.register(scriptUrl);
}

/** Registers the production app-shell worker once from each PWA root layout. */
export function useRegisterShellServiceWorker() {
  useEffect(() => {
    if (process.env.NODE_ENV !== "production") return;
    void registerTawafiqServiceWorker().catch(() => {});
  }, []);
}
