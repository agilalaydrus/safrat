// App-shell cache for the Pilgrim (/pilgrim/*) and Leader (/leader/*) PWAs.
// This is deliberately simple runtime caching, not a full precache manifest —
// see the offline decision in CLAUDE.md. It makes an already-visited page
// (and its static assets) load again while offline; it does not guarantee
// every route works offline on a completely cold start.
const CACHE_NAME = "safrat-shell-v1";
const CACHED_PREFIXES = ["/pilgrim", "/leader", "/_next/static"];

function shouldCache(url) {
  return CACHED_PREFIXES.some((prefix) => url.pathname.startsWith(prefix));
}

self.addEventListener("install", (event) => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((names) => Promise.all(names.filter((name) => name !== CACHE_NAME).map((name) => caches.delete(name))))
  );
  self.clients.claim();
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || url.origin !== self.location.origin || !shouldCache(url)) return;

  event.respondWith(
    fetch(event.request)
      .then((response) => {
        const copy = response.clone();
        caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
        return response;
      })
      .catch(() => caches.match(event.request))
  );
});
