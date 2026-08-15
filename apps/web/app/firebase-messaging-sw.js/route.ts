// Served at /firebase-messaging-sw.js — a service worker must be a real file
// at that URL, but it also needs the real Firebase config, which only exists
// as env vars. A static file in public/ can't read process.env at request
// time, so this route handler generates the script server-side instead.
export async function GET() {
  const config = {
    apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY ?? "",
    projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID ?? "",
    messagingSenderId: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID ?? "",
    appId: process.env.NEXT_PUBLIC_FIREBASE_APP_ID ?? "",
  };

  const script = `
importScripts("https://www.gstatic.com/firebasejs/10.14.1/firebase-app-compat.js");
importScripts("https://www.gstatic.com/firebasejs/10.14.1/firebase-messaging-compat.js");

firebase.initializeApp(${JSON.stringify(config)});

const messaging = firebase.messaging();
messaging.onBackgroundMessage((payload) => {
  const title = payload.notification?.title ?? "Safrat";
  const body = payload.notification?.body ?? "";
  self.registration.showNotification(title, { body, icon: "/icons/icon-192.png" });
});
`;

  return new Response(script, { headers: { "Content-Type": "application/javascript", "Service-Worker-Allowed": "/" } });
}
