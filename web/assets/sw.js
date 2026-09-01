// Service worker: caches the app shell only, never /api responses (KTD5).
// The app installs and launches offline, but logging still requires the network
// and fails visibly (AE3) rather than appearing to succeed.
const CACHE = "em-shell-v1";
const SHELL = [
  "/",
  "/review",
  "/assets/style.css",
  "/assets/app.js",
  "/assets/review.js",
  "/assets/icon.svg",
];

self.addEventListener("install", (e) => {
  e.waitUntil(
    caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (url.pathname.startsWith("/api/")) return; // never cache the API (KTD5)
  if (e.request.method !== "GET") return;
  e.respondWith(caches.match(e.request).then((cached) => cached || fetch(e.request)));
});
