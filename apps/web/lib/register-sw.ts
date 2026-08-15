"use client";

import { useEffect } from "react";

/** Registers the app-shell cache service worker (public/sw.js). Call once from a PWA's root layout. */
export function useRegisterShellServiceWorker() {
  useEffect(() => {
    if ("serviceWorker" in navigator) {
      navigator.serviceWorker.register("/sw.js").catch(() => {});
    }
  }, []);
}
