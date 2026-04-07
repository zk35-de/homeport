const CACHE = 'homeport-v3';
// Only cache static assets that don't change between releases (images, icons).
// JS and CSS are excluded so new versions are always fetched fresh.
const STATIC = [
  '/static/icon-192.png',
  '/static/icon-512.png',
];

// Install: pre-cache static assets
self.addEventListener('install', e => {
  e.waitUntil(
    caches.open(CACHE).then(c => c.addAll(STATIC)).then(() => self.skipWaiting())
  );
});

// Activate: remove old caches
self.addEventListener('activate', e => {
  e.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', e => {
  const url = new URL(e.request.url);

  // Cache-First only for images/icons that never change between releases.
  // JS and CSS use network-first so updates are always picked up.
  if (url.pathname.startsWith('/static/icon-') || url.pathname.startsWith('/static/logo')) {
    e.respondWith(
      caches.match(e.request).then(cached => cached || fetch(e.request).then(res => {
        const clone = res.clone();
        caches.open(CACHE).then(c => c.put(e.request, clone));
        return res;
      }))
    );
    return;
  }

  // Network-First for HTML pages
  if (e.request.mode === 'navigate') {
    e.respondWith(
      fetch(e.request).catch(() =>
        caches.match(e.request) ||
        new Response('<html><body><h1>Offline</h1><p><a href="/">Zurück</a></p></body></html>', {
          headers: { 'Content-Type': 'text/html' }
        })
      )
    );
    return;
  }

  // Default: network with cache fallback
  e.respondWith(fetch(e.request).catch(() => caches.match(e.request)));
});
