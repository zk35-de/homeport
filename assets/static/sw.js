// Minimaler Service Worker: nur Offline-Fallback
const CACHE = 'homeport-v1';
self.addEventListener('install', e => e.waitUntil(
  caches.open(CACHE).then(c => c.addAll(['/static/style.css']))
));
self.addEventListener('fetch', e => e.respondWith(
  fetch(e.request).catch(() => caches.match(e.request))
));
