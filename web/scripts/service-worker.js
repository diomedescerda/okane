/* Build replaces the tokens below. Only public app-shell files are cached. */
const CACHE = "okane-shell-__CACHE_VERSION__";
const SHELL = __PRECACHE_URLS__;

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(SHELL)));
  // Updates wait for existing tabs to close so unsaved forms stay intact.
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const names = await caches.keys();
      await Promise.all(
        names
          .filter((name) => name.startsWith("okane-shell-") && name !== CACHE)
          .map((name) => caches.delete(name)),
      );
      await self.clients.claim();
    })(),
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);
  if (
    request.method !== "GET" ||
    url.origin !== self.location.origin ||
    url.pathname.startsWith("/api/")
  )
    return;

  if (request.mode === "navigate") {
    event.respondWith(
      fetch(request).catch(async () => {
        const cached = await caches
          .open(CACHE)
          .then((cache) => cache.match("/"));
        return cached || Response.error();
      }),
    );
    return;
  }

  if (SHELL.includes(url.pathname)) {
    event.respondWith(
      caches
        .open(CACHE)
        .then(
          async (cache) => (await cache.match(url.pathname)) || fetch(request),
        ),
    );
  }
});
