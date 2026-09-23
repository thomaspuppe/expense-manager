// Service worker: caches the app shell only, never /api responses (KTD5).
// The app installs and launches offline, but logging still requires the network
// and fails visibly (AE3) rather than appearing to succeed.
//
// The shell is served network-first, not cache-first: a deploy is only ever a
// new binary at the same URLs, so a cache-first shell would pin an installed
// PWA to the HTML/JS it first saw and no fix would ever reach the phone. The
// cache is the offline fallback, not the source of truth.
const CACHE = "em-shell-v4";
const SHELL = [
  "/",
  "/review",
  "/assets/style.css",
  "/assets/app.js",
  "/assets/review.js",
  "/assets/icon.svg",
  "/assets/apple-touch-icon.png",
  "/assets/favicon-32.png",
];

// Only a 200 served straight from its own URL belongs in the shell cache. A
// redirect means the session is gone and we followed it to /login; caching that
// under "/" would strand the app on the login page.
function cacheable(res) {
  return res && res.ok && !res.redirected && res.type !== "opaque";
}

async function precache() {
  const cache = await caches.open(CACHE);
  // Individually, so one unreachable or session-gated entry cannot fail the
  // whole install the way cache.addAll would.
  await Promise.all(
    SHELL.map(async (url) => {
      try {
        const res = await fetch(url, { credentials: "same-origin" });
        if (cacheable(res)) await cache.put(url, res);
      } catch (e) {
        /* offline at install time: the fetch handler fills the cache later */
      }
    })
  );
}

self.addEventListener("install", (e) => {
  e.waitUntil(precache().then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

async function networkFirst(request) {
  try {
    const res = await fetch(request);
    if (cacheable(res)) {
      const cache = await caches.open(CACHE);
      cache.put(request, res.clone());
    }
    return res;
  } catch (e) {
    const cached = await caches.match(request);
    if (cached) return cached;
    throw e;
  }
}

self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== "GET") return;
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api/")) return; // never cache the API (KTD5)

  const isShell = e.request.mode === "navigate" || url.pathname.startsWith("/assets/");
  if (isShell) e.respondWith(networkFirst(e.request));
});
