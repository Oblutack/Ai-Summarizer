// A deliberately small service worker. It makes Inkling installable and shows a friendly page when
// the network is gone. It never stores summaries, documents or API answers: everything else goes
// straight to the network, so nothing stale or private can be served from a cache.
const OFFLINE_CACHE = "inkling-offline-v1";
const OFFLINE_URL = "/offline.html";

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(OFFLINE_CACHE).then((cache) => cache.add(OFFLINE_URL)));
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      // Drop the offline pages of older versions of this worker.
      for (const name of await caches.keys()) {
        if (name !== OFFLINE_CACHE) await caches.delete(name);
      }
      await self.clients.claim();
    })()
  );
});

self.addEventListener("fetch", (event) => {
  // Only whole-page visits are answered offline; scripts, images and API calls just fail, as they would without a worker.
  if (event.request.mode !== "navigate") return;
  event.respondWith(
    fetch(event.request).catch(async () => (await caches.match(OFFLINE_URL)) ?? Response.error())
  );
});
