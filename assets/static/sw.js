const CACHE = 'homeport-v2';
const STATIC = [
  '/static/style.css',
  '/static/htmx.min.js',
  '/static/sse.js',
  '/static/icon-192.png',
  '/static/icon-512.png',
  '/static/css/prism-tokens.css',
  '/static/css/prism-base.css',
  '/static/css/prism-aurora.css',
  '/static/css/prism-components.css',
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

  // Cache-First for static assets
  if (url.pathname.startsWith('/static/')) {
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
