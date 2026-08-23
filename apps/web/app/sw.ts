import { defaultCache } from "@serwist/next/worker";
import { getApps, initializeApp } from "firebase/app";
import { getMessaging, isSupported, onBackgroundMessage } from "firebase/messaging/sw";
import type { PrecacheEntry, SerwistGlobalConfig } from "serwist";
import { Serwist } from "serwist";

declare global {
  interface WorkerGlobalScope extends SerwistGlobalConfig {
    __SW_MANIFEST: (PrecacheEntry | string)[] | undefined;
  }
}

declare const self: ServiceWorkerGlobalScope;

const serwist = new Serwist({
  precacheEntries: self.__SW_MANIFEST,
  precacheOptions: { cleanupOutdatedCaches: true },
  runtimeCaching: defaultCache,
  navigationPreload: true,
  skipWaiting: true,
  clientsClaim: true,
});

serwist.addEventListeners();

const firebaseConfig = {
  apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  messagingSenderId: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  appId: process.env.NEXT_PUBLIC_FIREBASE_APP_ID,
};

const firebaseConfigured = Object.values(firebaseConfig).every(Boolean);

if (firebaseConfigured) {
  void isSupported()
    .then((supported) => {
      if (!supported) return;

      const app = getApps()[0] ?? initializeApp(firebaseConfig);
      const messaging = getMessaging(app);

      // Firebase displays notification payloads itself. Only data-only messages
      // need an explicit notification, otherwise browsers would show duplicates.
      onBackgroundMessage(messaging, (payload) => {
        if (payload.notification) return;

        const title = payload.data?.title ?? "Tawafiq Hub";
        const body = payload.data?.body ?? "";
        void self.registration.showNotification(title, { body });
      });
    })
    .catch(() => {
      // Push is optional; an unsupported browser must not break offline caching.
    });
}
