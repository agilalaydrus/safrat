// Development fallback served at /firebase-messaging-sw.js. Production bundles
// Firebase Messaging into the Serwist app-shell worker so only one worker owns
// the root scope. Keeping this route lets push registration work under Turbopack,
// where Serwist generation is intentionally disabled.
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

firebase.messaging();
`;

  return new Response(script, { headers: { "Content-Type": "application/javascript", "Service-Worker-Allowed": "/" } });
}
