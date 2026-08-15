"use client";

import { initializeApp, getApps } from "firebase/app";
import { getMessaging, getToken, isSupported } from "firebase/messaging";

const firebaseConfig = {
  apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
  projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
  messagingSenderId: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID,
  appId: process.env.NEXT_PUBLIC_FIREBASE_APP_ID,
};

function firebaseConfigured() {
  return Boolean(firebaseConfig.apiKey && firebaseConfig.projectId && firebaseConfig.messagingSenderId && firebaseConfig.appId);
}

/** Requests notification permission and returns an FCM token, or null if Firebase isn't configured, unsupported, or permission is denied. Never throws. */
export async function requestPushToken(): Promise<string | null> {
  if (!firebaseConfigured() || typeof window === "undefined") return null;
  try {
    if (!(await isSupported())) return null;
    const permission = await Notification.requestPermission();
    if (permission !== "granted") return null;
    const app = getApps()[0] ?? initializeApp(firebaseConfig);
    const registration = await navigator.serviceWorker.register("/firebase-messaging-sw.js");
    const token = await getToken(getMessaging(app), { vapidKey: process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY, serviceWorkerRegistration: registration });
    return token || null;
  } catch {
    return null;
  }
}
